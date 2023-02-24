package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"

	"github.com/docker/go-units"
	"github.com/pschou/go-flowfile"
)

var (
	about = `FF-Flood

This utility is intended to saturate the bandwidth of a FlowFile endpoint for
load testing.`

	count       = flag.Int("n", 0, "Count of number of FlowFiles to send")
	max         = flag.String("max", "20MB", "Max payload size for upload in bytes")
	min         = flag.String("min", "10MB", "Min Payload size for upload in bytes")
	hash        = flag.String("hash", "SHA1", "Hash to use in checksum value")
	addChecksum = flag.Bool("add-checksum", true, "Add a checksum to the attributes")
	threads     = flag.Int("threads", 4, "Parallel concurrent uploads")
	name        = flag.String("name-format", "file%04d.dat", "File naming format")
	hs          *flowfile.HTTPTransaction
	wd, _       = os.Getwd()
)

func main() {
	sender_flags()
	parse()

	if len(flag.Args()) != 0 {
		flag.Usage()
		return
	}

	maxBytes, err := units.FromHumanSize(*max)
	if err != nil {
		log.Fatal("Invalid max size", *max)
	}

	minBytes, err := units.FromHumanSize(*min)
	if err != nil {
		log.Fatal("Invalid min size", *min)
	}

	if maxBytes < minBytes {
		log.Fatal("Max is smaller than min")
	}

	// Connect to the server and establish a session
	hs, err = flowfile.NewHTTPTransaction(*url, tlsConfig)
	if err != nil {
		log.Fatal(err)
	}

	// Send off all the empty files and folders first
	log.Println("Sending...")

	var c int
	var wg sync.WaitGroup
	for th := 0; th < *threads; th++ {
		wg.Add(1)
		go func(th int) {
			defer wg.Done()
			var j, size int
			// do the work
			for {
				j, c = c, c+1
				if *count > 0 && j >= *count {
					break
				}
				if sp := int(maxBytes - minBytes); sp > 0 {
					size = rand.Intn(sp) + int(minBytes)
				} else {
					size = int(maxBytes)
				}
				f := flowfile.New(&zero{}, int64(size))
				f.Attrs.Set("path", "./")
				updateChain(f, nil, "FLOOD")
				filename := fmt.Sprintf(*name, j)
				f.Attrs.Set("filename", filename)
				f.Attrs.GenerateUUID()
				if *addChecksum && size > 0 {
					if err := f.AddChecksum(*hash); err != nil {
						log.Fatal(err)
					}
				}
				f.Reset()
				log.Println(th, "sending", filename, units.HumanSize(float64(size)))
				if err := hs.Send(f); err != nil {
					log.Println("  failed")
				}
			}
		}(th)
	}

	wg.Wait()
}

type zero struct{}

func (z zero) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
func (z zero) ReadAt(p []byte, off int64) (n int, err error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
