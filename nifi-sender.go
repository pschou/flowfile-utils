package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
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
	url     = flag.String("url", "http://localhost:8080/contentListener", "Where to send the files")
	retries = flag.Int("retries", 5, "Retries after failing to send a file")
)

var hs *flowfile.HTTPTransaction
var wd, _ = os.Getwd()

func main() {
	usage = "[options] path1 path2..."
	flag.Parse()
	if strings.HasPrefix(*url, "https") {
		loadTLS()
	}

	// Connect to the NiFi server and establish a session
	log.Println("creating sender...")
	var err error
	hs, err = flowfile.NewHTTPTransaction(*url, http.DefaultClient)
	if err != nil {
		log.Fatal(err)
	}

	//var entries []entry
	// Loop over the files sending them one at a time
	for _, arg := range flag.Args() {
		/*	var fileInfo os.FileInfo
			if fileInfo, err = os.Lstat(arg); err != nil {
				log.Fatal(err)
			}
			if err != nil {
				log.Fatal(err)
			} else if fileInfo.IsDir() {*/
		// Walk the directory and send files

		filepath.Walk(arg, func(filename string, fileInfo os.FileInfo, inerr error) (err error) {
			if inerr != nil {
				log.Fatal(inerr)
			}
			mode := fileInfo.Mode()
			if mode.IsRegular() {
				if *verbose {
					fmt.Println("regular file")
				}
				if err = sendFile(filename, fileInfo); err != nil {
					log.Fatal(err)
				}
				return
			}

			f := flowfile.New(strings.NewReader(""), 0)
			dn, fn := path.Split(filename)
			if dn == "" {
				dn = "./"
			}
			f.Attrs.Set("path", dn)
			f.Attrs.Set("filename", fn)
			f.Attrs.Set("modtime", fileInfo.ModTime().Format(time.RFC3339))
			f.Attrs.GenerateUUID()

			switch mode := fileInfo.Mode(); {
			case mode.IsDir():
				if *verbose {
					fmt.Println("directory")
				}
				if !isEmptyDir(filename) {
					return
				}
				f.Attrs.Set("kind", "dir")
				log.Println("  sending empty dir", filename, "...")
				/*if *verbose {
					adat, _ := json.Marshal(f.Attrs)
					fmt.Printf("  [dir] %s\n", adat)
				}*/
			case mode&fs.ModeSymlink != 0:
				cur := dn
				if !strings.HasPrefix(cur, "/") {
					cur = path.Join(wd, cur)
				}
				target, _ := os.Readlink(filename)
				if strings.HasPrefix(target, "/") {
					if rel, err := filepath.Rel(cur, target); err == nil &&
						!strings.HasPrefix(rel, "..") {
						target = rel
					} else {
						log.Fatal("Symbolic link", target, "out of scope", err, rel)
					}
				}
				log.Println("  sending symbolic link", filename, "->", target)
				f.Attrs.Set("kind", "link")
				f.Attrs.Set("target", target)
				if *verbose {
					adat, _ := json.Marshal(f.Attrs)
					fmt.Printf("  [link] %s\n", adat)
				}
			//case mode&fs.ModeNamedPipe != 0:
			//	fmt.Println("named pipe")
			default:
				return
			}
			go sendWithRetries(f)
			return
		})
	}
	hs.Close()
	log.Println("done.")
}

type entry struct {
	filename     string
	fileInfo     os.FileInfo
	lameChecksum [16]byte
	size         int
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
	defer fh.Close()
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
		ff.AddChecksum("SHA256")
		if *verbose {
			adat, _ := json.Marshal(ff.Attrs)
			fmt.Printf("  % 3d) %s\n", i, adat)
		}
		sendWithRetries(ff)
		ff.Close()
	}
	return
}

func sendWithRetries(ff *flowfile.File) (err error) {
	err = hs.Send(ff, nil)

	// Try a few more times before we give up
	for i := 1; err != nil && i < *retries; i++ {
		log.Println(i, "Error sending:", err)
		if err = ff.Reset(); err != nil {
			return
		}
		time.Sleep(10 * time.Second)
		if err = hs.Handshake(); err == nil {
			err = hs.Send(ff, nil)
		}
	}
	if err != nil {
		log.Fatal(err)
	}

	return
}
