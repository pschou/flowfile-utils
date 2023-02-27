package bunit

import (
	"time"

	"github.com/cymertek/go-big"
)

func NewBytes(n int64) *Bytes {
	v := Bytes(big.NewInt(n).Bytes())
	return &v
}
func NewBytesFromSlice(p []byte) *Bytes {
	v := Bytes(p)
	return &v
}
func NewBits(n int64) *Bits {
	v := Bits(big.NewInt(n).Bytes())
	return &v
}
func NewBitsFromSlice(p []byte) *Bits {
	v := Bits(p)
	return &v
}

func NewByteRate(n int64, d time.Duration) *ByteRate {
	v := ByteRate{big.NewInt(n).Bytes(), d}
	return &v
}
func NewByteRateFromSlice(p []byte, d time.Duration) *ByteRate {
	v := ByteRate{p, d}
	return &v
}
func NewBitRate(n int64, d time.Duration) *BitRate {
	v := BitRate{big.NewInt(n).Bytes(), d}
	return &v
}
func NewBitRateFromSlice(p []byte, d time.Duration) *BitRate {
	v := BitRate{p, d}
	return &v
}
