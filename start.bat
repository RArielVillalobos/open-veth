@echo off
setlocal
REM Use PowerShell for Docker checks, port detection, and readiness polling.
REM Options: -UpdateImages -NoPause -StartupTimeoutSeconds 180
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\start-windows.ps1" %*
exit /b %errorlevel%
