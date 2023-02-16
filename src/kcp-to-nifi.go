package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pschou/go-flowfile"
	"github.com/xtaci/kcp-go"
)

var (
	about = `KCP -to-> NiFi

This utility is intended to take input over a KCP connection and send FlowFiles
into a NiFi compatible port for speeding up throughput over long distances.`

	noChecksum   = flag.Bool("no-checksums", false, "Ignore doing checksum checks")
	kcpListen    = flag.String("kcp", ":2112", "Listen port for KCP connections")
	dataShards   = flag.Int("kcp-data", 5, "Number of data packets to send in a FEC grouping")
	parityShards = flag.Int("kcp-parity", 2, "Number of parity packets to send in a FEC grouping")

	hs *flowfile.HTTPTransaction
)

func main() {
	service_flags()
	sender_flags()
	parse()
	var err error

	// Connect to the destination NiFi to prepare to send files
	log.Println("Creating sender,", *url)

	// Create a HTTP Transaction with target URL
	if hs, err = flowfile.NewHTTPTransaction(*url, tlsConfig); err != nil {
		log.Fatal(err)
	}
	hs.RetryCount = 3
	hs.RetryDelay = 15 * time.Second

	if listener, err := kcp.ListenWithOptions(*kcpListen, nil, *dataShards, *parityShards); err != nil {
		log.Fatal(err)
	} else {
		defer listener.Close()
		for {
			if conn, err := listener.AcceptKCP(); err != nil {
				log.Printf("Error accepting connection:", err)
			} else {
				go post(conn)
			}
		}
	}
}

// Post handles every flowfile that is posted into the diode
func post(conn *kcp.UDPSession) {
	var err error
	var f *flowfile.File

	httpWriter := hs.NewHTTPPostWriter()

	defer func() {
		conn.SetACKNoDelay(true) // Flush quickly
		if err != nil {
			log.Println("err:", err)
			httpWriter.Terminate()
			conn.Write([]byte("FAIL"))
		} else {
			httpWriter.Close()
			conn.Write([]byte("OKAY"))
		}
		conn.Close()
	}()

	rdr := flowfile.NewScanner(conn)

	// Loop over all the files in the post payload
	for rdr.Scan() {
		f = rdr.File()

		// Flatten directory for ease of viewing
		dir := filepath.Clean(f.Attrs.Get("path"))

		// Make sure the client chain is added to attributes, 1 being the closest
		updateChain(f, nil, "KCP-NIFI")

		filename := f.Attrs.Get("filename")

		if id := f.Attrs.Get("fragment.index"); id != "" {
			i, _ := strconv.Atoi(id)
			fmt.Printf("  UnKCPing segment %d of %s of %s for %s\n", i,
				f.Attrs.Get("fragment.count"), path.Join(dir, filename), conn.RemoteAddr())
		} else {
			fmt.Printf("  UnKCPing file %s for %s\n", path.Join(dir, filename), conn.RemoteAddr())
		}

		if *verbose {
			adat, _ := json.Marshal(f.Attrs)
			fmt.Printf("    %s\n", adat)
		}

		// SLURP!
		buf := bufPool.Get().(*bytes.Buffer)
		defer func() { buf.Reset(); bufPool.Put(buf) }()
		io.Copy(buf, f)

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

		if *verbose {
			fmt.Println("checksum passed")
		}
		toSend := flowfile.New(buf, int64(buf.Len()))
		toSend.Attrs = f.Attrs

		if _, err = httpWriter.Write(toSend); err != nil {
			return
		}
		if *debug {
			fmt.Println("  file sent!")
		}
	}
	err = rdr.Err() // Pick up any reader errors
}
