@echo off
:: bin\kaboom-hooks.cmd -- Windows launcher for the kaboom-hooks binary.
::
:: Windows has no exec(), so cmd.exe remains as a thin parent -- but no Node runtime
:: sits in the chain. Hooks fire on every Edit/Write, so a Node launcher here would
:: leak once per tool call.
::
:: Resolution mirrors lib\runtime\resolve-binary.js. PATH is never consulted.
setlocal EnableExtensions EnableDelayedExpansion

set "BIN_DIR=%~dp0"
for %%I in ("%BIN_DIR%..") do set "PKG_DIR=%%~fI"
set "PKG_NAME=@brennhill\kaboom-agentic-browser-win32-x64"
set "BIN_NAME=kaboom-hooks.exe"

if defined KABOOM_HOOKS_BINARY_PATH (
  if not exist "%KABOOM_HOOKS_BINARY_PATH%" (
    echo KABOOM_HOOKS_BINARY_PATH is set to '%KABOOM_HOOKS_BINARY_PATH%' but no file exists there. 1>&2
    exit /b 1
  )
  "%KABOOM_HOOKS_BINARY_PATH%" %*
  exit /b !ERRORLEVEL!
)

for %%I in ("%PKG_DIR%\..") do set "PARENT_DIR=%%~nxI"
if /I "!PARENT_DIR!"=="npm" (
  set "DEV_BIN=%PKG_DIR%\..\..\dist\kaboom-hooks.exe"
  if exist "!DEV_BIN!" (
    "!DEV_BIN!" %*
    exit /b !ERRORLEVEL!
  )
)

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

echo kaboom-hooks binary not found for win32-x64. Expected the platform package @brennhill/kaboom-agentic-browser-win32-x64, which installs as an optionalDependency. Repair: npm install -g kaboom-agentic-browser@latest (without --no-optional). To point at a binary yourself, set KABOOM_HOOKS_BINARY_PATH. 1>&2
exit /b 1
