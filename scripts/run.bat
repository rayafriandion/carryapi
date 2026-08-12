@echo off
REM carryAPI 生产模式:构建单二进制(含内嵌前端)并运行
REM 用法: scripts\run.bat [CARRYAPI_PORT]
REM 环境变量: CARRYAPI_PORT, CARRYAPI_DB_PATH, CARRYAPI_ADMIN_EMAIL, CARRYAPI_ADMIN_PASSWORD 等
cd /d "%~dp0.."

set "BIN=carryapi.exe"
if not "%1"=="" set "CARRYAPI_PORT=%1"

echo ==^> 构建单二进制(内嵌前端)...
go build -o "%BIN%" ./cmd/carryapi

echo ==^> 启动 carryAPI (默认 http://localhost:%CARRYAPI_PORT%)
echo     首次启动会在控制台打印管理员密码,请记下。Ctrl+C 停止。
"%BIN%"
