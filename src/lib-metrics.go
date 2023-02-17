package main

import (
	"flag"
	"strings"
	"time"

	"github.com/pschou/go-flowfile"
)

var (
	metricsFF     bool
	metricsTarget string
)

// This function periodically sends metrics to the prom collector
func metrics_flags(def bool) {
	flag.BoolVar(&metricsFF, "metrics-ff", true, "Send local metrics as a FlowFile")
	//flag.StringVar(&metricsTarget, "metrics-send", "", "Path to Prometheus Collector to send metrics")
}

func send_metrics(send func(*flowfile.File), ffReceiver *flowfile.HTTPReceiver) {
	if metricsFF {
		act := func() {
			str := ffReceiver.Metrics()
			rdr := strings.NewReader(str)
			ff := flowfile.New(rdr, int64(len(str)))
			ff.Attrs.CustodyChainShift()
			ff.Attrs.Set("filename", "metrics.prom")
			ff.Attrs.Set("kind", "metrics")
			ff.Attrs.GenerateUUID()
			send(ff)
		}

		act() // Send initial block
		go func() {
			for {
				time.Sleep(5 * time.Minute)
				act()
			}
		}()
	}
}
