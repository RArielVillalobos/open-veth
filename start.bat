@echo off
echo ========================================
echo  OpenVeth - Iniciando...
echo ========================================
echo.

echo [1/4] Construyendo imagen: host...
docker build -t openveth/host:latest ./images/host-node
if %errorlevel% neq 0 ( echo ERROR al construir host & pause & exit /b 1 )

echo.
echo [2/4] Construyendo imagen: router...
docker build -t openveth/router:latest ./images/router-node
if %errorlevel% neq 0 ( echo ERROR al construir router & pause & exit /b 1 )

echo.
echo [3/4] Construyendo imagen: linux...
docker build -t openveth/linux:latest ./images/linux-node
if %errorlevel% neq 0 ( echo ERROR al construir linux & pause & exit /b 1 )

echo.
echo [4/5] Construyendo imagen: server...
docker build -t openveth/server:latest ./images/server-node
if %errorlevel% neq 0 ( echo ERROR al construir server & pause & exit /b 1 )

echo.
echo [5/5] Levantando OpenVeth...
docker compose up -d --build
if %errorlevel% neq 0 ( echo ERROR al levantar OpenVeth & pause & exit /b 1 )

echo.
echo ========================================
echo  OpenVeth listo en http://localhost
echo ========================================
pause
