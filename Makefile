.PHONY: all build clean linux

BIN_DIR := bin

all: build

build: build-clitool build-mcptool

build-clitool: build-markdown2pdf build-sshman build-viewcsv_xlsx build-occonfig build-shellman build-ansibleman

build-mcptool: build-sshtool

build-markdown2pdf:
	@echo "Building markdown2pdf..."
	cd clitool/markdown2pdf && go build -o ../../$(BIN_DIR)/clitool/markdown2pdf .

build-sshman:
	@echo "Building sshman..."
	cd clitool/ssh_config_manage && go build -ldflags "-s -w" -o ../../$(BIN_DIR)/clitool/sshman .

build-viewcsv_xlsx:
	@echo "Building viewcsv_xlsx..."
	cd clitool/viewcsv_xlsx && go build -ldflags "-s -w" -o ../../$(BIN_DIR)/clitool/viewcsv_xlsx .

build-occonfig:
	@echo "Building occonfig..."
	cd clitool/opencode_config && go build -ldflags "-s -w" -o ../../$(BIN_DIR)/clitool/occonfig .

build-shellman:
	@echo "Building shellman..."
	cd clitool/shell_rc_manage && go build -ldflags "-s -w" -o ../../$(BIN_DIR)/clitool/shellman .

build-ansibleman:
	@echo "Building ansibleman..."
	cd clitool/ansible_inventory && go build -ldflags "-s -w" -o ../../$(BIN_DIR)/clitool/ansibleman .

build-sshtool:
	@echo "Building sshtool..."
	cd mcptool && go build -o ../$(BIN_DIR)/mcptool/sshtool ./cmd/sshtool/

linux: linux-clitool linux-mcptool

linux-clitool: linux-markdown2pdf linux-sshman linux-viewcsv_xlsx linux-occonfig linux-shellman linux-ansiman

linux-mcptool: linux-sshtool

linux-markdown2pdf:
	@echo "Building markdown2pdf for Linux..."
	cd clitool/markdown2pdf && GOOS=linux GOARCH=amd64 go build -o ../../$(BIN_DIR)/clitool/markdown2pdf-linux .

linux-sshman:
	@echo "Building sshman for Linux..."
	cd clitool/ssh_config_manage && GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ../../$(BIN_DIR)/clitool/sshman-linux .

linux-viewcsv_xlsx:
	@echo "Building viewcsv_xlsx for Linux..."
	cd clitool/viewcsv_xlsx && GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ../../$(BIN_DIR)/clitool/viewcsv_xlsx-linux .

linux-occonfig:
	@echo "Building occonfig for Linux..."
	cd clitool/opencode_config && GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ../../$(BIN_DIR)/clitool/occonfig-linux .

linux-shellman:
	@echo "Building shellman for Linux..."
	cd clitool/shell_rc_manage && GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ../../$(BIN_DIR)/clitool/shellman-linux .

linux-ansiman:
	@echo "Building ansiman for Linux..."
	cd clitool/ansible_inventory && GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ../../$(BIN_DIR)/clitool/ansiman-linux .

linux-sshtool:
	@echo "Building sshtool for Linux..."
	cd mcptool && GOOS=linux GOARCH=amd64 go build -o ../$(BIN_DIR)/mcptool/sshtool-linux ./cmd/sshtool/

clean:
	rm -rf $(BIN_DIR)/*
