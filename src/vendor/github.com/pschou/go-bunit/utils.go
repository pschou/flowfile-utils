package bunit

import (
	"errors"

	"github.com/cymertek/go-big"
)

var errLeadingInt = errors.New("time: bad [0-9]*") // never printed

// leadingBigFloat consumes the leading [0-9.]* from s.
func leadingBigFloat(s string) (x *big.Float, rem string, err error) {
	i := 0
	var pt bool
	var num string
	for ; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (pt || c != '.') {
			break
		}
		num += string(c)
	}
	if num == "" {
		return nil, rem, errLeadingInt
	}
	x, _, _ = (&big.Float{}).Parse(num, 10)
	return x, s[i:], nil
}

// leadingInt consumes the leading [0-9]* from s.
func leadingInt(s string) (x uint64, rem string, err error) {
	i := 0
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		if x > 1<<63/10 {
			// overflow
			return 0, rem, errLeadingInt
		}
		x = x*10 + uint64(c) - '0'
		if x > 1<<63 {
			// overflow
			return 0, rem, errLeadingInt
		}
	}
	return x, s[i:], nil
}

// leadingFraction consumes the leading [0-9]* from s.
// It is used only for fractions, so does not return an error on overflow,
// it just stops accumulating precision.
func leadingFraction(s string) (x uint64, scale float64, rem string) {
	i := 0
	scale = 1
	overflow := false
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		if overflow {
			continue
		}
		if x > (1<<63-1)/10 {
			// It's possible for overflow to give a positive number, so take care.
			overflow = true
			continue
		}
		y := x*10 + uint64(c) - '0'
		if y > 1<<63 {
			overflow = true
			continue
		}
		x = y
		scale *= 10
	}
	return x, scale, s[i:]
}

// These are borrowed from unicode/utf8 and strconv and replicate behavior in
// that package, since we can't take a dependency on either.
const (
	lowerhex  = "0123456789abcdef"
	runeSelf  = 0x80
	runeError = '\uFFFD'
)

func quote(s string) string {
	buf := make([]byte, 1, len(s)+2) // slice will be at least len(s) + quotes
	buf[0] = '"'
	for i, c := range s {
		if c >= runeSelf || c < ' ' {
			// This means you are asking us to parse a time.Duration or
			// time.Location with unprintable or non-ASCII characters in it.
			// We don't expect to hit this case very often. We could try to
			// reproduce strconv.Quote's behavior with full fidelity but
			// given how rarely we expect to hit these edge cases, speed and
			// conciseness are better.
			var width int
			if c == runeError {
				width = 1
				if i+2 < len(s) && s[i:i+3] == string(runeError) {
					width = 3
				}
			} else {
				width = len(string(c))
			}
			for j := 0; j < width; j++ {
				buf = append(buf, `\x`...)
				buf = append(buf, lowerhex[s[i+j]>>4])
				buf = append(buf, lowerhex[s[i+j]&0xF])
			}
		} else {
			if c == '"' || c == '\\' {
				buf = append(buf, '\\')
			}
			buf = append(buf, string(c)...)
		}
	}
	buf = append(buf, '"')
	return string(buf)
}

func doPrintfSplit(format string) (string, string) {
	marker := 0
	if len(format) > 0 && format[0] == '%' {
	search:
		for i := 1; i < len(format); i++ {
			switch c := format[i]; c {
			case '#', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '+', '-', '.', ' ':
				if marker > 0 {
					marker = i + 1
				}
			default:
				if 'a' <= c && c <= 'z' && marker == 0 {
					marker = i + 1
					continue
				}
				// Format is more complex than simple flags and a verb or is malformed.
				break search
			}
		}
	}

	if marker > 0 {
		return format[:marker], format[marker:]
	}
	return "%v", format
}
