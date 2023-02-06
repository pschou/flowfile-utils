# FlowFile-Utils

A set of FlowFile routines for working with NiFi feeds.  The utilities include:

NiFi-Sender - take a file or directory and send them to a NiFi endpoint

NiFi-Reciever - take a NiFi feed and save off files while doing checksums for validity

NiFi-Stager - take a NiFi feed and temporarily store them to disk for processing later

NiFi-Unstager - take a directory of staged files and send them to a NiFi endpoint

NiFi-Diode - takes a NiFi feed and forwards the FlowFiles to another NiFi (assures one direction)


For more documentation about the go-flowfile library: https://pkg.go.dev/github.com/pschou/go-flowfile .

## NiFi Sender

NiFi sender does one thing, it will take data from the disk and upload it to a
NiFi endpoint.  Some advantages of using this NiFi sender over a full NiFi
instance are:

- One does not need to install NiFi or have it running

- It is ultra portable and can run on a minimal instance

- Enables segmenting, so an upstream stream handler with limited capabilities can get segments instead of a whole file

NiFi-Sender Usage:
```
NiFi Sender (github.com/pschou/flowfile-utils, version: 0.1.20230206.1228)

This utility is intended to capture a set of files or directory of files and
send them to a remote NiFi server for processing.

Usage: ./nifi-sender [options] path1 path2...
  -CA string
    	A PEM eoncoded CA's certificate file. (default "someCertCAFile")
  -cert string
    	A PEM eoncoded certificate file. (default "someCertFile")
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -retries int
    	Retries after failing to send a file (default 3)
  -url string
    	Where to send the files (default "http://localhost:8080/contentListener")
  -verbose
    	Turn on verbosity
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

## NiFi Reciever

NiFi Reciever listens on a port for NiFi flow files and then acts on them accordingly as they are streamed in.

NiFi-Reciever Usage:
```
NiFi Reciever (github.com/pschou/flowfile-utils, version: 0.1.20230206.1228)

This utility is intended to listen for flow files on a NifI compatible port and
then parse these files and drop them to disk for usage elsewhere.

Usage: ./nifi-reciever [options]
  -CA string
    	A PEM eoncoded CA's certificate file. (default "someCertCAFile")
  -cert string
    	A PEM eoncoded certificate file. (default "someCertFile")
  -debug
    	Turn on debug
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -listen string
    	Where to listen to incoming connections (example 1.2.3.4:8080) (default ":8080")
  -listenPath string
    	Path in URL where to expect FlowFiles to be posted (default "/contentListener")
  -path string
    	Directory in which to place files recieved (default "./output/")
  -rm
    	Automatically remove file after script has finished
  -script string
    	Shell script to be called on successful post
  -script-shell string
    	Shell to be used for script run (default "/bin/bash")
  -segment-max-size string
    	Set a maximum size for partitioning files in sending
  -tls
    	Enable TLS for secure transport
  -verbose
    	Turn on verbosity
```

Example:
```
$ ./nifi-reciever
Output set to ./output/
2023/02/06 08:58:25 Listening with HTTP on :8080 at /contentListener
2023/02/06 08:58:28   Recieving nifi file output/file1.dat size 18
2023/02/06 08:58:28   Verified file output/file1.dat
2023/02/06 08:58:28   Recieving nifi file output/file2.dat size 10
2023/02/06 08:58:28   Verified file output/file2.dat
```

if one wants to act on the files after they arrive, they can add a script
caller which performs functions on the files just after a successful send:

```
$ cat script.sh
#!/bin/bash
echo In Script, doing something:
sha256sum "$1"
$ ./nifi-reciever -script script.sh -verbose
Output set to ./output/
2023/02/06 08:59:38 Listening with HTTP on :8080 at /contentListener
2023/02/06 08:59:40   Recieving nifi file output/file1.dat size 18
    [{"Name":"path","Value":"./"},{"Name":"filename","Value":"file1.dat"},{"Name":"modtime","Value":"2023-02-06T08:47:47-05:00"},{"Name":"checksum-type","Value":"SHA256"},{"Name":"checksum","Value":"51fd71b1368a1b130b60cab1301b05bbef470cf4a21ef2956553def809edf4ec"},{"Name":"uuid","Value":"271d19fd-827a-4c9d-a21e-7ede9d652120"}]
