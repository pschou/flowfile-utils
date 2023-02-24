package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/pschou/go-flowfile"
	"github.com/pschou/go-memdiskbuf"
)

var (
	version   = ""
	usage     = "[options]"
	verbose   = new(bool)
	debug     = flag.Bool("debug", false, "Turn on debug in FlowFile library")
	enableTLS = new(bool)
	tmpFolder = ""
)

func temp_flags() {
	flag.StringVar(&tmpFolder, "tmp", "/tmp/", "Where to buffer to disk for large transfers")
}

func makeBuf() *memdiskbuf.Buffer {
	return memdiskbuf.NewBuffer(path.Join(tmpFolder, RandStringBytes(8)), 200<<10, 16<<10)
}

func RandStringBytes(n int) string {
	letterBytes := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

var bufPool = sync.Pool{New: func() any { return makeBuf() }}

func init() {
	rand.Seed(time.Now().UnixNano())
	flag.BoolVar(verbose, "verbose", false, "Turn on verbose")
	flag.BoolVar(verbose, "v", false, "Turn on verbose (shorthand)")
	flag.Usage = func() {
		lines := strings.SplitN(about, "\n", 2)
		fmt.Fprintf(os.Stderr, "%s (github.com/pschou/flowfile-utils, version: %s)\n%s\n\nUsage: %s %s\n",
			lines[0], version, lines[1], os.Args[0], usage)

		flag.PrintDefaults()
	}
}

func parse() {
	flag.Parse()
	if *debug {
		flowfile.Debug = true
	}
	if *enableTLS || strings.HasPrefix(*url, "https") {
		loadTLS()
	}
	loadAttributes(*attributes)
	if isService {
		service_init()
	}
}
