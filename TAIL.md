
# Usage

Here are some examples of the nifi-sender and nifi-receiver in action.  To set things up, we need some fake data first:

```
source$ dd if=/dev/urandom of=infile_rnd.dat count=100000
```

## Sender and receiver

Setting up the NiFi receiver first:
```
target$ ./nifi-receiver -path output/
2023/02/02 14:49:49 Listening with HTTP on :8080 at /contentListener
```

We can now send a file:
```
source$ ./nifi-sender -url=http://localhost:8080/contentListener infile_rnd.dat
2023/02/02 14:54:48 creating sender...
2023/02/02 14:54:48   sending infile_rnd.dat ...
2023/02/02 14:54:49 done.
```

Back at the NiFi receiver side:
```
target$ ls output/
infile_rnd.dat
```

The file has been sent and dropped to the folder output

## Sender, diode, and receiver

Here we will look at tying 3 of these utilities together, in this order we setup the NiFi receiver first:
```
target$ ./nifi-receiver -path output2
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
