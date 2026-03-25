@echo off
echo ========================================
echo  OpenVeth - Iniciando...
echo ========================================
echo.

echo [1/6] Construyendo imagen: host...
docker build -t openveth/host:latest ./images/host-node
if %errorlevel% neq 0 ( echo ERROR al construir host & pause & exit /b 1 )

echo.
echo [2/6] Construyendo imagen: router (Debian + FRRouting + snmpd)...
docker build -t openveth/router:latest ./images/router-node
if %errorlevel% neq 0 ( echo ERROR al construir router & pause & exit /b 1 )

echo.
echo [3/6] Construyendo imagen: linux...
docker build -t openveth/linux:latest ./images/linux-node
if %errorlevel% neq 0 ( echo ERROR al construir linux & pause & exit /b 1 )

echo.
echo [4/6] Construyendo imagen: server...
docker build -t openveth/server:latest ./images/server-node
if %errorlevel% neq 0 ( echo ERROR al construir server & pause & exit /b 1 )

echo.
echo [5/6] Construyendo imagen: monitor (Grafana + Prometheus + snmp_exporter)...
docker build -t openveth/monitor:latest ./images/monitor-node
if %errorlevel% neq 0 ( echo ERROR al construir monitor & pause & exit /b 1 )

echo.
echo [6/6] Levantando OpenVeth...
docker compose up -d --build
if %errorlevel% neq 0 ( echo ERROR al levantar OpenVeth & pause & exit /b 1 )

echo.
echo ========================================
echo  OpenVeth listo en http://localhost
echo ========================================
pause
