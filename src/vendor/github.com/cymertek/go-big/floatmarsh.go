// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements encoding/decoding of Floats.

package big // import "github.com/cymertek/go-big"

import (
	"encoding/binary"
	"fmt"
	"math/bits"
)

// Gob codec version. Permits backward-compatible changes to the encoding.
const floatGobVersion byte = 1

// GobEncode implements the gob.GobEncoder interface.
// The Float value and all its attributes (precision,
// rounding mode, accuracy) are marshaled.
func (x *Float) GobEncode() ([]byte, error) {
	if x == nil {
		return nil, nil
	}

	// determine max. space (bytes) required for encoding
	sz := 1 + 1 + 4 // version + mode|acc|form|neg (3+2+2+1bit) + prec
	n := 0          // number of mantissa words
	if x.form == finite {
		// add space for mantissa and exponent
		n = int((x.prec + (_W - 1)) / _W) // required mantissa length in words for given precision
		// actual mantissa slice could be shorter (trailing 0's) or longer (unused bits):
		// - if shorter, only encode the words present
		// - if longer, cut off unused words when encoding in bytes
		//   (in practice, this should never happen since rounding
		//   takes care of it, but be safe and do it always)
		if len(x.mant) < n {
			n = len(x.mant)
		}
		// len(x.mant) >= n
		sz += 4 + n*_S // exp + mant
	}
	buf := make([]byte, sz)

	buf[0] = floatGobVersion
	b := byte(x.mode&7)<<5 | byte((x.acc+1)&3)<<3 | byte(x.form&3)<<1
	if x.neg {
		b |= 1
	}
	buf[1] = b
	binary.BigEndian.PutUint32(buf[2:], x.prec)

	if x.form == finite {
		binary.BigEndian.PutUint32(buf[6:], uint32(x.exp))
		x.mant[len(x.mant)-n:].bytes(buf[10:]) // cut off unused trailing words
	}

	return buf, nil
}

// GobDecode implements the gob.GobDecoder interface.
// The result is rounded per the precision and rounding mode of
// z unless z's precision is 0, in which case z is set exactly
// to the decoded value.
func (z *Float) GobDecode(buf []byte) error {
	if len(buf) == 0 {
		// Other side sent a nil or default value.
		*z = Float{}
		return nil
	}

	if buf[0] != floatGobVersion {
		return fmt.Errorf("Float.GobDecode: encoding version %d not supported", buf[0])
	}

	oldPrec := z.prec
	oldMode := z.mode

	b := buf[1]
	z.mode = RoundingMode((b >> 5) & 7)
	z.acc = Accuracy((b>>3)&3) - 1
	z.form = form((b >> 1) & 3)
	z.neg = b&1 != 0
	z.prec = binary.BigEndian.Uint32(buf[2:])

	if z.form == finite {
		z.exp = int32(binary.BigEndian.Uint32(buf[6:]))
		z.mant = z.mant.setBytes(buf[10:])
	}

	if oldPrec != 0 {
		z.mode = oldMode
		z.SetPrec(uint(oldPrec))
	}

	return nil
}

