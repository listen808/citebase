@echo off
chcp 65001 >nul
title Citebase - Windows Package Builder

setlocal enabledelayedexpansion

:: This script lives in scripts\, so resolve the repository root one level up.
for %%i in ("%~dp0..") do set "ROOT_DIR=%%~fi\"
set "BUILD_DIR=%ROOT_DIR%dist"
set "OUTPUT_DIR=%ROOT_DIR%dist\citebase"
set "ZIP_FILE=%ROOT_DIR%citebase-windows.zip"

echo ========================================
echo   Citebase - Build ^& Package (Windows)
echo ========================================
echo.

:: ========================================
:: Prerequisites check
:: ========================================
echo [0/7] Checking environment...

where go >nul 2>&1
if errorlevel 1 (
    echo [FAIL] Go not found. Please install Go 1.26+
    pause
    exit /b 1
)

where node >nul 2>&1
if errorlevel 1 (
    echo [FAIL] Node.js not found. Please install Node.js 20.19+ or 22.12+
    pause
    exit /b 1
)

echo   Environment OK
echo.

:: ========================================
:: Clean previous build
:: ========================================
echo [1/7] Cleaning previous build...

if exist "%BUILD_DIR%" rmdir /s /q "%BUILD_DIR%"
if exist "%ZIP_FILE%" del "%ZIP_FILE%"
echo   Done
echo.

:: ========================================
:: Create output structure
:: ========================================
echo [2/7] Creating directories...

mkdir "%OUTPUT_DIR%"                    2>nul
mkdir "%OUTPUT_DIR%\configs"            2>nul
mkdir "%OUTPUT_DIR%\web\dist"           2>nul
mkdir "%OUTPUT_DIR%\databases"          2>nul
mkdir "%OUTPUT_DIR%\tools\import_oracle_ddl" 2>nul
mkdir "%OUTPUT_DIR%\data\index"         2>nul
mkdir "%OUTPUT_DIR%\data\dict\zh"       2>nul
mkdir "%OUTPUT_DIR%\data\markdowns"     2>nul
mkdir "%OUTPUT_DIR%\data\images"        2>nul
mkdir "%OUTPUT_DIR%\data\attachment"    2>nul
echo   Done
echo.

:: ========================================
:: Build backend
:: ========================================
echo [3/7] Building backend...

cd /d "%ROOT_DIR%"
REM Inject the commit into the binary so the installed instance can answer which commit it was built from.
REM No git (e.g. extracted from a source zip) -> keep dev, rather than showing a fake hash.
set "COMMIT=dev"
for /f "delims=" %%c in ('git rev-parse --short HEAD 2^>NUL') do set "COMMIT=%%c"
go build -ldflags="-s -w -X github.com/listen808/citebase/internal/version.Commit=%COMMIT%" -o "%OUTPUT_DIR%\citebase.exe" ./app/server
if errorlevel 1 (
    echo [FAIL] Backend build failed
    pause
    exit /b 1
)
for %%f in ("%OUTPUT_DIR%\citebase.exe") do set /a "SZ=%%~zf/1024"
echo   citebase.exe (!SZ! KB)
echo.

:: ========================================
:: Build frontend
:: ========================================
echo [4/7] Building frontend...

cd /d "%ROOT_DIR%web"
if not exist "node_modules" (
    echo   Installing npm dependencies...
    call npm install
    if errorlevel 1 (
        echo [FAIL] npm install failed
        pause
        exit /b 1
    )
)
call npm run build
if errorlevel 1 (
    echo [FAIL] Frontend build failed
    pause
    exit /b 1
)

if not exist "%ROOT_DIR%web\dist\index.html" (
    echo [FAIL] Frontend build incomplete - index.html missing
    pause
    exit /b 1
)
set /a "ASSET_COUNT=0"
for /f %%a in ('dir /b /s "%ROOT_DIR%web\dist\*" 2^>nul ^| find /c /v ""') do set /a "ASSET_COUNT=%%a"
echo   Frontend built (!ASSET_COUNT! files)
echo.

:: ========================================
:: Copy all resources
:: ========================================
echo [5/7] Copying resources...

