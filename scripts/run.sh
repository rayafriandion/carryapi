#!/usr/bin/env bash
# carryAPI 生产模式:构建单二进制(含内嵌前端)并运行
# 用法: bash scripts/run.sh [CARRYAPI_PORT]
# 环境变量: CARRYAPI_PORT, CARRYAPI_DB_PATH, CARRYAPI_ADMIN_EMAIL, CARRYAPI_ADMIN_PASSWORD 等
set -e
cd "$(dirname "$0")/.."

BIN="carryapi"
# 若传了端口参数则导出
if [ -n "$1" ]; then export CARRYAPI_PORT="$1"; fi

echo "==> 构建单二进制(内嵌前端)..."
go build -o "$BIN" ./cmd/carryapi

echo "==> 启动 carryAPI (默认 http://localhost:${CARRYAPI_PORT:-8067})..."
echo "    首次启动会在控制台打印管理员密码,请记下。Ctrl+C 停止。"
exec ./"$BIN"
