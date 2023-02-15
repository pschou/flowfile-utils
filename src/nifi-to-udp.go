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
)

var (
	about = `NiFi -to-> UDP

This utility is intended to take input over a NiFi compatible port and pass all
FlowFiles to a UDP endpoint after verifying checksums.  A chain of custody is
maintained by adding an action field with "NIFI-UDP" value.

Note: The port range used in the source UDP address directly affect the number
of concurrent sessions, and as payloads are buffered in memory (to do the
checksum) the memory bloat can be upwards on the order of NUM_PORTS *
MAX_PAYLOAD.  Please choose wisely.

The resend-delay will add latency (by delaying new connections until second
send is complete) but will add error resilience in the transfer.  In other
words, shortening the delay will likely mean more errors, while increaing will
slow down the number of accepted HTTP connections upstream.`

	//noChecksum = flag.Bool("no-checksums", false, "Ignore doing NiFi checksum checks")
	//udpSrcInt  = flag.String("udp-src-int", "ens192", "Interface where to send UDP packets")
	udpDstAddr = flag.String("udp-dst-addr", "10.12.128.249:2100-2200", "Target IP:PORT for UDP packet")
	udpSrcAddr = flag.String("udp-src-addr", "10.12.128.249:3100-3200", "Source IP:PORT for UDP packet")
	//udpDstMac  = flag.String("udp-dst-mac", "6c:3b:6b:ed:78:14", "Target MAC for UDP packet (only needed using raw)")
	//udpSrcMac  = flag.String("udp-src-mac", "00:0c:29:69:bd:3d", "Source MAC for UDP packet (only needed using raw)")
	resend      = flag.Duration("resend-delay", 1*time.Second, "Time between first transmit and second, set to 0s to disable.")
	throttle    = flag.Duration("throttle", 600*time.Nanosecond, "Additional seconds per frame\nThis scales up with concurrent connections (set to 0s to disable)")
	throttleGap = flag.Duration("throttle-gap", 60*time.Nanosecond, "Inter-packet gap\nThis is the time added after all packet (set to 0s to disable)")
	//throttleOverhead = flag.Int("throttle-overhead", 64, "Bytes in packet overhead (add to payload size)")
	maxConnections = flag.Int("max-http-sessions", 20, "Limit the number of allowed incoming HTTP connections")
	//throttleScaler         = flag.Float64("throttle-scaler", 3.05, "Scaler applied by the base bandwidth for tuning")

	// Additional math parts
	throttleDelay, throttleConnections time.Duration

	mtu            = flag.Int("mtu", 1200, "Maximum transmit unit")
	maxPayloadSize = 1280

	// TODO: Enable raw packet sending
	//useRaw         = flag.Bool("udp-raw", false, "Use raw UDP gopacket sender")
	ffWriter   *flowfile.Writer
	writerLock sync.Mutex
)

func throttleCalc(inc int64) {
	if tc := throttleConnections + time.Duration(inc); tc <= 1 {
		// update count
		throttleConnections, throttleDelay = tc, *throttle+*throttleGap
	} else {
		throttleConnections, throttleDelay = tc, (tc*(*throttle))+*throttleGap
	}
}

