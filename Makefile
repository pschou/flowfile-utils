VERSION = 0.1.$(shell date +%Y%m%d.%H%M)
FLAGS := "-s -w -X main.version=${VERSION}"

all: build readme

build:
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o nifi-stager nifi-stager.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o nifi-sender nifi-sender.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o nifi-unstager nifi-unstager.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o nifi-reciever nifi-reciever.go lib-*.go
	CGO_ENABLED=0 go build -ldflags=${FLAGS} -o nifi-diode nifi-diode.go lib-*.go

readme:
	cp HEAD.md README.md
	echo -e '\n```\n# nifi-stager -h' >> README.md
	./nifi-stager -h 2>> README.md
	echo -e '```\n\n```\n# nifi-sender -h' >> README.md
	./nifi-sender -h 2>> README.md
	echo -e '```\n\n```\n# nifi-unstager -h' >> README.md
	./nifi-unstager -h 2>> README.md
	echo -e '```\n\n```\n# nifi-reciever -h' >> README.md
	./nifi-reciever -h 2>> README.md
	echo -e '```\n\n```\n# nifi-diode -h' >> README.md
	./nifi-diode -h 2>> README.md
	echo -e '```' >> README.md
