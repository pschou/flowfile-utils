package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pschou/go-flowfile"
	"github.com/xtaci/kcp-go"
)

var (
	about = `NiFi -to-> KCP

This utility is intended to take input over a NiFi compatible port and pass all
FlowFiles into KCP endpoint for speeding up throughput over long distances.`

	noChecksum   = flag.Bool("no-checksums", false, "Ignore doing checksum checks")
	kcpTarget    = flag.String("kcp", "10.12.128.249:2112", "Target KCP server to send flowfiles")
	dataShards   = flag.Int("kcp-data", 5, "Number of data packets to send in a FEC grouping")
	parityShards = flag.Int("kcp-parity", 2, "Number of parity packets to send in a FEC grouping")
)

func main() {
	service_flags()
	listen_flags()
	parse()

	// Connect to the destination NiFi to prepare to send files
	log.Println("Creating sender,", *url)

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

// Post handles every flowfile that is posted into the diode
func post(rdr *flowfile.Scanner, w http.ResponseWriter, r *http.Request) {
	var err error
	var f *flowfile.File
	var conn *kcp.UDPSession

	// Create a KCP Transaction with target
	if conn, err = kcp.DialWithOptions(*kcpTarget, nil, *dataShards, *parityShards); err != nil {
		log.Fatal(err)
	}
	conn.SetStreamMode(true)
	conn.SetWriteDelay(false)
	conn.SetNoDelay(1, 10, 2, 1)
	//conn.SetNoDelay(config.NoDelay, config.Interval, config.Resend, config.NoCongestion)
	conn.SetWindowSize(128, 512)
	//conn.SetMtu(config.MTU)
	conn.SetACKNoDelay(false)

	if err := conn.SetDSCP(0); err != nil {
		log.Println("SetDSCP:", err)
	}
	if err := conn.SetReadBuffer(4194304); err != nil {
		log.Println("SetReadBuffer:", err)
	}
	if err := conn.SetWriteBuffer(4194304); err != nil {
		log.Println("SetWriteBuffer:", err)
	}

	// Create a writer and start sending flowfiles
	ffWriter := flowfile.NewWriter(conn)

	defer func() {
		if err != nil {
			log.Println("err:", err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		conn.Close()
	}()

	// Loop over all the files in the post payload
	for rdr.Scan() {
		f = rdr.File()

		// Flatten directory for ease of viewing
		dir := filepath.Clean(f.Attrs.Get("path"))

		// Make sure the client chain is added to attributes, 1 being the closest
		updateChain(f, r, "NIFI-KCP")

		filename := f.Attrs.Get("filename")

		if id := f.Attrs.Get("fragment.index"); id != "" {
			i, _ := strconv.Atoi(id)
			fmt.Printf("  KCPing segment %d of %s of %s for %v\n", i,
				f.Attrs.Get("fragment.count"), path.Join(dir, filename), r.RemoteAddr)
		} else {
			fmt.Printf("  KCPing file %s for %v\n", path.Join(dir, filename), r.RemoteAddr)
		}

		if *verbose {
			adat, _ := json.Marshal(f.Attrs)
			fmt.Printf("    %s\n", adat)
		}

		// SLURP!
		buf := bufPool.Get().(*bytes.Buffer)
		defer func() { buf.Reset(); bufPool.Put(buf) }()
		buf.ReadFrom(f)

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

		toSend := flowfile.New(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		toSend.Attrs = f.Attrs

		if _, err = ffWriter.Write(toSend); err != nil {
			return
		}
	}

	if err = rdr.Err(); err != nil { // Pick up any reader errors
		return
	}
	conn.SetACKNoDelay(true) // Flush quickly
	conn.Write([]byte("NiFiEOF"))

	//fmt.Println("Reading 4 bytes")
	dat := make([]byte, 4)
	conn.Read(dat)
	//fmt.Println("bytes:", string(dat))
	if string(dat) != "OKAY" {
		err = fmt.Errorf("Remote receiving error")
	}
}
