#!/bin/bash
# Lanza el API dentro de WSL2 (solucion al bloqueo de listen() de FortiEDR en Windows).
# Uso: bash deploy/run_api_wsl.sh   (desde Windows: wsl -d Ubuntu -- bash /mnt/c/.../deploy/run_api_wsl.sh)
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN="$SCRIPT_DIR/../dist/g360-ventas-api-linux"

if [ ! -x "$BIN" ]; then
  echo "No existe $BIN — compilar antes: pwsh deploy/build-linux.ps1" >&2
  exit 1
fi

# El API escucha en 127.0.0.1:8091 para que el "localhost forwarding" de WSL2
# exponga el puerto en Windows (127.0.0.1:8091). El forwarder.py en Windows
# expone ese puerto a la intranet en 0.0.0.0:8090.
# NOTA: con G360_API_ADDR=0.0.0.0 el relay localhost NO se activa (verificado).
export G360_DB_PATH="${G360_DB_PATH:-$HOME/g360data/historial.db}"
export G360_PRODUCER_CONFIG="${G360_PRODUCER_CONFIG:-/mnt/c/Users/ccusi/AppData/Roaming/g360-db-ventas/data/config.json}"
export G360_EXPORT_DIR="${G360_EXPORT_DIR:-$HOME/g360data/export}"
export G360_API_ADDR="${G360_API_ADDR:-127.0.0.1:8091}"

mkdir -p "$HOME/g360data/export"
echo "$(date -Is) iniciando API en $G360_API_ADDR con $G360_DB_PATH"
exec "$BIN" >> "$HOME/g360data/api.log" 2>&1
