package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/docker/go-units"
	"github.com/google/uuid"
	"github.com/pschou/go-flowfile"
	"github.com/pschou/go-iothrottler"
	"github.com/pschou/go-memdiskbuf"
)

var (
	about = `FF-HTTP-TO-UDP

This utility is intended to take input over a FlowFile compatible port and pass
all FlowFiles to a UDP endpoint after verifying checksums.  A chain of custody
is maintained by adding an action field with "HTTP-TO-UDP" value.

Note: The port range used in the source UDP address directly affect the number
of concurrent sessions.

The resend-delay will add latency (by delaying new connections until second
send is complete) but will add error resilience in the transfer.  In other
words, shortening the delay will likely mean more errors, while increaing will
slow down the number of accepted HTTP connections upstream.`

	//noChecksum = flag.Bool("no-checksums", false, "Ignore doing checksum checks")
	//udpSrcInt  = flag.String("udp-src-int", "ens192", "Interface where to send UDP packets")
	udpDstAddr = flag.String("udp-dst-addr", "127.0.0.1:2100-2107",
		"Target IP:PORT for sending UDP packet, to enable threading specify a port range\n"+
			"IE 10 threads split: 10.12.128.249:2100-2104,2106-2110")
	udpSrcAddr = flag.String("udp-src-addr", ":3100-3107",
		"Source IP:PORT for originating UDP packets, to enable threading specify a port range\n"+
			"IE 10 threads split: :3100-3104,3106-3110, 1 thread: :3100")
	//udpDstMac  = flag.String("udp-dst-mac", "6c:3b:6b:ed:78:14", "Target MAC for UDP packet (only needed using raw)")
	//udpSrcMac  = flag.String("udp-src-mac", "00:0c:29:69:bd:3d", "Source MAC for UDP packet (only needed using raw)")
	resend         = flag.Duration("resend-delay", 1*time.Second, "Time between first transmit and second, set to 0s to disable.")
	maxConnections = flag.Int("max-http-sessions", 1000, "Limit the number of allowed concurrent incoming HTTP connections")
	connTimeout    = flag.Duration("http-timeout", 10*time.Hour, "Limit the number total upload time")

	// Additional math parts
	//throttleDelay, throttleConnections time.Duration

	mtu            = flag.Int("mtu", 1200, "Maximum transmit unit")
	maxPayloadSize = 1280

	// TODO: Enable raw packet sending
	//useRaw         = flag.Bool("udp-raw", false, "Use raw UDP gopacket sender")
	ffWriter   *flowfile.Writer
	writerLock sync.Mutex

	throttle    = flag.Int("throttle", 83886080, "Bandwidth shape in bits per second (per thread), for example 80Mbps")
	throttleGap = flag.Int("throttle-spec", 0, "Frame spec defined by carrier/media, used to tune the tx rate.\n"+
		"This is the number of bytes added to the mtu which defines the time on the media between frames.\n"+
		"The value can be tuned (like -120 to 120). Frames are sent less frequently with a larger value.")
	throttleShared = flag.Bool("throttle-shared", false, "By default each thread is throttled, instead throttle all threads as one (not recommended).")

	hash        = flag.String("hash", "SHA1", "Hash to use in checksum value")
	addChecksum = flag.Bool("add-checksum", false, "Add a checksum to the attributes (if missing)")
)

