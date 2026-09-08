BINARY          := code-gate
FRONTEND_DIR    := frontend
DIST_DIR        := $(FRONTEND_DIR)/dist

VERSION         ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT          ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDTIME       ?= $(shell date -u '+%Y-%m-%d %H:%M:%S')
LDFLAGS         := -X 'main.Version=$(VERSION)' -X 'main.CommitID=$(COMMIT)' -X 'main.BuildTime=$(BUILDTIME)'

.PHONY: all build frontend backend clean run test fmt install lint

# 默认构建目标
all: build

# 依赖安装
install:
	@if [ -d "$(FRONTEND_DIR)" ]; then \
		echo "--> 安装 code-gate 前端依赖..."; \
		cd $(FRONTEND_DIR) && npm install; \
	fi

# 编译前端生产静态资源
frontend:
	@echo "--> 正在编译 code-gate 前端静态资源..."
	@cd $(FRONTEND_DIR) && npm run build

# 完整构建 (前端构建 + 后端单二进制编译)
build: frontend backend

# 编译后端可执行文件 (内嵌前端产物)
backend:
	@echo "--> 正在编译 code-gate 后端二进制..."
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

# 前端语法与类型检查
lint:
	@if [ -d "$(FRONTEND_DIR)" ]; then \
		echo "--> 正在检查前端静态语法与类型..."; \
		cd $(FRONTEND_DIR) && npm run lint; \
	fi

# 格式化 Go 代码
fmt:
	go fmt ./...

# 运行后端单元测试
test:
	go test -v ./...

# 快捷启动命令
run: build
	./$(BINARY)

# 清理构建产物
clean:
	rm -rf $(BINARY) $(DIST_DIR) *.log
