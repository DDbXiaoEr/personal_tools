.PHONY: all build clean linux

BIN_DIR := bin
DIST_DIR := dist

all: build

build: build-clitool build-mcptool

build-clitool: build-markdown2pdf build-sshman

build-mcptool: build-sshtool

build-markdown2pdf:
	@echo "Building markdown2pdf..."
	cd clitool/markdown2pdf && go build -o ../../$(BIN_DIR)/clitool/markdown2pdf .

build-sshman:
	@echo "Building sshman..."
	cd clitool/ssh_config_manage && go build -ldflags "-s -w" -o ../../$(BIN_DIR)/clitool/sshman .

build-sshtool:
	@echo "Building sshtool..."
	cd mcptool && go build -o ../$(BIN_DIR)/mcptool/sshtool ./cmd/sshtool/

linux: linux-clitool linux-mcptool

linux-clitool: linux-markdown2pdf linux-sshman

linux-mcptool: linux-sshtool

linux-markdown2pdf:
	@echo "Building markdown2pdf for Linux..."
	cd clitool/markdown2pdf && GOOS=linux GOARCH=amd64 go build -o ../../$(BIN_DIR)/clitool/markdown2pdf-linux .

linux-sshman:
	@echo "Building sshman for Linux..."
	cd clitool/ssh_config_manage && GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ../../$(BIN_DIR)/clitool/sshman-linux .

linux-sshtool:
	@echo "Building sshtool for Linux..."
	cd mcptool && GOOS=linux GOARCH=amd64 go build -o ../$(BIN_DIR)/mcptool/sshtool-linux ./cmd/sshtool/

clean:
	rm -rf $(BIN_DIR)/*
	rm -f $(DIST_DIR)/*
