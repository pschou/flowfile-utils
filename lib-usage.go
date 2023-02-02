package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var version = ""

func init() {
	flag.Usage = func() {
		lines := strings.SplitN(about, "\n", 2)
		fmt.Fprintf(os.Stderr, "%s (github.com/pschou/flowfile-utils, version: %s)\n%s\n\nUsage of %s:\n", lines[0], version, lines[1], os.Args[0])

		flag.PrintDefaults()
	}
}