// MarshalText implements the encoding.TextMarshaler interface.
// Only the Float value is marshaled (in full precision), other
// attributes such as precision or accuracy are ignored.
func (x *Float) MarshalText() (text []byte, err error) {
	if x == nil {
		return []byte("<nil>"), nil
	}
	var buf []byte
	return x.Append(buf, 'g', -1), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// The result is rounded per the precision and rounding mode of z.
// If z's precision is 0, it is changed to 64 before rounding takes
// effect.
func (z *Float) UnmarshalText(text []byte) error {
	// TODO(gri): get rid of the []byte/string conversion
	_, _, err := z.Parse(string(text), 0)
	if err != nil {
		err = fmt.Errorf("math/big: cannot unmarshal %q into a *big.Float (%v)", text, err)
	}
	return err
}

// Bytes returns a byte slice of the Float mantissa cut into two with the
// contents of the integer portion in the first and the contents of the decimal
// in the second.  The mantissa bytes are in BigEndian order.  The first byte
// slice is the same as converting the Float to an Integer, Int(), and then
// Bytes() to get the byte slice, while the second is the fractional portion of
// the float.
//
// The intended purpose of making this byte slice available is for doing binary
// manipulation of the mantissa without having to encode/decode into
// intermediary formats.  The full mantissa is encapsulated in the returned
// byte slice (with any additional zero padding) so one needs to be aware that
// the actual precision of the Float is not representative of the slice size.
//
// Note: Only finite and zero values can be converted.  With this conversion
// method, the precision, accuracy and negative sign are not maintaioned.
func (z *Float) Bytes() (integer, decimal []byte) {
	switch z.form {
	case zero:
		return []byte{0}, []byte{}
	case finite:

		// determine minimum required precision for x
		exp := z.exp
		//prec := uint(z.prec)
		m := make([]Word, len(z.mant))
		b := make([]byte, len(m)*_W/8+1)
		var i int

		// align on the byte
		if s := uint32(exp) % 8; s != 0 {
			val := shlVU(m, z.mant, uint(s))
			b[0] = uint8(val)
			i++
		} else {
			copy(m, z.mant)
		}

		// integer portion of the mantisa
		for j := len(m) - 1; j >= 0; j-- {
			switch _W {
			case 64:
				binary.BigEndian.PutUint64(b[i:i+_W/8], uint64(m[j]))
			case 32:
				binary.BigEndian.PutUint32(b[i:i+_W/8], uint32(m[j]))
			}
			i += _W / 8
		}

		// Trim down to the precision with one extra byte
		if prec := uint(z.prec+7)/8 + (uint(exp%8)+7)/8; int(prec) < len(b) {
			b = b[:prec]
		}
		if exp <= 0 {
			// When |z| < 1
			b = append(make([]byte, -exp/8), b...)
			return b[:0], b
		}
		// When |z| >= 1, split into two parts using the dec mark
		dec := (exp + 7) / 8
		if int(dec) > len(b) {
			b = append(b, make([]byte, int(dec)-len(b))...)
		}
		return b[:dec], b[dec:]
	default:
		panic(ErrNaN{"bytes called on a non-finite Float"})
	}
	return
}

// SetBytes sets a Float to the value mantissa to the contents of two byte slices
// in BigEndian order.  The first byte slice is the same integer portion of the mantissa
// while the second is the decimal porition.
//
// Note: Only finite and zero values can be converted.  With this conversion
// method, the precision, accuracy and negative sign are not maintaioned.
func (z *Float) SetBytes(integer, decimal []byte) *Float {
	// calculate the sizes
	idLen := len(integer) + len(decimal)
	words := (idLen*8-1)/_W + 1
	z.prec = uint32(idLen * 8)

	// allocate slices
	b := make([]byte, words*_W/8)
	if words < 2 {
		words = 2
	}
	m := make([]Word, words)

	// fill the source slice
	intSize := copy(b, integer)
	copy(b[intSize:], decimal)

	// determine the exponent
	firstbit := -1
	for i := 0; i < len(b); i++ {
		if exp := bits.LeadingZeros8(b[i]); exp < 8 {
			firstbit = i*8 + exp
			break
		}
	}
	if firstbit == -1 {
		z.acc = Exact
		z.form = zero
		return z
	}

	// build the word slice
	for i, j := len(m)-1, 0; j < len(b); i, j = i-1, j+8 {
		switch _W {
		case 64:
			m[i] = Word(binary.BigEndian.Uint64(b[j : j+8]))
		case 32:
			m[i] = Word(binary.BigEndian.Uint32(b[j : j+8]))
		}
	}
	fnorm(m)

	z.exp = int32(intSize*8 - firstbit)
	z.form = finite
	z.mant = m
	return z
}
