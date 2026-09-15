BINARY := sshman
PKG := .
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w

.PHONY: all build run test fmt vet lint tidy clean install

all: build

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PKG)

run:
	go run $(PKG)

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

lint: fmt vet

tidy:
	go mod tidy

clean:
	rm -rf bin

install:
	go install -ldflags "$(LDFLAGS)" $(PKG)
