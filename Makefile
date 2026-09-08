BINARY          := code-gate
CMD_DIR         := ./cmd/server
FRONTEND_DIR    := frontend
DIST_DIR        := $(FRONTEND_DIR)/dist
NODE_MODULES    := $(FRONTEND_DIR)/node_modules

VERSION         ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT          ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDTIME       ?= $(shell date -u '+%Y-%m-%d %H:%M:%S')
LDFLAGS         := -X 'main.Version=$(VERSION)' -X 'main.CommitID=$(COMMIT)' -X 'main.BuildTime=$(BUILDTIME)'

.PHONY: all build backend clean run test fmt

# 默认构建目标
all: build

# 完整构建
build: backend

# 编译后端可执行文件
backend:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD_DIR)

# 格式化代码
fmt:
	go fmt ./...

# 运行单元测试
test:
	go test -v ./...

# 快捷启动命令
run: build
	./$(BINARY)

# 清理构建产物
clean:
	rm -rf $(BINARY) *.log
