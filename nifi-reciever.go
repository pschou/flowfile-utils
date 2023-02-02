package main

import (
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

var about = `NiFi Reciever

This utility is intended to listen for flow files on a NifI compatible port and
then parse these files and drop them to disk for usage elsewhere.`

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
	http.Handle("/contentListener", ffReciever)
	log.Println("Listening on", listen)
	if *enableTLS {
		server := &http.Server{Addr: *listen, TLSConfig: tlsConfig}
		log.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
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
	output := path.Join(dir, filename)
	var fh *os.File
	fmt.Println("  Recieving nifi file", filename, "size", f.Size())
	fmt.Printf("  %#v\n", f.Attrs)
	if fh, err = os.Create(output); err != nil {
		return err
	}
	defer fh.Close() // Make sure file is closed at the end of the function
	_, err = io.Copy(fh, f)
	if err != nil {
		return err
	}
	return f.Verify() // Return the verification of the checksum
}
