package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

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
)

func main() {
	flag.Parse()
	if *enableTLS {
		loadTLS()
	}

	ffReciever := flowfile.HTTPReciever{Handler: post}
	http.Handle(*listenPath, ffReciever)
	log.Println("Listening on", *listen)
	if *enableTLS {
		log.Fatal(http.ListenAndServeTLS(*listen, *certFile, *keyFile, nil))
	} else {
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
		if err := os.Mkdir(dir, 0755); err != nil {
			return err
		}
	}

	filename := f.Attrs.Get("filename")
	uuid := f.Attrs.Get("uuid")
	output := path.Join(dir, uuid+"-"+filename)
	outputAttrs := path.Join(dir, uuid+".attrs")
	var fh, fha *os.File
	fmt.Println("  Recieving nifi file", filename, "size", f.Size(), "uuid", uuid)
	fmt.Printf("  %#v\n", f.Attrs)
	if fh, err = os.Create(output); err != nil {
		return err
	}
	defer fh.Close() // Make sure file is closed at the end of the function
	if fha, err = os.Create(outputAttrs); err != nil {
		return err
	}
	f.Attrs.Marshall(fha)
	binary.Write(fha, binary.BigEndian, uint64(f.Size()))
	fha.Close()
	_, err = io.Copy(fh, f)
	if err != nil {
		return err
	}
	return f.Verify() // Return the verification of the checksum
}
