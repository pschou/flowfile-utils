package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"hash"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pschou/go-bunit"
	"github.com/pschou/go-flowfile"
	"github.com/pschou/go-memdiskbuf"
	"github.com/pschou/go-tempfile"
)

var (
	about = `FlowFile UDP -to-> HTTP

This utility is intended to take input via UDP pass all FlowFiles to a UDP
endpoint after verifying checksums.  A chain of custody is maintained by adding
an action field with "UDP-TO-HTTP" value.`

	udpDstAddr = flag.String("udp-dst-addr", ":12100-12199", "Local target IP:PORT for UDP packet")
	mtu        = flag.Int("mtu", 1500, "MTU payload size for pre-allocating memory")
	udpBufSize = flag.String("udp-buf", "100kB", "Set read buffer size, note: this is multiplied by the number of listening ports in memory usage")
	noChecksum = flag.Bool("no-checksums", false, "Ignore doing checksum checks")
	hs         *flowfile.HTTPTransaction

	dst            *net.UDPAddr
	maxPayloadSize = 1280

	metrics = flowfile.NewMetrics()
)

func main() {
	service_flags()
	sender_flags()
	temp_flags()
	metrics_flags(true)
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
	hs.RetryCount = 3
	hs.RetryDelay = time.Minute

	send_metrics("UDP-TO-HTTP", func(f *flowfile.File) { hs.Send(f) }, metrics)

	log.Println("Listening on UDP", *udpDstAddr)

	setupUDP()
	doWork()
}

func setupUDP() {
	host, dstPortSpec, err := net.SplitHostPort(*udpDstAddr)
	if err != nil {
		log.Fatal("Error splitting host ports", err)
	}
	dstPorts := hypenRange(dstPortSpec)

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
		conn.SetReadBuffer(int(bunit.MustParseBytes(*udpBufSize).Int64()))
		go func() {
			handle(conn)
		}()
	}
}

type workUnit struct {
	hdr        ffHeader
	seenChunks []byte

	ip   net.IP
	port int

	wab         *memdiskbuf.WriterAtBuf
	fh          *os.File
	tmpfilename string
	attrs       flowfile.Attributes
	size        uint64
	hash        hash.Hash

	total int
	noBuf bool
}

var workChan = make(chan *workUnit, 10000)

// Worker unit for sending files
func doWork() {
	for {
		job := <-workChan
		metrics.MetricsThreadsQueued, metrics.MetricsThreadsActive =
			metrics.MetricsThreadsQueued-1, metrics.MetricsThreadsActive+1
		go func(job *workUnit) {
			defer func() {
				metrics.MetricsThreadsActive, metrics.MetricsThreadsTerminated =
					metrics.MetricsThreadsActive-1, metrics.MetricsThreadsTerminated+1
				// When this thread terminates, make sure files are cleared out
				job.fh.Close()
				tempfile.Remove(job.tmpfilename)
			}()

			// Do verifications
			if job.size > 0 { // When the payload has content, do checksums
				var verifyErr = errors.New("No checksum done")
				if !job.noBuf {
					if err := job.wab.Flush(); err != nil || job.hash == nil {
						job.noBuf = true
					} else {
						f := flowfile.File{Attrs: job.attrs}
						verifyErr = f.VerifyHash(job.hash)
					}
				}

				if job.noBuf {
					if len(job.attrs) == 0 {
						job.fh.Seek(0, io.SeekStart)
						job.attrs.ReadFrom(job.fh)
					}

					// Verification even when the buffer is missed
					job.hash = job.attrs.NewChecksumHash()
					if job.hash != nil {
						job.fh.Seek(0, io.SeekStart)
						io.Copy(job.hash, job.fh) // Do the checksum on the file
						f := flowfile.File{Attrs: job.attrs}
						verifyErr = f.VerifyHash(job.hash)
					}
				}

				if verifyErr != nil {
					if *verbose {
						log.Println("Checksum failed for job", verifyErr)
					}
					return
				}
				if *verbose {
					log.Println("Checksum passed for job")
				}
			} else {
				job.wab.Flush()
			}

			// add to metrics counter
			metrics.BucketCounter(int64(job.size))

			// go to the start and send the payload
			job.fh.Seek(0, io.SeekStart)
			scn := flowfile.NewScanner(job.fh)
			for scn.Scan() {
				if *debug {
					fmt.Println("Reading file")
				}
				f := scn.File()
				updateChain(f, nil, "UDP-TO-HTTP")

				if *verbose {
					fmt.Println("    ", f.Attrs)
				}

				log.Printf(" sending %s (%v)", f.Attrs.Get("filename"), bunit.NewBytes(int64(f.Size)))
				if err := hs.Send(f); err == nil {
					if *debug {
						fmt.Println("file sent")
					}
				}
			}
		}(job)
	}
}

var rcvBufPool = sync.Pool{
	New: func() any {
		return make([]byte, maxPayloadSize)
	},
}

