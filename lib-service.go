package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/mdlayher/watchdog"
)

var watchdog_max *time.Duration
var initScript, initScriptShell *string

func service_flag() {
	watchdog_max = flag.Duration("watchdog", time.Duration(0), "Trigger a reboot if no connection is seen within this time window\nYou'll neet to make sure you have the watchdog module enabled on the host and kernel.\nDefault is disabled (-watchdog=0s)")
	initScript = flag.String("init-script", "", "Shell script to be called on start\nUsed to manually setup the networking interfaces when this program is called from GRUB")
	initScriptShell = flag.String("init-script-shell", "/bin/bash", "Shell to be used for init script run")

}
func service_init() {
	if *initScript != "" {
		log.Println("  Calling init script", *initScriptShell, *initScript)
		log.Println("----- START", *initScript, "-----")

		cmd := exec.Command(*initScriptShell, *initScript)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		log.Println("----- END", *initScript, "-----")

		if err != nil {
			log.Printf("error %s", err)
		}
	}

	last_connection := time.Now()
	if *watchdog_max > time.Duration(1000) {
		d, err := watchdog.Open()
		if err != nil {
			log.Fatalf("failed to open watchdog: %v", err)
		}
		log.Println("Watchdog setup for interval:", *watchdog_max)

		// Handle control-c / sigterm by closing out the watchdog timer
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			for sig := range sigs {
				log.Printf("captured %v, stopping watchdog...", sig)
				d.Close()
				os.Exit(1)
			}
		}()

		go func() {
			// We purposely double-close the file to ensure that the explicit Close
			// later on also disarms the device as the program exits. Otherwise it's
			// possible we may exit early or with a subtle error and leave the system
			// in a doomed state.
			defer d.Close()

			timeout, err := d.Timeout()
			if err != nil {
				log.Fatalf("failed to fetch watchdog timeout: %v", err)
			}

			interval := 10 * time.Second
			if timeout < interval {
				interval = timeout
			}

			for {
				if time.Now().Sub(last_connection) < *watchdog_max {
					if err := d.Ping(); err != nil {
						log.Printf("failed to ping watchdog: %v", err)
					}
				}

				time.Sleep(interval)
			}

			// Safely disarm the device before exiting.
			if err := d.Close(); err != nil {
				log.Printf("failed to disarm watchdog: %v", err)
			}
		}()
	}
}
