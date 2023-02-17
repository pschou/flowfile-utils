package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pschou/go-flowfile"
	"github.com/xtaci/kcp-go"
	"golang.org/x/crypto/pbkdf2"
)

var (
	about = `FF-HTTP2KCP

This utility is intended to take input over a FlowFile compatible port and pass all
FlowFiles into KCP endpoint for speeding up throughput over long distances.`

	noChecksum   = flag.Bool("no-checksums", false, "Ignore doing checksum checks")
	kcpTarget    = flag.String("kcp", "10.12.128.249:2112", "Target KCP server to send flowfiles")
	dataShards   = flag.Int("kcp-data", 10, "Number of data packets to send in a FEC grouping")
	parityShards = flag.Int("kcp-parity", 3, "Number of parity packets to send in a FEC grouping")
	sndwnd       = flag.Int("sndwnd", 1024, "set send window size(num of packets)")
	rcvwnd       = flag.Int("rcvwnd", 128, "set receive window size(num of packets)")
	readbuf      = flag.Int("readbuf", 4194304, "per-socket read buffer in bytes")
	writebuf     = flag.Int("writebuf", 16777217, "per-socket write buffer in bytes")
	dscp         = flag.Int("dscp", 46, "set DSCP(6bit)")
	mtu          = flag.Int("mtu", 1350, "set maximum transmission unit for UDP packets")
	threads      = flag.Int("threads", 40, "Parallel concurrent uploads")
	crypto       = flag.String("crypto", "salsa20:ThisIsASecret", "Enable or disable crypto\n"+
		"\"none\" To use no cipher.")

	connBuf      chan *kcp.UDPSession
	worker       chan int
	successCount int
)

func main() {
	service_flags()
	listen_flags()
	parse()

	// Connect to the destination to prepare to send files
	log.Println("Creating sender,", *url)

	connBuf = make(chan *kcp.UDPSession, *threads+4)
	worker = make(chan int, *threads)
	for i := 0; i < *threads; i++ {
		// Create a KCP Transaction with target
		if conn, err := Dial(); err != nil {
			log.Fatal(err)
		} else {
			if i == 0 {
				if ping(conn) {
					log.Println("Ping check passed")
				} else {
					log.Println("Waiting for remote to become available")
				}
			}
			connBuf <- conn
		}
		worker <- i
	}

	// Do pings every so often to establish health status
	go func() {
		for {
			// We got through, so we'll put this connection into the channel and fill up the chan
			if *verbose && len(connBuf)+len(worker) <= *threads {
				log.Println("Rebuilding connection pool")
			}
			for len(connBuf)+len(worker) <= *threads {
				if conn, err := Dial(); err == nil {
					connBuf <- conn
				} else {
					break
				}
			}
			time.Sleep(time.Second)
		}
	}()

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
	//send_metrics(func(f *flowfile.File) { hs.Send(f) }, ffReceiver)

	// Open the local port to listen for incoming connections
	if *enableTLS {
		log.Println("Listening with HTTPS on", *listen, "at", *listenPath)
		log.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
	} else {
		log.Println("Listening with HTTP on", *listen, "at", *listenPath)
		log.Fatal(server.ListenAndServe())
	}
}

func Dial() (conn *kcp.UDPSession, err error) {
	defer func() {
		if err != nil && conn != nil {
			conn.Close()
		}
	}()
	// Setup encryption
	var block kcp.BlockCrypt
	switch {
	case *crypto == "", *crypto == "none":
	case strings.HasPrefix(*crypto, "salsa20:"):
		pass := pbkdf2.Key([]byte(strings.TrimPrefix(*crypto, "salsa20:")), []byte(SALT), 4096, 32, sha1.New)
		block, _ = kcp.NewSalsa20BlockCrypt(pass)
	default:
		log.Fatal("Unknown encryption method")
	}

	// Create a KCP Transaction with target
	if conn, err = kcp.DialWithOptions(*kcpTarget, block, *dataShards, *parityShards); err != nil {
		return
	}
	conn.SetStreamMode(false)
	conn.SetWriteDelay(false)
	conn.SetNoDelay(1, 10, 2, 1)
	//conn.SetNoDelay(config.NoDelay, config.Interval, config.Resend, config.NoCongestion)
	conn.SetWindowSize(*sndwnd, *rcvwnd)
	conn.SetMtu(*mtu)
	conn.SetACKNoDelay(false)

	if err = conn.SetDSCP(0); err != nil {
		log.Println("SetDSCP:", err)
	}
	if err = conn.SetReadBuffer(*readbuf); err != nil {
		log.Println("SetReadBuffer:", err)
	}
	if err = conn.SetWriteBuffer(*writebuf); err != nil {
		log.Println("SetWriteBuffer:", err)
	}
	return
}

