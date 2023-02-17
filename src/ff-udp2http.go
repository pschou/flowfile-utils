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
	"os"
	"sync"

	"github.com/docker/go-units"
	"github.com/pschou/go-flowfile"
)

var (
	about = `FlowFile UDP -to-> HTTP

This utility is intended to take input via UDP pass all FlowFiles to a UDP
endpoint after verifying checksums.  A chain of custody is maintained by adding
an action field with "UDP2HTTP" value.`

	udpDstAddr = flag.String("udp-dst-ip", ":2100-2200", "Local target IP:PORT for UDP packet")
	mtu        = flag.Int("mtu", 1400, "MTU payload size for pre-allocating memory")
	noChecksum = flag.Bool("no-checksums", false, "Ignore doing checksum checks")
	hs         *flowfile.HTTPTransaction

	dst            *net.UDPAddr
	maxPayloadSize = 1280
)

func main() {
	service_flags()
	sender_flags()
	parse()
	var err error

	maxPayloadSize = *mtu - 28 // IPv4 Header
	//maxPayloadSize = *mtu - 48 // IPv6 Header

	// Connect to the destination to prepare to send files
	log.Println("Creating sender,", *url)

	// Create a HTTP Transaction with target URL
	if hs, err = flowfile.NewHTTPTransaction(*url, tlsConfig); err != nil {
		log.Fatal(err)
	}

	log.Println("Listening on UDP", *udpDstAddr)

	setupUDP()
}

func handle(conn *net.UDPConn) {
	var (
		fileBufs [][]byte
		UUID     []byte
		//toFlush  chan []byte

		dat []byte

		done bool
		n    int
		//addr         *net.UDPAddr
		idx          uint32
		count, total uint32
		hdr          ffHeader
		err          error
	)

	//fmt.Println("Allocating listener on", conn)

	// Allocate a buf if we don't have one
	dat = make([]byte, maxPayloadSize)
	for {
		// Read from UDP connection
		if *debug && count%1000 == 0 {
			fmt.Println(conn, "reading packet", count, total)
		}
		if n, _, err = conn.ReadFromUDP(dat); err != nil {
			log.Printf("Error receiving packet", err)
			continue
		}

		//dat = dat[:n] // Set the slice end
		if *debug && count%1000 == 0 {
			fmt.Printf("raw %d: %q\n", n, string(dat[:20])) // Debug the raw packet
		}

		binary.Read(bytes.NewReader(dat), binary.BigEndian, &hdr)

		if hdr.MTU < 100 { // Invalid packet
			continue
		}

		//fmt.Printf("compare %02x == %02x, %v done %v\n", hdr.UUID[:], UUID, bytes.Equal(hdr.UUID[:], UUID), done)
		if !bytes.Equal(hdr.UUID[:], UUID) {
			if len(fileBufs) > 0 && !done {
				//fmt.Println("calling go process with flush", hdr, fileBufs, false)
				done = true
				go process(hdr, fileBufs, &done)
				count, dat = 0, nil
			}

			//dat = <-byteChan
			//toFlush <- dat
			count, total, done = 0, (hdr.Size-1)/uint32(hdr.MTU)+1, false
			//mid = total +total/2
			//for i := uint32(0); i < total; i++ {
			fileBufs = make([][]byte, total)
			//}
			UUID = hdr.UUID[:]
		}

		if done { // short circuit for we are done
			continue
		}

		idx = (hdr.Offset + 1) / uint32(hdr.MTU)
		//fmt.Println("offset", hdr.Offset, "idx", idx, "total", total, "count", count, "n", n, "hdrSize", ffHeaderSize, "mtu", hdr.MTU)
		if idx < total && n > int(ffHeaderSize) {
			// Flip pointers!
			fileBufs[idx], dat = dat, fileBufs[idx]
			if len(dat) == 0 {
				// allocate new!
				dat = make([]byte, maxPayloadSize)
			}
			//fmt.Println("assigning buf to", idx, len(fileBufs))
			count++
			//fmt.Println("assignment has", fileBufs[idx], buf)
			if count >= total {
				if (count%total)%1000 == 0 {
					//fmt.Println("calling go process", hdr, fileBufs, &done)
					go process(hdr, fileBufs, &done)
					//fmt.Println("returned from go process", &done)
				}
			}
			continue
		}
	}
}

