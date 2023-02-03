package main

import (
	"encoding/json"
	"flag"
	"fmt"
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
	basePath = flag.String("path", "stager", "Directory which to scan for FlowFiles")
	url      = flag.String("url", "http://localhost:8080/contentListener", "Where to send the files from staging")
	retries  = flag.Int("retries", 3, "Retries after failing to send a file")
)

func main() {
	flag.Parse()
	if strings.HasPrefix(*url, "https") {
		loadTLS()
	}

	// Connect to the NiFi server and establish a session
	log.Println("creating sender...")
	hs, err := flowfile.NewHTTPSender(*url, http.DefaultClient)
	if err != nil {
		log.Panic(err)
	}

	// Loop over the files sending them one at a time
	for _, arg := range flag.Args() {
		filepath.Walk(arg, func(filename string, fileInfo os.FileInfo, inerr error) (err error) {
			if inerr != nil {
				return inerr
			}
			if fileInfo.IsDir() {
				return nil
			}
			dn, fn := path.Split(filename)
			if dn == "" {
				dn = "./"
			}
			log.Println("  sending", filename, "...")
			var fh *os.File
			fh, err = os.Open(filename)
			if err != nil {
				log.Panic(err)
			}
			f := flowfile.New(fh, fileInfo.Size())
			f.Attrs.Set("path", dn)
			f.Attrs.Set("filename", fn)
			f.Attrs.Set("modtime", fileInfo.ModTime().Format(time.RFC3339))
			f.AddChecksum("SHA256")

			if hs.MaxPartitionSize > 0 {
				segments, err := flowfile.SegmentBySize(f, int64(hs.MaxPartitionSize))
				if err != nil {
					log.Panic(err)
				}
				for i, ff := range segments {
					adat, _ := json.Marshal(ff.Attrs)
					fmt.Printf("  % 3d) %s\n", i, adat)

					err = hs.Send(ff, nil)

					// Try a few more times before we give up
					for i := 0; err != nil && i < *retries; i++ {
						time.Sleep(10 * time.Second)
						err = hs.Handshake()
						if err != nil {
							err = hs.Send(ff, nil)
						}
					}
					if err != nil {
						log.Panic(err)
					}
				}
			} else {
				err = hs.Send(f, nil)
				if err != nil {
					log.Panic(err)
				}
			}
			return
		})
	}
	log.Println("done.")
}
