@echo off
:: bin\kaboom-agentic-browser.cmd -- Windows launcher for the Kaboom Agentic Browser server.
::
:: Windows has no exec(), so cmd.exe remains as a thin parent of the Go binary --
:: but no Node runtime sits in the chain, and cmd.exe exits with its child.
:: POSIX platforms use the sh shim beside this file, which replaces its own image.
::
:: Resolution mirrors lib\runtime\resolve-binary.js. PATH is never consulted.
setlocal EnableExtensions EnableDelayedExpansion

set "BIN_DIR=%~dp0"
for %%I in ("%BIN_DIR%..") do set "PKG_DIR=%%~fI"
set "PKG_NAME=@brennhill\kaboom-agentic-browser-win32-x64"
set "BIN_NAME=kaboom-agentic-browser.exe"

:: CLI commands are Node's job -- lib\cli\cli.js owns install/config/doctor.
for %%A in (%*) do (
  if /I "%%~A"=="--config"    goto :cli
  if /I "%%~A"=="-c"          goto :cli
  if /I "%%~A"=="--install"   goto :cli
  if /I "%%~A"=="-i"          goto :cli
  if /I "%%~A"=="--update"    goto :cli
  if /I "%%~A"=="--doctor"    goto :cli
  if /I "%%~A"=="--connect"   goto :cli
  if /I "%%~A"=="--uninstall" goto :cli
  if /I "%%~A"=="--help"      goto :cli
  if /I "%%~A"=="-h"          goto :cli
  if /I "%%~A"=="--version"   goto :cli
  if /I "%%~A"=="-v"          goto :cli
)

:: 1. Explicit operator override. Set but missing is a real failure, not a fallthrough.
if defined KABOOM_BINARY_PATH (
  if not exist "%KABOOM_BINARY_PATH%" (
    call :fail "KABOOM_BINARY_PATH is set to '%KABOOM_BINARY_PATH%' but no file exists there."
    exit /b 1
  )
  "%KABOOM_BINARY_PATH%" %*
  exit /b !ERRORLEVEL!
)

:: 2. Source-tree dist build, reachable only from the repo (parent dir "npm").
for %%I in ("%PKG_DIR%\..") do set "PARENT_DIR=%%~nxI"
if /I "!PARENT_DIR!"=="npm" (
  set "DEV_BIN=%PKG_DIR%\..\..\dist\kaboom-agentic-browser-win32-x64.exe"
  if exist "!DEV_BIN!" (
    "!DEV_BIN!" %*
    exit /b !ERRORLEVEL!
  )
)

:: 3. The platform optionalDependency, at the depths npm may place it.
for %%C in (
  "%PKG_DIR%\node_modules\%PKG_NAME%\bin\%BIN_NAME%"
  "%PKG_DIR%\..\%PKG_NAME%\bin\%BIN_NAME%"
  "%PKG_DIR%\..\..\%PKG_NAME%\bin\%BIN_NAME%"
) do (
  if exist %%C (
    %%C %*
    exit /b !ERRORLEVEL!
  )
)

call :fail "Kaboom binary not found for win32-x64. Expected the platform package @brennhill/kaboom-agentic-browser-win32-x64, which installs as an optionalDependency. Repair: npm install -g kaboom-agentic-browser@latest (without --no-optional). To point at a binary yourself, set KABOOM_BINARY_PATH."
exit /b 1

:cli
node "%PKG_DIR%\lib\cli\cli.js" %*
exit /b %ERRORLEVEL%

:: Report resolution failure as a JSON-RPC error so the MCP client shows a protocol
:: error rather than a process that vanished without explanation.
:fail
echo %~1 1>&2
echo {"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":%1,"data":{"isError":true}}}
exit /b 1
