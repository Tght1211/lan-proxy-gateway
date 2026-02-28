VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -s -w -X main.version=$(VERSION)
BINARY = gateway
INSTALL_PATH = /usr/local/bin/$(BINARY)

.PHONY: build install uninstall build-all clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

install: build
	@echo "安装 $(BINARY) 到 $(INSTALL_PATH)（需要 sudo 密码）..."
	sudo cp $(BINARY) $(INSTALL_PATH)
	@echo "安装完成！现在可以直接使用 gateway 命令了。"

uninstall:
	sudo rm -f $(INSTALL_PATH)
	@echo "已卸载 $(BINARY)"

build-all: clean
	@mkdir -p dist
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64  .
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64  .
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64   .
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64   .
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe .
	@echo "Build complete. Binaries in dist/"

clean:
	rm -rf dist/ $(BINARY)
