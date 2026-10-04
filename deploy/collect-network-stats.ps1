<#
.SYNOPSIS
  Agrega los reportes JSON de test-api-network.ps1 de varias PCs y muestra
  estadísticas por máquina y globales.

.DESCRIPTION
  Flujo:
    1. Cada PC corre test-api-network.ps1 (genera g360-nettest-<PC>-<fecha>.json).
    2. Los JSON se copian a una carpeta del servidor (red/USB/email).
    3. Este script lee la carpeta e imprime tabla por PC + resumen global,
       y guarda un CSV para Excel.

.USAGE
  powershell -ExecutionPolicy Bypass -File collect-network-stats.ps1 -Dir .\reportes
  powershell -ExecutionPolicy Bypass -File collect-network-stats.ps1 -Dir .\reportes -OutCsv resumen.csv
#>
param(
    [Parameter(Mandatory = $true, HelpMessage = "Carpeta con los g360-nettest-*.json")]
    [string]$Dir,
    [string]$OutCsv = ""
)

if (-not (Test-Path -LiteralPath $Dir -PathType Container)) {
    Write-Host "ERROR: no existe la carpeta: $Dir" -ForegroundColor Red
    exit 2
}

$files = Get-ChildItem -LiteralPath $Dir -Filter "g360-nettest-*.json" -File -ErrorAction SilentlyContinue
if (-not $files -or $files.Count -eq 0) {
    Write-Host "No hay reportes g360-nettest-*.json en $Dir" -ForegroundColor Yellow
    Write-Host "Cada PC debe correr: test-api-network.ps1 -Server <IP-servidor>"
    exit 1
}

$rows = @()
foreach ($f in $files) {
    try {
        $d = Get-Content -LiteralPath $f.FullName -Raw -Encoding UTF8 | ConvertFrom-Json
        $rows += [pscustomobject]@{
            PC        = $d.pc
            Fecha     = $d.timestamp
            Servidor  = $d.server
            TCP       = if ($d.tcp_ok) { "OK" } else { "FALLO" }
            Min_ms    = $d.min_ms
            Avg_ms    = $d.avg_ms
            Max_ms    = $d.max_ms
            Fallos    = ("{0}/{1}" -f $d.fails, $d.total)
            Veredicto = $d.verdict
            Archivo   = $f.Name
        }
    } catch {
        Write-Host ("Omitido (JSON inválido): {0}" -f $f.Name) -ForegroundColor Yellow
    }
}

if ($rows.Count -eq 0) {
    Write-Host "Ningún reporte válido." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host ("=== Red G360: {0} PCs reportadas ===" -f $rows.Count) -ForegroundColor Cyan
$rows | Sort-Object PC | Format-Table -AutoSize PC, Fecha, TCP, Min_ms, Avg_ms, Max_ms, Fallos, Veredicto | Out-String | Write-Host

# ── Resumen global (solo PCs con muestras) ───────────────────────────────
$conDatos = @($rows | Where-Object { $_.Avg_ms -ne $null })
if ($conDatos.Count -gt 0) {
    $avgs = $conDatos | Select-Object -ExpandProperty Avg_ms
    $gAvg = [int]($avgs | Measure-Object -Average).Average
    $gMin = ($avgs | Measure-Object -Minimum).Minimum
    $gMax = ($avgs | Measure-Object -Maximum).Maximum
    $ok = @($rows | Where-Object { $_.Veredicto -eq "OK" }).Count
    $acep = @($rows | Where-Object { $_.Veredicto -eq "ACEPTABLE" }).Count
    $lento = @($rows | Where-Object { $_.Veredicto -eq "LENTO" }).Count
    $mal = @($rows | Where-Object { $_.Veredicto -in @("INALCANZABLE", "API_CAIDA") }).Count
    Write-Host "--- Global ---"
    Write-Host ("  PCs con datos: {0}  |  avg-lan: {1} ms (min {2}, max {3})" -f $conDatos.Count, $gAvg, $gMin, $gMax)
    Write-Host ("  OK: {0}  ACEPTABLE: {1}  LENTO: {2}  CON PROBLEMAS: {3}" -f $ok, $acep, $lento, $mal)
    if ($mal -gt 0) {
        Write-Host "  PCs con problemas:" -ForegroundColor Yellow
        @($rows | Where-Object { $_.Veredicto -in @("INALCANZABLE", "API_CAIDA") }) | ForEach-Object {
            Write-Host ("    - {0}: {1}" -f $_.PC, $_.Veredicto) -ForegroundColor Yellow
        }
    }
    Write-Host ""
}

if ([string]::IsNullOrWhiteSpace($OutCsv)) {
    $OutCsv = Join-Path $Dir ("g360-red-resumen-{0}.csv" -f (Get-Date -Format "yyyyMMdd-HHmmss"))
}
$rows | Sort-Object PC | Export-Csv -LiteralPath $OutCsv -NoTypeInformation -Encoding UTF8
Write-Host ("CSV guardado: {0}" -f $OutCsv) -ForegroundColor Cyan