xcopy /E /I /Y "%ROOT_DIR%web\dist\*" "%OUTPUT_DIR%\web\dist\" >nul 2>&1
if not exist "%OUTPUT_DIR%\web\dist\index.html" (
    echo [FAIL] Frontend assets copy failed
    pause
    exit /b 1
)
set /a "COPY_COUNT=0"
for /f %%a in ('dir /b /s "%OUTPUT_DIR%\web\dist\*" 2^>nul ^| find /c /v ""') do set /a "COPY_COUNT=%%a"
echo   Frontend assets (!COPY_COUNT! files) [OK]

copy /Y "%ROOT_DIR%databases\table.sql"              "%OUTPUT_DIR%\databases\" >nul 2>&1
copy /Y "%ROOT_DIR%databases\data.sql"               "%OUTPUT_DIR%\databases\" >nul 2>&1
echo   Database scripts [OK]

if exist "%ROOT_DIR%tools\import_oracle_ddl" (
    copy /Y "%ROOT_DIR%tools\import_oracle_ddl\*.go" "%OUTPUT_DIR%\tools\import_oracle_ddl\" >nul 2>&1
    copy /Y "%ROOT_DIR%tools\import_oracle_ddl\*.md" "%OUTPUT_DIR%\tools\import_oracle_ddl\" >nul 2>&1
    echo   Import tools [OK]
)

copy /Y "%ROOT_DIR%configs\config_sample.yaml" "%OUTPUT_DIR%\configs\config.yaml" >nul 2>&1
echo   Config template [OK]
:: DATABASE-SETUP.sql is a template shipped with the package (it lives in build\, tracked by git; the bat only copies it verbatim).
:: A missing file must be visible - the old unconditional copy + echo [OK] reported success even when the file was absent.
if exist "%ROOT_DIR%packaging\DATABASE-SETUP.sql" (
    copy /Y "%ROOT_DIR%packaging\DATABASE-SETUP.sql" "%OUTPUT_DIR%\" >nul 2>&1
    echo   Database setup notes [OK]
) else (
    echo [WARN] packaging\DATABASE-SETUP.sql missing, package has no DB setup guide
)

:: The Chinese segmenter dict must ship in the package: gse's no-arg LoadDict() resolves the BUILD machine's module cache absolute path,
:: so on another machine the dict is not found and Chinese search silently degrades (symptom: search returns no results).
set "GSE_DIR="
for /f "delims=" %%i in ('go list -m -f "{{.Dir}}" github.com/go-ego/gse 2^>nul') do set "GSE_DIR=%%i"
if not defined GSE_DIR (
    echo [WARN] Cannot locate the gse module dir - copy data\dict\zh\s_1.txt and t_1.txt into the package data\dict\zh\ by hand.
) else (
    if exist "!GSE_DIR!\data\dict\zh\s_1.txt" (
        copy /Y "!GSE_DIR!\data\dict\zh\s_1.txt" "%OUTPUT_DIR%\data\dict\zh\" >nul 2>&1
        copy /Y "!GSE_DIR!\data\dict\zh\t_1.txt" "%OUTPUT_DIR%\data\dict\zh\" >nul 2>&1
        echo   Chinese segmenter dict [OK]
    ) else (
        echo [WARN] Segmenter dict not found at "!GSE_DIR!\data\dict\zh\s_1.txt" - Chinese search degrades after deployment.
    )
)

call :GenerateStartBat
call :GenerateStopBat
call :GenerateREADME
echo   All resources copied
echo.

:: ========================================
:: Create zip
:: ========================================
echo [6/7] Packaging...

cd /d "%ROOT_DIR%"
powershell -NoProfile -Command "Compress-Archive -Path '%OUTPUT_DIR%' -DestinationPath '%ZIP_FILE%' -Force"
if errorlevel 1 (
    echo [FAIL] Zip creation failed
    pause
    exit /b 1
)

echo ========================================
:: Build installer
:: ========================================
echo [7/7] Building installer ^(Inno Setup^)...

::: Single source of the version number = top-level "version" in web\package.json.
::: Read it with node instead of findstr: package.json carries version keys under dependencies too,
::: and the quote/indent filtering of findstr is fragile; node -p takes the top-level field with no ambiguity
::: (node must exist on this path - the previous step just ran npm run build).
set "APP_VERSION=0.0.0"
::: Do not append 2^>nul inside the backquoted command - for /f mangles the ^> and the whole command fails silently (version degrades to 0.0.0).
::: The path must use forward slashes: in a JS string "d:\Study\Citebase\web" the \S and \A are escape sequences, the backslashes just vanish.
set "PKG_JS=%ROOT_DIR:\=/%/web/package.json"
for /f "usebackq delims=" %%v in (`node -p "require('%PKG_JS%').version"`) do set "APP_VERSION=%%v"
if not defined APP_VERSION set "APP_VERSION=0.0.0"

