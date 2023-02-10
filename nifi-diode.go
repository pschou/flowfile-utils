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
	"time"

	"github.com/inhies/go-bytesize"
	"github.com/pschou/go-flowfile"
)

var (
	about = `NiFi Diode

This utility is intended to take input over a NiFi compatible port and pass all
FlowFiles into another NiFi port while updating the attributes with the
certificate and chaining any previous certificates.`

	noChecksum = flag.Bool("no-checksums", false, "Ignore doing checksum checks")
	hs         *flowfile.HTTPTransaction
)

func main() {
	service_flags()
	listen_flags()
	sender_flags()
	parse()

	// Settings for the flow file receiver
	ffReceiver := flowfile.NewHTTPReceiver(post)
	if *maxSize != "" {
		if bs, err := bytesize.Parse(*maxSize); err != nil {
			log.Fatal("Unable to parse max-size", err)
		} else {
			log.Println("Setting max-size to", bs)
			ffReceiver.MaxPartitionSize = int64(uint64(bs))
		}
	}

	// Connect to the destination NiFi to prepare to send files
	log.Println("Creating sender...")

	var err error
	hs, err = flowfile.NewHTTPTransaction(*url, tlsConfig)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			if hs.MaxPartitionSize > 0 || ffReceiver.MaxPartitionSize > 0 {
				if ffReceiver.MaxPartitionSize == 0 ||
					(hs.MaxPartitionSize > 0 && hs.MaxPartitionSize < ffReceiver.MaxPartitionSize) {
					log.Println("Setting max-size to", hs.MaxPartitionSize)
					ffReceiver.MaxPartitionSize = hs.MaxPartitionSize
				} else {
					log.Println("Keeping max-size at", ffReceiver.MaxPartitionSize)
				}
			}
			time.Sleep(10 * time.Minute)
			hs.Handshake()
		}
	}()

	server := &http.Server{
		Addr:           *listen,
		TLSConfig:      tlsConfig,
		ReadTimeout:    10 * time.Hour,
		WriteTimeout:   10 * time.Hour,
		MaxHeaderBytes: 1 << 20,
	}
	http.Handle(*listenPath, ffReceiver)

	// Open the local port to listen for incoming connections
	if *enableTLS {
		log.Println("Listening with HTTPS on", *listen, "at", *listenPath)
		log.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
	} else {
		log.Println("Listening with HTTP on", *listen, "at", *listenPath)
		log.Fatal(server.ListenAndServe())
	}
}

// Post handles every flowfile that is posted into the diode
func post(rdr *flowfile.Scanner, w http.ResponseWriter, r *http.Request) {
	var err error
	var f *flowfile.File

	httpWriter := hs.NewHTTPPostWriter()

	defer func() {
		if err != nil {
			log.Println("err:", err)
			httpWriter.Terminate()
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			httpWriter.Close()
			if httpWriter.Response == nil {
				err = fmt.Errorf("File did not send, no response")
				w.WriteHeader(http.StatusInternalServerError)
			} else if httpWriter.Response.StatusCode != 200 {
				err = fmt.Errorf("File did not send successfully, Server replied: %s", httpWriter.Response.Status)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		}
	}()

	// Loop over all the files in the post payload
	for rdr.Scan() {
		f = rdr.File()

		// Flatten directory for ease of viewing
		dir := filepath.Clean(f.Attrs.Get("path"))

		// Make sure the client chain is added to attributes, 1 being the closest
		updateChain(f, r, "DIODE")

		// Send the flowfile to the next NiFi port, if the send fails, it will come
		// back with an error and this in turn will be passed back to the sender
		// side.  All this is done without allowing any bytes to transfer from the
		// receiver side to the sender side.
		if xForwardFor := r.Header.Get("X-Forwarded-For"); xForwardFor != "" {
			httpWriter.Header.Set("X-Forwarded-For", r.RemoteAddr+","+xForwardFor)
		} else {
			httpWriter.Header.Set("X-Forwarded-For", r.RemoteAddr)
		}
		filename := f.Attrs.Get("filename")

		if id := f.Attrs.Get("fragment.index"); id != "" {
			i, _ := strconv.Atoi(id)
			fmt.Printf("  Dioding segment %d of %s of %s for %s\n", i,
				f.Attrs.Get("fragment.count"), path.Join(dir, filename), r.RemoteAddr)
		} else {
			fmt.Printf("  Dioding file %s for %s\n", path.Join(dir, filename), r.RemoteAddr)
		}

		if *verbose {
			adat, _ := json.Marshal(f.Attrs)
			fmt.Printf("    %s\n", adat)
		}

		_, err = httpWriter.Write(f)

		if err == nil && !*noChecksum {
			err = f.Verify()
			if err == flowfile.ErrorChecksumMissing {
				if *verbose && f.Size > 0 {
					log.Println("    No checksum found for", filename)
				}
				err = nil
			}
		}
		if err != nil {
			return
		}
	}
	err = rdr.Err() // Pick up any reader errors
}
