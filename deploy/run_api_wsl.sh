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

# Secreto persistente entre reinicios. Sin esto el API usa randomSecret()
# (config.go) y cada arranque invalida todos los tokens ya emitidos.
SECRET_FILE="${G360_API_SECRET_FILE:-$HOME/g360data/.api_secret}"
if [ ! -f "$SECRET_FILE" ]; then
  mkdir -p "$(dirname "$SECRET_FILE")"
  head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' > "$SECRET_FILE"
  chmod 600 "$SECRET_FILE"
  echo "secreto API generado en $SECRET_FILE"
fi
export G360_API_SECRET="${G360_API_SECRET:-$(cat "$SECRET_FILE")}"
export G360_TOKEN_TTL="${G360_TOKEN_TTL:-24h}"

# Refresh on-demand (POST /api/admin/refresh) corre dentro de WSL: necesita
# python3, la DB fuente NTFS, el stage y la ruta del script de backup.
export G360_PYTHON="${G360_PYTHON:-python3}"
export G360_NTFS_DB_PATH="${G360_NTFS_DB_PATH:-/mnt/c/Users/ccusi/AppData/Roaming/g360-db-ventas/data/historial.db}"
export G360_STAGE_DIR="${G360_STAGE_DIR:-/tmp/g360_snapshot}"
export G360_BACKUP_SCRIPT="${G360_BACKUP_SCRIPT:-/mnt/c/Users/ccusi/Documents/Proyect_Coder/G360-ecosystem/projects/g360-ventas-api/deploy/backup_snapshot.py}"

mkdir -p "$HOME/g360data/export"
echo "$(date -Is) iniciando API en $G360_API_ADDR con $G360_DB_PATH"
exec "$BIN" >> "$HOME/g360data/api.log" 2>&1
