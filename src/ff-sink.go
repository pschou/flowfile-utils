package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/pschou/go-bunit"
	"github.com/pschou/go-flowfile"
)

var about = `FlowFile Sink

This utility is intended to listen for FlowFiles on HTTP/HTTPS and drop them as
fast as they come in`

var (
	hs   *flowfile.HTTPTransaction
	dump = flag.String("dump", "", "Dump the payload to the screen")
)

func main() {
	service_flags()
	listen_flags()
	parse()

	// Configure the go HTTP server
	server := &http.Server{
		Addr:           *listen,
		TLSConfig:      tlsConfig,
		ReadTimeout:    10 * time.Hour,
		WriteTimeout:   10 * time.Hour,
		MaxHeaderBytes: 1 << 20,
	}

	// Setting up the FlowFile receiver
	ffReceiver := flowfile.NewHTTPFileReceiver(post)
	http.Handle(*listenPath, ffReceiver)
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

func post(f *flowfile.File, w http.ResponseWriter, r *http.Request) (err error) {
	if *verbose {
		adat, _ := json.Marshal(f.Attrs)
		fmt.Printf("  - %s\n", adat)
	}
	switch *dump {
	case "raw":
		io.Copy(os.Stdout, f)
		fmt.Println()
	case "hex":
		io.Copy(hex.Dumper(os.Stdout), f)
	case "":
		io.Copy(io.Discard, f)
	default:
		log.Fatal("Unknown dump kind", *dump)
	}
	f.Close()

	//if *verbose && f.Size > 0 {
	err = f.Verify()
	if *verbose {
		if err == nil {
			log.Println("    Checksum passed for file/segment", f.Attrs.Get("filename"),
				bunit.NewBytes(int64(f.Size)))
		} else {
			//if err == flowfile.ErrorChecksumMissing {
			log.Println("    Checksum Error", err, "for", f.Attrs.Get("filename"),
				bunit.NewBytes(int64(f.Size)))
		}
	}
	//}

	return nil
}
