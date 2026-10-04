"""Snapshot consistente de historial.db con la SQLite backup API (fuente solo lectura)."""

import argparse
import os
import sqlite3
import time


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument(
        "--source",
        # En WSL no existe APPDATA; el API siempre pasa --source explicito.
        default=os.path.join(
            os.environ.get("APPDATA", ""), "g360-db-ventas", "data", "historial.db"
        ),
    )
    ap.add_argument(
        "--dest",
        default=os.path.join(os.environ.get("TEMP", "/tmp"), "g360_snapshot", "historial.db"),
    )
    args = ap.parse_args()

    os.makedirs(os.path.dirname(args.dest), exist_ok=True)
    if os.path.exists(args.dest):
        os.remove(args.dest)

    t0 = time.time()
    src = sqlite3.connect(f"file:{args.source}?mode=ro", uri=True, timeout=60)
    dst = sqlite3.connect(args.dest)
    with dst:
        src.backup(dst, pages=10000)
    dst.close()
    src.close()
    size = os.path.getsize(args.dest)
    print(f"snapshot ok: {args.dest} ({size / 1e9:.2f} GB) en {time.time() - t0:.1f}s")


if __name__ == "__main__":
    main()