func main() {
	service_flags()
	listen_flags()
	parse()

	maxPayloadSize = *mtu - 28 // IPv4 Header
	//maxPayloadSize = *mtu - 48 // IPv6 Header

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
	conn := <-udpConns
	dst := <-dstPool
	defer func() {
		if close {
			dstPool <- dst
			udpConns <- conn
		}
	}()
	if *verbose {
		fmt.Println("using connection:", conn)
	}

	// Make sure the client chain is added to attributes, 1 being the closest
	updateChain(f, r, "NIFI-UDP")

	// Prepare to send initial sizing packet for allocation of memory remotely
	buf := bufPool.Get().(*bytes.Buffer)
	id, _ := uuid.Parse(f.Attrs.Get("uuid"))
	hdr := &ffHeader{
		UUID:   id,
		Size:   uint32(flowfile.HeaderSize(f)) + uint32(f.Size),
		Offset: 0,
		MTU:    uint16(maxPayloadSize) - uint16(ffHeaderSize),
	}
	//fmt.Println("sizes", flowfile.HeaderSize(f), f.Size, uint32(flowfile.HeaderSize(f))+uint32(f.Size), "ff:", f)

	// Write out the initial header
	binary.Write(buf, binary.BigEndian, hdr)
	toCopy := maxPayloadSize - int(ffHeaderSize)
	conn.WriteTo(buf.Bytes(), dst) // Send an empty payload

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

	close = false // prevent the connection return to pool while writes are happening
	go func() {   // spawn child thread to do the send so as to release the parent
		defer func() {
			dstPool <- dst
			udpConns <- conn
			throttleCalc(-1)
			//fmt.Println("throttleConnections close", throttleConnections)
		}()
		throttleCalc(1)
		//fmt.Println("throttleConnections", throttleConnections)
		bufBytes := buf.Bytes()
		b := bufBytes
		if throttleConnections > 1 {
			// A little more padding on first write to prevent nasty bursts
			time.Sleep(throttleDelay)
		}

		// Write out the payload
		for len(b) > maxPayloadSize {
			time.Sleep(throttleDelay)
			conn.WriteTo(b[:maxPayloadSize], dst)
			//fmt.Printf("tocopy: %q\n", string(b[:100]))
			b = b[maxPayloadSize:]
		}

		// Last packet
		time.Sleep(throttleDelay)
		//fmt.Printf("writing %q\n", b)
		if _, err = conn.WriteTo(b, dst); err != nil {
			return
		}

		if *resend > 0 {
			if *debug {
				fmt.Println("  waiting to do resend", filename, conn, *resend)
			}
			// Hold off, then do it all again
			throttleCalc(-1)
			time.Sleep(*resend)
			throttleCalc(1)
			time.Sleep(throttleDelay)

			if *debug {
				fmt.Println("  resending on same conn", filename, conn)
			}
			b = bufBytes

			// Write out the payload
			for len(b) > maxPayloadSize {
				time.Sleep(throttleDelay)
				conn.WriteTo(b[:maxPayloadSize], dst)
				b = b[maxPayloadSize:]
			}

			// Last packet
			time.Sleep(throttleDelay)
			if _, err = conn.WriteTo(b, dst); err != nil {
				return
			}
		}
		if throttleConnections > 1 {
			// A little more padding on last write to prevent nasty bursts
			time.Sleep(throttleDelay)
		}
	}()
	return
}

var udpConns chan *net.UDPConn
var dstPool chan *net.UDPAddr

func setupUDP() {
	host, srcPortSpec, err := net.SplitHostPort(*udpSrcAddr)
	if err != nil {
		log.Fatal("Error splitting host ports", err)
	}
	srcPorts := hypenRange(srcPortSpec)
	udpConns = make(chan *net.UDPConn, len(srcPorts))

	for _, port := range srcPorts {
		src, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, fmt.Sprintf("%d", port)))
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
		udpConns <- conn
	}

	host, dstPortSpec, err := net.SplitHostPort(*udpDstAddr)
	dstPorts := hypenRange(dstPortSpec)
	if len(dstPorts) != len(srcPorts) {
		log.Fatal("Destination port range count does not match the source port range count")
	}
	dstPool = make(chan *net.UDPAddr, len(dstPorts))

	for _, port := range dstPorts {
		dst, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, fmt.Sprintf("%d", port)))
		if *debug {
			fmt.Printf("dst udpSrv: %#v\n", dst)
		}
		if err != nil {
			println("ResolveUDPAddr failed:", err.Error())
			os.Exit(1)
		}
		dstPool <- dst
	}

	/*{ // timing test
		if *debug {
			fmt.Println("Testing throttle with:", *throttle, int64(*throttle))
		}
		conn := <-udpConns
		dst := <-dstPool
		payload := make([]byte, maxPayloadSize)
		tick1 := time.Now()
		tick2 := time.Now()
		time.Sleep(time.Nanosecond)
		if _, err = conn.WriteTo(payload[1:maxPayloadSize-1], dst); err != nil {
			log.Fatal("Error during test write,", err)
		}
		tock1 := time.Now().Sub(tick2)
		tock2 := time.Now().Sub(tick1)
		*throttle = *throttle - (tock2 - (tock2-tock1)/2 - time.Nanosecond)
		//if *throttle < 0 {
		//	*throttle = 0
		//}
		udpConns <- conn
		dstPool <- dst
		if *debug {
			fmt.Println("New throttle set to:", *throttle, int64(*throttle), int64(tock2), int64(tock1), int64(tock2-(tock2-tock1)/2-time.Nanosecond))
		}
	}*/
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
