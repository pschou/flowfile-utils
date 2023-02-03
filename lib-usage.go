package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var (
	version = ""
	usage   = "[options]"
	verbose = flag.Bool("verbose", false, "Turn on verbosity")
)

func init() {
	flag.Usage = func() {
		lines := strings.SplitN(about, "\n", 2)
		fmt.Fprintf(os.Stderr, "%s (github.com/pschou/flowfile-utils, version: %s)\n%s\n\nUsage: %s %s\n",
			lines[0], version, lines[1], os.Args[0], usage)

		flag.PrintDefaults()
	}
}
