package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/pschou/go-flowfile"
)

var (
	version   = ""
	usage     = "[options]"
	verbose   = flag.Bool("verbose", false, "Turn on verbosity")
	debug     = flag.Bool("debug", false, "Turn on debug in FlowFile library")
	enableTLS = new(bool)
)

func init() {
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
