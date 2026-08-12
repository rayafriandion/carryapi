@echo off
REM carryAPI 开发模式:同时启动前端热更新(Vite)与后端(go run)
REM 用法: scripts\dev.bat
cd /d "%~dp0.."

echo ==^> 启动前端 Vite dev (http://localhost:5173, /api /v1 代理到后端 8067)...
start "carryapi-frontend" cmd /c "cd web && npm run dev"

echo ==^> 启动后端 (http://localhost:8067)...
go run ./cmd/carryapi

echo 后端已退出。关闭前端窗口即可停止前端。
