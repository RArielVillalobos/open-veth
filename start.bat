@echo off
echo ========================================
echo  OpenVeth - Iniciando...
echo ========================================
echo.

echo [1/7] Descargando imagen: host...
docker pull openveth/host:latest
if %errorlevel% neq 0 ( echo ERROR al descargar host & pause & exit /b 1 )

echo.
echo [2/7] Descargando imagen: router...
docker pull openveth/router:latest
if %errorlevel% neq 0 ( echo ERROR al descargar router & pause & exit /b 1 )

echo.
echo [3/7] Descargando imagen: linux...
docker pull openveth/linux:latest
if %errorlevel% neq 0 ( echo ERROR al descargar linux & pause & exit /b 1 )

echo.
echo [4/7] Descargando imagen: server...
docker pull openveth/server:latest
if %errorlevel% neq 0 ( echo ERROR al descargar server & pause & exit /b 1 )

echo.
echo [5/7] Descargando imagen: monitor...
docker pull openveth/monitor:latest
if %errorlevel% neq 0 ( echo ERROR al descargar monitor & pause & exit /b 1 )

echo.
echo [6/7] Descargando imagen: tester...
docker pull openveth/tester:latest
if %errorlevel% neq 0 ( echo ERROR al descargar tester & pause & exit /b 1 )

echo.
echo [7/7] Levantando OpenVeth...
docker compose up -d --build
if %errorlevel% neq 0 ( echo ERROR al levantar OpenVeth & pause & exit /b 1 )

echo.
echo ========================================
echo  OpenVeth listo en http://localhost
echo ========================================
pause
