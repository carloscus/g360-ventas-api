# Refresca el snapshot que sirve el API en WSL2 y reinicia el API.
# 1) backup consistente del historial.db vivo a NTFS (~40s)
# 2) copia al disco ext4 de WSL (~2 min)
# 3) reinicia el API dentro de WSL
param(
    [string]$WslDistro = "Ubuntu",
    [switch]$NoRestart
)
$ErrorActionPreference = "Stop"
$proj = Split-Path -Parent $PSScriptRoot
$stage = Join-Path $env:TEMP "g360_snapshot\historial.db"

Write-Host "[1/3] Backup consistente..."
python (Join-Path $PSScriptRoot "backup_snapshot.py") --dest $stage
if ($LASTEXITCODE -ne 0) { throw "backup fallo" }

Write-Host "[2/3] Copia a ext4 de WSL (~2 min)..."
$stageWsl = "/mnt/c" + ($stage -replace "^[A-Za-z]:", "" -replace "\\", "/")
wsl -d $WslDistro -- bash -c "cp '$stageWsl' ~/g360data/historial.db.tmp && mv ~/g360data/historial.db.tmp ~/g360data/historial.db && ls -la ~/g360data/historial.db"
if ($LASTEXITCODE -ne 0) { throw "copia a WSL fallo" }

if (-not $NoRestart) {
    Write-Host "[3/3] Reiniciando API en WSL..."
    wsl -d $WslDistro -- bash -c "pkill -f g360-ventas || true"
    Start-Sleep -Seconds 1
    $runner = (Join-Path $PSScriptRoot "run_api_wsl.sh") -replace "^[A-Za-z]:", "/mnt/c" -replace "\\", "/"
    Start-Process -FilePath "wsl.exe" -ArgumentList @("-d", $WslDistro, "--", "bash", $runner) -WindowStyle Hidden
    Start-Sleep -Seconds 4
    curl.exe -sS -m 15 "http://127.0.0.1:8091/api/health"
    Write-Host ""
}
Write-Host "listo"
