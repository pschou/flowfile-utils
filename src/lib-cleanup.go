package main

import (
	"log"
	"os"
	"os/signal"

	"github.com/pschou/go-memdiskbuf"
	"github.com/pschou/go-tempfile"
)

func init() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			if *debug {
				log.Println("Caught signal", sig)
			}
			tempfile.Cleanup()
			memdiskbuf.Cleanup()
			os.Exit(1)
		}
	}()
}
