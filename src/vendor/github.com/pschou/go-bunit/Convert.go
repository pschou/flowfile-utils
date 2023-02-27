package bunit

import (
	"time"

	"github.com/cymertek/go-big"
)

// Get the int64 value of the bytes, may be cut off for large values
func (b Bytes) Int64() int64 {
	return (&big.Int{}).SetBytes(b).Int64()
}

// Get the int64 value of the bits, may be cut off for large values
func (b Bits) Int64() int64 {
	return (&big.Int{}).SetBytes(b).Int64()
}

// Get the big.Int value of the bytes, may be cut off for large values
func (b Bytes) Int() *big.Int {
	return (&big.Int{}).SetBytes(b)
}

// Get the big.Int value of the bits, may be cut off for large values
func (b Bits) Int() *big.Int {
	return (&big.Int{}).SetBytes(b)
}

// Get the int64 value of the bit rate per second, may be cut off for large values
func (b BitRate) Int64() int64 {
	v := (&big.Float{}).SetBytes(b.n, []byte{})
	v.Mul(v, big.NewFloat(float64(time.Second)/float64(b.d)))
	r, _ := v.Int64()
	return r
}

// Get the int64 value of the byte rate per second, may be cut off for large values
func (b ByteRate) Int64() int64 {
	v := (&big.Float{}).SetBytes(b.n, []byte{})
	v.Mul(v, big.NewFloat(float64(time.Second)/float64(b.d)))
	r, _ := v.Int64()
	return r
}

// Get the big.Float value of the bit rate per second, may be cut off for large values
func (b BitRate) Float() *big.Float {
	v := (&big.Float{}).SetBytes(b.n, []byte{})
	v.Mul(v, big.NewFloat(float64(time.Second)/float64(b.d)))
	return v
}

// Get the big.Float value of the byte rate per second, may be cut off for large values
func (b ByteRate) Float() *big.Float {
	v := (&big.Float{}).SetBytes(b.n, []byte{})
	v.Mul(v, big.NewFloat(float64(time.Second)/float64(b.d)))
	return v
}
