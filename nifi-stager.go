package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/inhies/go-bytesize"
	"github.com/pschou/go-flowfile"
)

var about = `NiFi Stager

This utility is intended to take input over a NiFi compatible port and drop all
FlowFiles into directory along with associated attributes which can then be
unstaged using the NiFi Unstager.`

var (
	basePath   = flag.String("path", "stager", "Directory which to scan for FlowFiles")
	listen     = flag.String("listen", ":8080", "Where to listen to incoming connections (example 1.2.3.4:8080)")
	listenPath = flag.String("listenPath", "/contentListener", "Where to expect FlowFiles to be posted")
	enableTLS  = flag.Bool("tls", false, "Enable TLS for secure transport")
	maxSize    = flag.String("segment-max-size", "", "Set a maximum partition size for partitioning files to send")
)

func main() {
	flag.Parse()
	if *enableTLS {
		loadTLS()
	}

	fmt.Println("output set to", *basePath)

	// Settings for the flow file reciever
	ffReciever := flowfile.HTTPReciever{Handler: post}
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

func post(f *flowfile.File, r *http.Request) (err error) {
	dir := filepath.Clean(f.Attrs.Get("path"))
	if strings.HasPrefix(dir, "..") {
		return fmt.Errorf("Invalid path %q", dir)
	}
	dir = path.Join(*basePath, dir)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	filename := f.Attrs.Get("filename")
	uuid := f.Attrs.Get("uuid")
	output := path.Join(dir, uuid+"-"+filename)
	outputTemp := path.Join(dir, uuid+".attrs_tmp")
	outputAttrs := path.Join(dir, uuid+".attrs")
	var fh, fha *os.File
	fmt.Println("  Recieving nifi file", filename, "size", f.Size(), "uuid", uuid)
	if *verbose {
		adat, _ := json.Marshal(f.Attrs)
		fmt.Printf("    %s\n", adat)
	}

	if fh, err = os.Create(output); err != nil {
		return err
	}
	defer fh.Close() // Make sure file is closed at the end of the function
	if fha, err = os.Create(outputTemp); err != nil {
		return err
	}
	f.Attrs.Marshal(fha)
	binary.Write(fha, binary.BigEndian, uint64(f.Size()))
	fha.Close()
	os.Rename(outputTemp, outputAttrs)
	_, err = io.Copy(fh, f)
	if err != nil {
		return err
	}
	if f.Attrs.Get("kind") == "dir" {
		return nil
	}
	return f.Verify() // Return the verification of the checksum
}
