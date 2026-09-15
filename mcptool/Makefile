.PHONY: build build-all linux clean run

SSH_BIN := sshtool
IDP_BIN := idpdbtool
SSH_LINUX_BIN := sshtool-linux
IDP_LINUX_BIN := idpdbtool-linux
SRC_SSH := ./cmd/sshtool/
SRC_IDP := ./cmd/idpdbtool/

build: build-all

build-all:
	go build -o $(SSH_BIN) $(SRC_SSH)
	go build -o $(IDP_BIN) $(SRC_IDP)

linux:
	GOOS=linux GOARCH=amd64 go build -o $(SSH_LINUX_BIN) $(SRC_SSH)
	GOOS=linux GOARCH=amd64 go build -o $(IDP_LINUX_BIN) $(SRC_IDP)

clean:
	rm -f $(SSH_BIN) $(IDP_BIN) $(SSH_LINUX_BIN) $(IDP_LINUX_BIN)

run: build
	./$(SSH_BIN)
