package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
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
	for _, filename := range flag.Args() {
		dn, fn := path.Split(filename)
		if dn == "" {
			dn = "./"
		}
		log.Println("  sending", filename, "...")
		fh, err := os.Open(filename)
		if err != nil {
			log.Panic(err)
		}
		fileInfo, _ := fh.Stat()
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
				fmt.Printf("%d) %#v\n", i, ff.Attrs)
				err = hs.Send(ff)
				if err != nil {
					log.Panic(err)
				}
			}
		} else {
			err = hs.Send(f)
			if err != nil {
				log.Panic(err)
			}
		}
	}
	log.Println("done.")
}
