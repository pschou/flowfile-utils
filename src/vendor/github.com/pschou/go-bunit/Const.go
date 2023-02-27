// Copyright 2023 github.com/pschou
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bunit

import (
	"fmt"
	"time"
)

type Bytes []byte
type Bits []byte
type ByteRate struct {
	n Bytes
	d time.Duration
}
type BitRate struct {
	n Bits
	d time.Duration
}

var byteZero Bytes
var bitZero Bits
var byteRateZero ByteRate
var bitRateZero BitRate
var _ fmt.Formatter = byteZero     // Bytes must implement fmt.Formatter
var _ fmt.Formatter = bitZero      // Bits must implement fmt.Formatter
var _ fmt.Formatter = byteRateZero // ByteRate must implement fmt.Formatter
var _ fmt.Formatter = bitRateZero  // BitRate must implement fmt.Formatter

const (
	// Decimal binary values
	Byte       = float64(1)
	KiloByte   = float64(1e3)
	MegaByte   = float64(1e6)
	GigaByte   = float64(1e9)
	TeraByte   = float64(1e12)
	PetaByte   = float64(1e15)
	ExaByte    = float64(1e18)
	ZettaByte  = float64(1e21)
	YottaByte  = float64(1e24)
	RonnaByte  = float64(1e27)
	QuettaByte = float64(1e30)

	// IEC Binary Values
	KibiByte = float64(8 << (iota * 3))
	MebiByte
	GibiByte
	TebiByte
	PebiByte
	ExbiByte
	ZebiByte
	YobiByte
	RobiByte
	QubiByte
)

var unitMap = map[string]float64{
	"": 1,

	// Si prefix list
	"k":      1e3,
	"K":      1e3, // Provided for compatibility
	"Kilo":   1e3,
	"kilo":   1e3,
	"M":      1e6,
	"Mega":   1e6,
	"mega":   1e6,
	"G":      1e9,
	"Giga":   1e9,
	"giga":   1e9,
	"T":      1e12,
	"Tera":   1e12,
	"tera":   1e12,
	"P":      1e15,
	"Peta":   1e15,
	"peta":   1e15,
	"E":      1e18,
	"Exa":    1e18,
	"exa":    1e18,
	"Z":      1e21,
	"Zetta":  1e21,
	"zetta":  1e21,
	"Y":      1e24,
	"Yotta":  1e24,
	"yotta":  1e24,
	"R":      1e27,
	"Ronna":  1e27,
	"ronna":  1e27,
	"Q":      1e30,
	"Quetta": 1e30,
	"quetta": 1e30,

	"Ki":   1 << 10,
	"Kibi": 1 << 10,
	"kibi": 1 << 10,
	"Mi":   1 << 20,
	"Mebi": 1 << 20,
	"mebi": 1 << 20,
	"Gi":   1 << 30,
	"Gibi": 1 << 30,
	"gibi": 1 << 30,
	"Ti":   1 << 40,
	"Tebi": 1 << 40,
	"tebi": 1 << 40,
	"Pi":   1 << 50,
	"Pebi": 1 << 50,
	"pebi": 1 << 50,
	"Ei":   1 << 60,
	"Exbi": 1 << 60,
	"exbi": 1 << 60,
	"Zi":   1 << 70,
	"Zebi": 1 << 70,
	"zebi": 1 << 70,
	"Yi":   1 << 80,
	"Yobi": 1 << 80,
	"yobi": 1 << 80,
	"Ri":   1 << 90,
	"Robi": 1 << 90,
	"robi": 1 << 90,
	"Qi":   1 << 100,
	"Qubi": 1 << 100,
	"qubi": 1 << 100,
}

var thousandVerb = "KMGTPEZYRQkmgtpezyrqkMGTPEZYRQ"
var thousandWord = []string{
	"Kilo",
	"Mega",
	"Giga",
	"Tera",
	"Peta",
	"Exa",
	"Zetta",
	"Yotta",
	"Ronna",
	"Quetta",
	"Kibi",
	"Mebi",
	"Gibi",
	"Tebi",
	"Pebi",
	"Exbi",
	"Zebi",
	"Yobi",
	"Robi",
	"Qubi",
}

var thousand = [][]byte{
	[]byte{0x4, 0x0},
	[]byte{0x10, 0x0, 0x0},
	[]byte{0x40, 0x0, 0x0, 0x0},
	[]byte{0x1, 0x0, 0x0, 0x0, 0x0, 0x0},
	[]byte{0x4, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
	[]byte{0x10, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
	[]byte{0x40, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
	[]byte{0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
	[]byte{0x4, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
	[]byte{0x10, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
	[]byte{0x3, 0xe8},
	[]byte{0xf, 0x42, 0x40},
	[]byte{0x3b, 0x9a, 0xca, 0x0},
	[]byte{0xe8, 0xd4, 0xa5, 0x10, 0x0},
	[]byte{0x3, 0x8d, 0x7e, 0xa4, 0xc6, 0x80, 0x0},
	[]byte{0xd, 0xe0, 0xb6, 0xb3, 0xa7, 0x64, 0x0, 0x0},
	[]byte{0x36, 0x35, 0xc9, 0xad, 0xc5, 0xde, 0xa0, 0x0, 0x0},
	[]byte{0xd3, 0xc2, 0x1b, 0xce, 0xcc, 0xed, 0xa1, 0x0, 0x0, 0x0},
	[]byte{0x3, 0x3b, 0x2e, 0x3c, 0x9f, 0xd0, 0x80, 0x3c, 0xe8, 0x0, 0x0, 0x0},
	[]byte{0xc, 0x9f, 0x2c, 0x9c, 0xd0, 0x46, 0x74, 0xed, 0xea, 0x40, 0x0, 0x0, 0x0},
}
