.PHONY: all build clean linux build-clitool build-mcptool linux-clitool linux-mcptool

BIN_DIR := bin
STRIP := -ldflags "-s -w"

all: build

build: build-clitool build-mcptool

linux: linux-clitool linux-mcptool

# $(call cli,target,dir,bin,linux_bin,flags)
define cli
.PHONY: build-$(1) linux-$(1)
build-$(1):
	@echo "Building $(3)..."
	@mkdir -p $(BIN_DIR)/clitool
	go -C $(2) build $(5) -o $$$$(pwd)/$(BIN_DIR)/clitool/$(3) .
linux-$(1):
	@echo "Building $(4) for Linux..."
	@mkdir -p $(BIN_DIR)/clitool
	GOOS=linux GOARCH=amd64 go -C $(2) build $(5) -o $$$$(pwd)/$(BIN_DIR)/clitool/$(4)-linux .
endef

$(eval $(call cli,markdown2pdf,clitool/markdown2pdf,markdown2pdf,markdown2pdf,))
$(eval $(call cli,sshman,clitool/ssh_config_manage,sshman,sshman,$(STRIP)))
$(eval $(call cli,viewcsv_xlsx,clitool/viewcsv_xlsx,viewcsv_xlsx,viewcsv_xlsx,$(STRIP)))
$(eval $(call cli,occonfig,clitool/opencode_config,occonfig,occonfig,$(STRIP)))
$(eval $(call cli,shellman,clitool/shell_rc_manage,shellman,shellman,$(STRIP)))
$(eval $(call cli,ansibleman,clitool/ansible_inventory,ansibleman,ansiman,$(STRIP)))

.PHONY: linux-ansiman
linux-ansiman: linux-ansibleman

build-clitool: build-markdown2pdf build-sshman build-viewcsv_xlsx build-occonfig build-shellman build-ansibleman

linux-clitool: linux-markdown2pdf linux-sshman linux-viewcsv_xlsx linux-occonfig linux-shellman linux-ansiman

# $(call mcp,target,pkg)
define mcp
.PHONY: build-$(1) linux-$(1)
build-$(1):
	@echo "Building $(1)..."
	@mkdir -p $(BIN_DIR)/mcptool
	go -C mcptool build -o $$$$(pwd)/$(BIN_DIR)/mcptool/$(1) $(2)
linux-$(1):
	@echo "Building $(1) for Linux..."
	@mkdir -p $(BIN_DIR)/mcptool
	GOOS=linux GOARCH=amd64 go -C mcptool build -o $$$$(pwd)/$(BIN_DIR)/mcptool/$(1)-linux $(2)
endef

$(eval $(call mcp,sshtool,./cmd/sshtool/))

build-mcptool: build-sshtool

linux-mcptool: linux-sshtool

clean:
	rm -rf $(BIN_DIR)/*
