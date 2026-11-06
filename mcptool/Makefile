.PHONY: build build-all linux clean run

SSH_BIN := sshtool
NOTIFY_BIN := notifytool
SSH_LINUX_BIN := sshtool-linux
NOTIFY_LINUX_BIN := notifytool-linux
SRC_SSH := ./cmd/sshtool/
SRC_NOTIFY := ./cmd/notifytool/

build: build-all

build-all:
	go build -o $(SSH_BIN) $(SRC_SSH)
	go build -o $(NOTIFY_BIN) $(SRC_NOTIFY)

linux:
	GOOS=linux GOARCH=amd64 go build -o $(SSH_LINUX_BIN) $(SRC_SSH)
	GOOS=linux GOARCH=amd64 go build -o $(NOTIFY_LINUX_BIN) $(SRC_NOTIFY)

clean:
	rm -f $(SSH_BIN) $(NOTIFY_BIN) $(SSH_LINUX_BIN) $(NOTIFY_LINUX_BIN)

run: build
	./$(SSH_BIN)
