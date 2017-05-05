CWD = $(shell pwd)
PKG = github.com/nathan-osman/cloudanchor
CMD = cloudanchor

SOURCES = $(shell find -type f -name '*.go')
BINDATA = $(shell find server/static)

all: dist/${CMD}

dist/${CMD}: dist server/ab0x.go
	docker run \
	    --rm \
	    -e CGO_ENABLED=0 \
	    -v ${CWD}:/go/src/${PKG} \
	    -v ${CWD}/dist:/go/bin \
	    -w /go/src/${PKG} \
	    golang:latest \
	    go get ./...

dist:
	@mkdir dist

server/ab0x.go: dist/fileb0x
	dist/fileb0x b0x.yaml

dist/fileb0x: dist
	docker run \
		--rm \
		-e CGO_ENABLED=0 \
		-v ${CWD}/dist:/go/bin \
		golang:latest \
		go get github.com/UnnoTed/fileb0x

clean:
	@rm -f server/ab0x.go
	@rm -rf dist

.PHONY: clean
