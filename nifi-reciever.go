package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"path"
	"path/filepath"

	"github.com/pschou/go-flowfile"
)

var about = `NiFi Reciever

This utility is intended to listen for flow files on a NifI compatible port and
then parse these files and drop them to disk for usage elsewhere.`

var (
	basePath   = flag.String("path", "output", "Directory which to scan for FlowFiles")
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
	// Save the flowfile into the base path with the file structure defined by
	// the flowfile attributes.

	dir := filepath.Clean(f.Attrs.Get("path"))
	filename := f.Attrs.Get("filename")
	fmt.Println("  Recieving nifi file", path.Join(dir, filename), "size", f.Size())
	fmt.Printf("    %#v\n", f.Attrs)

	_, err = f.Save(*basePath)
	return
}
