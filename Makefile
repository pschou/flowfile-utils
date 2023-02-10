VERSION = 0.1.$(shell date +%Y%m%d.%H%M)
FLAGS := "-s -w -X main.version=${VERSION}"

all: build readme

pure: go build readme

go:
	go clean -modcache
	go get github.com/pschou/go-flowfile

build:
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o nifi-stager src/nifi-stager.go src/lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o nifi-sender src/nifi-sender.go src/lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o nifi-unstager src/nifi-unstager.go src/lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o nifi-receiver src/nifi-receiver.go src/lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o nifi-diode src/nifi-diode.go src/lib-*.go

readme:
	./src/readme.sh

clean:
	rm nifi-diode nifi-receiver nifi-sender nifi-stager nifi-unstager