set "ISCC_EXE=%ISCC_PATH%"
if not exist "%ISCC_EXE%" set "ISCC_EXE=%LOCALAPPDATA%\Programs\Inno Setup 6\ISCC.exe"
if not exist "%ISCC_EXE%" set "ISCC_EXE=%ProgramFiles(x86)%\Inno Setup 6\ISCC.exe"
if not exist "%ISCC_EXE%" set "ISCC_EXE=%ProgramFiles%\Inno Setup 6\ISCC.exe"

::: Setup.exe goes to the repo root, next to the zip - dist\ is wiped by the rmdir at the end of this script.
set "SETUP_FILE=%ROOT_DIR%Citebase-Setup-%APP_VERSION%.exe"
if not exist "%ISCC_EXE%" goto :skip_installer
echo   ISCC: %ISCC_EXE%
echo   Version: %APP_VERSION%
pushd "%ROOT_DIR%"
::: The /O switch of ISCC takes the output DIRECTORY; the Setup.exe file name comes from OutputBaseFilename in [Setup].
"%ISCC_EXE%" /DStageDir="%OUTPUT_DIR%" /DBuildVersion=%APP_VERSION% /O"%ROOT_DIR%" packaging\installer.iss
set "ISCC_RC=%errorlevel%"
popd
if not "%ISCC_RC%"=="0" goto :installer_failed
if not exist "%SETUP_FILE%" goto :installer_failed
set "SETUP_BYTES="
for %%f in ("%SETUP_FILE%") do set "SETUP_BYTES=%%~zf"
echo   Installer: Citebase-Setup-%APP_VERSION%.exe [OK] !SETUP_BYTES! bytes
goto :installer_done

:installer_failed
echo   [WARN] Installer build failed, ISCC exit code %ISCC_RC%. Portable zip is unaffected.
goto :installer_done

:skip_installer
echo   [WARN] Inno Setup 6 ^(ISCC.exe^) not found - installer skipped.
echo          Install Inno Setup 6, or point ISCC_PATH at ISCC.exe, then re-run this script.
echo          The portable zip above is complete and usable without it.

:installer_done
echo.

rmdir /s /q "%BUILD_DIR%"

for %%f in ("%ZIP_FILE%") do set /a "SZ=%%~zf/1024/1024"
set "SETUP_MB="
if exist "%SETUP_FILE%" for %%f in ("%SETUP_FILE%") do set /a "SETUP_MB=%%~zf/1024/1024"
echo.
echo ========================================
echo   Build complete.
echo.
echo   Output: %ZIP_FILE%  (!SZ! MB^)
if defined SETUP_MB echo   Output: %SETUP_FILE%  (!SETUP_MB! MB^)
echo.
echo   How to use - portable zip:
echo     1. Extract the zip file
echo     2. Create an empty database: CREATE DATABASE citebase
echo        DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;
echo     3. Edit configs\config.yaml ^(set database DSN^)
echo     4. Double-click start.bat to run
echo     5. Visit http://localhost:3001 - first start creates all tables
echo.
echo   How to use - installer:
echo     1. Run Citebase-Setup-*.exe ^(installs to %%LOCALAPPDATA%%\Citebase by default^)
echo     2. Create an empty database and edit configs\config.yaml ^(DSN^)
echo     3. Launch from the desktop shortcut
echo.
echo   No need to import table.sql / data.sql: the first start runs
echo   AutoMigrate + seedDefaults and creates every table, the permission
echo   list, roles, system config and the admin account. The random admin
echo   password is printed once into data\logs.
echo ========================================
echo.
pause
exit /b 0

:: ========================================
:: Generated package files
:: ========================================

