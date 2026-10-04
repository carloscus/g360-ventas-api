@echo off
rem ---------------------------------------------------------------------------
rem Refresca el snapshot que sirve el API en WSL2 y reinicia el API.
rem   1) backup consistente del historial.db de Tauri (~40-90s, 2.6 GB)
rem   2) backup rotativo del snapshot vigente (archiva ultimas 3 copias)
rem   3) copia del stage a ext4 de WSL (~1 min)
rem   4) reinicia el API en WSL y espera /api/health
rem
rem Entry point para Task Scheduler. Se usa cmd.exe y no PowerShell a proposito:
rem bajo el scheduler, PowerShell esperando a un hijo de IO pesado (el backup)
rem muere con 0xC000013A a los ~85s; el mismo backup como hijo de cmd.exe
rem completa correctamente (verificado).
rem
rem Uso manual:  deploy\refresh_snapshot.cmd [ruta_python]
rem Log:         %TEMP%\g360_snapshot\refresh_snapshot.log
rem ---------------------------------------------------------------------------
setlocal enabledelayedexpansion

set "PROJ=%~dp0"
set "DISTRO=Ubuntu"
set "PY=%~1"
if "%PY%"=="" set "PY=C:\Users\ccusi\AppData\Local\Programs\Python\Python314\python.exe"

set "STAGE=%TEMP%\g360_snapshot"
set "LOG=%STAGE%\refresh_snapshot.log"
if not exist "%STAGE%" mkdir "%STAGE%"

set "SNAP=%STAGE%\historial.db"
set "HEALTH=%STAGE%\health.out"

rem Ojo: sin "|" en los mensajes, call reparsea y los trata como pipes.
call :log "inicio - distro=%DISTRO% - python=%PY%"

rem --- 1) backup consistente -------------------------------------------------
call :log "[1/4] Backup consistente (2.6 GB, ~90s)..."
"%PY%" "%PROJ%backup_snapshot.py" --dest "%SNAP%" >> "%LOG%" 2>&1
if errorlevel 1 (
    call :log "FALLO backup (errorlevel %errorlevel%)"
    exit /b 1
)
call :log "backup ok"

rem --- 0.5) conversion de ruta del stage a WSL (compartida por 2a y 2b) --------
if /i not "%SNAP:~0,2%"=="C:" (
    call :log "FALLO: se esperaba la unidad C: en %SNAP%"
    exit /b 1
)
set "SNAPTAIL=%SNAP:~3%"
set "SNAPTAIL=%SNAPTAIL:\=/%"
set "SNAPWSL=/mnt/c/%SNAPTAIL%"

rem --- 2a) backup rotativo pre-copia (archiva la version anterior) ------------
call :log "[2/4] Archivando snapshot previo (ultimas 3 copias)..."
set "ARCHIVER=%PROJ%backup_snapshot_archive.sh"
if not exist "%ARCHIVER%" (
    call :log "FALLO: no existe %ARCHIVER%"
    exit /b 1
)
if /i not "%ARCHIVER:~0,2%"=="C:" (
    call :log "FALLO: se esperaba la unidad C: en %ARCHIVER%"
    exit /b 1
)
set "ARCHWSL=/mnt/c/%ARCHIVER:~3%"
set "ARCHWSL=%ARCHWSL:\=/%"
wsl -d %DISTRO% -- bash "%ARCHWSL%" 3 >> "%LOG%" 2>&1
if errorlevel 1 (
    call :log "FALLO backup archival (errorlevel %errorlevel%)"
    exit /b 1
)

rem --- 2b) copia a ext4 (rename atomico) --------------------------------------
call :log "[3/4] Copia del snapshot a ext4 de WSL..."
wsl -d %DISTRO% -- bash -c "cp '%SNAPWSL%' ~/g360data/historial.db.tmp && mv ~/g360data/historial.db.tmp ~/g360data/historial.db && ls -la ~/g360data/historial.db" >> "%LOG%" 2>&1
if errorlevel 1 (
    call :log "FALLO copia a WSL (errorlevel %errorlevel%)"
    exit /b 1
)

rem --- 3) reinicio del API + espera de health --------------------------------
call :log "[4/4] Reiniciando API en WSL..."
wsl -d %DISTRO% -- bash -c "pkill -f g360-ventas || true" >> "%LOG%" 2>&1
ping -n 2 127.0.0.1 >nul

set "RUNNER=%PROJ%run_api_wsl.sh"
if not exist "%RUNNER%" (
    call :log "FALLO: no existe %RUNNER%"
    exit /b 1
)
if /i not "%RUNNER:~0,2%"=="C:" (
    call :log "FALLO: se esperaba la unidad C: en %RUNNER%"
    exit /b 1
)
set "RUNTAIL=%RUNNER:~3%"
set "RUNTAIL=%RUNTAIL:\=/%"
set "RUNNERWSL=/mnt/c/%RUNTAIL%"
call :log "runnerWSL = %RUNNERWSL%"
rem Arrancar el API como wsl.exe en primer plano y desacoplado de esta consola.
rem No usar "setsid ... ^&" dentro de bash: al terminar el wsl.exe de una sola
rem llamada la instancia WSL queda ociosa y se apaga, matando el proceso en
rem segundo plano. Con start /b el wsl.exe vive mientras el API corra.
start "" /b wsl.exe -d %DISTRO% -- bash %RUNNERWSL%

set "ATTEMPT=0"
:health
set /a ATTEMPT+=1
ping -n 4 127.0.0.1 >nul
curl.exe -sS -m 15 "http://127.0.0.1:8091/api/health" > "%HEALTH%" 2>&1
if not errorlevel 1 (
    findstr /c:"\"status\":\"ok\"" "%HEALTH%" >nul 2>&1 && (
        set /a SEG=!ATTEMPT!*3
        call :log "health ok tras ~!SEG!s"
        type "%HEALTH%" >> "%LOG%"
        goto :fin_ok
    )
)
if !ATTEMPT! lss 15 (
    call :log "health aun no listo (intento !ATTEMPT!/15)"
    goto :health
)
call :log "FALLO: el API no quedo saludable tras reiniciar"
exit /b 1

:fin_ok
call :log "listo"
exit /b 0

rem ---------------------------------------------------------------------------
:log
echo %date% %time% %~1
echo %date% %time% %~1>> "%LOG%"
goto :eof