2023/02/06 08:59:40   Verified file output/file1.dat
2023/02/06 08:59:40   Calling script /bin/bash script.sh output/file1.dat
2023/02/06 08:59:40 ----- START script.sh output/file1.dat -----
In Script, doing something:
51fd71b1368a1b130b60cab1301b05bbef470cf4a21ef2956553def809edf4ec  output/file1.dat

2023/02/06 08:59:40 ----- END script.sh output/file1.dat -----
2023/02/06 08:59:40   Recieving nifi file output/file2.dat size 10
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
$ ./nifi-reciever -script script.sh -rm
Output set to ./output/
2023/02/06 09:06:18 Listening with HTTP on :8080 at /contentListener
2023/02/06 09:06:20   Recieving nifi file output/file1.dat size 18
2023/02/06 09:06:20   Verified file output/file1.dat
2023/02/06 09:06:20   Calling script /bin/bash script.sh output/file1.dat
2023/02/06 09:06:20   Removed output/file1.dat
2023/02/06 09:06:20   Recieving nifi file output/file2.dat size 10
2023/02/06 09:06:20   Verified file output/file2.dat
2023/02/06 09:06:20   Calling script /bin/bash script.sh output/file2.dat
2023/02/06 09:06:20   Removed output/file2.dat
^C
$ ls output/
$
```

## NiFi Stager

This tool enables files to be layed down to disk, to be replayed at a later time or different location into a flowfile feed.  Note that the binary payload that is layed down is FlowFile encoded and not parsed out for making sure the exact binary payload is replayed.

NiFi-Stager Usage:
```
NiFi Stager (github.com/pschou/flowfile-utils, version: 0.1.20230206.1228)

This utility is intended to take input over a NiFi compatible port and drop all
FlowFiles into directory along with associated attributes which can then be
unstaged using the NiFi Unstager.

Usage: ./nifi-stager [options]
  -CA string
    	A PEM eoncoded CA's certificate file. (default "someCertCAFile")
  -cert string
    	A PEM eoncoded certificate file. (default "someCertFile")
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
    	Automatically remove partial files (default true)
  -script string
    	Shell script to be called on successful post
  -script-shell string
    	Shell to be used for script run (default "/bin/bash")
  -segment-max-size string
    	Set a maximum size for partitioning files in sending
  -tls
    	Enable TLS for secure transport
  -verbose
    	Turn on verbosity
```

Example:
```
$ ./nifi-stager
Output set to stager
2023/02/06 12:01:10 Listening with HTTP on :8080 at /contentListener
  Recieving nifi file file1.dat size 18
  Recieving nifi file file2.dat size 10
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
  Recieving nifi file file1.dat size 18
    [{"Name":"path","Value":"./"},{"Name":"filename","Value":"file1.dat"},{"Name":"modtime","Value":"2023-02-06T08:47:47-05:00"},{"Name":"checksum-type","Value":"SHA256"},{"Name":"checksum","Value":"51fd71b1368a1b130b60cab1301b05bbef470cf4a21ef2956553def809edf4ec"},{"Name":"uuid","Value":"f0aa041f-3302-4358-acd1-136ba76078cf"}]
2023/02/06 12:03:06   Calling script /bin/bash stager_send.sh stager/60af9b0c-7f23-48a6-bc0e-4f44879e9f3a.dat stager/60af9b0c-7f23-48a6-bc0e-4f44879e9f3a.json
2023/02/06 12:03:06 ----- START stager_send.sh 60af9b0c-7f23-48a6-bc0e-4f44879e9f3a -----
moving content stager/60af9b0c-7f23-48a6-bc0e-4f44879e9f3a.dat to another folder /tmp

