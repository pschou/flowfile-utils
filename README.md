# FlowFile-Utils

A set of FlowFile routines for working with NiFi feeds.  The utilities include:

[NiFi-Sender](#NiFi-Sender) - take a file or directory and send them to a NiFi endpoint

[NiFi-to-UDP](#NiFi-to-UDP) - takes a NiFi feed and sends it to a UDP endpoint

[UDP-to-NiFi](#UDP-to-NiFi) - listens for a UDP feed and sends it to a NiFi endpoint

[NiFi-Sink](#NiFi-Sink) - a listener that listens and accepts FlowFiles and does nothing

[NiFi-Flood](#NiFi-Flood) - a sender which sends a continuous stream of NiFi FlowFiles

[NiFi-Receiver](#NiFi-Receiver) - take a NiFi feed and save off files while doing checksums for validity

[NiFi-Stager](#NiFi-Stager) - take a NiFi feed and temporarily store them to disk for processing later

[NiFi-Unstager](#NiFi-Unstager) - listens to a directory of staged files and send them to a NiFi endpoint

[NiFi-Diode](#NiFi-Diode) - takes a NiFi feed and forwards the FlowFiles to another NiFi (assures one direction)

[NiFi-to-KCP](#NiFi-to-KCP) - takes a NiFi feed and sends it to a KCP listener, enabling forward error correction (FEC)

[KCP-to-NiFi](#KCP-to-NiFi) - listens for a KCP feed and sends it to a NiFi endpoint, verifying and correcting errors


For more documentation about the go-flowfile library: https://pkg.go.dev/github.com/pschou/go-flowfile .



## NiFi Sender

NiFi sender does one thing, it will take data from the disk and upload it to a
NiFi endpoint.  Some advantages of using this NiFi sender over a full NiFi
instance are:

- One does not need to install NiFi or have it running

- It is ultra portable and can run on a minimal instance

- Enables segmenting, so an upstream stream handler with limited capabilities can get segments instead of a whole file

![NiFi-Sender](images/NiFi-Sender.png)

NiFi-Sender Usage:
```
NiFi Sender (github.com/pschou/flowfile-utils, version: 0.1.20230216.1006)

This utility is intended to capture a set of files or directory of files and
send them to a remote NiFi server for processing.

Usage: ../nifi-sender [options] path1 path2...
  -CA string
    	A PEM encoded CA's certificate file. (default "someCertCAFile")
  -attributes string
    	File with additional attributes to add to FlowFiles
  -cert string
    	A PEM encoded certificate file. (default "someCertFile")
  -debug
    	Turn on debug in FlowFile library
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -no-dedup
    	Deduplication by checksums (default true)
  -retries int
    	Retries after failing to send a file to NiFi listening point (default 5)
  -retry-timeout duration
    	Time between retries (default 10s)
  -update-chain
    	Update the connection chain attributes: "custodyChain.#.*"
    	To disable use -update-chain=false (default true)
  -url string
    	Where to send the files (default "http://localhost:8080/contentListener")
  -v	Turn on verbose (shorthand)
  -verbose
    	Turn on verbose
```

Example:
```
$ ./nifi-sender -url http://localhost:8080/contentListener file1.dat file2.dat myDir/
2023/02/06 08:43:26 creating sender...
2023/02/06 08:43:26   sending file1.dat ...
2023/02/06 08:43:26   sending file2.dat ...
2023/02/06 08:43:26   sending empty dir myDir/ ...
2023/02/06 08:43:26 done.
```

## NiFi to UDP

NiFi to UDP listens on a NiFi endpoint and forwards all NiFi connections to an
array of UDP ports.  This is intended to be a one way fire and forget setup
where the sender has no idea if the receiver got the packets, but the FlowFile
payload is broken up into indexed frames and sent so order can be restored on
the receiving side.

![NiFi-to-UDP](images/NiFi-to-UDP.png)

NiFi-to-UDP Usage:
```
NiFi -to-> UDP (github.com/pschou/flowfile-utils, version: 0.1.20230216.1006)

This utility is intended to take input over a NiFi compatible port and pass all
FlowFiles to a UDP endpoint after verifying checksums.  A chain of custody is
maintained by adding an action field with "NIFI-UDP" value.

Note: The port range used in the source UDP address directly affect the number
of concurrent sessions, and as payloads are buffered in memory (to do the
checksum) the memory bloat can be upwards on the order of NUM_PORTS *
MAX_PAYLOAD.  Please choose wisely.

The resend-delay will add latency (by delaying new connections until second
send is complete) but will add error resilience in the transfer.  In other
words, shortening the delay will likely mean more errors, while increaing will
slow down the number of accepted HTTP connections upstream.

Usage: ../nifi-to-udp [options]
  -CA string
    	A PEM encoded CA's certificate file. (default "someCertCAFile")
  -attributes string
    	File with additional attributes to add to FlowFiles
  -cert string
    	A PEM encoded certificate file. (default "someCertFile")
  -debug
    	Turn on debug in FlowFile library
  -init-script string
    	Shell script to be called on start
    	Used to manually setup the networking interfaces when this program is called from GRUB
  -init-script-shell string
    	Shell to be used for init script run (default "/bin/bash")
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -listen string
    	Where to listen to incoming connections (example 1.2.3.4:8080) (default ":8080")
  -listenPath string
    	Path in URL where to expect FlowFiles to be posted (default "/contentListener")
  -max-http-sessions int
    	Limit the number of allowed incoming HTTP connections (default 20)
  -mtu int
    	Maximum transmit unit (default 1200)
  -resend-delay duration
    	Time between first transmit and second, set to 0s to disable. (default 1s)
  -segment-max-size string
    	Set a maximum size for partitioning files in sending
  -throttle duration
    	Additional seconds per frame
    	This scales up with concurrent connections (set to 0s to disable) (default 600ns)
  -throttle-gap duration
    	Inter-packet gap
    	This is the time added after all packet (set to 0s to disable) (default 60ns)
  -tls
    	Enforce TLS secure transport on incoming connections
  -udp-dst-addr string
    	Target IP:PORT for UDP packet (default "10.12.128.249:2100-2200")
  -udp-src-addr string
    	Source IP:PORT for UDP packet (default "10.12.128.249:3100-3200")
  -update-chain
    	Update the connection chain attributes: "custodyChain.#.*"
    	To disable use -update-chain=false (default true)
  -v	Turn on verbose (shorthand)
  -verbose
    	Turn on verbose
  -watchdog duration
    	Trigger a reboot if no connection is seen within this time window
    	You'll neet to make sure you have the watchdog module enabled on the host and kernel.
    	Default is disabled (-watchdog=0s)
```

Example:
```
$ ./nifi-to-udp -listen :8082 -throttle 167us -throttle-gap 67ns -segment-max-size 10MB
2023/02/15 12:40:01 Creating senders for UDP from: 10.12.128.249:3100-3200
2023/02/15 12:40:01 Creating destinations for UDP: 10.12.128.249:2100-2200
2023/02/15 12:40:01 Creating listener on: :8082
2023/02/15 12:40:01 Setting max-size to 10.00MB
2023/02/15 12:40:01 Listening with HTTP on :8082 at /contentListener
```


## UDP to NiFi

UDP to NiFi listens on an array of UDP endpoint and forwards all FlowFiles to a
NiFi connection after doing consistency checks.  Here the heavy work is done to 
reconstruct a FlowFile and then do a checksum before forwarding onward.

![UDP-to-NiFi](images/UDP-to-NiFi.png)

UDP-to-NiFi Usage:
```
UDP -to-> NiFi (github.com/pschou/flowfile-utils, version: 0.1.20230216.1006)

This utility is intended to take input via UDP pass all FlowFiles to a UDP
endpoint after verifying checksums.  A chain of custody is maintained by adding
an action field with "UDP-NIFI" value.

Usage: ../udp-to-nifi [options]
  -CA string
    	A PEM encoded CA's certificate file. (default "someCertCAFile")
  -attributes string
    	File with additional attributes to add to FlowFiles
  -cert string
    	A PEM encoded certificate file. (default "someCertFile")
  -debug
    	Turn on debug in FlowFile library
  -init-script string
    	Shell script to be called on start
    	Used to manually setup the networking interfaces when this program is called from GRUB
  -init-script-shell string
    	Shell to be used for init script run (default "/bin/bash")
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -mtu int
    	MTU payload size for pre-allocating memory (default 1400)
  -no-checksums
    	Ignore doing checksum checks
  -udp-dst-ip string
    	Local target IP:PORT for UDP packet (default ":2100-2200")
  -update-chain
    	Update the connection chain attributes: "custodyChain.#.*"
    	To disable use -update-chain=false (default true)
  -url string
    	Where to send the files (default "http://localhost:8080/contentListener")
  -v	Turn on verbose (shorthand)
  -verbose
    	Turn on verbose
  -watchdog duration
    	Trigger a reboot if no connection is seen within this time window
    	You'll neet to make sure you have the watchdog module enabled on the host and kernel.
    	Default is disabled (-watchdog=0s)
```

Example:
```
$ ../udp-to-nifi
2023/02/15 12:41:13 Creating NiFi sender, http://localhost:8080/contentListener
2023/02/15 12:41:13 Listening on UDP :2100-2200
```

## NiFi Sink

NiFi listens on a NiFi endpoint and accepts every file while doing nothing.

![NiFi-Sink](images/NiFi-Sink.png)

NiFi-Sink Usage:
```
NiFi Sink (github.com/pschou/flowfile-utils, version: 0.1.20230216.1006)

This utility is intended to listen for FlowFiles on a NifI compatible port and
drop them as fast as they come in

Usage: ../nifi-sink [options]
  -CA string
    	A PEM encoded CA's certificate file. (default "someCertCAFile")
  -cert string
    	A PEM encoded certificate file. (default "someCertFile")
  -debug
    	Turn on debug in FlowFile library
  -init-script string
    	Shell script to be called on start
    	Used to manually setup the networking interfaces when this program is called from GRUB
  -init-script-shell string
    	Shell to be used for init script run (default "/bin/bash")
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -listen string
    	Where to listen to incoming connections (example 1.2.3.4:8080) (default ":8080")
  -listenPath string
    	Path in URL where to expect FlowFiles to be posted (default "/contentListener")
  -segment-max-size string
    	Set a maximum size for partitioning files in sending
  -tls
    	Enforce TLS secure transport on incoming connections
  -update-chain
    	Update the connection chain attributes: "custodyChain.#.*"
    	To disable use -update-chain=false (default true)
  -v	Turn on verbose (shorthand)
  -verbose
    	Turn on verbose
  -watchdog duration
    	Trigger a reboot if no connection is seen within this time window
    	You'll neet to make sure you have the watchdog module enabled on the host and kernel.
    	Default is disabled (-watchdog=0s)
```

Example:

```
$ ./nifi-sink -v
2023/02/15 20:04:34 Listening with HTTP on :8080 at /contentListener
 - [{"Name":"path","Value":"./"},{"Name":"custodyChain.0.time","Value":"2023-02-15T20:04:37-05:00"},{"Name":"custodyChain.0.local.hostname","Value":"centos7.schou.me"},{"Name":"custodyChain.0.action","Value":"FLOOD"},{"Name":"filename","Value":"file0001.dat"},{"Name":"uuid","Value":"8a7ad2d7-9e6c-4b85-b2af-6a7b59dd8ed4"},{"Name":"checksumType","Value":"SHA1"},{"Name":"checksum","Value":"8bf6dc847a281d5042d681eb93b760f3d00c8df3"}]
2023/02/15 20:04:39     Checksum passed for file/segment file0000.dat 318.5MB
  - [{"Name":"path","Value":"./"},{"Name":"custodyChain.0.time","Value":"2023-02-15T20:04:37-05:00"},{"Name":"custodyChain.0.local.hostname","Value":"centos7.schou.me"},{"Name":"custodyChain.0.action","Value":"FLOOD"},{"Name":"filename","Value":"file0003.dat"},{"Name":"uuid","Value":"170c7599-dfe3-43ba-9499-ca97f3548dec"},{"Name":"checksumType","Value":"SHA1"},{"Name":"checksum","Value":"7a4e3a3ab5ef4656d391bcf54657b4e1436bd866"}]
  - [{"Name":"path","Value":"./"},{"Name":"custodyChain.0.time","Value":"2023-02-15T20:04:37-05:00"},{"Name":"custodyChain.0.local.hostname","Value":"centos7.schou.me"},{"Name":"custodyChain.0.action","Value":"FLOOD"},{"Name":"filename","Value":"file0002.dat"},{"Name":"uuid","Value":"f73084ea-97d8-4961-85d5-03a68de6382d"},{"Name":"checksumType","Value":"SHA1"},{"Name":"checksum","Value":"424e243fec99ce3d37086e76ae78c2b3dfe9a019"}]
2023/02/15 20:04:40     Checksum passed for file/segment file0001.dat 447.1MB
```

## NiFi Flood

NiFi Flood sends files (of various sizes) to a NiFi endpoint to saturate the bandwidth.

![NiFi-Flood](images/NiFi-Flood.png)

NiFi-Flood Usage:
```
NiFi Flood (github.com/pschou/flowfile-utils, version: 0.1.20230216.1006)

This utility is intended to saturate the bandwidth of a NiFi endpoint for
load testing.

Usage: ../nifi-flood [options]
  -CA string
    	A PEM encoded CA's certificate file. (default "someCertCAFile")
  -attributes string
    	File with additional attributes to add to FlowFiles
  -cert string
    	A PEM encoded certificate file. (default "someCertFile")
  -debug
    	Turn on debug in FlowFile library
  -hash string
    	Hash to use in checksum value (default "SHA1")
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -max string
    	Max payload size for upload in bytes (default "20MB")
  -min string
    	Min Payload size for upload in bytes (default "10MB")
  -name-format string
    	File naming format (default "file%04d.dat")
  -threads int
    	Parallel concurrent uploads (default 4)
  -update-chain
    	Update the connection chain attributes: "custodyChain.#.*"
    	To disable use -update-chain=false (default true)
  -url string
    	Where to send the files (default "http://localhost:8080/contentListener")
  -v	Turn on verbose (shorthand)
  -verbose
    	Turn on verbose
```

Example:
```
$ ../nifi-flood  -max 1G
2023/02/15 20:04:37 Sending...
2023/02/15 20:04:38 3 sending file0000.dat 318.5MB
2023/02/15 20:04:38 1 sending file0001.dat 447.1MB
2023/02/15 20:04:40 2 sending file0003.dat 921.9MB
2023/02/15 20:04:40 0 sending file0002.dat 950MB
```

## NiFi Receiver

NiFi Receiver listens on a port for NiFi FlowFiles and then acts on them accordingly as they are streamed in.

![NiFi-Receiver](images/NiFi-Receiver.png)

NiFi-Receiver Usage:
```
NiFi Receiver (github.com/pschou/flowfile-utils, version: 0.1.20230216.1006)

This utility is intended to listen for FlowFiles on a NifI compatible port and
then parse these files and drop them to disk for usage elsewhere.

Usage: ../nifi-receiver [options]
  -CA string
    	A PEM encoded CA's certificate file. (default "someCertCAFile")
  -cert string
    	A PEM encoded certificate file. (default "someCertFile")
  -debug
    	Turn on debug in FlowFile library
  -init-script string
    	Shell script to be called on start
    	Used to manually setup the networking interfaces when this program is called from GRUB
  -init-script-shell string
    	Shell to be used for init script run (default "/bin/bash")
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -listen string
    	Where to listen to incoming connections (example 1.2.3.4:8080) (default ":8080")
  -listenPath string
    	Path in URL where to expect FlowFiles to be posted (default "/contentListener")
  -path string
    	Directory in which to place files received (default "./output/")
  -rm
    	Automatically remove file after script has finished
  -script string
    	Shell script to be called on successful post
  -script-shell string
    	Shell to be used for script run (default "/bin/bash")
  -segment-max-size string
    	Set a maximum size for partitioning files in sending
  -tls
    	Enforce TLS secure transport on incoming connections
  -update-chain
    	Update the connection chain attributes: "custodyChain.#.*"
    	To disable use -update-chain=false (default true)
  -v	Turn on verbose (shorthand)
  -verbose
    	Turn on verbose
  -watchdog duration
    	Trigger a reboot if no connection is seen within this time window
    	You'll neet to make sure you have the watchdog module enabled on the host and kernel.
    	Default is disabled (-watchdog=0s)
```

Example:
```
$ ./nifi-receiver
Output set to ./output/
2023/02/06 08:58:25 Listening with HTTP on :8080 at /contentListener
2023/02/06 08:58:28   Receiving nifi file output/file1.dat size 18
2023/02/06 08:58:28   Verified file output/file1.dat
2023/02/06 08:58:28   Receiving nifi file output/file2.dat size 10
2023/02/06 08:58:28   Verified file output/file2.dat
```

if one wants to act on the files after they arrive, they can add a script
caller which performs functions on the files just after a successful send:

```
$ cat script.sh
#!/bin/bash
echo In Script, doing something:
sha256sum "$1"
$ ./nifi-receiver -script script.sh -verbose
Output set to ./output/
2023/02/06 08:59:38 Listening with HTTP on :8080 at /contentListener
2023/02/06 08:59:40   Receiving nifi file output/file1.dat size 18
    [{"Name":"path","Value":"./"},{"Name":"filename","Value":"file1.dat"},{"Name":"modtime","Value":"2023-02-06T08:47:47-05:00"},{"Name":"checksum-type","Value":"SHA256"},{"Name":"checksum","Value":"51fd71b1368a1b130b60cab1301b05bbef470cf4a21ef2956553def809edf4ec"},{"Name":"uuid","Value":"271d19fd-827a-4c9d-a21e-7ede9d652120"}]
2023/02/06 08:59:40   Verified file output/file1.dat
2023/02/06 08:59:40   Calling script /bin/bash script.sh output/file1.dat
2023/02/06 08:59:40 ----- START script.sh output/file1.dat -----
In Script, doing something:
51fd71b1368a1b130b60cab1301b05bbef470cf4a21ef2956553def809edf4ec  output/file1.dat

2023/02/06 08:59:40 ----- END script.sh output/file1.dat -----
2023/02/06 08:59:40   Receiving nifi file output/file2.dat size 10
    [{"Name":"path","Value":"./"},{"Name":"filename","Value":"file2.dat"},{"Name":"modtime","Value":"2023-02-06T08:47:53-05:00"},{"Name":"checksum-type","Value":"SHA256"},{"Name":"checksum","Value":"1e26ce5588db2ef5080a3df10385a731af2a4bfd0d2515f691d05d9dd900e18a"},{"Name":"uuid","Value":"08f48cce-b09f-47b7-9e5f-fbd0bf2e2b56"}]
2023/02/06 08:59:40   Verified file output/file2.dat
2023/02/06 08:59:40   Calling script /bin/bash script.sh output/file2.dat
2023/02/06 08:59:40 ----- START script.sh output/file2.dat -----
In Script, doing something:
1e26ce5588db2ef5080a3df10385a731af2a4bfd0d2515f691d05d9dd900e18a  output/file2.dat

2023/02/06 08:59:40 ----- END script.sh output/file2.dat -----
```

If one desires for the files to be removed after the script is ran:
```
$ ./nifi-receiver -script script.sh -rm
Output set to ./output/
2023/02/06 09:06:18 Listening with HTTP on :8080 at /contentListener
2023/02/06 09:06:20   Receiving nifi file output/file1.dat size 18
2023/02/06 09:06:20   Verified file output/file1.dat
2023/02/06 09:06:20   Calling script /bin/bash script.sh output/file1.dat
2023/02/06 09:06:20   Removed output/file1.dat
2023/02/06 09:06:20   Receiving nifi file output/file2.dat size 10
2023/02/06 09:06:20   Verified file output/file2.dat
2023/02/06 09:06:20   Calling script /bin/bash script.sh output/file2.dat
2023/02/06 09:06:20   Removed output/file2.dat
^C
$ ls output/
$
```

## NiFi Stager

This tool enables files to be layed down to disk, to be replayed at a later time or different location into a FlowFile feed.  Note that the binary payload that is layed down is FlowFile encoded and not parsed out for making sure the exact binary payload is replayed.

![NiFi-Stager](images/NiFi-Stager.png)

NiFi-Stager Usage:
```
NiFi Stager (github.com/pschou/flowfile-utils, version: 0.1.20230216.1006)

This utility is intended to take input over a NiFi compatible port and drop all
FlowFiles into directory along with associated attributes which can then be
unstaged using the NiFi Unstager.

Usage: ../nifi-stager [options]
  -CA string
    	A PEM encoded CA's certificate file. (default "someCertCAFile")
  -cert string
    	A PEM encoded certificate file. (default "someCertFile")
  -debug
    	Turn on debug in FlowFile library
  -init-script string
    	Shell script to be called on start
    	Used to manually setup the networking interfaces when this program is called from GRUB
  -init-script-shell string
    	Shell to be used for init script run (default "/bin/bash")
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -listen string
    	Where to listen to incoming connections (example 1.2.3.4:8080) (default ":8080")
  -listenPath string
    	Path in URL where to expect FlowFiles to be posted (default "/contentListener")
  -path string
    	Directory in which stage FlowFiles (default "stager")
  -rm
    	Automatically remove file after script has finished
  -rm-partial
    	Automatically remove partial files
    	To unset this default use -rm-partial=false . (default true)
  -script string
    	Shell script to be called on successful post
  -script-shell string
    	Shell to be used for script run (default "/bin/bash")
  -segment-max-size string
    	Set a maximum size for partitioning files in sending
  -tls
    	Enforce TLS secure transport on incoming connections
  -update-chain
    	Update the connection chain attributes: "custodyChain.#.*"
    	To disable use -update-chain=false (default true)
  -v	Turn on verbose (shorthand)
  -verbose
    	Turn on verbose
  -watchdog duration
    	Trigger a reboot if no connection is seen within this time window
    	You'll neet to make sure you have the watchdog module enabled on the host and kernel.
    	Default is disabled (-watchdog=0s)
```

Example:
```
$ ./nifi-stager
Output set to stager
2023/02/06 12:01:10 Listening with HTTP on :8080 at /contentListener
  Receiving nifi file file1.dat size 18
  Receiving nifi file file2.dat size 10
```

The sending side sends like this:
```
$ ./nifi-sender -url http://localhost:8080/contentListener file1.dat file2.dat
2023/02/06 12:03:06 creating sender...
2023/02/06 12:03:06   sending file1.dat ...
2023/02/06 12:03:06   sending file2.dat ...
2023/02/06 12:03:06 done.
```

Example with script triggered after a successful send:
```
$ cat stager_send.sh
#!/bin/bash
echo moving content "$1" to another folder /tmp
mv "$1" /tmp

$ ./nifi-stager  -script stager_send.sh -verbose -rm
Output set to stager
2023/02/06 12:03:00 Listening with HTTP on :8080 at /contentListener
  Receiving nifi file file1.dat size 18
    [{"Name":"path","Value":"./"},{"Name":"filename","Value":"file1.dat"},{"Name":"modtime","Value":"2023-02-06T08:47:47-05:00"},{"Name":"checksum-type","Value":"SHA256"},{"Name":"checksum","Value":"51fd71b1368a1b130b60cab1301b05bbef470cf4a21ef2956553def809edf4ec"},{"Name":"uuid","Value":"f0aa041f-3302-4358-acd1-136ba76078cf"}]
2023/02/06 12:03:06   Calling script /bin/bash stager_send.sh stager/60af9b0c-7f23-48a6-bc0e-4f44879e9f3a.dat stager/60af9b0c-7f23-48a6-bc0e-4f44879e9f3a.json
2023/02/06 12:03:06 ----- START stager_send.sh 60af9b0c-7f23-48a6-bc0e-4f44879e9f3a -----
moving content stager/60af9b0c-7f23-48a6-bc0e-4f44879e9f3a.dat to another folder /tmp

2023/02/06 12:03:06 ----- END stager_send.sh 60af9b0c-7f23-48a6-bc0e-4f44879e9f3a -----
2023/02/06 12:03:06   Removed 60af9b0c-7f23-48a6-bc0e-4f44879e9f3a
  Receiving nifi file file2.dat size 10
    [{"Name":"path","Value":"./"},{"Name":"filename","Value":"file2.dat"},{"Name":"modtime","Value":"2023-02-06T08:47:53-05:00"},{"Name":"checksum-type","Value":"SHA256"},{"Name":"checksum","Value":"1e26ce5588db2ef5080a3df10385a731af2a4bfd0d2515f691d05d9dd900e18a"},{"Name":"uuid","Value":"794f5452-7aae-4748-8218-6892a9b0b4b5"}]
2023/02/06 12:03:06   Calling script /bin/bash stager_send.sh stager/02b1ee5d-432a-478d-920b-0e3052dc2344.dat stager/02b1ee5d-432a-478d-920b-0e3052dc2344.json
2023/02/06 12:03:06 ----- START stager_send.sh 02b1ee5d-432a-478d-920b-0e3052dc2344 -----
moving content stager/02b1ee5d-432a-478d-920b-0e3052dc2344.dat to another folder /tmp

2023/02/06 12:03:06 ----- END stager_send.sh 02b1ee5d-432a-478d-920b-0e3052dc2344 -----
2023/02/06 12:03:06   Removed 02b1ee5d-432a-478d-920b-0e3052dc2344
^C
```

Note the '-rm' makes sure any leftover artifacts are deleted (to prevent the
staging directory to be cluttered with leftover FlowFiles).

Using the '-rm-partial=false' will keep files from being deleted if they fail verifications.

## NiFi Unstager

The purpose of the nifi-unstager is to replay the files layed to disk in the nifi-stager operation.

![NiFi-Unstager](images/NiFi-Unstager.png)

NiFi-Unstager Usage:
```
NiFi Unstager (github.com/pschou/flowfile-utils, version: 0.1.20230216.1006)

This utility is intended to take a directory of NiFi FlowFiles and ship them
out to a listening NiFi endpoint while maintaining the same set of attribute
headers.

Usage: ../nifi-unstager [options]
  -CA string
    	A PEM encoded CA's certificate file. (default "someCertCAFile")
  -attributes string
    	File with additional attributes to add to FlowFiles
  -cert string
    	A PEM encoded certificate file. (default "someCertFile")
  -debug
    	Turn on debug in FlowFile library
  -init-script string
    	Shell script to be called on start
    	Used to manually setup the networking interfaces when this program is called from GRUB
  -init-script-shell string
    	Shell to be used for init script run (default "/bin/bash")
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -path string
    	Directory which to scan for FlowFiles (default "stager")
  -retries int
    	Retries after failing to send a file to NiFi listening point (default 5)
  -retry-timeout duration
    	Time between retries (default 10s)
  -update-chain
    	Update the connection chain attributes: "custodyChain.#.*"
    	To disable use -update-chain=false (default true)
  -url string
    	Where to send the files (default "http://localhost:8080/contentListener")
  -v	Turn on verbose (shorthand)
  -verbose
    	Turn on verbose
  -watchdog duration
    	Trigger a reboot if no connection is seen within this time window
    	You'll neet to make sure you have the watchdog module enabled on the host and kernel.
    	Default is disabled (-watchdog=0s)
```

Example:

The unstager listens to a directory and sends files to a remote url:
```
$ ./nifi-unstager -url http://localhost:8080/contentListener -path stager
2023/02/06 12:22:10 Creating FlowFile sender to url http://localhost:8080/contentListener
2023/02/06 12:22:10 Creating directory listener on stager
  Unstaging file file2.dat
  Unstaging file file1.dat
```

The remote side sees the files come in as if they were just sent out from a NiFi server.
```
$ ./nifi-receiver
Output set to ./output/
2023/02/06 12:22:09 Listening with HTTP on :8080 at /contentListener
2023/02/06 12:22:10   Receiving nifi file output/file2.dat size 10
2023/02/06 12:22:10   Verified file output/file2.dat
2023/02/06 12:22:10   Receiving nifi file output/file1.dat size 18
2023/02/06 12:22:10   Verified file output/file1.dat
^C
```

## NiFi to KCP

NiFi to KCP tool will take a TCP FlowFile session, add forward error
correction, and stream it to a KCP listening server to reduce the latency over
long distances. As the sets of FlowFiles are handled as a continuous block,
and the entire block determines the success or failure-- one should send
batches of many FlowFiles instead of one at a time, but not too many, to not
have to restart if the connection gets lost.

![NiFi-to-KCP](images/NiFi-to-KCP.png)

NiFi-to-KCP Usage:
```
NiFi -to-> KCP (github.com/pschou/flowfile-utils, version: 0.1.20230216.1006)

This utility is intended to take input over a NiFi compatible port and pass all
FlowFiles into KCP endpoint for speeding up throughput over long distances.

Usage: ../nifi-to-kcp [options]
  -CA string
    	A PEM encoded CA's certificate file. (default "someCertCAFile")
  -cert string
    	A PEM encoded certificate file. (default "someCertFile")
  -debug
    	Turn on debug in FlowFile library
  -dscp int
    	set DSCP(6bit) (default 46)
  -init-script string
    	Shell script to be called on start
    	Used to manually setup the networking interfaces when this program is called from GRUB
  -init-script-shell string
    	Shell to be used for init script run (default "/bin/bash")
  -kcp string
    	Target KCP server to send flowfiles (default "10.12.128.249:2112")
  -kcp-data int
    	Number of data packets to send in a FEC grouping (default 10)
  -kcp-parity int
    	Number of parity packets to send in a FEC grouping (default 3)
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -listen string
    	Where to listen to incoming connections (example 1.2.3.4:8080) (default ":8080")
  -listenPath string
    	Path in URL where to expect FlowFiles to be posted (default "/contentListener")
  -mtu int
    	set maximum transmission unit for UDP packets (default 1350)
  -no-checksums
    	Ignore doing checksum checks
  -rcvwnd int
    	set receive window size(num of packets) (default 128)
  -readbuf int
    	per-socket read buffer in bytes (default 4194304)
  -segment-max-size string
    	Set a maximum size for partitioning files in sending
  -sndwnd int
    	set send window size(num of packets) (default 1024)
  -threads int
    	Parallel concurrent uploads (default 40)
  -tls
    	Enforce TLS secure transport on incoming connections
  -update-chain
    	Update the connection chain attributes: "custodyChain.#.*"
    	To disable use -update-chain=false (default true)
  -v	Turn on verbose (shorthand)
  -verbose
    	Turn on verbose
  -watchdog duration
    	Trigger a reboot if no connection is seen within this time window
    	You'll neet to make sure you have the watchdog module enabled on the host and kernel.
    	Default is disabled (-watchdog=0s)
  -writebuf int
    	per-socket write buffer in bytes (default 16777217)
```

Example:

```
$ ./nifi-to-kcp -listen :8082  -kcp-data 10 -kcp-parity 3 -v -segment-max-size 3MB
2023/02/15 22:23:19 Creating sender,
2023/02/15 22:23:19 Setting max-size to 3.00MB
2023/02/15 22:23:19 Listening with HTTP on :8082 at /contentListener
```

## KCP to NiFi

KCP FlowFile server listening for connections from the NiFi-to-KCP and then
forwarding the FlowFiles to a NiFi compatible port while correcting errors in
transmission.

![KCP-to-NiFi](images/KCP-to-NiFi.png)

KCP-to-NiFi Usage:
```
KCP -to-> NiFi (github.com/pschou/flowfile-utils, version: 0.1.20230216.1006)

This utility is intended to take input over a KCP connection and send FlowFiles
into a NiFi compatible port for speeding up throughput over long distances.

Usage: ../kcp-to-nifi [options]
  -CA string
    	A PEM encoded CA's certificate file. (default "someCertCAFile")
  -attributes string
    	File with additional attributes to add to FlowFiles
  -cert string
    	A PEM encoded certificate file. (default "someCertFile")
  -debug
    	Turn on debug in FlowFile library
  -init-script string
    	Shell script to be called on start
    	Used to manually setup the networking interfaces when this program is called from GRUB
  -init-script-shell string
    	Shell to be used for init script run (default "/bin/bash")
  -kcp string
    	Listen port for KCP connections (default ":2112")
  -kcp-data int
    	Number of data packets to send in a FEC grouping (default 10)
  -kcp-parity int
    	Number of parity packets to send in a FEC grouping (default 3)
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -mtu int
    	set maximum transmission unit for UDP packets (default 1350)
  -no-checksums
    	Ignore doing checksum checks
  -rcvwnd int
    	set receive window size(num of packets) (default 1024)
  -sndwnd int
    	set send window size(num of packets) (default 128)
  -update-chain
    	Update the connection chain attributes: "custodyChain.#.*"
    	To disable use -update-chain=false (default true)
  -url string
    	Where to send the files (default "http://localhost:8080/contentListener")
  -v	Turn on verbose (shorthand)
  -verbose
    	Turn on verbose
  -watchdog duration
    	Trigger a reboot if no connection is seen within this time window
    	You'll neet to make sure you have the watchdog module enabled on the host and kernel.
    	Default is disabled (-watchdog=0s)
```

Example:

```
$ ./kcp-to-nifi  -v -kcp-data 10 -kcp-parity 3
2023/02/15 22:26:18 Creating sender, http://localhost:8080/contentListener
```

## NiFi Diode

A simple NiFi Diode that does one thing, takes in data and passes it on to
another NiFi without letting anything go the other direction.  Hence it is a
simple, no-cache-diode.

The idea here is this server listens on a IP:Port and then any incomming
connection is streamed to another IP:Port, but data can only transfer one way.
The sending side will have no idea what server it is sending to nor be able to
get any information from the downstream NiFi.

Think of the NiFi diode like a one way glass.  The downstream NiFi sees
everything from the sending NiFi server as if the diode is not there but the
sending side only sees "NiFi Diode".  So if the downstream NiFi is expecting a
POST to go to an alternate content listener URL, the client must be configured
to point to the same URL as the diode will only reply with success if a success
happens.  No notifications are sent to the sending side what may have gone
wrong if a transfer fails.  This is to ensure nothing gets leaked back to the
sending side via server replies.

The only fields which are changed going through the diode in the forward
direction are the `Host` field (which NiFi needs to ensure connection
authenticity) and `X-Forwarded-For` which lists the remote endpoint(s).  If
multiple NiFi diode servers are stacked one upon another, the X-Forwarded-For
will have a complete list of source IP addresses seperated by commas.


![NiFi-Diode diagram showing a NiFi box on the left, and arrow representing a TCP flow pointing to a NiFi Diode in the middle, and another arrow to the right going to a NiFi box on the right, again representing a TCP flow](images/NiFi-Diode.png)

### Why do I need this?

- Free and open-source (FOSS is good)
- Has a memory footprint of 1-50 MB. (It runs on cheap hardware)
- Can run directly from a kernel INIT= call - Avoid purchasing a dedicated
  hardware appliance.  Download your favorite distro and then modify the grub
  init to point to this binary instead of the standard init and you have yourself
  a linux based "hardware" NiFi diode.
- Standard data diode protections apply - place this on a box in a locked server
  room, and by denying physical access, you effectively prohibit data from going
  the wrong way. (It's a win-win!)
- Threadable, the server handles multiple concurrent flows which can help with
  latency issues and increase throughput.

What are the pitfalls?

- If the server dies, who knows for what reason, you need to get to the
  physical server to restore the diode.  A watchdog timer is included for this
  purpose, so set the timeout to an acceptable outage window size.  (overall this
  seems like a good risk when preventing reverse data flow)
- It's command line, so you have to know Linux. (my manager included this, but
  techies should know Linux)
- If you like burning all your data to a DVD and sneaker netting it between
  buildings, you'll have to find a gym now.  :(

NiFi-Diode Usage:
```
NiFi Diode (github.com/pschou/flowfile-utils, version: 0.1.20230216.1006)

This utility is intended to take input over a NiFi compatible port and pass all
FlowFiles into another NiFi port while updating the attributes with the
certificate and chaining any previous certificates.

Usage: ../nifi-diode [options]
  -CA string
    	A PEM encoded CA's certificate file. (default "someCertCAFile")
  -attributes string
    	File with additional attributes to add to FlowFiles
  -cert string
    	A PEM encoded certificate file. (default "someCertFile")
  -debug
    	Turn on debug in FlowFile library
  -init-script string
    	Shell script to be called on start
    	Used to manually setup the networking interfaces when this program is called from GRUB
  -init-script-shell string
    	Shell to be used for init script run (default "/bin/bash")
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -listen string
    	Where to listen to incoming connections (example 1.2.3.4:8080) (default ":8080")
  -listenPath string
    	Path in URL where to expect FlowFiles to be posted (default "/contentListener")
  -no-checksums
    	Ignore doing checksum checks
  -segment-max-size string
    	Set a maximum size for partitioning files in sending
  -tls
    	Enforce TLS secure transport on incoming connections
  -update-chain
    	Update the connection chain attributes: "custodyChain.#.*"
    	To disable use -update-chain=false (default true)
  -url string
    	Where to send the files (default "http://localhost:8080/contentListener")
  -v	Turn on verbose (shorthand)
  -verbose
    	Turn on verbose
  -watchdog duration
    	Trigger a reboot if no connection is seen within this time window
    	You'll neet to make sure you have the watchdog module enabled on the host and kernel.
    	Default is disabled (-watchdog=0s)
```

# Example:

Here are some examples of the nifi-sender and nifi-receiver in action.  To set things up, we need some fake data first:

```
source$ dd if=/dev/urandom of=infile_rnd.dat count=100000
```

## Sender and receiver

Setting up the NiFi receiver first:
```
target$ ./nifi-receiver
Output set to ./output/
2023/02/06 20:20:03 Listening with HTTP on :8080 at /contentListener
```

We can now send a file:
```
source$ ./nifi-sender -url=http://localhost:8080/contentListener output2/
2023/02/06 20:20:16 creating sender...
2023/02/06 20:20:16   sending empty dir output2/a ...
2023/02/06 20:20:16   sending empty dir output2/b ...
2023/02/06 20:20:16   sending output2/infile.dat ...
2023/02/06 20:20:16   sending output2/infile_rnd.dat ...
2023/02/06 20:20:17   sending output2/output/infile_rnd.dat ...
2023/02/06 20:20:18   sending output2/random ...
2023/02/06 20:20:22 done.
```

Back at the NiFi receiver side:
```
target$ ./nifi-receiver
Output set to ./output/
2023/02/06 20:20:03 Listening with HTTP on :8080 at /contentListener
2023/02/06 20:20:16   Receiving nifi flowfile output/output2/infile.dat size 512000
2023/02/06 20:20:16   Verified file output/output2/infile.dat
2023/02/06 20:20:16   Receiving nifi flowfile output/output2/infile_rnd.dat size 51200000
2023/02/06 20:20:17   Verified file output/output2/infile_rnd.dat
2023/02/06 20:20:18   Receiving nifi flowfile output/output2/output/infile_rnd.dat size 51200000
2023/02/06 20:20:18   Verified file output/output2/output/infile_rnd.dat
2023/02/06 20:20:20   Receiving nifi flowfile output/output2/random size 153600000
2023/02/06 20:20:22   Verified file output/output2/random
```

The files have been sent and dropped to the folder output

## Sender, diode, and receiver

Here we will look at tying 3 of these utilities together, in this order we setup the NiFi receiver first:
```
target$ ./nifi-receiver -path output2
2023/02/02 14:57:33 Listening with HTTP on :8080 at /contentListener
```

Setup the diode to make the connections in-between:
```
diode$ ./nifi-diode -segment-max-size 10MB -url=http://localhost:8080/contentListener -listen :8082
2023/02/02 14:58:28 creating sender...
2023/02/02 14:58:28 Listening with HTTP on :8082 at /contentListener
```

and finally we send a file:
```
source$ ./nifi-sender -url=http://localhost:8082/contentListener infile_rnd.dat
2023/02/02 14:59:04 creating sender...
2023/02/02 14:59:04   sending infile_rnd.dat ...
```

back at the target, we can verify that the file has shown up:
```
target$ ls output2/
infile_rnd.dat
```

## Restrict size throughput

```bash
$ ./nifi-diode -segment-max-size 10MB -url https://localhost:8080/contentListener -listen :8082 -tls
```

## Add Additional Custom Attributes

```bash
$ cat example_attributes.yml
# Some attributes example, this is one-per-line and "key: value" format
MY_poc:     dan
MY_sidecar: 123
MY_group:   TeamA

$ ./nifi-diode -segment-max-size 10MB -CA test/ca_cert_DONOTUSE.pem -key test/npe2_key_DONOTUSE.pem -cert test/npe2_cert_DONOTUSE.pem  -url https://localhost:8080/contentListener -listen :8082 -tls -attributes example_attributes.yml
```

## Chain of Custody

When using these flowfile-util tools, the attributes are updated to include
chain of custody details,  Here is an example of one such set of metadata from
a flow that went through two diodes, put to disk in a staging folder, and then
sent to an additional two diodes.

```json
[
  {"Name":"path","Value":"output2/"},
  {"Name":"filename","Value":"infile_rnd.dat"},
  {"Name":"file.lastModifiedTime","Value":"2023-02-03T12:17:36-05:00"},
  {"Name":"file.creationTime","Value":"2023-02-03T12:17:36-05:00"},
  {"Name":"custodyChain.4.action","Value":"SENDER"},
  {"Name":"custodyChain.4.time","Value":"2023-02-09T13:46:46-05:00"},
  {"Name":"custodyChain.4.local.hostname","Value":"centos7.schou.me"},
  {"Name":"fragment.identifier","Value":"4416fe67-7138-43a1-b6c3-05db0a91a080"},
  {"Name":"segment.original.size","Value":"51200000"},
  {"Name":"segment.original.filename","Value":"infile_rnd.dat"},
  {"Name":"segment.original.checksumType","Value":"SHA256"},
  {"Name":"segment.original.checksum","Value":"3663d5284cc37ffcafc21b9425a231389c7661d569adf4e18835998c18463f7d"},
  {"Name":"merge.reason","Value":"MAX_BYTES_THRESHOLD_REACHED"},
  {"Name":"fragment.offset","Value":"10485760"},
  {"Name":"fragment.index","Value":"2"},
  {"Name":"fragment.count","Value":"5"},
  {"Name":"uuid","Value":"866c54c4-c045-4fba-8939-39e7f50d6e19"},
  {"Name":"checksumType","Value":"SHA256"},
  {"Name":"checksum","Value":"7509c1318832ae0f69b373db94315b81c59b18d596af4a31429e66c16f63565a"},
  {"Name":"MY_poc","Value":"dan"},
  {"Name":"MY_sidecar","Value":"123"},
  {"Name":"MY_group","Value":"TeamA"},
  {"Name":"custodyChain.3.action","Value":"DIODE"},
  {"Name":"custodyChain.3.time","Value":"2023-02-09T13:46:46-05:00"},
  {"Name":"custodyChain.3.local.hostname","Value":"centos7.schou.me"},
  {"Name":"custodyChain.3.user.dn","Value":"CN=localhost,O=Global Security npe4,C=US"},
  {"Name":"custodyChain.3.issuer.dn","Value":"CN=localhost,O=Test Security,C=US"},
  {"Name":"custodyChain.3.request.uri","Value":"/contentListener"},
  {"Name":"custodyChain.3.source.host","Value":"::1"},
  {"Name":"custodyChain.3.source.port","Value":"45822"},
  {"Name":"custodyChain.3.local.port","Value":"8082"},
  {"Name":"custodyChain.3.protocol","Value":"HTTPS"},
  {"Name":"custodyChain.3.tls.cipher","Value":"TLS_AES_128_GCM_SHA256"},
  {"Name":"custodyChain.3.tls.host","Value":"localhost"},
  {"Name":"custodyChain.3.tls.version","Value":"1.3"},
  {"Name":"custodyChain.2.action","Value":"TO-DISK"},
  {"Name":"custodyChain.2.time","Value":"2023-02-09T13:46:46-05:00"},
  {"Name":"custodyChain.2.local.hostname","Value":"centos7.schou.me"},
  {"Name":"custodyChain.2.user.dn","Value":"CN=localhost,O=Global Security npe2,C=US"},
  {"Name":"custodyChain.2.issuer.dn","Value":"CN=localhost,O=Test Security,C=US"},
  {"Name":"custodyChain.2.request.uri","Value":"/contentListener"},
  {"Name":"custodyChain.2.source.host","Value":"::1"},
  {"Name":"custodyChain.2.source.port","Value":"53180"},
  {"Name":"custodyChain.2.local.port","Value":"8080"},
  {"Name":"custodyChain.2.protocol","Value":"HTTPS"},
  {"Name":"custodyChain.2.tls.cipher","Value":"TLS_AES_128_GCM_SHA256"},
  {"Name":"custodyChain.2.tls.host","Value":"localhost"},
  {"Name":"custodyChain.2.tls.version","Value":"1.3"},
  {"Name":"custodyChain.1.action","Value":"FROM-DISK"},
  {"Name":"custodyChain.1.time","Value":"2023-02-09T13:50:31-05:00"},
  {"Name":"custodyChain.1.local.hostname","Value":"centos7.schou.me"},
  {"Name":"custodyChain.0.action","Value":"DIODE"},
  {"Name":"custodyChain.0.time","Value":"2023-02-09T13:50:31-05:00"},
  {"Name":"custodyChain.0.local.hostname","Value":"centos7.schou.me"},
  {"Name":"custodyChain.0.user.dn","Value":"CN=localhost,O=Global Security npe4,C=US"},
  {"Name":"custodyChain.0.issuer.dn","Value":"CN=localhost,O=Test Security,C=US"},
  {"Name":"custodyChain.0.request.uri","Value":"/contentListener"},
  {"Name":"custodyChain.0.source.host","Value":"::1"},
  {"Name":"custodyChain.0.source.port","Value":"45850"},
  {"Name":"custodyChain.0.local.port","Value":"8082"},
  {"Name":"custodyChain.0.protocol","Value":"HTTPS"},
  {"Name":"custodyChain.0.tls.cipher","Value":"TLS_AES_128_GCM_SHA256"},
  {"Name":"custodyChain.0.tls.host","Value":"localhost"},
  {"Name":"custodyChain.0.tls.version","Value":"1.3"}
]
```
