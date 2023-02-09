package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
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
)

var hs *flowfile.HTTPTransaction

func main() {
	service_flags()
	origin_flags()
	sender_flags()
	parse()

	log.Println("Creating FlowFile sender to url", *url)

	var err error
	hs, err = flowfile.NewHTTPTransaction(*url, tlsConfig)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Creating directory listener on", *basePath)

	// infinite loop for scanning the directory
	for ; true; time.Sleep(3 * time.Second) {
		dirEntries, err := os.ReadDir(*basePath)
		if err != nil {
			log.Println("Error listing files:", err)
			continue
		}

		for _, entry := range dirEntries {
			// Loop over the files in the directory looking for .json files
			jsonFile := path.Join(*basePath, entry.Name())
			if !strings.HasSuffix(jsonFile, ".json") {
				continue
			}
			datFile := strings.TrimSuffix(jsonFile, ".json") + ".dat"

			processFile := func() (err error) { // Break out the thread
				var f *flowfile.File
				var fh *os.File

				// Open the file
				if fh, err = os.Open(datFile); err != nil {
					err = fmt.Errorf("Error opening staged file:", err)
					return
				}

				hw := hs.NewHTTPBufferedPostWriter()
				defer func() {
					fh.Close()
					if *verbose && err != nil {
						log.Println("err:", err)
					}
					if err == nil {
						// Success!  Remove all the artifacts (clean things up)
						os.Remove(jsonFile)
						os.Remove(datFile)
					}
				}()

				// Read in the FlowFile
				s := flowfile.NewScanner(fh)
				for s.Scan() {
					if f, err = s.File(); err != nil {
						return
					}

					// Make sure the client chain is added to attributes, 1 being the closest
					updateChain(f, nil, "FROM-DISK")

					// Quick sanity check that paths are not in a bad state
					dir := filepath.Clean(f.Attrs.Get("path"))
					filename := f.Attrs.Get("filename")
					if strings.HasPrefix(dir, "..") {
						err = fmt.Errorf("Invalid path %q", dir)
						return
					}

					if id := f.Attrs.Get("segment-index"); id != "" {
						i, _ := strconv.Atoi(id)
						fmt.Printf("  Unstaging segment %d of %s of %s\n", i+1,
							f.Attrs.Get("segment-count"), path.Join(dir, filename))
					} else {
						fmt.Printf("  Unstaging file %s\n", path.Join(dir, filename))
					}

					if *verbose {
						adat, _ := json.Marshal(f.Attrs)
						fmt.Printf("    %s\n", adat)
					}

					if _, err = hw.Write(f); err != nil {
						return
					}
				}
				if hwerr := hw.Close(); err == nil {
					err = hwerr
				}
				return
			}

			err = processFile()
			fmt.Println("processFile", err)

			// Try a few more times before we give up
			for i := 1; err != nil && i < *retries; i++ {
				log.Println(i, "Error sending:", err)
				time.Sleep(*retryTimeout)
				if hserr := hs.Handshake(); hserr == nil {
					err = processFile()
				}
			}

			if err != nil {
				log.Fatal(err)
			}
		}
	}
	log.Println("done.")
}
