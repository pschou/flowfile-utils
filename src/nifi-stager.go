package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/pschou/go-flowfile"
)

var about = `NiFi Stager

This utility is intended to take input over a NiFi compatible port and drop all
FlowFiles into directory along with associated attributes which can then be
unstaged using the NiFi Unstager.`

var (
	basePath         = flag.String("path", "stager", "Directory in which stage FlowFiles")
	script           = flag.String("script", "", "Shell script to be called on successful post")
	scriptShell      = flag.String("script-shell", "/bin/bash", "Shell to be used for script run")
	remove           = flag.Bool("rm", false, "Automatically remove file after script has finished")
	removeIncomplete = flag.Bool("rm-partial", true, "Automatically remove partial files\nTo unset this default use -rm-partial=false .")
	hs               *flowfile.HTTPTransaction
)

func main() {
	service_flags()
	listen_flags()
	parse()
	fmt.Println("Output set to", *basePath)

	// Configure the go HTTP server
	server := &http.Server{
		Addr:           *listen,
		TLSConfig:      tlsConfig,
		ReadTimeout:    10 * time.Hour,
		WriteTimeout:   10 * time.Hour,
		MaxHeaderBytes: 1 << 20,
	}

	// Setting up the flow file receiver
	ffReceiver := flowfile.NewHTTPReceiver(post)
	http.Handle(*listenPath, ffReceiver)

	// Setup a timer to update the maximums and minimums for the sender
	handshaker(nil, ffReceiver)

	// Open the local port to listen for incoming connections
	if *enableTLS {
		log.Println("Listening with HTTPS on", *listen, "at", *listenPath)
		log.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
	} else {
		log.Println("Listening with HTTP on", *listen, "at", *listenPath)
		log.Fatal(server.ListenAndServe())
	}
}

func post(s *flowfile.Scanner, w http.ResponseWriter, r *http.Request) {
	os.MkdirAll(*basePath, 0755)

	uuid := uuid.New().String()
	output := path.Join(*basePath, uuid)
	outputDat := output + ".dat"
	outputTemp := output + ".inprogress"
	outputAttrs := output + ".json"

	var err error
	var attrSlice []flowfile.Attributes
	var fh, fha *os.File
	defer func() {
		if fha != nil {
			fha.Close() // Make sure file is closed at the end of the function
		}
		if fh != nil {
			fh.Close() // Make sure file is closed at the end of the function
		}
		if err == nil {
			os.Rename(outputTemp, outputAttrs)
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}()

	// Create file for writing to
	if fh, err = os.Create(outputDat); err != nil {
		return
	}
	enc := flowfile.NewWriter(fh)
	if fha, err = os.Create(outputTemp); err != nil {
		return
	}

	var f *flowfile.File
	for s.Scan() {
		f = s.File()

		// Make sure the client chain is added to attributes, 1 being the closest
		updateChain(f, r, "TO-DISK")

		fmt.Println("  Receiving nifi file", f.Attrs.Get("filename"), "size", f.Size)
		if *verbose {
			adat, _ := json.Marshal(f.Attrs)
			fmt.Printf("    %s\n", adat)
		}

		if _, err = enc.Write(f); err != nil {
			return
		}

		if err = f.Verify(); err != nil {
			if *removeIncomplete {
				os.Remove(outputDat)
				os.Remove(outputAttrs)
				log.Printf("  Removed %s (unverified)\n", uuid)
			}
			return
		}

		if *script != "" {
			log.Println("  Calling script", *scriptShell, *script, outputDat, outputAttrs)
			output, err := exec.Command(*scriptShell, *script, outputDat, outputAttrs).Output()
			if *verbose {
				log.Println("----- START", *script, uuid, "-----")
				fmt.Println(string(output))
				log.Println("----- END", *script, uuid, "-----")
				if err != nil {
					log.Printf("error %s", err)
				}
			}

			if *remove {
				os.Remove(outputDat)
				os.Remove(outputAttrs)
				//if *verbose {
				log.Printf("  Removed %s\n", uuid)
				//}
			}
		}

		f.Attrs.Set("size", fmt.Sprintf("%d", f.Size))
		attrSlice = append(attrSlice, f.Attrs)
	}

	{ // Write out JSON file with attributes
		enc := json.NewEncoder(fha)
		err = enc.Encode(&attrSlice)
	}
	return
}
