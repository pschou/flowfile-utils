VERSION = 0.1.$(shell date +%Y%m%d.%H%M)
FLAGS := "-s -w -X main.version=${VERSION}"

all: build readme

pure: go build readme

go:
	go clean -modcache
	GOPRIVATE=github.com/pschou go get github.com/pschou/go-flowfile
	GOPRIVATE=github.com/pschou go get github.com/pschou/go-iothrottler
	go mod vendor

build:
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../udp-to-nifi udp-to-nifi.go lib-*.go
	go build -ldflags=${FLAGS} -o ../nifi-to-udp nifi-to-udp.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../nifi-stager nifi-stager.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../nifi-sender nifi-sender.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../nifi-unstager nifi-unstager.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../nifi-receiver nifi-receiver.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../nifi-diode nifi-diode.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../nifi-sink nifi-sink.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../nifi-flood nifi-flood.go lib-*.go

readme:
	./readme.sh

clean:
	rm nifi-diode nifi-receiver nifi-sender nifi-stager nifi-unstager
