BINARY := psman
PKG := .
LDFLAGS := -s -w

.PHONY: all build windows linux test fmt vet tidy clean

all: build

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PKG)

windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY).exe $(PKG)

linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY)-linux $(PKG)

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin
