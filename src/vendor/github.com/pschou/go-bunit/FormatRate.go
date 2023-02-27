package bunit

import (
	"errors"
	"fmt"
)

// Format the number of bits into a given unit for display
func PrintRatef(val float64, formatUnit string) (string, error) {
	format, s := doPrintfSplit(formatUnit)
	f := float64(1)

	// Consume unit.
	i, b := 0, 0
	for ; i < len(s); i++ {
		c := s[i]
		if c == 'b' || c == 'B' {
			b = i
		}
	}
	if i == 0 {
		return "", errors.New("binary unit: missing unit in format " + quote(formatUnit))
	}
	switch s[b:] {
	case "bit/s", "bps", "b/s", "b/S", "Bit/s", "bits/s", "Bits/s":
	case "Bps", "B/s", "B/S", "bytes/s", "Bytes/s":
		f *= 8
	default:
		return "", errors.New("binary unit: missing byte or bit unit in format " + quote(formatUnit))
	}
	u := s[:b]
	unit, ok := unitMap[u]
	if !ok {
		return "", errors.New("binary unit: unknown unit " + quote(u) + " in format " + quote(formatUnit))
	}

	return fmt.Sprintf(format, val/(f*unit)) + s, nil
}
