# go-numstr

A sorting function for comparison of arbitrary strings with numbers mixed in.

```golang
  a, b := "abc123", "abc12"
  fmt.Printf("%s < %s  %v\n", a, b, numstr.LessThanFold(a, b)) // false
  a, b = "abc100", "abc121"
  fmt.Printf("%s < %s  %v\n", a, b, numstr.LessThanFold(a, b)) // true
  a, b = "abc10a", "abc1d"
  fmt.Printf("%s < %s  %v\n", a, b, numstr.LessThanFold(a, b)) // false
```
