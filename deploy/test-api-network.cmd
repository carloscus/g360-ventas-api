@echo off
rem ---------------------------------------------------------------------------
rem Prueba la conectividad y latencia hacia el API Go de ventas desde otra PC.
rem No requiere credenciales, WSL ni Python. Solo curl.exe (incluido en Win10+).
rem
rem Uso:  test-api-network.cmd 192.168.1.50 [puerto]
rem ---------------------------------------------------------------------------
setlocal enabledelayedexpansion

if "%~1"=="" (
    echo Uso: %~nx0 SERVIDOR [PUERTO]
    echo Ejemplo: %~nx0 192.168.1.50
    exit /b 2
)
set "SERVER=%~1"
set "PORT=%~2"
if "%PORT%"=="" set "PORT=8090"
set "BASE=http://%SERVER%:%PORT%"

echo.
echo === Test API G360 [%BASE%] ===
echo.

rem --- 1) TCP via powershell ---
echo [1/2] TCP %SERVER%:%PORT% ...
powershell -NoProfile -Command "$c=New-Object Net.Sockets.TcpClient; try { $c.Connect('%SERVER%',%PORT%); exit 0 } catch { exit 1 }"
if %ERRORLEVEL% NEQ 0 goto :tcp_fail
echo [1/2] TCP %SERVER%:%PORT% ... OK
goto :http_test

:tcp_fail
echo [1/2] TCP %SERVER%:%PORT% ... FALLO
echo.
echo VEREDICTO: INALCANZABLE
echo   - Servidor encendido?
echo   - Misma red? ping %SERVER%
echo   - Firewall permite puerto %PORT%?
echo   - forwarder.py corriendo en el servidor?
exit /b 1

:http_test
rem --- 2) HTTP /api/health (3 muestras con curl) ---
echo [2/2] Latencia GET /api/health ...
set "FAILS=0"
set "SUM=0"
set "N=0"
for /L %%i in (1,1,3) do call :muestra %%i
goto :veredicto

:muestra
set "IDX=%~1"
set "MS=-1"
curl.exe -sS -m 15 -o "%TEMP%\g360_probe%IDX%.out" -w "%%{time_total}" "%BASE%/api/health" > "%TEMP%\g360_ms%IDX%.txt" 2>nul
set /p SECT=<"%TEMP%\g360_ms%IDX%.txt" 2>nul
del "%TEMP%\g360_ms%IDX%.txt" 2>nul
if "%SECT%"=="" goto :muestra_fallo
for /f %%m in ('powershell -NoProfile -Command "[int](%SECT% * 1000)"') do set "MS=%%m"
if "%MS%"=="" set "MS=0"
findstr /c:"status" "%TEMP%\g360_probe%IDX%.out" >nul 2>&1
if %ERRORLEVEL% NEQ 0 goto :muestra_fallo
findstr /c:"ok" "%TEMP%\g360_probe%IDX%.out" >nul 2>&1
if %ERRORLEVEL% NEQ 0 goto :muestra_fallo
echo   muestra %IDX%: %MS% ms  ok
set /a SUM+=%MS%
set /a N+=1
del "%TEMP%\g360_probe%IDX%.out" 2>nul
goto :eof

:muestra_fallo
echo   muestra %IDX%: RESPUESTA NO-OK
set /a FAILS+=1
set /a N+=1
del "%TEMP%\g360_probe%IDX%.out" 2>nul
goto :eof

:veredicto
echo.
if %FAILS%==3 goto :api_caido
set /a AVG=%SUM%/%N%
echo Resumen: avg=%AVG% ms  fallos=%FAILS%/3
echo.
if %AVG% LEQ 2000 goto :veredicto_ok
echo VEREDICTO: LENTO - sync por red sera doloroso
echo   - Para carga inicial o brechas grandes, preferir cartucho/USB.
echo   - Revisar cableado/WiFi entre esta PC y %SERVER%.
exit /b 0

:veredicto_ok
echo VEREDICTO: OK - red apta para sync incremental
exit /b 0

:api_caido
echo VEREDICTO: PUERTO ABIERTO PERO API NO RESPONDE
echo   - El forwarder escucha pero el API Go en WSL esta caido.
echo   - Avisar al admin del servidor.
exit /b 2
