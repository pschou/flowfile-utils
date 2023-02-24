# IOThrottler

This routine is a channel driven, time adding, bandwidth limiting routine which
delays the write command a specific amount of time derived from the previous
call.  This effectively makes a bandwidth shaper for writes to a connection.

```golang
  throttler := iothrottler.NewIOThrottler(100 << 20) // simulate a 100Mbps
  throttler.SendN(50 << 20) // Send 50Mb
  // Write command here to send 50Mb

  throttler.SendN(75 << 20) // Send 75Mb
  // Write command here to send 75Mb
```
