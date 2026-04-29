@echo off

REM === ПУТЬ К EXE ===
set APP=..\.build\apiCall_legacy.exe

REM === ПУТЬ К КОНФИГУ ===
set CONF=..\config.yml


REM === ПАРАМЕТРЫ ===
set URL=/settings
set METHOD=GET

echo ======================================
echo Starting ApiCaller...
echo ======================================

%APP% ^
-url=%URL% ^
-method=%METHOD% ^
-conf=%CONF% ^
-xml ^
-debug ^
-batch

echo ======================================
echo Finished
echo ======================================

pause