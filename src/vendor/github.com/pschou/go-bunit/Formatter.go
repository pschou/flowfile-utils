package bunit

import (
	"fmt"
	"time"

	"github.com/cymertek/go-big"
)

// Format for use in Printf
func (b Bits) Format(f fmt.State, verb rune) {
	formatByte(b, 1, f, verb, 'b', "b")
}

// Format for use with stringify
func (b Bits) String() string {
	return fmt.Sprintf("%v", b)
}

// Format for use in Printf
func (b BitRate) Format(f fmt.State, verb rune) {
	formatByte(b.n, float64(time.Second)/float64(b.d), f, verb, 'b', "bps")
}

// Format for use with stringify
func (b BitRate) String() string {
	return fmt.Sprintf("%v", b)
}

// Format for use in Printf
func (b Bytes) Format(f fmt.State, verb rune) {
	formatByte(b, 1, f, verb, 'B', "B")
}

// Format for use with stringify
func (b Bytes) String() string {
	return fmt.Sprintf("%V", b)
}

// Format for use in Printf
func (b ByteRate) Format(f fmt.State, verb rune) {
	formatByte(b.n, float64(time.Second)/float64(b.d), f, verb, 'B', "B/s")
}

// Format for use with stringify
func (b ByteRate) String() string {
	return fmt.Sprintf("%V", b)
}

func formatByte(b []byte, scale float64, f fmt.State, verb, def rune, suf string) {
	v := (&big.Float{}).SetBytes(b, []byte{})
	if scale != 1 {
		v = v.Mul(v, big.NewFloat(scale))
	}
	switch verb {
	case def:
	case 'v':
		// Auto with 1000 multiples
		n, _ := v.Int(nil)
		n = n.Lsh(n, 2)
		if (&big.Int{}).SetBytes(thousand[10]).Cmp(n) <= 0 {
			n = n.Rsh(n, 10)
			for i := range thousandVerb[:10] {
				if (&big.Int{}).SetBytes(thousand[10+i]).Cmp(n) > 0 {
					v.Quo(v, (&big.Float{}).SetBytes(thousand[10+i], []byte{}))
					suf = string(thousandVerb[i+20]) + suf
					break
				}
			}
		}
	case 's':
		// String with 1000 multiples
		n, _ := v.Int(nil)
		n = n.Lsh(n, 2)
		switch suf {
		case "b":
			suf = "Bit"
		case "B":
			suf = "Byte"
		}
		if (&big.Int{}).SetBytes(thousand[10]).Cmp(n) <= 0 {
			n = n.Rsh(n, 10)
			for i := range thousandVerb[:10] {
				if (&big.Int{}).SetBytes(thousand[10+i]).Cmp(n) > 0 {
					v.Quo(v, (&big.Float{}).SetBytes(thousand[10+i], []byte{}))
					suf = thousandWord[i] + suf
					break
				}
			}
		}

	case 'V':
		// Auto with 1024 multiples
		n, _ := v.Int(nil)
		n = n.Lsh(n, 2)
		if (&big.Int{}).SetBytes(thousand[0]).Cmp(n) <= 0 {
			n = n.Rsh(n, 10)
			for i, c := range thousandVerb[:10] {
				if (&big.Int{}).SetBytes(thousand[i]).Cmp(n) > 0 {
					v.Quo(v, (&big.Float{}).SetBytes(thousand[i], []byte{}))
					suf = string(c) + "i" + suf
					break
				}
			}
		}

	case 'S':
		// String with 1024 multiples
		n, _ := v.Int(nil)
		n = n.Lsh(n, 2)
		switch suf {
		case "b":
			suf = "Bit"
		case "B":
			suf = "Byte"
		}
		if (&big.Int{}).SetBytes(thousand[0]).Cmp(n) <= 0 {
			n = n.Rsh(n, 10)
			for i := range thousandVerb[:10] {
				if (&big.Int{}).SetBytes(thousand[i]).Cmp(n) > 0 {
					v.Quo(v, (&big.Float{}).SetBytes(thousand[i], []byte{}))
					suf = thousandWord[10+i] + suf
					break
				}
			}
		}
	default:
		// All the SI Byte units
		for i, c := range thousandVerb[:20] {
			//fmt.Printf("comparing %q %q %v\n", verb, c, thousand[i])
			if c == verb {
				//fmt.Printf("%v / %v\n", v, (&big.Float{}).SetBytes(thousand[i+1], []byte{}))
				v.Quo(v, (&big.Float{}).SetBytes(thousand[i], []byte{}))
				if i < 10 {
					suf = string(thousandVerb[i+20]) + suf
				} else {
					suf = string(thousandVerb[i%10]) + "i" + suf
				}
				break
			}
		}
		if suf == "" {
			f.Write([]byte("%!(INVALID " + quote(string(verb)) + ")"))
			return
		}
	}
	v.Format(f, 'g')
	f.Write([]byte(suf))
}
