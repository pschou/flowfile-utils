package main

import (
	"log"
	"time"

	"github.com/inhies/go-bytesize"
	"github.com/pschou/go-flowfile"
)

// This function periodically handshakes the connections to maintain state

func handshaker(hs *flowfile.HTTPTransaction, ffReceiver *flowfile.HTTPReceiver) {
	var localMaxPartitionSize, new int64
	if *maxSize != "" {
		if bs, err := bytesize.Parse(*maxSize); err != nil {
			log.Fatal("Unable to parse max-size", err)
		} else {
			log.Println("Setting max-size to", bs)
			localMaxPartitionSize = int64(uint64(bs))
			ffReceiver.MaxPartitionSize = int64(uint64(bs))
		}
	}

	// Don't need to do checks on the transaction side as they will never happen
	if hs == nil {
		return
	}

	go func() {
		for {
			new = localMaxPartitionSize
			if hs.MaxPartitionSize > 0 || new > 0 {
				if new == 0 || (hs.MaxPartitionSize > 0 && hs.MaxPartitionSize < new) {
					new = hs.MaxPartitionSize
				}
			}
			if new != ffReceiver.MaxPartitionSize {
				log.Println("Setting max-size to", hs.MaxPartitionSize)
				ffReceiver.MaxPartitionSize = new
			}
			time.Sleep(10 * time.Minute)
			hs.Handshake()
		}
	}()
}
