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

	"github.com/google/uuid"
	"github.com/inhies/go-bytesize"
	"github.com/pschou/go-flowfile"
)

var about = `NiFi Stager

This utility is intended to take input over a NiFi compatible port and drop all
FlowFiles into directory along with associated attributes which can then be
unstaged using the NiFi Unstager.`

var (
	basePath         = flag.String("path", "stager", "Directory in which stage FlowFiles")
	listen           = flag.String("listen", ":8080", "Where to listen to incoming connections (example 1.2.3.4:8080)")
	listenPath       = flag.String("listenPath", "/contentListener", "Path in URL where to expect FlowFiles to be posted")
	enableTLS        = flag.Bool("tls", false, "Enable TLS for secure transport")
	maxSize          = flag.String("segment-max-size", "", "Set a maximum size for partitioning files in sending")
	script           = flag.String("script", "", "Shell script to be called on successful post")
	scriptShell      = flag.String("script-shell", "/bin/bash", "Shell to be used for script run")
	remove           = flag.Bool("rm", false, "Automatically remove file after script has finished")
	removeIncomplete = flag.Bool("rm-partial", true, "Automatically remove partial files")
)

func main() {
	flag.Parse()
	if *enableTLS {
		loadTLS()
	}

	fmt.Println("Output set to", *basePath)
	os.MkdirAll(*basePath, 0755)

	// Settings for the flow file receiver
	ffReciever := flowfile.NewHTTPReciever(post)
	if *maxSize != "" {
		if bs, err := bytesize.Parse(*maxSize); err != nil {
			log.Fatal("Unable to parse max-size", err)
		} else {
			log.Println("Setting max-size to", bs)
			ffReciever.MaxPartitionSize = int(uint64(bs))
		}
	}

	http.Handle(*listenPath, ffReciever)
	if *enableTLS {
		log.Println("Listening with HTTPS on", *listen, "at", *listenPath)
		server := &http.Server{Addr: *listen, TLSConfig: tlsConfig}
		log.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
	} else {
		log.Println("Listening with HTTP on", *listen, "at", *listenPath)
		log.Fatal(http.ListenAndServe(*listen, nil))
	}
}

func post(s *flowfile.Scanner, r *http.Request) (err error) {
	uuid := uuid.New().String()
	output := path.Join(*basePath, uuid)
	outputDat := output + ".dat"
	outputTemp := output + ".inprogress"
	outputAttrs := output + ".json"

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
		}
	}()

	// Create file for writing to
	if fh, err = os.Create(outputDat); err != nil {
		return err
	}
	if fha, err = os.Create(outputTemp); err != nil {
		return err
	}

	var f *flowfile.File
	for s.Scan() {
		if f, err = s.File(); err != nil {
			return
		}
		fmt.Println("  Recieving nifi file", f.Attrs.Get("filename"), "size", f.Size)
		if *verbose {
			adat, _ := json.Marshal(f.Attrs)
			fmt.Printf("    %s\n", adat)
		}

		if err = f.WriteTo(fh); err != nil {
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

	enc := json.NewEncoder(fha)
	err = enc.Encode(&attrSlice)
	return
}
