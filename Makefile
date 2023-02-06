VERSION = 0.1.$(shell date +%Y%m%d.%H%M)
FLAGS := "-s -w -X main.version=${VERSION}"

all: build readme

build:
	#go clean -modcache
	#go mod tidy
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o nifi-stager nifi-stager.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o nifi-sender nifi-sender.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o nifi-unstager nifi-unstager.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o nifi-receiver nifi-receiver.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o nifi-diode nifi-diode.go lib-*.go

readme:
	./readme.sh
