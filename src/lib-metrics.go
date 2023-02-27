package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pschou/go-flowfile"
)

var (
	metricsFF        bool
	metricsPass      bool
	metricsTarget    string
	metricsFrequency time.Duration
	metricsBuilder   strings.Builder
)

// This function periodically sends metrics to the prom collector
func metrics_flags(def bool) {
	flag.BoolVar(&metricsPass, "metrics-pass", true, "Continue sending metrics downstream")
	flag.BoolVar(&metricsFF, "metrics-ff", def, "Send local metrics as a FlowFile")
	flag.DurationVar(&metricsFrequency, "metrics-freq", 5*time.Minute, "Frequency of sending local metrics as a FlowFile")
	flag.StringVar(&metricsTarget, "metrics-url", "", "Path to Prometheus Collector to send metrics\n"+
		"Example: -metrics-url=http://localhost:9550/collector/project/FFUtils")
}

func handle_metrics(f *flowfile.File) bool {
	var buf bytes.Buffer
	buf.ReadFrom(f)
	b := buf.Bytes()

	toWrite := bytes.NewReader(b)
	io.Copy(&metricsBuilder, toWrite)
	if metricsPass {
		// Reset the flowfile
		ff := flowfile.New(bytes.NewReader(b), int64(buf.Len()))
		ff.Attrs = f.Attrs
		*f = *ff
	}
	return !metricsPass
}

func send_metrics(proc string, send func(*flowfile.File), metrics *flowfile.Metrics) {
	if metricsFF {
		act := func() {
			var cur strings.Builder
			cur, metricsBuilder = metricsBuilder, strings.Builder{}
			//cur.Write([]byte("\n# Local metrics\n"))
			hn, _ := os.Hostname()
			cur.Write([]byte(metrics.String("host", hn, "action", proc)))
			str := cur.String()
			rdr := strings.NewReader(str)
			ff := flowfile.New(rdr, int64(len(str)))
			ff.Attrs.CustodyChainShift()
			ff.Attrs.Set("filename", "metrics.prom")
			ff.Attrs.Set("kind", "metrics")
			ff.Attrs.GenerateUUID()
			ff.AddChecksum("SHA1")
			ff.ChecksumInit()
			fmt.Println("flowfile", ff)
			send(ff)

			if strings.HasPrefix(metricsTarget, "http") {
				r, err := http.NewRequest("POST", metricsTarget, strings.NewReader(str))
				if err == nil {
					if *debug {
						log.Println("sending metrics", metricsTarget)
					}
					_, err = http.DefaultClient.Do(r)
					if err != nil && *debug {
						log.Println("error sending metrics", metricsTarget)
					}
				}
			}
		}

		act() // Send initial block
		go func() {
			for {
				time.Sleep(metricsFrequency)
				act()
			}
		}()
	}
}
