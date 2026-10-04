#!/bin/bash
# Inicia la API de ventas en WSL2 con variables de entorno explicitas.
# Uso: bash deploy/start_api.sh [puerto]   (defecto 8091)
# Disenado para correr DENTRO de WSL: todas las rutas/export ocurren aqui,
# asi no depende de que PowerShell propague variables de entorno.
set -e

API_PORT="${1:-8091}"
API_ADDR="127.0.0.1:${API_PORT}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN="$SCRIPT_DIR/../dist/g360-ventas-api-linux"
DATA_DIR="$HOME/g360data"
DB_PATH="$DATA_DIR/historial.db"
CONFIG_PATH="/mnt/c/Users/ccusi/AppData/Roaming/g360-db-ventas/data/config.json"
EXPORT_DIR="$DATA_DIR/export"
LOG_FILE="$DATA_DIR/api.log"

echo "=== Verificando entorno ==="
if [ ! -x "$BIN" ]; then
  echo "ERROR: binario no encontrado en $BIN" >&2
  echo "Compilar antes: pwsh deploy/build-linux.ps1" >&2
  exit 1
fi
if [ ! -f "$DB_PATH" ]; then
  echo "ERROR: base de datos no encontrada en $DB_PATH" >&2
  exit 1
fi
if [ ! -f "$CONFIG_PATH" ]; then
  echo "ADVERTENCIA: config no encontrada en $CONFIG_PATH (login rechazara todo)" >&2
fi
mkdir -p "$DATA_DIR" "$EXPORT_DIR"

# Idempotencia: si ya hay una API sana en este puerto, no se toca nada.
# Sin este guarda, cualquier llamada a asegurar_api() durante una falla
# transitoria hace pkill de una instancia SANA y reabre el snapshot de 2.6 GB
# (~6 s sin responder). Ese hueco dispara el siguiente health check, que
# vuelve a llamar asegurar_api(): bucle de reinicios que tumba la API sola.
if curl -sf --max-time 3 "http://127.0.0.1:${API_PORT}/api/health" 2>/dev/null \
    | grep -q '"status":"ok"'; then
  echo "La API ya responde en 127.0.0.1:${API_PORT} — no se reinicia."
  exit 0
fi

echo "Deteniendo instancia anterior (si existe)..."
pkill -f "g360-ventas-api-linux" 2>/dev/null || true
# El cierre es graceful y tarda (reopen del snapshot); esperar a que el puerto
# se libere de verdad en vez de asumir 1 s.
for _ in $(seq 1 20); do
  ss -tln 2>/dev/null | grep -q ":${API_PORT} " || break
  sleep 0.5
done

if ss -tln 2>/dev/null | grep -q ":${API_PORT} "; then
  echo "ERROR: el puerto ${API_PORT} sigue ocupado" >&2
  ss -tln | grep ":${API_PORT}" || true
  exit 1
fi
echo "Puerto ${API_PORT} libre."

export G360_API_ADDR="$API_ADDR"
export G360_DB_PATH="$DB_PATH"
export G360_PRODUCER_CONFIG="$CONFIG_PATH"
export G360_EXPORT_DIR="$EXPORT_DIR"

# Secreto persistente entre reinicios (mismo motivo que run_api_wsl.sh).
SECRET_FILE="$DATA_DIR/.api_secret"
if [ ! -f "$SECRET_FILE" ]; then
  mkdir -p "$(dirname "$SECRET_FILE")"
  head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' > "$SECRET_FILE"
  chmod 600 "$SECRET_FILE"
  echo "=== $(date -Is) secreto API generado en $SECRET_FILE ===" >> "$LOG_FILE"
fi
export G360_API_SECRET="${G360_API_SECRET:-$(cat "$SECRET_FILE")}"
export G360_TOKEN_TTL="${G360_TOKEN_TTL:-24h}"

{
  echo "=== Inicio $(date -Is) ==="
  echo "Binario: $BIN"
  printenv G360_API_ADDR | sed 's/^/G360_API_ADDR=/' || echo "G360_API_ADDR=(vacio)"
  printenv G360_DB_PATH | sed 's/^/G360_DB_PATH=/' || echo "G360_DB_PATH=(vacio)"
  printenv G360_PRODUCER_CONFIG | sed 's/^/G360_PRODUCER_CONFIG=/' || echo "G360_PRODUCER_CONFIG=(vacio)"
  printenv G360_EXPORT_DIR | sed 's/^/G360_EXPORT_DIR=/' || echo "G360_EXPORT_DIR=(vacio)"
  echo "G360_API_SECRET=(configurado, ${#G360_API_SECRET} chars, $SECRET_FILE)"
  echo "G360_TOKEN_TTL=${G360_TOKEN_TTL:-24h}"
} >> "$LOG_FILE"

echo "Iniciando API en $API_ADDR con $DB_PATH (log: $LOG_FILE)"

# Desacoplado por defecto. Con `exec` el binario queda como hijo de la sesion
# de wsl.exe: cuando ese proceso termina (o el lanzador se cierra) el API cae y
# el forwarder se queda escuchando sin upstream -> "TCP OK + HTTP reset".
# G360_API_FOREGROUND=1 conserva el modo primer plano para depurar a mano.
if [ "${G360_API_FOREGROUND:-0}" = "1" ]; then
  exec "$BIN" >> "$LOG_FILE" 2>&1
fi

setsid nohup "$BIN" >> "$LOG_FILE" 2>&1 < /dev/null &
disown || true
echo "API lanzada en segundo plano (log: $LOG_FILE)"