func main() {
	service_flags()
	listen_flags()
	temp_flags()
	attributes = flag.String("attributes", "", "File with additional attributes to add to FlowFiles")
	parse()

	maxPayloadSize = *mtu - 8 //28 // IPv4 Header
	//maxPayloadSize = *mtu - 48 // IPv6 Header

	if *throttleShared {
		log.Println("Rate limit overall set to:", *throttle, " bps")
	} else {
		log.Println("Rate limit per thread set to:", *throttle, " bps")
	}

	// Connect to the destination
	log.Println("Creating senders for UDP from:", *udpSrcAddr)
	log.Println("Creating destinations for UDP:", *udpDstAddr)
	setupUDP()

	log.Println("Creating listener on:", *listen)
	// Configure the go HTTP server
	server := &http.Server{
		Addr:           *listen,
		TLSConfig:      tlsConfig,
		ReadTimeout:    *connTimeout,
		WriteTimeout:   *connTimeout,
		MaxHeaderBytes: 1 << 20,
	}

	// Setting up the flow file receiver
	ffReceiver := flowfile.NewHTTPFileReceiver(post)
	ffReceiver.MaxConnections = *maxConnections
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
func post(f *flowfile.File, w http.ResponseWriter, r *http.Request) (err error) {
	close := true
	wk := <-workers
	defer func() {
		if *debug && err != nil {
			log.Println("post failed:", err)
		}
		if close {
			wk.buf.Reset()
			workers <- wk
		}
		runtime.GC()
	}()
	if *verbose {
		fmt.Println("using connection:", wk.conn)
	}

	// Make sure the client chain is added to attributes, 1 being the closest
	updateChain(f, r, "HTTP-TO-UDP")

	// Send initial wakeup packet for allocation of job remotely
	id, _ := uuid.Parse(f.Attrs.Get("uuid"))
	hdr := ffHeader{
		UUID:   id,
		Size:   uint64(f.HeaderSize()) + uint64(f.Size),
		Offset: 0,
		MTU:    uint16(maxPayloadSize) - uint16(ffHeaderSize),
	}

	{ // Write out the initial header
		var initBuf bytes.Buffer
		binary.Write(&initBuf, binary.BigEndian, &hdr)
		wk.conn.WriteTo(initBuf.Bytes(), wk.dst) // Send an empty payload
	}

	toCopy := maxPayloadSize - int(ffHeaderSize)

	// Flatten directory for ease of log viewing
	dir := filepath.Clean(f.Attrs.Get("path"))
	filename := f.Attrs.Get("filename")

	// Print out to the screen activity
	if id := f.Attrs.Get("fragment.index"); id != "" {
		i, _ := strconv.Atoi(id)
		log.Printf("  Got segment %d of %s of %s (%s) for %s\n", i,
			f.Attrs.Get("fragment.count"), path.Join(dir, filename), units.HumanSize(float64(f.Size)), r.RemoteAddr)
	} else {
		log.Printf("  Got file %s (%s) for %s\n", path.Join(dir, filename), units.HumanSize(float64(f.Size)), r.RemoteAddr)
	}

	// Show the details of the file being sent
	if *verbose {
		fmt.Printf("incoming: %s\n", f.Attrs)
	}

	// Add a checksum
	var checksumAdded bool
	if ck := f.Attrs.Get("checksumType"); *addChecksum && ck == "" {
		f.Attrs.Set("checksumType", *hash)
		checksumAdded = true
		if err := f.ChecksumInit(); err != nil {
			log.Println("Error adding checksum", *hash, err)
		}
	}

	// Copy the entire file payload to MemDiskBuffer
	io.Copy(wk.buf, f)

	//fmt.Printf("buf: %#v\n", string(buf.Bytes()))
	if checksumAdded {
		f.AddChecksumFromVerify() // Add the correct checksum to payload
	}

	// Verify the checksum
	err = f.Verify()
	switch err {
	case flowfile.ErrorChecksumMissing:
		if f.Size > 0 {
			log.Println("    No checksum found for", filename)
			return
		}
		log.Println("    Empty", filename, "sending")
	case nil:
		if *verbose && f.Size > 0 {
			if checksumAdded {
				log.Println("    Checksum added for", filename, "sending")
			} else {
				log.Println("    Checksum passed for", filename, "sending")
			}
		}
	default:
		log.Println("    Checksum failed for", filename, f.VerifyDetails(), wk.buf.Len())
		return
	}

	{ // First send
		// Create output file handle
		f1 := flowfile.New(wk.buf, wk.buf.Cap())
		f1.Attrs = f.Attrs

		// Read the FlowFile through an EncodedReader for dropping on the wire
		rdr1 := f1.EncodedReader()
		hdr1 := hdr

		// Write out the payload
		var writeBuf bytes.Buffer
		var a int64
		var b int
		hdr1.Offset = 0
		var copy_err error
		for copy_err == nil {
			binary.Write(&writeBuf, binary.BigEndian, &hdr1)
			a, copy_err = io.CopyN(&writeBuf, rdr1, int64(toCopy))
			<-wk.throttler.C
			if b, err = wk.conn.WriteTo(writeBuf.Bytes(), wk.dst); err != nil {
				return
			}
			if int(a)+int(ffHeaderSize) != b {
				err = fmt.Errorf("Buffer to packet size error %d != %d", a, b)
				return
			}
			hdr1.Offset += uint64(toCopy)
			writeBuf.Reset()
		}

		if copy_err != io.EOF {
			err = copy_err
			return
		}
	}

	if *resend > 0 {
		close = false // prevent the connection return to pool while writes are happening
		go func() {   // spawn child thread to do the send so as to release the parent
			defer func() {
				wk.buf.Reset()
				workers <- wk
				runtime.GC()
			}()
			if *debug {
				fmt.Println("  waiting to do resend", filename, wk.conn, *resend)
			}
			// Hold off, then do it all again
			time.Sleep(*resend)

			if *debug {
				fmt.Println("  resending ", filename)
			}

			// Create output file handle
			f1 := flowfile.New(wk.buf, wk.buf.Cap())
			f1.Attrs = f.Attrs

			// Read the FlowFile through an EncodedReader for dropping on the wire
			rdr1 := f1.EncodedReader()
			hdr1 := hdr

			// Write out the payload
			var writeBuf bytes.Buffer
			var a int64
			var b int
			hdr1.Offset = 0
			var copy_err error
			for copy_err == nil {
				binary.Write(&writeBuf, binary.BigEndian, &hdr1)
				a, copy_err = io.CopyN(&writeBuf, rdr1, int64(toCopy))
				<-wk.throttler.C
				if b, err = wk.conn.WriteTo(writeBuf.Bytes(), wk.dst); err != nil {
					return
				}
				if int(a)+int(ffHeaderSize) != b {
					err = fmt.Errorf("Buffer to packet size error %d != %d", a, b)
					return
				}
				hdr1.Offset += uint64(toCopy)
				writeBuf.Reset()
			}
		}()
	}
	return
}

type worker struct {
	conn      *net.UDPConn
	dst       *net.UDPAddr
	throttler *iothrottler.Limit
	buf       *memdiskbuf.Buffer
}

var workers chan *worker

func setupUDP() {
	srcHost, srcPortSpec, err := net.SplitHostPort(*udpSrcAddr)
	if err != nil {
		log.Fatal("Error splitting host ports", err)
	}
	srcPorts := hypenRange(srcPortSpec)

	dstHost, dstPortSpec, err := net.SplitHostPort(*udpDstAddr)
	dstPorts := hypenRange(dstPortSpec)

	if len(dstPorts) != len(srcPorts) {
		log.Fatal("Destination port range count does not match the source port range count")
	}

	// If a throttler for all is defined
	var throttler *iothrottler.Limit
	if *throttleShared {
		throttler = iothrottler.NewLimit(*throttle, maxPayloadSize, *throttleGap)
	}
	workers = make(chan *worker, len(srcPorts))

	for i := 0; i < len(srcPorts); i++ {
		src, err := net.ResolveUDPAddr("udp", net.JoinHostPort(srcHost, fmt.Sprintf("%d", srcPorts[i])))
		if *debug {
			fmt.Printf("src udpSrv: %#v\n", src)
		}
		if err != nil {
			println("ResolveUDPAddr failed:", err.Error())
			os.Exit(1)
		}

		conn, err := net.ListenUDP("udp", src)
		if err != nil {
			log.Fatal("Listen UDP failed:", err)
		}

		dst, err := net.ResolveUDPAddr("udp", net.JoinHostPort(dstHost, fmt.Sprintf("%d", dstPorts[i])))
		if *debug {
			fmt.Printf("dst udpSrv: %#v\n", dst)
		}
		if err != nil {
			println("ResolveUDPAddr failed:", err.Error())
			os.Exit(1)
		}

		wk := &worker{
			throttler: throttler,
			conn:      conn,
			dst:       dst,
			buf:       makeBuf(),
		}

		if *throttleShared {
			fmt.Println("worker", i, "built with shared throttler")
			wk.throttler = throttler
		} else {
			fmt.Println("worker", i, "built with threaded throttler")
			wk.throttler = iothrottler.NewLimit(*throttle, maxPayloadSize, *throttleGap)
		}
		workers <- wk
	}

}
