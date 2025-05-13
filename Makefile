.PHONY: build clean install

BINARY_NAME=kubeforge
VERSION=1.0.0
LDFLAGS=-ldflags "-X main.Version=${VERSION}"

build:
	go build ${LDFLAGS} -o bin/${BINARY_NAME} cmd/kubeforge/main.go

install: build
	sudo cp bin/${BINARY_NAME} /usr/local/bin/

clean:
	rm -rf bin

test:
	go test ./...

lint:
	golangci-lint run