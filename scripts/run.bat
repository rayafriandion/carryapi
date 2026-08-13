@echo off
REM carryAPI production mode: build single binary (with embedded frontend) and run
REM Usage: scripts\run.bat [CARRYAPI_PORT]
REM Env vars: CARRYAPI_HOST, CARRYAPI_PORT, CARRYAPI_DB_PATH, CARRYAPI_ADMIN_EMAIL, CARRYAPI_ADMIN_PASSWORD, ...
cd /d "%~dp0.."

set "BIN=carryapi.exe"
if not "%1"=="" set "CARRYAPI_PORT=%1"

echo ==^> Building single binary (embedded frontend)...
go build -o "%BIN%" ./cmd/carryapi
if errorlevel 1 (
    echo Build failed. Is Go installed and on PATH?
    pause
    exit /b 1
)

echo ==^> Starting carryAPI (default http://localhost:%CARRYAPI_PORT%)
echo     Default listen mode is all interfaces: [::]:%CARRYAPI_PORT% (dual-stack).
echo     Press Ctrl+C to stop.
"%BIN%"
if errorlevel 1 (
    echo Failed to start. Port %CARRYAPI_PORT% may be in use or config is invalid.
    pause
)
