# Refresca el snapshot que sirve el API en WSL2 y reinicia el API.
# 1) backup consistente del historial.db vivo a NTFS (~40s)
# 2) copia al disco ext4 de WSL (~2 min)
# 3) reinicia el API dentro de WSL y espera que responda health
#
# Apto para Scheduled Task: log a archivo, exit code != 0 ante cualquier fallo.
#   powershell -File refresh_snapshot.ps1 [-WslDistro Ubuntu] [-NoRestart] [-Python <ruta>]
#
# Nota: los hijos se lanzan con Start-Process (no con el call operator `&`).
# Bajo Task Scheduler PowerShell no tiene consola y el `&` deja al hijo
# huerfano: el script muere con 0xC000013A a mitad del backup.
param(
    [string]$WslDistro = "Ubuntu",
    [switch]$NoRestart,
    [string]$Python = "",
    [string]$LogFile = ""
)
$ErrorActionPreference = "Stop"
$proj = Split-Path -Parent $PSScriptRoot
$stage = Join-Path $env:TEMP "g360_snapshot\historial.db"

if (-not $LogFile) {
    $LogFile = Join-Path $env:TEMP "g360_snapshot\refresh_snapshot.log"
}
$logDir = Split-Path -Parent $LogFile
if ($logDir) { New-Item -ItemType Directory -Force -Path $logDir | Out-Null }

function Write-Log {
    param([string]$Msg)
    Write-Host $Msg
    Add-Content -LiteralPath $LogFile -Value ("{0} {1}" -f (Get-Date -Format "yyyy-MM-dd HH:mm:ss"), $Msg)
}

# Lanza un proceso hijo, captura su salida al log y devuelve su exit code.
function Invoke-Step {
    param(
        [string]$FilePath,
        [string[]]$ArgList,
        [string]$Tag
    )
    $out = Join-Path $logDir "$Tag.out"
    $err = Join-Path $logDir "$Tag.err"
    Remove-Item -LiteralPath $out, $err -ErrorAction SilentlyContinue
    Write-Log "  > ${Tag}: $FilePath $($ArgList -join ' ')"
    # -WindowStyle Hidden y NO -NoNewWindow: bajo Task Scheduler, -NoNewWindow hace
    # que el hijo herede la consola del padre y el backup de 2.6 GB muere con
    # 0xC000013A (verificado). Con ventana oculta el mismo backup completa.
    $p = Start-Process -FilePath $FilePath -ArgumentList $ArgList -WindowStyle Hidden -Wait -PassThru `
        -RedirectStandardOutput $out -RedirectStandardError $err
    if (Test-Path -LiteralPath $out) {
        Get-Content -LiteralPath $out | ForEach-Object { if ($_ -ne "") { Write-Log "  | $_" } }
    }
    $code = $p.ExitCode
    if ($code -ne 0 -and (Test-Path -LiteralPath $err)) {
        $e = (Get-Content -LiteralPath $err -Raw)
        if ($e) { Write-Log "  ! stderr: $($e.Trim())" }
    }
    return $code
}

# Intérprete explícito: en tarea programada el PATH heredado no es el del shell.
if (-not $Python) {
    $cmd = Get-Command python -ErrorAction SilentlyContinue
    if ($cmd) { $Python = $cmd.Source }
    else {
        $cmd = Get-Command py -ErrorAction SilentlyContinue
        if ($cmd) { $Python = $cmd.Source }
    }
    if (-not $Python) { throw "no se encontro interprete python (usar -Python <ruta>)" }
}
try {
Write-Log "inicio | distro=$WslDistro | python=$Python | restart=$(-not $NoRestart)"

Write-Log "[1/3] Backup consistente..."
$code = Invoke-Step -FilePath $Python -Tag "backup" -ArgList @(
    (Join-Path $PSScriptRoot "backup_snapshot.py"), "--dest", $stage
)
if ($code -ne 0) { throw "backup fallo (exit=$code)" }

Write-Log "[2/3] Copia a ext4 de WSL (~2 min)..."
$stageWsl = "/mnt/c" + ($stage -replace "^[A-Za-z]:", "" -replace "\\", "/")
$code = Invoke-Step -FilePath "wsl.exe" -Tag "copy" -ArgList @(
    "-d", $WslDistro, "--", "bash", "-c",
    "cp '$stageWsl' ~/g360data/historial.db.tmp && mv ~/g360data/historial.db.tmp ~/g360data/historial.db && ls -la ~/g360data/historial.db"
)
if ($code -ne 0) { throw "copia a WSL fallo (exit=$code)" }

if (-not $NoRestart) {
    Write-Log "[3/3] Reiniciando API en WSL..."
    Invoke-Step -FilePath "wsl.exe" -Tag "pkill" -ArgList @(
        "-d", $WslDistro, "--", "bash", "-c", "pkill -f g360-ventas || true"
    ) | Out-Null
    Start-Sleep -Seconds 1
    $runner = (Join-Path $PSScriptRoot "run_api_wsl.sh") -replace "^[A-Za-z]:", "/mnt/c" -replace "\\", "/"
    Start-Process -FilePath "wsl.exe" -ArgumentList @("-d", $WslDistro, "--", "bash", $runner) -WindowStyle Hidden

    # Reintentos: el API puede tardar en abrir la DB de 2.6 GB.
    $health = ""
    for ($i = 1; $i -le 15; $i++) {
        Start-Sleep -Seconds 3
        $health = Invoke-Step -FilePath "curl.exe" -Tag "health" -ArgList @(
            "-sS", "-m", "15", "http://127.0.0.1:8091/api/health"
        )
        $h = ""
        $hFile = Join-Path $logDir "health.out"
        if (Test-Path -LiteralPath $hFile) { $h = (Get-Content -LiteralPath $hFile -Raw) }
        if ($health -eq 0 -and $h -match '"status"\s*:\s*"ok"') {
            Write-Log "health ok en $($i * 3)s: $h"
            break
        }
        Write-Log "health aun no listo (intento $i/15)"
    }
    if ($health -ne 0) { throw "API no quedo saludable tras reinicio" }
}
Write-Log "listo"
exit 0
}
catch {
    Write-Log "ERROR: $($_.Exception.Message)"
    Write-Log "  tipo   : $($_.Exception.GetType().FullName)"
    Write-Log "  origen : $($_.InvocationInfo.Line.Trim())"
    Write-Log "FALLO refresh snapshot"
    exit 1
}
