<#
.SYNOPSIS
  Prueba la conectividad y latencia hacia el API Go de ventas (g360-ventas-api)
  desde cualquier PC de la misma red. No requiere credenciales ni WSL.

.DESCRIPTION
  1. Verifica TCP al puerto 8090 del servidor.
  2. Mide latencia de GET /api/health (5 muestras).
  3. Da veredicto: OK / LENTO / INALCANZABLE con guía de acción.

.USAGE
  powershell -ExecutionPolicy Bypass -File test-api-network.ps1 -Server 192.168.1.50
  powershell -ExecutionPolicy Bypass -File test-api-network.ps1 -Server 192.168.1.50 -Port 8090
  powershell -ExecutionPolicy Bypass -File test-api-network.ps1 -Server 192.168.1.50 -OutJson reporte.json
  # -OutJson "auto" (default): guarda g360-nettest-<PC>-<fecha>.json junto al script.
  # Llevar ese JSON al servidor para agregarlo con collect-network-stats.ps1.
#>
param(
    [Parameter(Mandatory = $true, HelpMessage = "IP o hostname del servidor (ej: 192.168.1.50)")]
    [string]$Server,
    [int]$Port = 8090,
    [int]$Samples = 5,
    [string]$OutJson = "auto"
)

$ErrorActionPreference = "Continue"
$base = "http://${Server}:${Port}"
$verdict = "DESCONOCIDO"
$tcpOk = $false
$lat = @()
$fails = 0

function Save-Report {
    param([string]$OutJson, [string]$verdict)
    if ([string]::IsNullOrWhiteSpace($OutJson) -or $OutJson -eq "none") { return "" }
    $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
    if ($OutJson -eq "auto") {
        $dir = if ($PSScriptRoot) { $PSScriptRoot } else { (Get-Location).Path }
        $OutJson = Join-Path $dir ("g360-nettest-{0}-{1}.json" -f $env:COMPUTERNAME, $stamp)
    }
    $stats = $null
    if ($lat.Count -gt 0) {
        $stats = @{
            min_ms = ($lat | Measure-Object -Minimum).Minimum
            avg_ms = [int]($lat | Measure-Object -Average).Average
            max_ms = ($lat | Measure-Object -Maximum).Maximum
        }
    }
    $report = [ordered]@{
        pc        = $env:COMPUTERNAME
        timestamp = (Get-Date).ToString("o")
        server    = $Server
        port      = $Port
        tcp_ok    = $tcpOk
        samples_ms = @($lat)
        min_ms    = $(if ($stats) { $stats.min_ms } else { $null })
        avg_ms    = $(if ($stats) { $stats.avg_ms } else { $null })
        max_ms    = $(if ($stats) { $stats.max_ms } else { $null })
        fails     = $fails
        total     = $Samples
        verdict   = $verdict
        script    = "test-api-network.ps1 v2"
    }
    $report | ConvertTo-Json -Depth 3 | Set-Content -LiteralPath $OutJson -Encoding UTF8
    Write-Host ""
    Write-Host ("Reporte guardado: {0}" -f $OutJson) -ForegroundColor Cyan
    return $OutJson
}

Write-Host ""
Write-Host "=== Test API G360 ($base) ===" -ForegroundColor Cyan
Write-Host "Fecha: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"
Write-Host ""

# ── 1) TCP ───────────────────────────────────────────────────────────────
Write-Host ("[1/2] Conectividad TCP {0}:{1} ..." -f $Server, $Port) -NoNewline
try {
    $client = New-Object Net.Sockets.TcpClient
    $iar = $client.BeginConnect($Server, $Port, $null, $null)
    if ($iar.AsyncWaitHandle.WaitOne(5000)) {
        $client.EndConnect($iar)
        $tcpOk = $true
    }
    $client.Close()
} catch { $tcpOk = $false }

if (-not $tcpOk) {
    Write-Host " FALLÓ" -ForegroundColor Red
    Write-Host ""
    Write-Host "VEREDICTO: INALCANZABLE" -ForegroundColor Red
    Write-Host "  - ¿El servidor está encendido?"
    Write-Host "  - ¿Están en la misma red? (ping $Server)"
    Write-Host "  - ¿El firewall permite el puerto $Port?"
    Write-Host "  - ¿El forwarder.py está corriendo en el servidor?"
    $verdict = "INALCANZABLE"
    Save-Report -OutJson $OutJson -verdict $verdict | Out-Null
    exit 1
}
Write-Host " OK" -ForegroundColor Green

# ── 2) HTTP /api/health ──────────────────────────────────────────────────
Write-Host ("[2/2] Latencia GET /api/health ({0} muestras) ..." -f $Samples)
for ($i = 1; $i -le $Samples; $i++) {
    $sw = [Diagnostics.Stopwatch]::StartNew()
    try {
        $r = curl.exe -sS -m 15 "$base/api/health" 2>$null
        $sw.Stop()
        $ms = [int]$sw.Elapsed.TotalMilliseconds
        $lat += $ms
        $ok = $r -match '"status"\s*:\s*"ok"'
        Write-Host ("  muestra {0}: {1} ms  {2}" -f $i, $ms, ($(if ($ok) { "ok" } else { "RESPUESTA RARA: $r" })))
        if (-not $ok) { $fails++ }
    } catch {
        $sw.Stop()
        $fails++
        Write-Host ("  muestra {0}: ERROR ({1})" -f $i, $_.Exception.Message)
    }
}

Write-Host ""
if ($fails -eq $Samples) {
    Write-Host "VEREDICTO: PUERTO ABIERTO PERO API NO RESPONDE" -ForegroundColor Red
    Write-Host "  - El forwarder escucha pero el API Go en WSL está caído."
    Write-Host "  - Avisar al admin del servidor para que lo reinicie."
    $verdict = "API_CAIDA"
    Save-Report -OutJson $OutJson -verdict $verdict | Out-Null
    exit 2
}

$avg = [int]($lat | Measure-Object -Average).Average
$min = ($lat | Measure-Object -Minimum).Minimum
$max = ($lat | Measure-Object -Maximum).Maximum
Write-Host ("Resumen: min={0} ms  avg={1} ms  max={2} ms  fallos={3}/{4}" -f $min, $avg, $max, $fails, $Samples)

Write-Host ""
if ($avg -le 500 -and $fails -eq 0) {
    Write-Host "VEREDICTO: OK — red apta para sync incremental" -ForegroundColor Green
    $verdict = "OK"
    Save-Report -OutJson $OutJson -verdict $verdict | Out-Null
    exit 0
} elseif ($avg -le 2000) {
    Write-Host "VEREDICTO: ACEPTABLE pero lento — el sync funcionará, más despacio" -ForegroundColor Yellow
    Write-Host "  - Si el sync de 7 días tarda >10 min, considerar cartucho/USB."
    $verdict = "ACEPTABLE"
    Save-Report -OutJson $OutJson -verdict $verdict | Out-Null
    exit 0
} else {
    Write-Host "VEREDICTO: LENTO — sync por red será doloroso" -ForegroundColor Yellow
    Write-Host "  - Para carga inicial o brechas grandes, preferir cartucho/USB."
    Write-Host "  - Revisar cableado/WiFi entre esta PC y $Server."
    $verdict = "LENTO"
    Save-Report -OutJson $OutJson -verdict $verdict | Out-Null
    exit 0
}
