@echo off
REM Blackbox Fleet Demo - One-Click Startup Script for Windows

cd /d "%~dp0"
set PROJECT_DIR=%CD%
set RELAY_DIR=%PROJECT_DIR%\relay
set SIMULATOR_DIR=%PROJECT_DIR%\simulator
set FRONTEND_DIR=%PROJECT_DIR%\frontend

echo.
echo 🚀 Starting Blackbox Fleet Demo...
echo.

REM Start Relay
echo 🔄 Starting Relay Server...
cd /d "%RELAY_DIR%"
start "Relay Server" cmd /k "go run relay.go"

REM Wait for relay
timeout /t 1 /nobreak

REM Start Simulator
echo 🤖 Starting Robot Fleet Simulator...
cd /d "%SIMULATOR_DIR%"
start "Fleet Simulator" cmd /k "go run main.go"

REM Setup Frontend
echo 🎨 Installing frontend dependencies...
cd /d "%FRONTEND_DIR%"
if not exist "node_modules" (
    call npm install --legacy-peer-deps
)

REM Start Frontend
echo 🌐 Starting React Dashboard...
echo Dashboard URL: http://localhost:3000
echo.
echo ✨ All services started!
echo.

call npm start

pause

