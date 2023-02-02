package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/pschou/go-flowfile"
)

var about = `NiFi Diode

This utility is intended to take input over a NiFi compatible port and pass all
FlowFiles into another NiFi port while updating the attributes with the
certificate and chaining any previous certificates.`

var (
	basePath   = flag.String("path", "stager", "Directory which to scan for FlowFiles")
	listen     = flag.String("listen", ":8080", "Where to listen to incoming connections (example 1.2.3.4:8080)")
	listenPath = flag.String("listenPath", "/contentListener", "Where to expect FlowFiles to be posted")
	enableTLS  = flag.Bool("tls", false, "Enforce TLS for secure transport on incoming connections")
	url        = flag.String("url", "http://localhost:8080/contentListener", "Where to send the files from staging")
	chain      = flag.Bool("update-chain", true, "Add the client certificate to the connection-chain-# header")

	hs *flowfile.HTTPSender
)

func main() {
	flag.Parse()
	if *enableTLS || strings.HasPrefix(*url, "https") {
		loadTLS()
	}

	log.Println("creating sender...")
	var err error
	hs, err = flowfile.NewHTTPSender(*url, http.DefaultClient)
	if err != nil {
		log.Panic(err)
	}

	ffReciever := flowfile.HTTPReciever{Handler: post}
	http.Handle(*listenPath, ffReciever)
	log.Println("Listening on", *listen)
	if *enableTLS {
		server := &http.Server{Addr: *listen, TLSConfig: tlsConfig}
		log.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
	} else {
		log.Fatal(http.ListenAndServe(*listen, nil))
	}
}

func post(f *flowfile.File, r *http.Request) (err error) {
	// Quick sanity check that paths are not in a bad state
	dir := filepath.Clean(f.Attrs.Get("path"))
	if strings.HasPrefix(dir, "..") {
		return fmt.Errorf("Invalid path %q", dir)
	}

	if *enableTLS && *chain {
		// Make sure the client certificate is added to the certificate chain, 1 being the closest
		if err = updateChain(f, r); err != nil {
			return err
		}
	}

	return hs.Send(f)
}
