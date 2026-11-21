@echo off
chcp 65001 >nul

echo ========================================
echo   Citebase - Windows Dev Launcher
echo ========================================
echo.

for %%i in ("%~dp0..") do set "ROOT_DIR=%%~fi"
cd /d "%ROOT_DIR%"

:: Check ports
echo [0/4] Checking ports 3000 / 3001...
echo   NOTE: any process listening on these two ports will be terminated.

for /f "tokens=5" %%a in ('netstat -ano ^| findstr :3001 ^| findstr LISTENING') do (
    taskkill /F /PID %%a >nul 2>&1
)

for /f "tokens=5" %%a in ('netstat -ano ^| findstr :3000 ^| findstr LISTENING') do (
    taskkill /F /PID %%a >nul 2>&1
)

echo Ports checked!
echo.

:: Check npm dependencies
echo [1/4] Checking npm dependencies...
if not exist "web\node_modules" (
    echo Installing dependencies...
    cd web
    call npm install
    cd ..
)

:: Build backend
echo [2/4] Building backend...
go build -o citebase.exe ./app/server
if errorlevel 1 (
    echo Backend build failed!
    pause
    exit /b 1
)

:: Start backend
echo [3/4] Starting backend (port 3001)...
start "citebase-backend" cmd /k "citebase.exe"

timeout /t 3 /nobreak >nul

:: Start frontend
echo [4/4] Starting frontend (port 3000)...
cd web
start "citebase-frontend" cmd /k "npm run dev"

timeout /t 2 /nobreak >nul
start "" "http://localhost:3000"

cd /d "%ROOT_DIR%"

echo.
echo ========================================
echo   Services Started!
echo   Frontend: http://localhost:3000
echo   Backend: http://localhost:3001
echo ========================================
echo.
pause
