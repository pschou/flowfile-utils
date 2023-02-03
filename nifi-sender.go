package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pschou/go-flowfile"
)

var about = `NiFi Sender

This utility is intended to capture a set of files or directory of files and
send them to a remote NiFi server for processing.`

var (
	url     = flag.String("url", "http://localhost:8080/contentListener", "Where to send the files from staging")
	retries = flag.Int("retries", 3, "Retries after failing to send a file")
)

var hs *flowfile.HTTPSender

func main() {
	usage = "[options] path1 path2..."
	flag.Parse()
	if strings.HasPrefix(*url, "https") {
		loadTLS()
	}

	// Connect to the NiFi server and establish a session
	log.Println("creating sender...")
	var err error
	hs, err = flowfile.NewHTTPSender(*url, http.DefaultClient)
	if err != nil {
		log.Fatal(err)
	}

	// Loop over the files sending them one at a time
	for _, arg := range flag.Args() {
		fileInfo, err := os.Stat(arg)
		if err != nil {
			log.Fatal(err)
		} else if fileInfo.IsDir() {
			// Walk the directory and send files
			filepath.Walk(arg, func(filename string, fileInfo os.FileInfo, inerr error) (err error) {
				if inerr != nil {
					return inerr
				}
				if fileInfo.IsDir() {
					if isEmptyDir(filename) {
						log.Println("  sending empty dir", filename, "...")
						f := flowfile.New(strings.NewReader(""), 0)
						dn, fn := path.Split(filename)
						if dn == "" {
							dn = "./"
						}
						f.Attrs.Set("path", dn)
						f.Attrs.Set("filename", fn)
						f.Attrs.Set("kind", "dir")
						f.Attrs.Set("modtime", fileInfo.ModTime().Format(time.RFC3339))
						if *verbose {
							adat, _ := json.Marshal(f.Attrs)
							fmt.Printf("  [dir] %s\n", adat)
						}
						sendWithRetries(f)
					}
					return
				}
				return sendFile(filename, fileInfo)
			})
		} else {
			// Send a single file
			if err = sendFile(arg, fileInfo); err != nil {
				log.Fatal(err)
			}
		}
	}
	log.Println("done.")
}

func isEmptyDir(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	defer f.Close()
	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	return err == io.EOF
}

func sendFile(filename string, fileInfo os.FileInfo) (err error) {
	dn, fn := path.Split(filename)
	if dn == "" {
		dn = "./"
	}
	log.Println("  sending", filename, "...")
	var fh *os.File
	fh, err = os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	f := flowfile.New(fh, fileInfo.Size())
	f.Attrs.Set("path", dn)
	f.Attrs.Set("filename", fn)
	f.Attrs.Set("modtime", fileInfo.ModTime().Format(time.RFC3339))
	f.AddChecksum("SHA256")
	f.Attrs.GenerateUUID()

	//fmt.Printf("hs = %#v\n", hs)

	segments, err := flowfile.SegmentBySize(f, int64(hs.MaxPartitionSize))
	if err != nil {
		log.Fatal(err)
	}
	for i, ff := range segments {
		if *verbose {
			adat, _ := json.Marshal(ff.Attrs)
			fmt.Printf("  % 3d) %s\n", i, adat)
		}
		sendWithRetries(ff)
	}
	return
}

func sendWithRetries(ff *flowfile.File) (err error) {
	err = hs.Send(ff, nil)

	// Try a few more times before we give up
	for i := 1; err != nil && i < *retries; i++ {
		time.Sleep(10 * time.Second)
		err = hs.Handshake()
		if err != nil {
			err = hs.Send(ff, nil)
		}
	}
	if err != nil {
		log.Fatal(err)
	}

	return
}