// Handle connections to the UDP port and parses the packets as they come in.
func handle(conn *net.UDPConn) {
	dat := rcvBufPool.Get().([]byte)
	defer rcvBufPool.Put(dat)

	var (
		UUID uuid.UUID
		addr *net.UDPAddr
		n    int
		job  *workUnit
		err  error
		done bool
	)

	for {
		if n, addr, err = conn.ReadFromUDP(dat); err != nil {
			log.Printf("Error receiving packet", err)
			continue
		}

		// Print out packet for debugging
		/*if *debug {
			n := 30
			if len(dat) < 30 {
				n = len(dat)
			}
			fmt.Printf("raw %d: %q %s\n", n, string(dat[:n])) // Debug the raw packet
		}*/

		// Parse the incoming packet's header for position and UUID info
		var hdr ffHeader
		binary.Read(bytes.NewReader(dat), binary.BigEndian, &hdr)
		if n < int(ffHeaderSize) || hdr.MTU < 100 { // Invalid packet
			continue
		}

		// If we have a new UUID
		if !bytes.Equal(hdr.UUID[:], UUID[:]) {
			if job != nil {
				job.wab.Reset(nil)
				fmt.Println("  Could not reconstruct UUID:", UUID)
				job.fh.Close()
				tempfile.Remove(job.tmpfilename)
			}
			// Create a temporary file for udp writes
			tmpfilename := tempfile.New()
			var fh *os.File
			if fh, err = os.OpenFile(tmpfilename, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666); err != nil {
				log.Fatal("Unable to create working file", err)
			}

			// Reset for new file
			total := int(hdr.Size-1)/int(hdr.MTU) + 1
			done = false

			// Populate a new job unit
			job = &workUnit{
				hdr:   hdr,
				total: total,

				fh:          fh,
				wab:         memdiskbuf.NewWriterAtBuf(fh, 32<<20),
				tmpfilename: tmpfilename,

				ip:         addr.IP,
				port:       addr.Port,
				seenChunks: make([]byte, total),
			}

			{
				// We would like to know the checksum value early (if possible) so as
				// to get a confirmation that the file is good to send!  So use the
				// StreamFunc to parse out the header than checksum the payload as it
				// is being written to disk.
				var hdrBuf bytes.Buffer
				var parsed bool
				var sf_job *workUnit
				sf_job = job
				job.wab.StreamFunc = func(p []byte) {
					if sf_job.hash != nil {
						sf_job.hash.Write(p)
					} else if !parsed {
						hdrBuf.Write(p)

						// Parse out the FlowFile header to get the checksum attribute
						// and start checksumming the stream.
						b := hdrBuf.Bytes()
						if err = sf_job.attrs.UnmarshalBinary(b); err == nil {
							// found ff, make an empty flowfile.File just to use verify functions
							f := flowfile.File{Attrs: sf_job.attrs}
							if hdrSize := f.HeaderSize(); hdrBuf.Len() >= hdrSize+8 {
								// found ff and right sized header
								parsed = true
								if *verbose {
									log.Println("Attrs:", sf_job.attrs)
								}
								sf_job.size = binary.BigEndian.Uint64(b[hdrSize : hdrSize+8])

								// making hash
								sf_job.hash = sf_job.attrs.NewChecksumHash()
								if sf_job.hash != nil {
									// copy over header buf
									io.CopyN(io.Discard, &hdrBuf, int64(f.HeaderSize()))
									io.Copy(sf_job.hash, &hdrBuf)
								}
								hdrBuf.Reset()
							}

						} else if err == flowfile.ErrorNoFlowFileHeader || hdrBuf.Len() > 64<<10 {
							// File is an invalid FlowFile, give up early
							parsed = true
						}
					}
				}
			}
			//if *verbose {
			//	fmt.Printf("creating job %v\n", job.hdr)
			//}

			copy(UUID[:], hdr.UUID[:])
		}

		// The current file is done, do nothing
		if done || n == int(ffHeaderSize) { // short circuit for we are done
			continue
		}

		// Determine the offset
		idx := int(hdr.Offset+1) / int(hdr.MTU)
		if idx < job.total {
			if job.seenChunks[idx] == 1 { // Duplicate packet, ignore
				continue
			}
			job.seenChunks[idx] = 1

			// Send to WriteAt
			if !job.noBuf {
				// Try the buffered WriteAt first
				if _, err = job.wab.WriteAt(dat[ffHeaderSize:n], int64(hdr.Offset)); err != nil {
					// Something bad happened, so flush it to disk and write all
					job.fh.Truncate(int64(hdr.Size)) // Build out the file to the right size
					job.wab.FlushAll()               // Wright all we have to disk
					job.fh.Truncate(int64(hdr.Size)) // Ensure we are at the right size
					job.noBuf = true                 // Prevent any further use of this buffer
				}
			}
			if job.noBuf { // Write without buffering
				//if *debug {
				//	fmt.Println("fall back to nobuf")
				//}
				job.fh.WriteAt(dat[ffHeaderSize:n], int64(hdr.Offset))
			}

			done = true
			for _, ck := range job.seenChunks {
				if ck == 0 {
					done = false
					break
				}
			}
			if done {
				//if *verbose {
				//	fmt.Printf("sending job %v\n", job.hdr)
				//}
				metrics.MetricsThreadsQueued++
				workChan <- job
				job = nil
			}
		}
	}
}
