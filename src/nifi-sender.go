package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/docker/go-units"
	"github.com/pschou/go-flowfile"
)

var (
	about = `NiFi Sender

This utility is intended to capture a set of files or directory of files and
send them to a remote NiFi server for processing.`

	hs            *flowfile.HTTPTransaction
	wd, _         = os.Getwd()
	seenChecksums = make(map[string]string)

	dedup = flag.Bool("no-dedup", true, "Deduplication by checksums")
)

func main() {
	usage = "[options] path1 path2..."
	sender_flags()
	origin_flags()
	parse()

	if len(flag.Args()) == 0 {
		flag.Usage()
		return
	}

	// Connect to the NiFi server and establish a session
	log.Println("Creating list of files...")
	var err error

	hs, err = flowfile.NewHTTPTransaction(*url, tlsConfig)
	if err != nil {
		log.Fatal(err)
	}

	var zeros, content []*flowfile.File

	// Loop over the files sending them one at a time
	for _, arg := range flag.Args() {

		// Walk the directory args and send files
		filepath.Walk(arg, func(filename string, fileInfo os.FileInfo, inerr error) (err error) {
			if inerr != nil {
				log.Fatal(inerr)
			}

			if fileInfo.Mode().IsDir() && !isEmptyDir(filename) {
				// Skip sending details about non-empty directories
				return
			}

			var f *flowfile.File
			if f, err = flowfile.NewFromDisk(filename); err != nil {
				log.Fatal(err)
			}

			updateChain(f, nil, "SENDER")

			if f.Size == 0 {
				switch kind := f.Attrs.Get("kind"); kind {
				default:
					fmt.Printf("  [%s] %s\n", kind, filename)
				case "link":
					fmt.Printf("  [%s] %s -> %s\n", kind, filename, f.Attrs.Get("target"))
				}
				zeros = append(zeros, f)
			} else {
				fmt.Printf("  [file] %s (%s)\n", filename, units.HumanSize(float64(f.Size)))
				content = append(content, f)
			}
			return
		})
	}

	hs.RetryCount = *retries
	hs.RetryDelay = *retryTimeout
	hs.OnRetry = func(ff []*flowfile.File, retry int, err error) {
		log.Println("   Retrying", retry, "due to", err)
	}

	// Send off all the empty files and folders first
	log.Println("Sending meta data...")

	// do the work
	if err = hs.Send(zeros...); err != nil {
		log.Fatal("Failed to send, ", err)
	}

	// Send off the regular files
	log.Println("Sending content...")
	for _, c := range content {
		filename := c.FilePath()
		log.Printf(" sending %s (%s)", filename, units.HumanSize(float64(c.Size)))

		if *debug {
			log.Println("   Doing checksum...")
		}
		c.AddChecksum("SHA256")

		if *dedup {
			ckval := fmt.Sprintf("%q%q%d", c.Attrs.Get("checksumType"), c.Attrs.Get("checksum"), c.Size)
			if tgt, ok := seenChecksums[ckval]; ok {
				dn, _ := path.Split(filename)
				if fp, err := filepath.Rel(dn, tgt); err == nil {
					log.Println("  file matched previous content, sending link instead")
					c.Attrs.Set("kind", "link")
					c.Attrs.Set("target", fp)
					c.Size = 0
				}
			} else {
				seenChecksums[ckval] = filename
			}
		}

		segments, err := flowfile.SegmentBySize(c, int64(hs.MaxPartitionSize))
		if err != nil {
			log.Fatal(err)
		}
		for _, f := range segments {
			if *verbose {
				if f.Attrs.Get("kind") == "link" {
					log.Printf("  [link] %s -> %s\n", filename, f.Attrs.Get("target"))
				} else if ct := f.Attrs.Get("fragment.count"); ct == "" {
					log.Printf("  [file] %s (%s)\n", filename, units.HumanSize(float64(f.Size)))
				} else {
					log.Printf("  [seg %s of %s] %s (%s)\n", f.Attrs.Get("fragment.index"),
						ct, filename, units.HumanSize(float64(f.Size)))
				}
				if *debug {
					adat, _ := json.Marshal(f.Attrs)
					fmt.Printf("  %s\n", adat)
				}
			}

			f.AddChecksum("SHA256")

			// do the work
			if err = hs.Send(f); err != nil {
				log.Fatal("Failed to send", filename, err)
			}
		}
	}

	log.Println("done.")
}

func isEmptyDir(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	defer f.Close()
	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	return err == io.EOF
}
