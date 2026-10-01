# Compila el binario Linux que corre dentro de WSL2.
param()
$ErrorActionPreference = "Stop"
$proj = Split-Path -Parent $PSScriptRoot
$go = "C:\Program Files\Go\bin\go.exe"
if (-not (Test-Path $go)) { $go = "go" }
$env:GOOS = "linux"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
& $go build -o (Join-Path $proj "dist\g360-ventas-api-linux") ./cmd/server
$code = $LASTEXITCODE
$env:GOOS = $null; $env:GOARCH = $null; $env:CGO_ENABLED = $null
if ($code -ne 0) { throw "build fallo" }
Write-Host "OK: dist\g360-ventas-api-linux"
