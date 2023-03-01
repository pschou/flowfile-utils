package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pschou/go-flowfile"
	"github.com/pschou/go-unixmode"
	"github.com/relvacode/iso8601"
)

var about = `FlowFile Receiver

This utility is intended to listen for FlowFiles via HTTP/HTTPS and then parse
these files and drop them to disk for usage elsewhere.`

var (
	basePath    = flag.String("path", "./output/", "Directory in which to place files received")
	script      = flag.String("script", "", "Shell script to be called on successful post")
	scriptShell = flag.String("script-shell", "/bin/bash", "Shell to be used for script run")
	remove      = flag.Bool("rm", false, "Automatically remove file after script has finished")
	hs          *flowfile.HTTPTransaction
)

func main() {
	service_flags()
	listen_max()
	listen_flags()
	parse()

	if len(flag.Args()) != 0 {
		flag.Usage()
		return
	}

	fmt.Println("Output set to", *basePath)

	// Configure the go HTTP server
	server := &http.Server{
		Addr:           *listen,
		TLSConfig:      tlsConfig,
		ReadTimeout:    10 * time.Hour,
		WriteTimeout:   10 * time.Hour,
		MaxHeaderBytes: 1 << 20,
		ConnState:      ConnStateEvent,
	}

	// Setting up the FlowFile receiver
	ffReceiver := flowfile.NewHTTPFileReceiver(post)
	http.Handle(*listenPath, ffReceiver)

	// Setup a timer to update the maximums and minimums for the sender
	handshaker(hs, ffReceiver)

	// Open the local port to listen for incoming connections
	if *enableTLS {
		log.Println("Listening with HTTPS on", *listen, "at", *listenPath)
		log.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
	} else {
		log.Println("Listening with HTTP on", *listen, "at", *listenPath)
		log.Fatal(server.ListenAndServe())
	}
}

func post(f *flowfile.File, w http.ResponseWriter, r *http.Request) (err error) {
	defer func() {
		if err != nil {
			log.Println(err)
		}
	}()

	// Save the flowfile into the base path with the file structure defined by
	// the flowfile attributes.
	dir := filepath.Clean(f.Attrs.Get("path"))
	if strings.HasPrefix(dir, "..") {
		err = fmt.Errorf("Unclean path in FlowFile %q", f.Attrs.Get("path"))
		return
	}
	filename := f.Attrs.Get("filename")
	fp := path.Join(*basePath, dir, filename)
	err = os.MkdirAll(path.Join(*basePath, dir), 0755)
	if err != nil {
		return
	}

	if *verbose {
		adat, _ := json.Marshal(f.Attrs)
		fmt.Printf("  - %s\n", adat)
	}

	var unixMode *unixmode.Mode
	if fm := f.Attrs.Get("file.permissions"); len(fm) >= 9 && runtime.GOOS != "windows" {
		t, _ := unixmode.Parse(fm)
		unixMode = &t
	}

	switch kind := f.Attrs.Get("kind"); kind {
	case "file", "":
		log.Println("  Receiving flowfile", fp, "size", f.Size)

		// Safe off the file
		_, err = f.Save(*basePath)
		if err == nil {
			if id := f.Attrs.Get("fragment.index"); id != "" {
				i, _ := strconv.Atoi(id)
				count, _ := strconv.Atoi(f.Attrs.Get("fragment.count"))
				log.Printf("  Verified segment %d of %d of %s\n", i, count, fp)
				if !updateIndexFile(f.Attrs.Get("fragment.identifier"), fp+".progress", i-1, count) {
					// The file is not complete yet
					return
				}
				if err = f.VerifyParent(fp); err != nil {
					// Verification failed
					return
				}
				log.Println("  Verified segmented file", fp)
				os.Remove(fp + ".progress")
			} else {
				log.Printf("  Verified file %s\n", fp)
			}

			// If a script file is provided, call it
			if *script != "" {
				log.Println("  Calling script", *scriptShell, *script, fp)
				output, err := exec.Command(*scriptShell, *script, fp).Output()
				if *verbose {
					log.Println("----- START", *script, fp, "-----")
					fmt.Println(string(output))
					log.Println("----- END", *script, fp, "-----")
					if err != nil {
						log.Printf("error %s", err)
					}
				}

				// If the removal of the file is requested
				if *remove {
					os.Remove(fp)
					if *verbose {
						log.Printf("  Removed %s\n", fp)
					}
				}
			}
		}
		if unixMode != nil {
			unixmode.Chmod(fp, *unixMode)
		}
		// Update file time from sender
		if mt := f.Attrs.Get("file.lastModifiedTime"); mt != "" {
			if fileTime, err := iso8601.ParseString(mt); err == nil {
				os.Chtimes(fp, fileTime, fileTime)
			}
		}

	case "dir":
		err = os.MkdirAll(fp, 0755)
		if unixMode != nil {
			unixmode.Chmod(fp, *unixMode)
		}
		if *debug {
			fmt.Println("making directory", fp, err)
		}
		// Update file time from sender
		if mt := f.Attrs.Get("file.lastModifiedTime"); mt != "" {
			if fileTime, err := iso8601.ParseString(mt); err == nil {
				os.Chtimes(fp, fileTime, fileTime)
			}
		}
	case "link":
		if target := f.Attrs.Get("target"); target != "" && !strings.HasPrefix(target, "/") {
			cleanedTarget := filepath.Clean(path.Join(dir, target))
			if !strings.HasPrefix(cleanedTarget, "..") {
				if *debug {
					fmt.Println("making link", target, fp)
				}
				err = os.Symlink(target, fp)
				if err != nil {
					if *debug {
						log.Println(err)
					}
					err = nil
				}
			} else if *debug {
				fmt.Println("invalid relative link", target, fp)
			}
		}
	case "metrics":
	default:
		if *verbose {
			log.Println("Cannot accept kind:", kind)
		}
		return fmt.Errorf("Unknown kind %q", kind)
	}

	return
}

var updateIndexMutex sync.Mutex

func updateIndexFile(puuid, f string, idx, count int) bool {
	if idx < 0 || idx >= count || count <= 1 {
		return false
	}
	updateIndexMutex.Lock()
	defer updateIndexMutex.Unlock()

	// If this is a new file, create it and write out current state
	fh, err := os.OpenFile(f, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0662)
	if err == nil {
		defer fh.Close()
		b := make([]byte, len(puuid)+count)
		b[idx] = 1
		fh.Write([]byte(puuid))
		fh.Write(b)
		return false
	}

	// If this is a new file, create it and write out current state
	fh, err = os.OpenFile(f, os.O_RDWR, 0662)
	if err != nil {
		return false
	}
	defer fh.Close()

	// Now read it all
	b := make([]byte, len(puuid)+count)
	n, err := fh.ReadAt(b, 0)
	if n != len(puuid)+count || err != nil {
		return false
	}

	// Verify the original uuid is the same, if not, start over
	if string(b[:len(puuid)]) != puuid {
		if *verbose {
			log.Println("  non matching uuid found", puuid)
		}
		b = make([]byte, len(puuid)+count)
		copy(b, []byte(puuid))
		fh.WriteAt(b, 0)
	}

	// Write what we just did
	fh.WriteAt([]byte{1}, int64(len(puuid)+idx))

	// Now read it all
	b = make([]byte, count)
	n, err = fh.ReadAt(b, int64(len(puuid)))
	if n != count || err != nil {
		return false
	}

	//if *verbose {
	//	log.Println("  Parts seen:", b)
	//}

	// If anything is a zero, bail ship
	for _, t := range b {
		if t == 0 {
			return false
		}
	}
	return true
}