// Post handles every flowfile that is posted into the diode
func post(rdr *flowfile.Scanner, w http.ResponseWriter, r *http.Request) {
	var err error
	var f *flowfile.File
	var conn *kcp.UDPSession
	for conn = <-connBuf; !ping(conn); conn = <-connBuf {
		conn.Close()
	}

	iw := <-worker
	defer func() {
		worker <- iw
		conn.SetDeadline(time.Time{}) // Restore previous deadline
		if err != nil {
			if *debug {
				log.Println("err:", err)
			}
			w.WriteHeader(http.StatusInternalServerError)
			conn.Close()
		} else {
			if len(connBuf) < *threads-1 {
				connBuf <- conn
			} else {
				conn.Close()
			}
			w.WriteHeader(http.StatusOK)
		}
	}()

	// Loop over all the files in the post payload
	for rdr.Scan() {
		f = rdr.File()
		successCount++

		// Flatten directory for ease of viewing
		dir := filepath.Clean(f.Attrs.Get("path"))

		// Make sure the client chain is added to attributes, 1 being the closest
		updateChain(f, r, "HTTP2KCP")

		filename := f.Attrs.Get("filename")

		if *verbose {
			if id := f.Attrs.Get("fragment.index"); id != "" {
				i, _ := strconv.Atoi(id)
				fmt.Printf("  KCPing segment %d of %s of %s for %v\n", i,
					f.Attrs.Get("fragment.count"), path.Join(dir, filename), r.RemoteAddr)
			} else {
				fmt.Printf("  KCPing file %s for %v\n", path.Join(dir, filename), r.RemoteAddr)
			}

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
				err = nil // Let non-checksummed flowfiles through
			}
		}
		if err != nil {
			return
		}

		err = f.Attrs.WriteTo(conn)
		if err != nil && err != io.EOF {
			return
		}

		err = binary.Write(conn, binary.BigEndian, uint64(f.Size))
		if err != nil && err != io.EOF {
			return
		}

		// Grab the underlying buffer and send it off
		b := buf.Bytes()
		var n int
		for err == nil && len(b) > 4000 {
			conn.SetDeadline(time.Now().Add(10 * time.Second)) // We need to have a result
			n, err = conn.Write(b[:4000])
			b = b[n:]
		}
		if err != nil && err != io.EOF {
			return
		}
		n, err = conn.Write(b)
		if err != nil && err != io.EOF {
			return
		}
	}

	if err = rdr.Err(); err != nil { // Pick up any reader errors
		return
	}
	conn.SetACKNoDelay(true)                           // Flush quickly
	conn.SetDeadline(time.Now().Add(10 * time.Second)) // We need to have a result
	conn.Write([]byte(flowfile.FlowFileEOF))           // Send EOF
	dat := make([]byte, 4)
	var n int
	if n, err = conn.Read(dat); err != nil { // Read in 4 bytes (OKAY/FAIL)
		return
	}
	if n != 4 || string(dat) != "OKAY" { // Test if it is what we expect
		successCount++
		err = fmt.Errorf("Remote receiving error")
		return
	}
	conn.SetACKNoDelay(false) // Flush slowly
}

func ping(conn *kcp.UDPSession) (ok bool) {
	if *debug {
		log.Println("pinging connection, th:", *threads, "bl:", len(connBuf), "cc:", len(worker))
	}
	conn.SetACKNoDelay(true)                          // Flush quickly
	conn.SetDeadline(time.Now().Add(5 * time.Second)) // We need to have a result
	conn.Write([]byte(flowfile.FlowFileEOF))          // Send EOF
	dat := make([]byte, 4)
	conn.Read(dat)
	if string(dat) == "OKAY" { // Test if it is what we expect
		if *debug {
			log.Println("  success")
		}
		conn.SetACKNoDelay(false)     // Flush slowly
		conn.SetDeadline(time.Time{}) // Restore previous deadline
		ok = true
	} else {
		if *debug {
			log.Println("  failed")
		}
	}
	return
}
