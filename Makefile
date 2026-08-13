# carryAPI 统一构建/运行入口(跨平台)
# 需要 Go 1.22+;开发模式额外需要 Node.js。
# Windows 用 git-bash 或 WSL 运行 make;或用 scripts/ 下的 .bat/.sh 脚本。

# 默认端口和监听地址
PORT ?= 8067
HOST ?= all

.PHONY: help build run dev test frontend-build clean

help: ## 显示帮助
	@echo "carryAPI 可用命令:"
	@echo "  make build         构建单二进制(内嵌前端)到 ./carryapi"
	@echo "  make run           构建并运行(生产模式,默认端口 $(PORT),监听 $(HOST))"
	@echo "  make dev           开发模式:同时启动前端热更新 + 后端"
	@echo "  make test          运行全部 Go 测试 + 前端测试"
	@echo "  make frontend-build 重新构建前端(web/dist)"
	@echo "  make clean         清理构建产物"

build: ## 构建单二进制
	go build -o carryapi ./cmd/carryapi

run: ## 构建并运行
	go build -o carryapi ./cmd/carryapi
	CARRYAPI_HOST=$(HOST) CARRYAPI_PORT=$(PORT) ./carryapi

dev: ## 开发模式(前后端同时)
	@echo "前端 dev 在 http://localhost:5173(代理到后端 $(PORT));后端在 http://localhost:$(PORT)"
	@bash scripts/dev.sh

frontend-build: ## 重新构建前端
	cd web && npm run build

test: ## 全部测试
	go test ./...
	cd web && npm run test

clean: ## 清理
	rm -f carryapi carryapi.exe
