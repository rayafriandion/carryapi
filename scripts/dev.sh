#!/usr/bin/env bash
# carryAPI 开发模式:同时启动前端热更新(Vite)与后端(go run)
# 用法: bash scripts/dev.sh
set -e
cd "$(dirname "$0")/.."

echo "==> 启动前端 Vite dev (http://localhost:5173, /api /v1 代理到后端 8067)..."
(cd web && npm run dev) &
FRONT=$!

echo "==> 启动后端 (http://localhost:8067)..."
go run ./cmd/carryapi &
BACK=$!

# 捕获中断,同时结束两个进程
trap "echo; echo '==> 停止...'; kill $FRONT $BACK 2>/dev/null; wait 2>/dev/null" INT TERM
wait
