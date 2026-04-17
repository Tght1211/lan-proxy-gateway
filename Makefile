VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -s -w -X main.version=$(VERSION)
BINARY = gateway
INSTALL_PATH = /usr/local/bin/$(BINARY)
DIST_DIR = dist

.PHONY: build install uninstall build-all clean test test-core

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./...

test-core:
	go test ./internal/config/... ./internal/traffic/... ./internal/source/... ./internal/engine/...

install: build
	@echo "安装 $(BINARY) 到 $(INSTALL_PATH)（需要 sudo 密码）..."
	sudo cp $(BINARY) $(INSTALL_PATH)
	@echo "安装完成！现在可以直接使用 gateway 命令了。"

uninstall:
	sudo rm -f $(INSTALL_PATH)
	@echo "已卸载 $(BINARY)"

build-all: clean
	@mkdir -p $(DIST_DIR)
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY)-darwin-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY)-darwin-amd64 .
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY)-linux-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY)-windows-amd64.exe .
	tar -C $(DIST_DIR) -czf $(DIST_DIR)/$(BINARY)-darwin-arm64.tar.gz $(BINARY)-darwin-arm64
	tar -C $(DIST_DIR) -czf $(DIST_DIR)/$(BINARY)-darwin-amd64.tar.gz $(BINARY)-darwin-amd64
	tar -C $(DIST_DIR) -czf $(DIST_DIR)/$(BINARY)-linux-amd64.tar.gz $(BINARY)-linux-amd64
	tar -C $(DIST_DIR) -czf $(DIST_DIR)/$(BINARY)-linux-arm64.tar.gz $(BINARY)-linux-arm64
	cd $(DIST_DIR) && zip -q $(BINARY)-windows-amd64.zip $(BINARY)-windows-amd64.exe
	if command -v shasum >/dev/null 2>&1; then \
		cd $(DIST_DIR) && shasum -a 256 gateway-* > SHA256SUMS; \
	else \
		cd $(DIST_DIR) && sha256sum gateway-* > SHA256SUMS; \
	fi
	@echo "Build complete. Assets in $(DIST_DIR)/"

clean:
	rm -rf $(DIST_DIR)/ $(BINARY)
