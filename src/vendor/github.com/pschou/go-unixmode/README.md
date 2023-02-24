# go-unixmode

This module is intended to help developers work directly with file permissions on Linux.

As of the time this module was built, there are no valid String() method in Go
which correctly, in a POSIX manner, identifies sticky / set uid or gid bits in
9 character permission string.  Hence the birth of this module.

One can go either way and preserve information:

```golang
um := unixmode.Parse("rwsr-sr-x")
fmt.Printf("%05o", um) // 06755

perm := unixmode.Mode(06755)
fmt.Println(perm.PermString()) // rwsr-sr-x
```

More documentation: https://pkg.go.dev/github.com/pschou/go-unixmode
