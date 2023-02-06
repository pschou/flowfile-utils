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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/inhies/go-bytesize"
	"github.com/pschou/go-flowfile"
)

var about = `NiFi Receiver

This utility is intended to listen for flow files on a NifI compatible port and
then parse these files and drop them to disk for usage elsewhere.`

var (
	basePath    = flag.String("path", "./output/", "Directory in which to place files received")
	listen      = flag.String("listen", ":8080", "Where to listen to incoming connections (example 1.2.3.4:8080)")
	listenPath  = flag.String("listenPath", "/contentListener", "Path in URL where to expect FlowFiles to be posted")
	enableTLS   = flag.Bool("tls", false, "Enable TLS for secure transport")
	maxSize     = flag.String("segment-max-size", "", "Set a maximum size for partitioning files in sending")
	debug       = flag.Bool("debug", false, "Turn on debug")
	script      = flag.String("script", "", "Shell script to be called on successful post")
	scriptShell = flag.String("script-shell", "/bin/bash", "Shell to be used for script run")
	remove      = flag.Bool("rm", false, "Automatically remove file after script has finished")
)

func main() {
	service_flag()
	flag.Parse()
	service_init()
	if *debug {
		flowfile.Debug = true
	}
	if *enableTLS {
		loadTLS()
	}
	fmt.Println("Output set to", *basePath)

	// Settings for the flow file receiver
	ffReceiver := flowfile.NewHTTPFileReceiver(post)
	if *maxSize != "" {
		if bs, err := bytesize.Parse(*maxSize); err != nil {
			log.Fatal("Unable to parse max-size", err)
		} else {
			log.Println("Setting max-size to", bs)
			ffReceiver.MaxPartitionSize = int(uint64(bs))
		}
	}

	http.Handle(*listenPath, ffReceiver)
	if *enableTLS {
		log.Println("Listening with HTTPS on", *listen, "at", *listenPath)
		server := &http.Server{Addr: *listen, TLSConfig: tlsConfig}
		log.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
	} else {
		log.Println("Listening with HTTP on", *listen, "at", *listenPath)
		log.Fatal(http.ListenAndServe(*listen, nil))
	}
}

func post(f *flowfile.File, r *http.Request) (err error) {
	// Save the flowfile into the base path with the file structure defined by
	// the flowfile attributes.
	dir := filepath.Clean(f.Attrs.Get("path"))
	filename := f.Attrs.Get("filename")
	fp := path.Join(*basePath, dir, filename)
	err = os.MkdirAll(path.Join(*basePath, dir), 0755)
	if err != nil {
		log.Fatal(err)
	}

	if *debug {
		fmt.Println("kind = ", f.Attrs.Get("kind"), "fp", fp)
	}

	switch kind := f.Attrs.Get("kind"); kind {
	case "file", "":
		log.Println("  Receiving nifi flowfile", fp, "size", f.Size)
		if *verbose {
			adat, _ := json.Marshal(f.Attrs)
			fmt.Printf("    %s\n", adat)
		}

		// Safe off the file
		_, err = f.Save(*basePath)
		if err == nil {
			if id := f.Attrs.Get("segment-index"); id != "" {
				i, _ := strconv.Atoi(id)
				count, _ := strconv.Atoi(f.Attrs.Get("segment-count"))
				log.Printf("  Verified segment %d of %s of %s\n", i+1, f.Attrs.Get("segment-count"), fp)
				if !updateIndexFile(fp+".progress", i, count) {
					// The file is not complete yet
					return
				}
				if err = f.VerifyParent(fp); err != nil {
					// Verification failed
					return
				}
			} else {
				log.Printf("  Verified file %s\n", fp)
			}
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

				if *remove {
					os.Remove(fp + ".progress")
					os.Remove(fp)
					//if *verbose {
					log.Printf("  Removed %s\n", fp)
					//}
				}
			}
		}

	case "dir":
		err = os.MkdirAll(fp, 0755)
		if *debug {
			fmt.Println("making directory", fp, err)
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
	default:
		if *verbose {
			log.Println("Cannot accept kind:", kind)
		}
		return fmt.Errorf("Unknown kind %q", kind)
	}

	// Update file time from sender
	if mt := f.Attrs.Get("modtime"); err == nil && mt != "" {
		if fileTime, err := time.Parse(time.RFC3339, mt); err == nil {
			os.Chtimes(fp, fileTime, fileTime)
		}
	}

	return
}

var updateIndexMutex sync.Mutex

func updateIndexFile(f string, idx, count int) bool {
	if idx < 0 || idx >= count || count <= 1 {
		return false
	}
	updateIndexMutex.Lock()
	defer updateIndexMutex.Unlock()

	// If this is a new file, create it and write out current state
	fh, err := os.OpenFile(f, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	if err == nil {
		defer fh.Close()
		b := make([]byte, count)
		b[idx] = 1
		fh.Write(b)
		return false
	}

	// If this is a new file, create it and write out current state
	fh, err = os.OpenFile(f, os.O_RDWR, 0666)
	if err != nil {
		return false
	}
	defer fh.Close()

	// Write what we just did
	fh.WriteAt([]byte{1}, int64(idx))

	// Now read it all
	b := make([]byte, count)
	n, err := fh.ReadAt(b, 0)
	if n != count || err != nil {
		return false
	}

	if *verbose {
		log.Println("  Parts seen:", b)
	}

	// If anything is a zero, bail ship
	for _, t := range b {
		if t == 0 {
			return false
		}
	}
	return true
}