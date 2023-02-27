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
	"errors"
	"time"

	"github.com/cymertek/go-big"
)

// Parse a string into a ByteRate value
func ParseByteRate(s string) (*ByteRate, error) {
	r, err := ParseBitRate(s)
	// Scale up the demonitor (time) for Bytes
	return &ByteRate{Bytes(r.n), r.d << 3}, err
}

// Parse a string into a BitRate value
func ParseBitRate(s string) (*BitRate, error) {
	orig := s
	d := &big.Float{}
	neg := false

	// Consume [-+]?
	if s != "" {
		c := s[0]
		if c == '-' || c == '+' {
			neg = c == '-'
			s = s[1:]
		}
	}
	// Special case: if all that is left is "0", this is zero.
	if s == "0" {
		return nil, nil
	}
	if s == "" {
		return nil, errors.New("binary unit: invalid value " + quote(orig))
	}

	for s != "" && s[0] != '/' {
		var v *big.Float
		var err error

		// The next character must be [0-9.]
		if !(s[0] == '.' || '0' <= s[0] && s[0] <= '9') {
			return nil, errors.New("binary unit: invalid value " + quote(orig))
		}

		// Consume [0-9.]*
		v, s, err = leadingBigFloat(s)
		if err != nil {
			return nil, errors.New("binary unit: invalid value " + quote(orig))
		}

		// Get rid of spaces
		for len(s) > 0 && s[0] == ' ' {
			s = s[1:]
		}

		// Consume unit.
		i, b := 0, -1
		for ; i < len(s); i++ {
			c := s[i]
			if c == 'b' || c == 'B' {
				b = i
			} else if c == '.' || '0' <= c && c <= '9' || c == '/' {
				break
			}
		}
		if i == 0 {
			return nil, errors.New("binary unit: missing unit in value " + quote(orig))
		}

		// Test for the case that we only have the SI suffix
		if unit, ok := unitMap[s[:i]]; ok {
			v.Mul(v, big.NewFloat(unit))
			d.Add(d, v)
			s = s[i:]
			continue // Look for more SI prefixes
		}

		if b < 0 {
			return nil, errors.New("binary unit: missing rate unit in value " + quote(orig))
		}

		// Test for the case that we have the SI suffix and unit
		u := s[:b]
		unit, ok := unitMap[u]
		if !ok {
			return nil, errors.New("binary unit: unknown unit " + quote(u) + " in value " + quote(orig))
		}
		v.Mul(v, big.NewFloat(unit))
		d.Add(d, v)

		// Find a 'p' instead of a slash
		if len(s) > b+2 && s[b+1] == 'p' {
			s = s[:b+1] + "/" + s[b+2:]
			i = b + 1
		}

		switch s[b:i] {
		case "bit", "b", "Bit", "bits", "Bits":
		case "byte", "B", "Byte", "bytes", "Bytes":
			v.Mul(v, eight)
		default:
			return nil, errors.New("binary unit: missing byte or bit unit in value " + quote(orig))
		}
		s = s[i:]
	}

	if s == "" || s[0] != '/' || s == "/" {
		return nil, errors.New("binary unit: missing time in value " + quote(orig))
	}
	s = s[1:]

	// Consume the duration
	var t time.Duration
	if s == "s" { // Do the simple stuff first
		t = time.Second
	} else {
		if c := s[0]; c < '0' || c > '9' {
			s = "1" + s
		}
		var err error
		t, err = time.ParseDuration(s)
		if err != nil {
			return nil, errors.New("binary unit: error parsing time in value " + quote(orig))
		}
	}
	if neg {
		t = -t
	}
	b, _ := d.Bytes()
	return &BitRate{b, t}, nil
}