2023/02/06 12:03:06 ----- END stager_send.sh 60af9b0c-7f23-48a6-bc0e-4f44879e9f3a -----
2023/02/06 12:03:06   Removed 60af9b0c-7f23-48a6-bc0e-4f44879e9f3a
  Recieving nifi file file2.dat size 10
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

NiFi-Unstager Usage:
```
NiFi Unstager (github.com/pschou/flowfile-utils, version: 0.1.20230206.1228)

This utility is intended to take a directory of NiFi flow files and ship them
out to a listening NiFi endpoint while maintaining the same set of attribute
headers.

Usage: ./nifi-unstager [options]
  -CA string
    	A PEM eoncoded CA's certificate file. (default "someCertCAFile")
  -cert string
    	A PEM eoncoded certificate file. (default "someCertFile")
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -path string
    	Directory which to scan for FlowFiles (default "stager")
  -retries int
    	Retries after failing to send a file (default 3)
  -url string
    	Where to send the files from staging (default "http://localhost:8080/contentListener")
  -verbose
    	Turn on verbosity
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
$ ./nifi-reciever
Output set to ./output/
2023/02/06 12:22:09 Listening with HTTP on :8080 at /contentListener
2023/02/06 12:22:10   Recieving nifi file output/file2.dat size 10
2023/02/06 12:22:10   Verified file output/file2.dat
2023/02/06 12:22:10   Recieving nifi file output/file1.dat size 18
2023/02/06 12:22:10   Verified file output/file1.dat
^C
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


![NiFi-Diode diagram showing a NiFi box on the left, and arrow representing a TCP flow pointing to a NiFi Diode in the middle, and another arrow to the right going to a NiFi box on the right, again representing a TCP flow](NiFi-Diode.png)

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
NiFi Diode (github.com/pschou/flowfile-utils, version: 0.1.20230206.1228)

This utility is intended to take input over a NiFi compatible port and pass all
FlowFiles into another NiFi port while updating the attributes with the
certificate and chaining any previous certificates.

Usage: ./nifi-diode [options]
  -CA string
    	A PEM eoncoded CA's certificate file. (default "someCertCAFile")
  -cert string
    	A PEM eoncoded certificate file. (default "someCertFile")
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -listen string
    	Where to listen to incoming connections (example 1.2.3.4:8080) (default ":8082")
  -listenPath string
    	Path in URL where to expect FlowFiles to be posted (default "/contentListener")
  -no-checksums
    	Ignore doing checksum checks
  -retries int
    	Retries after failing to send a file (default 3)
  -segment-max-size string
    	Set a maximum size for partitioning files in sending
  -tls
    	Enforce TLS for secure transport on incoming connections
  -update-chain
    	Add the client certificate to the connection-chain-# header (default true)
  -url string
    	Where to send the files from staging (default "http://localhost:8080/contentListener")
  -verbose
    	Turn on verbosity
```

# Example:

Here are some examples of the nifi-sender and nifi-reciever in action.  To set things up, we need some fake data first:

```
source$ dd if=/dev/urandom of=infile_rnd.dat count=100000
```

## Sender and reciever

Setting up the NiFi reciever first:
```
target$ ./nifi-reciever -path output/
2023/02/02 14:49:49 Listening with HTTP on :8080 at /contentListener
```

We can now send a file:
```
source$ ./nifi-sender -url=http://localhost:8080/contentListener infile_rnd.dat
2023/02/02 14:54:48 creating sender...
2023/02/02 14:54:48   sending infile_rnd.dat ...
2023/02/02 14:54:49 done.
```

Back at the NiFi reciever side:
```
target$ ls output/
infile_rnd.dat
```

The file has been sent and dropped to the folder output

## Sender, diode, and reciever

Here we will look at tying 3 of these utilities together, in this order we setup the NiFi reciever first:
```
target$ ./nifi-reciever -path output2
2023/02/02 14:57:33 Listening with HTTP on :8080 at /contentListener
```

Setup the diode to make the connections in-between:
```
diode$ ./nifi-diode -segment-max-size 10MB
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

