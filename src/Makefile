VERSION = 0.1.$(shell date +%Y%m%d.%H%M)
FLAGS := "-s -w -X main.version=${VERSION}"

all: build readme

pure: go build readme

go:
	go clean -modcache
	GOPRIVATE=github.com/pschou go get github.com/pschou/go-flowfile
	GOPRIVATE=github.com/pschou go get github.com/pschou/go-memdiskbuf
	GOPRIVATE=github.com/pschou go get github.com/pschou/go-unixmode
	GOPRIVATE=github.com/pschou go get github.com/pschou/go-iothrottler
	GOPRIVATE=github.com/pschou go get github.com/pschou/go-sorting/numstr
	GOPRIVATE=github.com/pschou go get github.com/pschou/go-tempfile
	GOPRIVATE=github.com/pschou go get github.com/pschou/go-bunit
	go mod vendor

build:
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../ff-http-to-kcp ff-http-to-kcp.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../ff-kcp-to-http ff-kcp-to-http.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../ff-udp-to-http ff-udp-to-http.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../ff-http-to-udp ff-http-to-udp.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../ff-stager ff-stager.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../ff-sender ff-sender.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../ff-unstager ff-unstager.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../ff-receiver ff-receiver.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../ff-diode ff-diode.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../ff-sink ff-sink.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o ../ff-flood ff-flood.go lib-*.go

readme:
	./readme.sh

clean:
	rm ff-diode ff-receiver ff-sender ff-stager ff-unstager
