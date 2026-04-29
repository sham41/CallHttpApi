@echo off

set GO=C:\Go1.18\bin\go.exe

REM перейти в корень проекта
cd /d %~dp0\..

set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0

%GO% mod tidy -modfile=go.legacy.mod
%GO% build -modfile=go.legacy.mod -o .build/apiCall_legacy.exe

echo Legacy Build Done!
pause