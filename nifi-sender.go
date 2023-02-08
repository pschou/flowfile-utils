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

	"github.com/djherbis/times"
	"github.com/pschou/go-flowfile"
)

var about = `NiFi Sender

This utility is intended to capture a set of files or directory of files and
send them to a remote NiFi server for processing.`

var (
	url     = flag.String("url", "http://localhost:8080/contentListener", "Where to send the files")
	retries = flag.Int("retries", 5, "Retries after failing to send a file")
	debug   = flag.Bool("debug", false, "Turn on debug")
)

var hs *flowfile.HTTPTransaction
var wd, _ = os.Getwd()

func main() {
	usage = "[options] path1 path2..."
	flag.Parse()
	if *debug {
		flowfile.Debug = true
	}
	if strings.HasPrefix(*url, "https") {
		loadTLS()
	}

	// Connect to the NiFi server and establish a session
	log.Println("Creating list of files")
	var err error
	hs, err = flowfile.NewHTTPTransaction(*url, http.DefaultClient)
	if err != nil {
		log.Fatal(err)
	}

	var content []WorkUnit
	var zeros []*flowfile.File

	// Loop over the files sending them one at a time
	for _, arg := range flag.Args() {

		// Walk the directory args and send files
		filepath.Walk(arg, func(filename string, fileInfo os.FileInfo, inerr error) (err error) {
			if inerr != nil {
				log.Fatal(inerr)
			}
			//mode := fileInfo.Mode()
			//f := flowfile.New(strings.NewReader(""), 0)
			var Attrs flowfile.Attributes
			Size := int64(0)
			dn, fn := path.Split(filename)
			if dn == "" {
				dn = "./"
			}
			Attrs.Set("path", dn)
			Attrs.Set("filename", fn)
			Attrs.Set("file.lastModifiedTime", fileInfo.ModTime().Format(time.RFC3339))
			Attrs.GenerateUUID()

			switch mode := fileInfo.Mode(); {
			case mode.IsRegular():
				if *verbose {
					log.Println("  file", filename)
				}
				Size = fileInfo.Size()
				//if err = sendFile(filename, fileInfo); err != nil {
				//	log.Fatal(err)
				//}
			case mode.IsDir():
				if !isEmptyDir(filename) {
					return
				}
				Attrs.Set("kind", "dir")
				if *verbose {
					log.Println("  empty dir", filename, "...")
				}
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
				if *verbose {
					log.Println("  symbolic link", filename, "->", target)
				}
				Attrs.Set("kind", "link")
				Attrs.Set("target", target)
			//case mode&fs.ModeNamedPipe != 0:
			//	fmt.Println("named pipe")
			default:
				if *verbose {
					log.Println("  skipping ", filename)
				}
				return
			}
			if Size == 0 {
				ff := flowfile.New(strings.NewReader(""), 0)
				ff.Attrs = Attrs
				zeros = append(zeros, ff)
			} else {
				content = append(content, WorkUnit{filename: filename, fileInfo: fileInfo, Attrs: Attrs, Size: Size})
			}
			return
		})
	}

	// Send off all the empty files and folders first
	log.Println("Sending...")
	{
		sender := func() error {
			pw := hs.NewHTTPBufferedPostWriter()
			for i, f := range zeros {
				if *verbose {
					adat, _ := json.Marshal(f.Attrs)
					fmt.Printf("  %d) %s\n", i, adat)
				}
				if _, err = pw.Write(f); err != nil {
					log.Println(err)
					return err
				}
			}
			pw.Close()
			return nil
		}

		err = sender()
		// Try a few more times before we give up
		for i := 1; err != nil && i < *retries; i++ {
			log.Println(i, "Error sending:", err)
			time.Sleep(10 * time.Second)
			if err = hs.Handshake(); err == nil {
				err = sender()
			}
		}
		if err != nil {
			log.Fatal(err)
		}
	}

	for _, u := range content {
		sendFile(u.filename, u.fileInfo)
	}

	hs.Close()

	//log.Println("zeros:", zeros, "content:", content)
	log.Println("done.")
}

type WorkUnit struct {
	filename string
	fileInfo os.FileInfo
	Attrs    flowfile.Attributes
	Size     int64
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
	f.Attrs.Set("file.lastModifiedTime", fileInfo.ModTime().Format(time.RFC3339))
	f.Attrs.Set("file.creationTime", fileInfo.ModTime().Format(time.RFC3339))
	if ts, err := times.Stat(filename); err == nil && ts.HasBirthTime() {
		f.Attrs.Set("file.creationTime", ts.BirthTime().Format(time.RFC3339))
	}
	f.AddChecksum("SHA256")
	f.Attrs.GenerateUUID()

	//fmt.Printf("hs = %#v\n", hs)

	segments, err := flowfile.SegmentBySize(f, int64(hs.MaxPartitionSize))
	if err != nil {
		log.Fatal(err)
	}
	for _, ff := range segments {
		ff.AddChecksum("SHA256")
		if *verbose {
			adat, _ := json.Marshal(ff.Attrs)
			fmt.Printf("  - %s %d\n", adat, ff.Size)
		}
		sendWithRetries(ff)
		ff.Close()
	}
	return
}

func sendWithRetries(ff *flowfile.File) (err error) {
	err = hs.Send(ff)

	// Try a few more times before we give up
	for i := 1; err != nil && i < *retries; i++ {
		log.Println(i, "Error sending:", err)
		if err = ff.Reset(); err != nil {
			return
		}
		time.Sleep(10 * time.Second)
		if err = hs.Handshake(); err == nil {
			err = hs.Send(ff)
		}
	}
	if err != nil {
		log.Fatal(err)
	}

	return
}
