@echo off
setlocal

set "ENVNAME=%~1"

if /I "%ENVNAME%"=="win10" (
    set "VDIR=%~dp0..\infra\vagrant\windows10"
) else if /I "%ENVNAME%"=="deb12" (
    set "VDIR=%~dp0..\infra\vagrant\debian12\deb-test"
) else if /I "%ENVNAME%"=="opwrt" (
    set "VDIR=%~dp0..\infra\vagrant\openwrt"
) else (
    echo Unknown environment: %ENVNAME%
    exit /b 1
)

shift

set "ARGS="

:collect
if "%~1"=="" goto run
set "ARGS=%ARGS% "%~1""
shift
goto collect

:run
set "VAGRANT_CWD=%VDIR%"
vagrant %ARGS%
exit /b %ERRORLEVEL%