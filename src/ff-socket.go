package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pschou/go-flowfile"
	shaker "github.com/tevino/tcp-shaker"
)

var about = `FF-Socket

This utility is intended to send and receive input over a FlowFile compatible
port and drop all FlowFiles into directory along with associated attributes
which can then be unstaged using the FF-Unstager.

Operations can happen in either FORWARD mode or CONNECT mode.  With FORWARD mode
a TCP port is opened and accepts incoming connections forwarding the packets
via FlowFiles.  With CONNECT mode FlowFile packets are received and translated
into a TCP session.

For example:
listen$ ./ff-socket incoming-at :2222            # Either :PORT or IP:PORT
target$ ./ff-socket forward-to myserver.com:2222 # Final destination for TCP`

var (
	hs        *flowfile.HTTPTransaction
	target    string
	tcpListen *net.TCPListener

	socketMap   = make(map[string]*session)
	socketMutex sync.Mutex
	metrics     = flowfile.NewMetrics()

	hash = flag.String("hash", "SHA1", "Hash to use in checksum value")
)

type session struct {
	rw     io.ReadWriteCloser
	tn, rn uint64
	buf    map[uint64]*packet
	seen   time.Time
	mutex  sync.Mutex
}

type packet struct {
	data []byte
	seen time.Time
	op   string
}

func main() {
	listen_flags()
	sender_flags()
	metrics_flags(true)
	usage = "[options] [forward-to IP:PORT] / [incoming-at IP:PORT]"
	parse()

	if len(flag.Args()) != 2 {
		flag.Usage()
		return
	}

	// Connect to the destination to prepare to send files
	log.Println("Creating sender,", *url)

	// Create a HTTP Transaction handle with target URL
	hs = flowfile.NewHTTPTransactionNoHandshake(*url, tlsConfig)

	var mode string
	switch flag.Arg(0) {
	case "forward-to", "forward", "f", "fwd":
		mode = "DST"
		setupSendTCP(flag.Arg(1))
	case "incoming-at", "incoming", "i", "in":
		mode = "SRC"
		setupListenTCP(flag.Arg(1))
	default:
		fmt.Printf("Unknown action %q", flag.Arg(0))
		os.Exit(1)
	}

	// Configure the go HTTP server
	server := &http.Server{
		Addr:           *listen,
		TLSConfig:      tlsConfig,
		ReadTimeout:    10 * time.Hour,
		WriteTimeout:   10 * time.Hour,
		MaxHeaderBytes: 1 << 20,
		ConnState:      ConnStateEvent,
	}

	// Setting up the flow file receiver
	ffReceiver := flowfile.NewHTTPFileReceiver(post)
	http.Handle(*listenPath, ffReceiver)

	// Setup a timer to update the maximums and minimums for the sender
	handshaker(hs, ffReceiver)
	send_metrics("SOCKET-"+mode, func(f *flowfile.File) { hs.Send(f) }, metrics)

	// Open the local port to listen for incoming connections
	if *enableTLS {
		log.Println("Listening with HTTPS on", *listen, "at", *listenPath)
		log.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
	} else {
		log.Println("Listening with HTTP on", *listen, "at", *listenPath)
		log.Fatal(server.ListenAndServe())
	}
}

func setupSendTCP(dst string) {
	// Do a quick handshake test, a two way handshake, leaving off the third part
	c := shaker.NewChecker()
	ctx, stopChecker := context.WithCancel(context.Background())
	defer stopChecker()
	go func() {
		if err := c.CheckingLoop(ctx); err != nil {
			fmt.Println("checking loop stopped due to fatal error: ", err)
		}
	}()
	<-c.WaitReady()
	timeout := time.Second * 10
	err := c.CheckAddr(dst, timeout)
	if err != nil {
		log.Fatal("Could not test connect to endpoint:", dst, err)
	}
	target = flag.Arg(1)
}