type rawFF struct {
	i, size, mtu uint32
	cur          []byte
	dat          [][]byte
}

func (rf *rawFF) next() error {
	//fmt.Printf("next %#v\n", rf)
	if rf.i < uint32(len(rf.dat)) {
		rf.cur = rf.dat[rf.i]
		rf.i++
		//fmt.Println("mtu", rf.mtu, "rf.i", rf.i, "rf.size", rf.size, "math", (rf.size%rf.mtu)+ffHeaderSize)
		if rf.mtu*rf.i < rf.size {
			rf.cur = rf.cur[ffHeaderSize : rf.mtu+ffHeaderSize]
			//fmt.Printf("cur %q\n", string(rf.cur))
		} else {
			rf.cur = rf.cur[ffHeaderSize : (rf.size%rf.mtu)+ffHeaderSize]
		}
		//fmt.Printf("next return %#v\n", rf)
		return nil
	}
	return io.EOF
}

func (rf *rawFF) Read(p []byte) (n int, err error) {
	//fmt.Printf("read called %#v\n", rf)
	if len(rf.cur) == 0 {
		err = rf.next()
	}
	for err == nil && len(p) > 0 {
		cp := copy(p, rf.cur)
		rf.cur = rf.cur[cp:]
		p, n = p[cp:], n+cp
		if len(rf.cur) == 0 {
			err = rf.next()
		}
	}
	return
}

func process(hdr ffHeader, fileBufs [][]byte, done *bool) {
	// Verify we have all the sections before we send
	if len(fileBufs) > 0 {
		for _, fb := range fileBufs {
			if len(fb) == 0 {
				//fmt.Println("found empty,", i)
				return
			}
		}
	}
	//log.Printf("making scanner process: %#v\n", *feed)

	/*var buf bytes.Buffer
	buf.ReadFrom(feed)
	fmt.Printf("dat: %q\n", buf.Bytes())*/
	if !*noChecksum && hdr.Size > 0 {
		if *debug {
			fmt.Println("doing checksum...")
		}
		feed := &rawFF{size: hdr.Size, mtu: uint32(hdr.MTU), dat: fileBufs}
		scn := flowfile.NewScanner(feed)
		for scn.Scan() {
			f := scn.File()
			if *debug {
				fmt.Println("doing checksum on", f.Attrs.Get("filename"))
			}
			io.Copy(io.Discard, f)
			verr := f.Verify()
			if *verbose && verr != nil {
				log.Println("Checksum failed", verr, f.VerifyDetails())
				return
			}
			if *verbose {
				fmt.Println("checksum passed for", f.Attrs.Get("filename"))
			}
		}
		*done = true
	}

	feed := &rawFF{size: hdr.Size, mtu: uint32(hdr.MTU), dat: fileBufs}
	scn := flowfile.NewScanner(feed)
	for scn.Scan() {
		if *debug {
			fmt.Println("Reading file")
		}
		f := scn.File()
		updateChain(f, nil, "UDP2HTTP")

		if *verbose {
			adat, _ := json.Marshal(f.Attrs)
			fmt.Printf("    %s\n", adat)
		}

		log.Printf(" sending %s (%s)", f.Attrs.Get("filename"), units.HumanSize(float64(f.Size)))
		if err := hs.Send(f); err == nil {
			if *debug {
				fmt.Println("file sent")
			}
			*done = true
		}
	}
	if *verbose {
		if err := scn.Err(); err != nil {
			log.Println("Scanner error:", err)
		}
	}
}

func setupUDP() {
	host, dstPortSpec, err := net.SplitHostPort(*udpDstAddr)
	if err != nil {
		log.Fatal("Error splitting host ports", err)
	}
	dstPorts := hypenRange(dstPortSpec)

	wg := sync.WaitGroup{}
	for _, port := range dstPorts {
		dst, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, fmt.Sprintf("%d", port)))
		if *debug {
			fmt.Printf("dst udpSrv: %#v\n", dst)
		}
		if err != nil {
			println("ResolveUDPAddr failed:", err.Error())
			os.Exit(1)
		}

		conn, err := net.ListenUDP("udp", dst)
		if err != nil {
			log.Fatal("Listen UDP failed:", err)
		}
		wg.Add(1)
		go func() {
			handle(conn)
			wg.Done()
		}()
	}
	wg.Wait()
}
