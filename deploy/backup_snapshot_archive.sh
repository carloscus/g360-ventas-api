#!/bin/bash
# Archiva el historial.db vigente en ~/g360data/backup/ con timestamp.
# Mantiene las ultimas N copias (default 3); pasa si no existe DB vigente.
# Uso: bash backup_snapshot_archive.sh [keep_n]
# Log a stdout (el caller lo redirige al log del refresh).

set -e
KEEP=${1:-3}
BACKUP_DIR="$HOME/g360data/backup"
SRC="$HOME/g360data/historial.db"

if [ ! -f "$SRC" ]; then
    echo "no hay historial.db vigente; skip archive"
    exit 0
fi

mkdir -p "$BACKUP_DIR"
TS=$(date +%Y%m%d-%H%M%S)
DST="$BACKUP_DIR/historial_${TS}.db"
cp "$SRC" "$DST"
echo "archivado: $DST ($(du -sh "$DST" | cut -f1))"

# Limpia copias antiguas, deja solo las ultimas KEEP.
ls -1t "$BACKUP_DIR"/historial_*.db 2>/dev/null | tail -n +"$(($KEEP + 1))" | xargs -r rm -f
echo "keep=$KEEP - restos limpiados"
