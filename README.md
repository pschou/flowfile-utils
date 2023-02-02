# FlowFile-Utils

A set of FlowFile routines for working with NiFi feeds.  The utilities include:

NiFi-Reciever - take a NiFi feed and save off files while doing checksums for validity

NiFi-Sender - take a file or directory and send them to a NiFi endpoint

NiFi-Stager - take a NiFi feed and temporarily store them to disk for processing later

NiFi-Unstager - take a directory of staged files and send them to a NiFi endpoint

NiFi-Diode - takes a NiFi feed and forwards the FlowFiles to another NiFi (assures one direction)


For more documentation about the go-flowfile library: https://pkg.go.dev/github.com/pschou/go-flowfile .

## NiFi Diode

Super simple NiFi Diode that does one thing, takes in data and passes it on to
another NiFi without letting anything go the other direction.  Hence it's a
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

# Usage Examples:

```
# nifi-stager -h
NiFi Stager (github.com/pschou/flowfile-utils, version: 0.1.20230202.1501)

This utility is intended to take input over a NiFi compatible port and drop all
FlowFiles into directory along with associated attributes which can then be
unstaged using the NiFi Unstager.

Usage of ./nifi-stager:
  -CA string
    	A PEM eoncoded CA's certificate file. (default "someCertCAFile")
  -cert string
    	A PEM eoncoded certificate file. (default "someCertFile")
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -listen string
    	Where to listen to incoming connections (example 1.2.3.4:8080) (default ":8080")
  -listenPath string
    	Where to expect FlowFiles to be posted (default "/contentListener")
  -path string
    	Directory which to scan for FlowFiles (default "stager")
  -tls
    	Enable TLS for secure transport
```

```
# nifi-sender -h
NiFi Sender (github.com/pschou/flowfile-utils, version: 0.1.20230202.1501)

This utility is intended to capture a set of files or directory of files and
send them to a remote NiFi server for processing.

Usage of ./nifi-sender:
  -CA string
    	A PEM eoncoded CA's certificate file. (default "someCertCAFile")
  -cert string
    	A PEM eoncoded certificate file. (default "someCertFile")
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -path string
    	Directory which to scan for FlowFiles (default "stager")
  -url string
    	Where to send the files from staging (default "http://localhost:8080/contentListener")
```

```
# nifi-unstager -h
NiFi Unstager (github.com/pschou/flowfile-utils, version: 0.1.20230202.1501)

This utility is intended to take a directory of NiFi flow files and ship them
out to a listening NiFi endpoint while maintaining the same set of attribute
headers.

Usage of ./nifi-unstager:
  -CA string
    	A PEM eoncoded CA's certificate file. (default "someCertCAFile")
  -cert string
    	A PEM eoncoded certificate file. (default "someCertFile")
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -path string
    	Directory which to scan for FlowFiles (default "stager")
  -url string
    	Where to send the files from staging (default "http://localhost:8080/contentListener")
```

```
# nifi-reciever -h
NiFi Reciever (github.com/pschou/flowfile-utils, version: 0.1.20230202.1501)

This utility is intended to listen for flow files on a NifI compatible port and
then parse these files and drop them to disk for usage elsewhere.

Usage of ./nifi-reciever:
  -CA string
    	A PEM eoncoded CA's certificate file. (default "someCertCAFile")
  -cert string
    	A PEM eoncoded certificate file. (default "someCertFile")
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -listen string
    	Where to listen to incoming connections (example 1.2.3.4:8080) (default ":8080")
  -listenPath string
    	Where to expect FlowFiles to be posted (default "/contentListener")
  -path string
    	Directory which to scan for FlowFiles (default "output")
  -tls
    	Enable TLS for secure transport
```

```
# nifi-diode -h
NiFi Diode (github.com/pschou/flowfile-utils, version: 0.1.20230202.1501)

This utility is intended to take input over a NiFi compatible port and pass all
FlowFiles into another NiFi port while updating the attributes with the
certificate and chaining any previous certificates.

Usage of ./nifi-diode:
  -CA string
    	A PEM eoncoded CA's certificate file. (default "someCertCAFile")
  -cert string
    	A PEM eoncoded certificate file. (default "someCertFile")
  -ff-max-size string
    	Set a maximum partition size for partitioning files to send
  -key string
    	A PEM encoded private key file. (default "someKeyFile")
  -listen string
    	Where to listen to incoming connections (example 1.2.3.4:8080) (default ":8082")
  -listenPath string
    	Where to expect FlowFiles to be posted (default "/contentListener")
  -path string
    	Directory which to scan for FlowFiles (default "stager")
  -tls
    	Enforce TLS for secure transport on incoming connections
  -update-chain
    	Add the client certificate to the connection-chain-# header (default true)
  -url string
    	Where to send the files from staging (default "http://localhost:8080/contentListener")
```

# Usage

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
diode$ ./nifi-diode -ff-max-size 10MB
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
