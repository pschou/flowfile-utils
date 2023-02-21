package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/uuid"
	"github.com/pschou/go-flowfile"
	"github.com/pschou/go-iothrottler"
)

var (
	about = `FF-HTTP-TO-UDP

This utility is intended to take input over a FlowFile compatible port and pass
all FlowFiles to a UDP endpoint after verifying checksums.  A chain of custody
is maintained by adding an action field with "HTTP-UDP" value.

Note: The port range used in the source UDP address directly affect the number
of concurrent sessions, and as payloads are buffered in memory (to do the
checksum) the memory bloat can be upwards on the order of NUM_PORTS *
MAX_PAYLOAD.  Please choose wisely.

The resend-delay will add latency (by delaying new connections until second
send is complete) but will add error resilience in the transfer.  In other
words, shortening the delay will likely mean more errors, while increaing will
slow down the number of accepted HTTP connections upstream.`

	//noChecksum = flag.Bool("no-checksums", false, "Ignore doing checksum checks")
	//udpSrcInt  = flag.String("udp-src-int", "ens192", "Interface where to send UDP packets")
	udpDstAddr = flag.String("udp-dst-addr", "10.12.128.249:2100-2107",
		"Target IP:PORT for sending UDP packet, to enable threading specify a port range\n"+
			"IE 10 threads split: 10.12.128.249:2100-2104,2106-2110, 1 thread: 10.12.128.249:2100")
	udpSrcAddr = flag.String("udp-src-addr", "10.12.128.249:3100-3107",
		"Source IP:PORT for originating UDP packets, to enable threading specify a port range\n"+
			"IE 10 threads split: 10.12.128.249:3100-3104,3106-3110, 1 thread: 10.12.128.249:3100")
	//udpDstMac  = flag.String("udp-dst-mac", "6c:3b:6b:ed:78:14", "Target MAC for UDP packet (only needed using raw)")
	//udpSrcMac  = flag.String("udp-src-mac", "00:0c:29:69:bd:3d", "Source MAC for UDP packet (only needed using raw)")
	resend         = flag.Duration("resend-delay", 1*time.Second, "Time between first transmit and second, set to 0s to disable.")
	maxConnections = flag.Int("max-http-sessions", 20, "Limit the number of allowed incoming HTTP connections")

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
)

func main() {
	service_flags()
	listen_flags()
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
		ReadTimeout:    10 * time.Hour,
		WriteTimeout:   10 * time.Hour,
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
		if close {
			workers <- wk
		}
	}()
	if *verbose {
		fmt.Println("using connection:", wk.conn)
	}

	// Make sure the client chain is added to attributes, 1 being the closest
	updateChain(f, r, "HTTP-UDP")

	// Prepare to send initial sizing packet for allocation of memory remotely
	buf := bufPool.Get().(*bytes.Buffer)
	id, _ := uuid.Parse(f.Attrs.Get("uuid"))
	hdr := &ffHeader{
		UUID:   id,
		Size:   uint32(f.HeaderSize()) + uint32(f.Size),
		Offset: 0,
		MTU:    uint16(maxPayloadSize) - uint16(ffHeaderSize),
	}
	//fmt.Println("sizes", flowfile.HeaderSize(f), f.Size, uint32(flowfile.HeaderSize(f))+uint32(f.Size), "ff:", f)

	// Write out the initial header
	binary.Write(buf, binary.BigEndian, hdr)
	toCopy := maxPayloadSize - int(ffHeaderSize)
	wk.conn.WriteTo(buf.Bytes(), wk.dst) // Send an empty payload

	// Flatten directory for ease of viewing
	dir := filepath.Clean(f.Attrs.Get("path"))

	filename := f.Attrs.Get("filename")

	if id := f.Attrs.Get("fragment.index"); id != "" {
		i, _ := strconv.Atoi(id)
		fmt.Printf("  Sending segment %d of %s of %s for %s\n", i,
			f.Attrs.Get("fragment.count"), path.Join(dir, filename), r.RemoteAddr)
	} else {
		fmt.Printf("  Sending file %s for %s\n", path.Join(dir, filename), r.RemoteAddr)
	}

	if *verbose {
		adat, _ := json.Marshal(f.Attrs)
		fmt.Printf("incoming: %s\n", adat)
	}

	// Wrap the reader in a buffer so partial reads will be concatinated
	//rdr := bufio.NewReader(f.EncodedReader())
	rdr := f.EncodedReader()

	//_ = hdr

	// Bring the entire send into memory to verify checksum before forwarding
	//_, err = io.Copy(
	//	buf, //&ffHeaderWriter{h: hdr, w: buf, mtu: uint32(maxPayloadSize - ffHeaderSize)},
	//	f.EncodedReader())
	var n int64
	//cp := make([]byte, toCopy)
	//fmt.Println("tocopy", toCopy)
	for err == nil {
		n, err = io.CopyN(buf, rdr, int64(toCopy))
		//fmt.Println("len", buf.Len())
		if int(n) == toCopy {
			hdr.Offset = hdr.Offset + uint32(toCopy)
			binary.Write(buf, binary.BigEndian, hdr)
		}
		//buf.Write(cp[:n])
	}

	//fmt.Printf("buf: %#v\n", string(buf.Bytes()))

	// Verify the checksum
	err = f.Verify()
	switch err {
	case flowfile.ErrorChecksumMissing:
		if *verbose && f.Size > 0 {
			log.Println("    No checksum found for", filename)
		}
	case nil:
		if *verbose && f.Size > 0 {
			log.Println("    Checksum passed for", filename)
		}
	default:
		log.Println("    Checksum failed for", filename, f.VerifyDetails(), buf.Len())
		return
	}

	bufBytes := buf.Bytes()
	b := bufBytes

	// Write out the payload
	for len(b) > maxPayloadSize {
		<-wk.throttler.C
		wk.conn.WriteTo(b[:maxPayloadSize], wk.dst)
		//fmt.Printf("tocopy: %q\n", string(b[:100]))
		b = b[maxPayloadSize:]
	}

	<-wk.throttler.C
	if _, err = wk.conn.WriteTo(b, wk.dst); err != nil {
		return
	}

	close = false // prevent the connection return to pool while writes are happening
	go func() {   // spawn child thread to do the send so as to release the parent
		defer func() {
			workers <- wk
		}()
		if *resend > 0 {
			if *debug {
				fmt.Println("  waiting to do resend", filename, wk.conn, *resend)
			}
			// Hold off, then do it all again
			time.Sleep(*resend)

			if *debug {
				fmt.Println("  resending on same conn", filename, wk.conn)
			}
			b = bufBytes

			// Write out the payload
			for len(b) > maxPayloadSize {
				<-wk.throttler.C
				wk.conn.WriteTo(b[:maxPayloadSize], wk.dst)
				b = b[maxPayloadSize:]
			}

			// Last packet
			<-wk.throttler.C
			if _, err = wk.conn.WriteTo(b, wk.dst); err != nil {
				return
			}
		}
	}()
	return
}

type worker struct {
	conn      *net.UDPConn
	dst       *net.UDPAddr
	throttler *iothrottler.Limit
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

var (
	handle *pcap.Handle
	eth    layers.Ethernet
	//lo             layers.Loopback
	ip      layers.IPv4
	udp     layers.UDP
	options gopacket.SerializeOptions

	UDP_HEADER_LEN = 8
)
