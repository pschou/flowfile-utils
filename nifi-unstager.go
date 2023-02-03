package main

import (
	"encoding/binary"
	"flag"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pschou/go-flowfile"
)

var about = `NiFi Unstager

This utility is intended to take a directory of NiFi flow files and ship them
out to a listening NiFi endpoint while maintaining the same set of attribute
headers.`

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

	log.Println("Creating FlowFile sender to url", *url)
	hs, err := flowfile.NewHTTPSender(*url, http.DefaultClient)
	if err != nil {
		log.Panic(err)
	}

	log.Println("Creating directory listener on", *basePath)
	for ; true; time.Sleep(3 * time.Second) {
		dirEntries, err := os.ReadDir(*basePath)
		if err != nil {
			log.Println("Error listing files:", err)
			continue
		}

		for _, entry := range dirEntries {
			// Loop over the files in the directory looking for .attrs files
			filenamea := path.Join(*basePath, entry.Name())
			if !strings.HasSuffix(filenamea, ".attrs") {
				continue
			}

			func() { // Break out the thread
				var fh, fha *os.File
				defer func() {
					if fh != nil {
						fh.Close()
					}
					if fha != nil {
						fha.Close()
					}
				}()

				// Open the file and read the metadata
				if fha, err = os.Open(filenamea); err != nil {
					log.Print("Error opening attribute file:", err)
					return
				}

				// Read in the FlowFile attributes
				var a flowfile.Attributes
				var size uint64
				if err = a.Unmarshall(fha); err != nil {
					log.Print("Error parsing attribute file:", err)
					return
				}
				if err = binary.Read(fha, binary.BigEndian, &size); err != nil {
					log.Print("Error parsing attribute file for size:", err)
					return
				}
				fha.Close()
				fha = nil

				filename := strings.TrimSuffix(filenamea, ".attrs") + "-" + a.Get("filename")
				fileInfo, err := os.Stat(filename)
				if err != nil {
					log.Println("could not stat file:", filename)
					return
				}

				//fmt.Println("comparing sizes", int64(size), fileInfo.Size())
				if int64(size) == fileInfo.Size() {
					log.Println("  sending", a.Get("filename"), "...")
					fh, err := os.Open(filename)
					if err != nil {
						log.Panic(err)
					}
					fileInfo, _ := fh.Stat()
					f := flowfile.New(fh, fileInfo.Size())
					f.Attrs = a

					err = hs.Send(f, nil)

					// Try a few more times before we give up
					for i := 0; err != nil && i < *retries; i++ {
						time.Sleep(10 * time.Second)
						err = hs.Handshake()
						if err != nil {
							err = hs.Send(f, nil)
						}
					}

					if err != nil {
						log.Println("Error sending file", err)
						return
					}

					// Success!  Remove all the artifacts (clean things up)
					fh.Close()
					fh = nil
					os.Remove(filename)
					os.Remove(filenamea)
				}
			}()
		}
	}
	log.Println("done.")
}