:GenerateStartBat
(
echo @echo off
echo chcp 65001 ^>nul
echo title Citebase
echo.
echo set "ROOT_DIR=%%~dp0"
echo cd /d "%%ROOT_DIR%%"
echo.
echo if not exist "%%ROOT_DIR%%configs\config.yaml" ^(
echo     echo [ERROR] configs\config.yaml not found
echo     echo Copy config_sample.yaml to config.yaml and edit database DSN
echo     pause
echo     exit /b 1
echo ^)
echo.
echo if not exist "%%ROOT_DIR%%citebase.exe" ^(
echo     echo [ERROR] citebase.exe not found
echo     pause
echo     exit /b 1
echo ^)
echo.
echo if not exist "%%ROOT_DIR%%data\index" mkdir "%%ROOT_DIR%%data\index"
echo if not exist "%%ROOT_DIR%%data\markdowns" mkdir "%%ROOT_DIR%%data\markdowns"
echo if not exist "%%ROOT_DIR%%data\images" mkdir "%%ROOT_DIR%%data\images"
echo if not exist "%%ROOT_DIR%%data\attachment" mkdir "%%ROOT_DIR%%data\attachment"
echo.
echo start "citebase" /NORMAL citebase.exe
echo timeout /t 3 /nobreak ^>nul
echo.
echo echo ========================================
echo echo   Citebase started on http://localhost:3001
echo echo   Press Ctrl+C in the server window, or run stop.bat
echo echo ========================================
echo pause
) > "%OUTPUT_DIR%\start.bat"
exit /b 0

:GenerateStopBat
(
echo @echo off
echo chcp 65001 ^>nul
echo title Citebase - Stop
echo.
echo tasklist /fi "imagename eq citebase.exe" 2^>nul ^| find /i "citebase.exe" ^>nul
echo if errorlevel 1 ^(
echo     echo Server is not running.
echo ^) else ^(
echo     taskkill /f /im citebase.exe 2^>nul
echo     if errorlevel 1 ^(
echo         echo [WARN] Could not stop server. Close the window manually.
echo     ^) else ^(
echo         echo Server stopped.
echo     ^)
echo ^)
echo timeout /t 2 /nobreak ^>nul
echo pause
) > "%OUTPUT_DIR%\stop.bat"
exit /b 0

:GenerateREADME
(
echo ========================================
echo  Citebase - Windows Deployment Package
echo ========================================
echo.
echo  I. Prerequisites
echo     - MySQL 8.0 ^(utf8mb4^)
echo     - Port 3001 available
echo.
echo  II. Initialize Database
echo.
echo     Only an empty database is needed. Do NOT import table.sql / data.sql:
echo     the first start runs AutoMigrate + seedDefaults, which creates all 94
echo     tables, the permission list, roles, system config and the admin account.
echo
echo     1. Create database:
echo        CREATE DATABASE citebase
echo          DEFAULT CHARSET utf8mb4
echo          COLLATE utf8mb4_unicode_ci;
echo.
echo     2. Edit configs\config.yaml, set database.dsn to that database.
echo.
echo     3. Start. The random admin password is printed once into data\logs.
echo.
echo  III. Configure
echo.
echo     Edit configs\config.yaml, update database.dsn:
echo       database:
echo         dsn: "user:password@tcp^(127.0.0.1:3306^)/citebase?charset=utf8mb4^&parseTime=True"
echo.
echo     For production also set a random 32-byte security.encrypt_key,
echo     or inject it as the CITEBASE_ENCRYPT_KEY environment variable ^(legacy AIWIKI_ENCRYPT_KEY works too^).
echo.
echo  IV. Run
echo.
echo     start.bat  - start the server in its own window
echo     stop.bat   - stop the server
echo.
echo  V. Access
echo.
echo     Open http://localhost:3001 - the first visit runs the setup wizard.
echo.
echo  VI. Text-to-SQL schema scan ^(Optional^)
echo.
echo     Go to AI Settings ^> Data Query to scan connected databases,
echo     or import your own DDL file with tools\import_oracle_ddl.
echo.
echo  VII. Chinese Search Dictionary
echo.
echo     data\dict\zh\s_1.txt and t_1.txt are required for Chinese word segmentation.
echo     They are copied automatically by scripts\package-windows.bat. If missing, Chinese
echo     search degrades to CJK bigram fallback. Override the location with CITEBASE_JIEBA_DICT ^(legacy AIWIKI_JIEBA_DICT works too^).
echo.
echo ========================================
) > "%OUTPUT_DIR%\README.txt"
exit /b 0
