# Go-Big

A numerical library for GoLang forked off the built-in math/big, with extra functionality, and designed to be portable.

To align with the GoLang package, the majority of the functions are exactly the same.

Documentation: https://pkg.go.dev/github.com/cymertek/go-big

## Float Additions:

1. Bytes() and SetBytes() - Ability to work with the mantissa directly as a []byte slice

2. Mod1() - The opposite of Int(), in which only the decimal portion of the Float is maintained.
