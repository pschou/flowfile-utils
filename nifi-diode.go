package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/inhies/go-bytesize"
	"github.com/pschou/go-flowfile"
)

var about = `NiFi Diode

This utility is intended to take input over a NiFi compatible port and pass all
FlowFiles into another NiFi port while updating the attributes with the
certificate and chaining any previous certificates.`

var (
	listen     = flag.String("listen", ":8082", "Where to listen to incoming connections (example 1.2.3.4:8080)")
	listenPath = flag.String("listenPath", "/contentListener", "Path in URL where to expect FlowFiles to be posted")
	enableTLS  = flag.Bool("tls", false, "Enforce TLS for secure transport on incoming connections")
	url        = flag.String("url", "http://localhost:8080/contentListener", "Where to send the files from staging")
	chain      = flag.Bool("update-chain", true, "Add the client certificate to the connection-chain-# header")
	maxSize    = flag.String("segment-max-size", "", "Set a maximum size for partitioning files in sending")
	retries    = flag.Int("retries", 3, "Retries after failing to send a file")
	noChecksum = flag.Bool("no-checksums", false, "Ignore doing checksum checks")
	//ecc        = flag.Float("ff-ecc", 0, "Set the amount of error correction to be sent (decimal percent)")

	hs *flowfile.HTTPSession
)

func main() {
	service_flag()
	flag.Parse()
	service_init()
	if *enableTLS || strings.HasPrefix(*url, "https") {
		loadTLS()
	}

	// Settings for the flow file receiver
	ffReceiver := flowfile.NewHTTPReceiver(post)
	if *maxSize != "" {
		if bs, err := bytesize.Parse(*maxSize); err != nil {
			log.Fatal("Unable to parse max-size", err)
		} else {
			log.Println("Setting max-size to", bs)
			ffReceiver.MaxPartitionSize = int(uint64(bs))
		}
	}

	// Connect to the destination NiFi to prepare to send files
	log.Println("creating sender...")
	var err error
	hs, err = flowfile.NewHTTPSession(*url, http.DefaultClient)
	if err != nil {
		log.Fatal(err)
	}
	if hs.MaxPartitionSize > 0 || ffReceiver.MaxPartitionSize > 0 {
		if ffReceiver.MaxPartitionSize == 0 ||
			(hs.MaxPartitionSize > 0 && hs.MaxPartitionSize < ffReceiver.MaxPartitionSize) {
			log.Println("Setting max-size to", hs.MaxPartitionSize)
			ffReceiver.MaxPartitionSize = hs.MaxPartitionSize
		} else {
			log.Println("Keeping max-size at", ffReceiver.MaxPartitionSize)
		}
	}

	// Open the local port to listen for incoming connections
	http.Handle(*listenPath, ffReceiver)
	if *enableTLS {
		log.Println("Listening with HTTPS on", *listen, "at", *listenPath)
		server := &http.Server{Addr: *listen, TLSConfig: tlsConfig}
		log.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
	} else {
		log.Println("Listening with HTTP on", *listen, "at", *listenPath)
		log.Fatal(http.ListenAndServe(*listen, nil))
	}
}

// Post handles every flowfile that is posted into the diode
func post(rdr *flowfile.Scanner, r *http.Request) (err error) {
	var f *flowfile.File

	httpWriter := hs.NewHTTPWriter(nil)
	//httpWriter.Write(f)
	defer func() {
		httpWriter.Close()
		if httpWriter.Response == nil {
			err = fmt.Errorf("File did not send, no response")
		} else if httpWriter.Response.StatusCode != 200 {
			err = fmt.Errorf("File did not send successfully, code %d", httpWriter.Response.StatusCode)
		}
		if *verbose {
			log.Println("err:", err)
		}
	}()

	// Loop over all the files in the post payload
	for rdr.Scan() {
		if f, err = rdr.File(); err != nil {
			return
		}
		// Quick sanity check that paths are not in a bad state
		dir := filepath.Clean(f.Attrs.Get("path"))
		if strings.HasPrefix(dir, "..") {
			err = fmt.Errorf("Invalid path %q", dir)
			return
		}

		if *enableTLS && *chain {
			// Make sure the client certificate is added to the certificate chain, 1 being the closest
			if err = updateChain(f, r); err != nil {
				return
			}
		}

		// Send the flowfile to the next NiFi port, if the send fails, it will come
		// back with an error and this in turn will be passed back to the sender
		// side.  All this is done without allowing any bytes to transfer from the
		// receiver side to the sender side.
		sendConfig := &flowfile.SendConfig{}
		if xForwardFor := r.Header.Get("X-Forwarded-For"); xForwardFor != "" {
			sendConfig.SetHeader("X-Forwarded-For", r.RemoteAddr+","+xForwardFor)
		} else {
			sendConfig.SetHeader("X-Forwarded-For", r.RemoteAddr)
		}
		filename := f.Attrs.Get("filename")

		if id := f.Attrs.Get("segment-index"); id != "" {
			i, _ := strconv.Atoi(id)
			fmt.Printf("  Dioding segment %d of %s of %s for %s\n", i+1,
				f.Attrs.Get("segment-count"), path.Join(dir, filename), r.RemoteAddr)
		} else {
			fmt.Printf("  Dioding file %s for %s\n", path.Join(dir, filename), r.RemoteAddr)
		}

		if *verbose {
			adat, _ := json.Marshal(f.Attrs)
			fmt.Printf("    %s\n", adat)
		}

		err = hs.Send(f, sendConfig)

		// Try a few more times before we give up
		for i := 1; err != nil && i < *retries; i++ {
			log.Println("  Upstream not accepting", filename, "retrying", i, "of", *retries)
			time.Sleep(10 * time.Second)
			err = hs.Handshake()
			if err == nil {
				err = hs.Send(f, nil)
			}
		}
		/*
			if err == nil && !*noChecksum {
				err = f.Verify()
				if err == flowfile.ErrorChecksumMissing {
					if *verbose && f.Size > 0 {
						log.Println("  No checksum found for", filename)
					}
					err = nil
				}
			}
		*/
	}
	return
}
