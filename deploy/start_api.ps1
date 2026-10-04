# Wrapper PowerShell: lanza la API en WSL2 en segundo plano.
# Uso: pwsh deploy/start_api.ps1 [-Port 8091]
param([string]$Port = "8091")
$ErrorActionPreference = "Stop"
$proj = Split-Path -Parent $PSScriptRoot

$bin = Join-Path $proj "dist\g360-ventas-api-linux"
if (-not (Test-Path $bin)) {
  Write-Host "ERROR: binario no encontrado en $bin" -ForegroundColor Red
  Write-Host "Ejecuta: pwsh deploy/build-linux.ps1" -ForegroundColor Yellow
  exit 1
}

Write-Host "Iniciando API en WSL2 (puerto $Port)..." -ForegroundColor Cyan
# El script bash hace export de TODAS las variables dentro de WSL,
# asi no depende del passthrough de entorno PowerShell -> WSL.
Start-Job -Name "g360-api" -ScriptBlock {
  param($p, $root)
  wsl -d Ubuntu -- bash "$root/deploy/start_api.sh" $p
} -ArgumentList $Port, "/mnt/c/Users/ccusi/Documents/Proyect_Coder/G360-ecosystem/projects/g360-ventas-api" | Out-Null

Start-Sleep -Seconds 4
try {
  $health = Invoke-RestMethod -Uri "http://127.0.0.1:$Port/api/health" -TimeoutSec 10
  Write-Host "API OK: $($health.status) db=$($health.db)" -ForegroundColor Green
} catch {
  Write-Host "API aun no responde en http://127.0.0.1:$Port/api/health" -ForegroundColor Yellow
  Write-Host "Revisa: wsl -d Ubuntu -- tail -20 /home/ccusi/g360data/api.log" -ForegroundColor Yellow
  exit 1
}
Write-Host "Listo. Probar login:" -ForegroundColor Cyan
Write-Host "  curl -X POST http://127.0.0.1:$Port/api/login -H 'Content-Type: application/json' -d '{\"user\":\"ccusi\",\"password\":\"123456\"}'"
