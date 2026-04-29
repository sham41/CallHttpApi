@echo off

set APP=..\.build\apiCall.exe
set CONF=..\config.yml

%APP% -url=/settings/import_count -method=GET -conf="%CONF%" -xml -debug -batch

pause