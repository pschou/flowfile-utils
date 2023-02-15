package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/pschou/go-flowfile"
)

var (
	about = `NiFi Flood

This utility is intended to saturate the bandwidth of a NiFi endpoint for
load testing.`

	payload       = flag.Int("payload", 20<<20, "Payload size for upload")
	hs            *flowfile.HTTPTransaction
	wd, _         = os.Getwd()
	seenChecksums = make(map[string]string)
)

func main() {
	sender_flags()
	parse()

	if len(flag.Args()) != 0 {
		flag.Usage()
		return
	}

	var err error

	// Connect to the NiFi server and establish a session
	hs, err = flowfile.NewHTTPTransaction(*url, tlsConfig)
	if err != nil {
		log.Fatal(err)
	}

	// Send off all the empty files and folders first
	log.Println("Sending...")

	// do the work
	f := flowfile.New(bytes.NewReader(make([]byte, *payload)), int64(*payload))
	f.Attrs.Set("path", "./")
	for i := 1; ; i++ {
		f.Reset()
		f.Attrs.Set("filename", fmt.Sprintf("0x%02x.tmp", i))
		//log.Println("i=", i)
		if err := hs.Send(f); err != nil {
			log.Println("  failed")
		}
	}
}
