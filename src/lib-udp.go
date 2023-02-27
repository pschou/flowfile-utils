package main

import (
	"bytes"
	"encoding/binary"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
)

func hypenRange(r string) (out []int) {
	outVal := make(map[int]struct{})
	for _, commaPt := range strings.Split(r, ",") {
		hyphenPts := strings.Split(commaPt, "-")
		switch len(hyphenPts) {
		case 1:
			if p, err := strconv.Atoi(hyphenPts[0]); err != nil {
				log.Fatal("Error parsing port:", err)
			} else {
				outVal[p] = struct{}{}
			}
		case 2:
			if st, err := strconv.Atoi(hyphenPts[0]); err != nil {
				log.Fatal("Error parsing range start:", err)
			} else if en, err := strconv.Atoi(hyphenPts[1]); err != nil {
				log.Fatal("Error parsing range end:", err)
			} else {
				for ; st <= en; st++ {
					outVal[st] = struct{}{}
				}
			}
		default:
			log.Fatal("Invalid range:", commaPt)
		}
	}
	for k, _ := range outVal {
		if k > 1024 && k < 65536 {
			out = append(out, k)
		} else {
			log.Fatal("Port spec out of range:", k)
		}
	}
	return
}

// manual checksum calculation for UDP
func csum(sum int, data []byte) int {
	for i, b := range data {
		if i&1 == 0 {
			sum += (int(b) << 8)
		} else {
			sum += int(b)
		}
	}
	for sum > 0xffff {
		sum += (sum >> 16)
		sum &= 0xffff
	}
	return sum
}

type ffHeader struct {
	UUID   uuid.UUID
	Size   uint64
	Offset uint64
	MTU    uint16
}

var ffHeaderSize = func() uint64 {
	buf := &bytes.Buffer{}
	hdr := ffHeader{}
	binary.Write(buf, binary.BigEndian, &hdr)
	return uint64(buf.Len())
}()

var copyBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 32<<10)
		return &b
	},
}

/*var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

var serBufPool = sync.Pool{
	New: func() any {
		return gopacket.NewSerializeBuffer()
	},
}


/*
var ffHeaderSize = func() int {
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.BigEndian, &ffHeader{})
	return buf.Len()
}()

type ffHeaderWriter struct {
	h           *ffHeader
	w           io.Writer
	offset, mtu uint32
}

func (w *ffHeaderWriter) Write(p []byte) (n int, err error) {
	var t int
	for err == nil {
		// If we're on a boundary, write the header
		if w.offset%w.mtu == 0 {
			w.h.Offset = w.offset
			if err = binary.Write(w, binary.BigEndian, w.h); err != nil {
				return
			}
		}

		// Determine the marker
		mk := int(w.mtu - (w.offset % w.mtu))

		// Write up to the next marker
		if len(p) <= mk {
			t, err = w.w.Write(p)
			n, w.offset = n+t, w.offset+uint32(n)
			return
		}
		t, err = w.w.Write(p[:mk])
		n, w.offset = n+t, w.offset+uint32(n)
		p = p[mk:]
	}
	return
}

func buildUDPPacket(dst, src *net.UDPAddr) ([]byte, error) {
	buffer := gopacket.NewSerializeBuffer()
	payload := gopacket.Payload("HELLO")
	ip := &layers.IPv4{
		DstIP:    dst.IP,
		SrcIP:    src.IP,
		Version:  4,
		TTL:      64,
		Protocol: layers.IPProtocolUDP,
	}
	udp := &layers.UDP{
		SrcPort: layers.UDPPort(src.Port),
		DstPort: layers.UDPPort(dst.Port),
	}
	if err := udp.SetNetworkLayerForChecksum(ip); err != nil {
		return nil, fmt.Errorf("Failed calc checksum: %s", err)
	}
	if err := gopacket.SerializeLayers(buffer, gopacket.SerializeOptions{ComputeChecksums: true, FixLengths: true}, ip, udp, payload); err != nil {
		return nil, fmt.Errorf("Failed serialize packet: %s", err)
	}
	return buffer.Bytes(), nil
}
*/
