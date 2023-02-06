package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/inhies/go-bytesize"
	"github.com/pschou/go-flowfile"
)

var about = `NiFi Reciever

This utility is intended to listen for flow files on a NifI compatible port and
then parse these files and drop them to disk for usage elsewhere.`

var (
	basePath   = flag.String("path", "output", "Directory in which to place files recieved")
	listen     = flag.String("listen", ":8080", "Where to listen to incoming connections (example 1.2.3.4:8080)")
	listenPath = flag.String("listenPath", "/contentListener", "Path in URL where to expect FlowFiles to be posted")
	enableTLS  = flag.Bool("tls", false, "Enable TLS for secure transport")
	maxSize    = flag.String("segment-max-size", "", "Set a maximum size for partitioning files in sending")
	debug      = flag.Bool("debug", false, "Turn on debug")
)

func main() {
	flag.Parse()
	if *debug {
		flowfile.Debug = true
	}
	if *enableTLS {
		loadTLS()
	}
	fmt.Println("output set to", *basePath)

	// Settings for the flow file reciever
	ffReciever := flowfile.NewHTTPFileReciever(post)
	if *maxSize != "" {
		if bs, err := bytesize.Parse(*maxSize); err != nil {
			log.Fatal("Unable to parse max-size", err)
		} else {
			log.Println("Setting max-size to", bs)
			ffReciever.MaxPartitionSize = int(uint64(bs))
		}
	}

	http.Handle(*listenPath, ffReciever)
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
		fmt.Println("  Recieving nifi file", fp, "size", f.Size)
		if *verbose {
			adat, _ := json.Marshal(f.Attrs)
			fmt.Printf("    %s\n", adat)
		}

		// Safe off the file
		_, err = f.Save(*basePath)
		if err == nil {
			if id := f.Attrs.Get("segment-index"); id != "" {
				i, _ := strconv.Atoi(id)
				fmt.Printf("  Verified segment %d of %s of %s\n", i+1, f.Attrs.Get("segment-count"), path.Join(dir, filename))
			} else {
				fmt.Printf("  Verified file %s\n", path.Join(dir, filename))
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
