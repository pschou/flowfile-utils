# Binary Unit Tools

The purpose of this module is to enable both Parsing and Formatting binary data
volume values.  As the underlying data format is a byte slice, there is no
maximum size and the precision is perfect as there is no loss due to Float64
round off errors.

Parsing numbers is done through one of four parsers: ParseBytes, ParseBits,
ParseByteRate, and ParseBitRate.  Parsing may look like this ignoring errors:

```golang
  val, _ := bunit.ParseBytes("1kB")
  fmt.Println("1kB =", val)
  // 1kB = 1kB

  val, _ = bunit.ParseBytes("1KiB")
  fmt.Println("1KiB =", val)
  // 1KiB = 1.024kB

  // Notice the lower case 'b' indicating bits
  val, _ = bunit.ParseBytes("1k200b")
  fmt.Println("1k200b =", val)
  // 1k200b = 150B
```

The ability to format numbers is done through the standard fmt.Printf package.
The verbs which are available are:

```
%v - auto pick from below
%s - auto pick from below and print in long string format
%k - Kilo   1000^1
%m - Mega   1000^2
%g - Giga   1000^3
%t - Tera   1000^4
%p - Peta   1000^5
%e - Exa    1000^6
%z - Zetta  1000^7
%y - Yotta  1000^8
%r - Ronna  1000^9
%q - Quetta 1000^10

%V - auto pick from below
%S - auto pick from below and print in long string format
%K - Kibi   1024^1
%M - Mebi   1024^2
%G - Gibi   1024^3
%T - Tebi   1024^4
%P - Pebi   1024^5
%E - Exbi   1024^6
%Z - Qebi   1024^7
%Y - Yobi   1024^8
%R - Robi   1024^9
%Q - Qubi   1024^10
```

To use the values, one can use either Int64() or Int()/Float() to retrieve the
values.  When dealing with rates, the returned value will be scaled to the
second value.

The standard verb modifiers can be applied, like `%0.5v`.

```golang
  // Print out a unit in KiB format (1000):
  fmt.Printf("%%k = %k\n", bunit.NewBits(32768))
  // %k = 32.768Kib

  // Print out a unit in kB format (1024):
  fmt.Printf("%%K = %K\n", bunit.NewBits(32768))
  // %K = 32kb

  // Print out a unit in MB format (1024^2):
  fmt.Printf("%%M = %M\n", bunit.NewBits(32768))
  // %M = 0.03125Mb

  // Print out a unit in Auto Byte format:
  fmt.Printf("%%0.5v = %0.5v\n", bunit.NewBits(13529101000))
  // %0.5v = 13.529Gb
```

Documentation and examples can be found here:

https://pkg.go.dev/github.com/pschou/go-bunit
