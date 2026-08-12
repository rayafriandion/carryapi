@echo off
REM carryAPI dev mode: start frontend hot-reload (Vite) and backend together
REM Usage: scripts\dev.bat
REM Frontend dev server: http://localhost:5173 (proxies /api /v1 to backend 8067)
cd /d "%~dp0.."

echo ==^> Starting frontend Vite dev (http://localhost:5173, /api /v1 proxied to backend 8067)...
start "carryapi-frontend" cmd /c "cd web && npm run dev"

echo ==^> Starting backend (http://localhost:8067)...
go run ./cmd/carryapi
echo Backend exited. Close the frontend window to stop it.
