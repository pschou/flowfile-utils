# FlowFile-Utils

A set of FlowFile routines for working with NiFi feeds.  The utilities include:

NiFi-Reciever - take a NiFi feed and save off files while doing checksums for validity

NiFi-Sender - take a file or directory and send them to a NiFi endpoint

NiFi-Stager - take a NiFi feed and temporarily store them to disk for processing later

NiFi-Unstager - take a directory of staged files and send them to a NiFi endpoint

NiFi-Diode - takes a NiFi feed and forwards the FlowFiles to another NiFi (assures one direction)


```
# nifi-stager -h
NiFi Stager (github.com/pschou/flowfile-utils, version: 0.1.20230202.1022)

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
NiFi Sender (github.com/pschou/flowfile-utils, version: 0.1.20230202.1022)

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
NiFi Unstager (github.com/pschou/flowfile-utils, version: 0.1.20230202.1022)

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
NiFi Reciever (github.com/pschou/flowfile-utils, version: 0.1.20230202.1022)

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
    	Directory which to scan for FlowFiles (default "stager")
  -tls
    	Enable TLS for secure transport
```

```
# nifi-diode -h
NiFi Diode (github.com/pschou/flowfile-utils, version: 0.1.20230202.1022)

This utility is intended to take input over a NiFi compatible port and pass all
FlowFiles into another NiFi port while updating the attributes with the
certificate and chaining any previous certificates.

Usage of ./nifi-diode:
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
    	Enforce TLS for secure transport on incoming connections
  -update-chain
    	Add the client certificate to the connection-chain-# header (default true)
  -url string
    	Where to send the files from staging (default "http://localhost:8080/contentListener")
```
