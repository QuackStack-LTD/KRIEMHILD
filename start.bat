@echo off
cd /d "%~dp0"
if not exist node_modules call npm.cmd install
if errorlevel 1 goto error
call npm.cmd start
if errorlevel 1 goto error
exit /b 0
:error
pause
exit /b 1