func setupListenTCP(listen string) {
	// Attempt to open local port for listening
	addr, err := net.ResolveTCPAddr("tcp", listen)
	if err != nil {
		log.Fatal("Invalid address", err)
	}
	if tcpListen, err = net.ListenTCP("tcp", addr); err != nil {
		log.Fatal("Could not listen on address:", addr)
	}
	go func(tcpListen *net.TCPListener) {
		for {
			conn, err := tcpListen.AcceptTCP()
			if err != nil {
				log.Fatal(err)
			}
			ses := &session{
				rw:   conn,
				buf:  make(map[uint64]*packet),
				seen: time.Now(),
			}
			id := uuid.New().String()
			go func(id string, ses *session, addr string) {
				host, port, _ := net.SplitHostPort(addr)
				metrics.MetricsThreadsActive++
				defer func() {
					metrics.MetricsThreadsActive, metrics.MetricsThreadsTerminated =
						metrics.MetricsThreadsActive-1, metrics.MetricsThreadsTerminated+1
				}()
				buf := make([]byte, 32<<10)
				var n int
				var err error
				for err == nil {
					n, err = ses.rw.Read(buf)
					if n == 0 && err == nil {
						// Nothing was read, reader timed out, try again
						time.Sleep(time.Second)
						continue
					}

					// Determine next packet number
					ses.tn = (ses.tn + 1) % (1 << 63)
					f := flowfile.New(bytes.NewReader(buf[:n]), int64(n))
					f.Attrs.Set("socketId", id)
					f.Attrs.Set("socketN", fmt.Sprintf("%d", ses.tn))
					updateChain(f, nil, "SOCKET-SRC")
					if err != nil {
						f.Attrs.Set("socketOp", "EOF")
						socketMutex.Lock()
						delete(socketMap, id)
						socketMutex.Unlock()
						runtime.GC()
					}
					f.AddChecksum(*hash)
					f.Attrs.Set("custodyChain.0.source.host", host)
					f.Attrs.Set("custodyChain.0.source.port", port)
					hs.Send(f)
				}
			}(id, ses, conn.RemoteAddr().String())
			socketMutex.Lock()
			socketMap[id] = ses
			socketMutex.Unlock()
		}
	}(tcpListen)
}

// Connect to a new TCP end point when a target is specified
func newSession(id string) (ses *session, err error) {
	var conn net.Conn
	if conn, err = net.Dial("tcp", target); err != nil {
		log.Println("error making new connection", target, err)
		return
	}
	ses = &session{
		rw:   conn,
		buf:  make(map[uint64]*packet),
		seen: time.Now(),
	}
	//id := uuid.New().String()
	go func(id string, ses *session) {
		metrics.MetricsThreadsActive++
		defer func() {
			metrics.MetricsThreadsActive, metrics.MetricsThreadsTerminated =
				metrics.MetricsThreadsActive-1, metrics.MetricsThreadsTerminated+1
		}()
		buf := make([]byte, 32<<10)
		var n int
		var err error
		for err == nil {
			n, err = ses.rw.Read(buf)
			if n == 0 && err == nil {
				// Nothing was read, reader timed out, try again
				time.Sleep(time.Second)
				continue
			}

			// Determine next packet number
			ses.tn = (ses.tn + 1) % (1 << 63)
			f := flowfile.New(bytes.NewReader(buf[:n]), int64(n))
			f.Attrs.Set("socketId", id)
			f.Attrs.Set("socketN", fmt.Sprintf("%d", ses.tn))
			updateChain(f, nil, "SOCKET-DST")
			if err != nil {
				f.Attrs.Set("socketOp", "EOF")
				socketMutex.Lock()
				delete(socketMap, id)
				socketMutex.Unlock()
				runtime.GC()
			}
			f.AddChecksum(*hash)
			hs.Send(f)
		}
	}(id, ses)
	socketMutex.Lock()
	socketMap[id] = ses
	socketMutex.Unlock()
	return
}

// Post handles every flowfile that is posted into the diode
func post(f *flowfile.File, w http.ResponseWriter, r *http.Request) (err error) {
	var ses *session
	id := f.Attrs.Get("socketId")

	if id == "" {
		return errors.New("Missing socketId")
	}

	socketMutex.Lock()
	ses, ok := socketMap[id]
	socketMutex.Unlock()

	if !ok {
		if target != "" {
			// Make new connection
			if ses, err = newSession(id); err != nil {
				return
			}
		} else {
			// No target means no new connection
			return errors.New("Unknown socketId")
		}
	}

	n, err := strconv.ParseUint(f.Attrs.Get("socketN"), 10, 64)
	if err != nil {
		return err
	}

	// Determine next packet number
	next := (ses.rn + 1) % (1 << 63)

	if next != n { // Unordered packet found
		// Write packet to buffer and return
		ses.mutex.Lock()
		buf := &bytes.Buffer{}
		io.Copy(buf, f)
		ses.buf[n] = &packet{
			data: buf.Bytes(),
			seen: time.Now(),
			op:   f.Attrs.Get("socketOp"),
		}
		ses.mutex.Unlock()
		return nil
	}

	// If we got the next packet, go ahead and write it out
	ses.rn, next, ses.seen = next, (next+1)%(1<<63), time.Now()
	io.Copy(ses.rw, f)

	// Test if the socket should be closed
	if op := f.Attrs.Get("socketOp"); op == "EOF" {
		ses.rw.Close()
		return nil
	}

	ses.mutex.Lock()
	defer ses.mutex.Unlock()

	// Play out anything up to this point
	for pkt, ok := ses.buf[next]; ok; pkt, ok = ses.buf[next] {
		io.Copy(ses.rw, bytes.NewReader(pkt.data))
		delete(ses.buf, next)
		ses.rn, next, ses.seen = next, (next+1)%(1<<63), time.Now()
		if pkt.op == "EOF" {
			ses.rw.Close()
			socketMutex.Lock()
			delete(socketMap, id)
			socketMutex.Unlock()
			runtime.GC()
			return nil
		}
	}
	return
}
