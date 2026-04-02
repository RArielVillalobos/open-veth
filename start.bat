@echo off
setlocal enabledelayedexpansion
echo ========================================
echo  OpenVeth - Starting...
echo ========================================
echo.

REM --- 1. Check Docker is running ---
echo Checking Docker...
docker info >nul 2>&1
if %errorlevel% neq 0 (
    echo.
    echo ERROR: Docker is not running.
    echo Please start Docker Desktop and try again.
    echo.
    pause
    exit /b 1
)

REM --- 2. Check WSL2 backend ---
docker info 2>&1 | findstr /i "wsl" >nul
if %errorlevel% neq 0 (
    echo.
    echo ERROR: Docker is not using WSL2 as backend.
    echo OpenVeth requires WSL2 for Linux networking.
    echo.
    echo Fix: Docker Desktop ^> Settings ^> General
    echo      ^> Enable "Use the WSL 2 based engine"
    echo.
    pause
    exit /b 1
)
echo Docker OK ^(WSL2 backend active^).
echo.

REM --- 3. Remove previous containers ---
echo Removing previous containers...
docker rm -f openveth-backend openveth-frontend >nul 2>&1
echo.

REM --- 4. Pull images ---
echo [1/8] Pulling image: host...
docker pull openveth/host:latest
if %errorlevel% neq 0 ( echo ERROR pulling host & pause & exit /b 1 )

echo.
echo [2/8] Pulling image: router...
docker pull openveth/router:latest
if %errorlevel% neq 0 ( echo ERROR pulling router & pause & exit /b 1 )

echo.
echo [3/8] Pulling image: switch...
docker pull openveth/switch:latest
if %errorlevel% neq 0 ( echo ERROR pulling switch & pause & exit /b 1 )

echo.
echo [4/8] Pulling image: linux...
docker pull openveth/linux:latest
if %errorlevel% neq 0 ( echo ERROR pulling linux & pause & exit /b 1 )

echo.
echo [5/8] Pulling image: server...
docker pull openveth/server:latest
if %errorlevel% neq 0 ( echo ERROR pulling server & pause & exit /b 1 )

echo.
echo [6/8] Pulling image: monitor...
docker pull openveth/monitor:latest
if %errorlevel% neq 0 ( echo ERROR pulling monitor & pause & exit /b 1 )

echo.
echo [7/8] Pulling image: tester...
docker pull openveth/tester:latest
if %errorlevel% neq 0 ( echo ERROR pulling tester & pause & exit /b 1 )

echo.
echo [8/8] Starting OpenVeth...
docker compose up -d --build
if %errorlevel% neq 0 ( echo ERROR starting OpenVeth & pause & exit /b 1 )

REM --- 5. Read actual ports from .env ---
set FRONTEND_PORT=80
set BACKEND_PORT=8080
if exist .env (
    for /f "tokens=1,2 delims==" %%a in (.env) do (
        if "%%a"=="FRONTEND_PORT" set FRONTEND_PORT=%%b
        if "%%a"=="BACKEND_PORT" set BACKEND_PORT=%%b
    )
)

echo.
echo ========================================
echo  OpenVeth is ready!
echo  Open your browser at:
echo  http://localhost:!FRONTEND_PORT!
echo ========================================
pause
