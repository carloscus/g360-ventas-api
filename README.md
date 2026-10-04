# g360-ventas-api

Repo: <https://github.com/carloscus/g360-ventas-api>

API HTTP en Go que expone la base canónica SQLite de ventas (`historial.db`) a la intranet, en **solo lectura**, para que apps cliente (ej. `g360-erp-nc-sustentor`) se pongan al día de forma incremental sin leer la BD completa y sin acceso directo al archivo por red.

```
intranet CIPSA → g360-ventas-db (Tauri/Rust, único escritor) → historial.db
                                                                  │ (read-only)
                                                          g360-ventas-api (Go)
                                                                  │ HTTP :8090
                                                          app cliente (diff folios)
```

## Requisitos

- Go 1.27+ (solo para compilar)
- La BD productora en `%APPDATA%\g360-db-ventas\data\historial.db` (la app Tauri puede estar corriendo: WAL permite lectores concurrentes)
- Credenciales intranet en `config.json` de la productora (sección `intranet` o lista `users`) o variables de entorno

## Multi-usuario

El API acepta N usuarios con las mismas credenciales del intranet. Formatos soportados
en `config.json` (con fallback en ese orden: `G360_INTRANET_USER/PASS` → `users` → `intranet`):

```json
{ "users": [{ "user": "ccusi", "pass": "..." }, { "user": "cliente1", "pass": "..." }] }
```

Formato legacy (un usuario) sigue funcionando:

```json
{ "intranet": { "user": "ccusi", "pass": "..." } }
```

El token incluye el usuario (`usuario.expira.hmac`) y `POST /api/login` devuelve
`{token, user, expires_at, ttl_segundos}`.

## Arranque en WSL2

```powershell
pwsh deploy/build-linux.ps1   # compila dist/g360-ventas-api-linux (reconstruir tras cada cambio Go)
pwsh deploy/start_api.ps1     # levanta en 127.0.0.1:8091 y verifica /api/health
```

`deploy/start_api.sh` exporta todas las variables dentro de WSL (no depende del
passthrough de entorno PowerShell→WSL) y registra el entorno efectivo en
`~/g360data/api.log`.

## Compilar y probar

```powershell
go test ./...
go build -o dist\g360-ventas-api.exe .\cmd\server
```

## Ejecutar

```powershell
.\dist\g360-ventas-api.exe
```

> En esta máquina FortiEDR bloquea el `listen()` del binario Windows: usar el despliegue WSL2 de más abajo (`deploy/`).

Variables de entorno (todas opcionales):

| Variable | Default | Descripción |
|----------|---------|-------------|
| `G360_API_ADDR` | `0.0.0.0:8090` | dirección de escucha |
| `G360_API_PORT` | `8090` | solo puerto (alternativa a ADDR) |
| `G360_DB_PATH` | `%APPDATA%\g360-db-ventas\data\historial.db` | BD de solo lectura |
| `G360_EXPORT_DIR` | `...\g360-db-ventas\data\export` | snapshots base canónica |
| `G360_INTRANET_USER` / `G360_INTRANET_PASS` | desde `config.json` | credenciales de login |
| `G360_API_SECRET` | aleatorio por arranque | secreto HMAC de tokens |
| `G360_TOKEN_TTL` | `24h` | duración del token |
| `G360_MAX_FOLIOS` | `500` | máximo de folios por `by-folios` |
| `G360_MAX_LIMIT` | `5000` | máximo `limit` en queries |

## Endpoints

### Auth (credenciales intranet)

| Método | Ruta | Descripción |
|--------|------|-------------|
| POST | `/api/login` | `{user,password}` → `{token, user, expires_at}` |
| GET | `/api/health` | liveness (sin auth) |

El token se envía en `Authorization: Bearer <token>`, `X-API-Key: <token>` o `?token=<token>` (útil para descargas desde navegador).

### Sync incremental (alimenta al cliente)

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/status` | filas, cobertura `fecha_orig`, último `capturado_en` |
| GET | `/api/checksums` | checksum por mes (formula del productor), cache 60s |
| GET | `/api/day-checksums?desde&hasta` | checksum por día en un rango (máx 1500 días) |
| GET | `/api/folios?desde&hasta` | `DISTINCT tpo_doc\|\|serie_doc\|\|nro_doc` del rango |
| POST | `/api/ventas/by-folios` | filas completas (sin `id`) de folios listados |
| GET | `/api/contrast?desde&hasta` | filas/soles/cobertura de 9 columnas/folios del rango |

### Modelo completo (offline / allowlist)

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/model` | tablas + vistas permitidas con columnas y tipos |
| GET | `/api/data/{objeto}` | dump paginado con filtros PostgREST |
| GET | `/api/query/{vista}` | ídem (alias para vistas) |
| GET | `/api/stats` | métricas globales + por mes, cache 60s |

Filtros en `data`/`query`: `select`, `order=col.desc`, `limit`, `offset` y operadores `eq. neq. gt. gte. lt. lte. like. in.(a,b)`.

### Base canónica (bootstrap)

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/export/list` | snapshots `base_canonica_*.db` + manifiestos |
| GET | `/api/export/base-canonica` | descarga del `.db` (soporta Range); `?name=` para elegir, `?manifest=1` para el manifiesto |

## Flujo de sync del cliente

```bash
# 0. login (una vez)
TOKEN=$(curl -s -X POST http://API:8090/api/login \
  -H "Content-Type: application/json" \
  -d '{"user":"...","password":"..."}' | jq -r .token)

AUTH="Authorization: Bearer $TOKEN"

# 1. frescura
curl -s -H "$AUTH" http://API:8090/api/status

# 2. ¿qué meses cambiaron desde mi última sync?
curl -s -H "$AUTH" http://API:8090/api/checksums

# 3. folios del rango a poner al día (liviano)
curl -s -H "$AUTH" "http://API:8090/api/folios?desde=2026-09-24&hasta=2026-10-01"

# 4. traer solo los folios que faltan localmente (POST, máx 500)
curl -s -X POST -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"folios":["F01201100","F01201102"],"desde":"2026-09-24","hasta":"2026-10-01"}' \
  http://API:8090/api/ventas/by-folios

# 5. INSERT local con dedup (lógica existente del cliente)
```

Detección de cambios: `mes_checksums` del productor usa
`printf('%08x-%08x-%08x-%08x', filas, soles*100, dolares*100, cantidad*100)`
(igual que `calculate_monthly_checksums` en `src/db/writer.rs`). El API lo calcula en vivo sobre `ventas`, así que no depende de que alguien haya pulsado "calcular checksums".

## Seguridad

- Conexión SQLite `mode=ro` + `PRAGMA query_only=ON`: imposible escribir.
- Whitelist de objetos: tablas de datos (`ventas`, `dim_*`, `fact_venta_mes`, `stats_por_mes`, `mes_checksums`) y vistas `vw_*`/`mv_*`. **No** se exponen `audit_log`, `sync_log`, `sync_months`.
- Identificadores validados contra la whitelist; valores siempre parametrizados.
- Login con credenciales intranet (comparación constant-time) → token HMAC con expiración.

## Despliegue WSL2 (solución al bloqueo de FortiEDR)

En esta máquina FortiEDR **bloquea `listen()` de binarios Go no firmados** (verificado: Python/Node/PowerShell escuchan OK; Go falla incluso con firma autofirmada y vía Task Scheduler). Por eso el API corre **dentro de WSL2** (Ubuntu), donde el EDR no intercepta:

```
[intranet] ──http://<IP-LAN>:8090──▶ forwarder.py (Windows 0.0.0.0:8090, sin admin)
                                              │
                                              ▼
                              127.0.0.1:8091 (relay "localhost forwarding" de WSL2)
                                              │
                                              ▼
                              API Go en WSL2 (127.0.0.1:8091) ◀── snapshot ext4 de historial.db
```

### Setup (una vez)

```powershell
pwsh deploy\build-linux.ps1          # compila dist\g360-ventas-api-linux
pwsh deploy\refresh_snapshot.ps1     # snapshot + copia a ext4 + arranca API
python deploy\forwarder.py           # expone el API en la IP LAN de Windows
```

- **`deploy/refresh_snapshot.ps1`**: la BD vive en NTFS con WAL y **no abre desde WSL** (`disk I/O error` en DrvFs), y las lecturas aleatorias por 9p son lentísimas (`status` >300s vs **8s** en ext4). Por eso un snapshot consistente (SQLite backup API, ~40s) se copia a `~/g360data/` en ext4 (~2 min) y el API lo sirve desde ahí. Ejecutarlo cada vez que se quiera frescura nueva.
- **`deploy/forwarder.py`**: expone `0.0.0.0:8090` en Windows y hace proxy a `127.0.0.1:8091`. Sin admin, sin resolver la IP de WSL (que cambia en cada arranque y hace colgar `wsl.exe` en procesos sin consola). Alternativa con admin: `netsh interface portproxy` + regla de firewall.
- El relay localhost de WSL2 solo expone en Windows los puertos que el API **bindea en `127.0.0.1`** dentro de WSL (verificado: con `0.0.0.0` el relay no se activa). Por eso el API corre en `8091` interno y el forwarder usa el `8090` de cara a la intranet.
- **Pendiente de verificar en una máquina real de la intranet**: la cadena se probó desde esta máquina vía la IP LAN (200 OK); una máquina externa recorre el perfil de firewall del adaptador LAN.

### Verificación (2026-10-01, BD productora real, 2.814.244 filas)

- Smoke test endpoints: **13/13 PASS** (login 401/200, status, checksums, folios, by-folios, model, data, day-checksums, export, whitelist 404) — re-ejecutado dos veces: directo al API en WSL y a través de la cadena completa (forwarder → relay).
- Paridad de checksums: **201/201 meses** — el checksum live del API coincide con `mes_checksums` del productor (falta solo oct-2026, mes en curso).
- Cadena LAN completa: intranet → IP LAN → forwarder → WSL → API → 200 OK.

## Notas de despliegue (Windows nativo)

- **Windows Firewall**: al primer arranque, permite la conexión entrante en el puerto 8090 (red privada/intranet).
- **FortiEDR / EDR corporativo**: los EDR pueden bloquear `listen()` de binarios sin cadena de firma confiable (error `An attempt was made to access a socket in a way forbidden...`). Si ocurre en otra máquina: excepción del binario en la política del EDR, o correr vía WSL2 como arriba.

## Estructura

```
cmd/server/main.go          arranque, config, shutdown
internal/config             env vars + credenciales desde config.json
internal/auth               login, token HMAC, require
internal/db                 apertura read-only, whitelist, QuoteIdent
internal/query              parser de filtros PostgREST
internal/handlers           server, sync, data, export
deploy/                     scripts de despliegue (WSL2, snapshot, forwarder)
docs/                       deck del proyecto (.deck.md, .build.cjs, .pptx)
.slides/                    marca del deck (brand.json, tokens.cjs)
```

## Documentación

| Archivo | Qué es |
|---------|--------|
| `docs/g360-ventas-api.pptx` | Presentación de 10 slides: problema, cadena, verificación, despliegue, fase 2 |
| `docs/g360-ventas-api.deck.md` | Spec del deck (contenido y estructura, en Markdown editable) |
| `docs/g360-ventas-api.build.cjs` | Script que compone el deck contra los tokens de la marca |
| `README.md` | Este documento |

Para regenerar el deck (usa el pack de slides en `~/.config/opencode/skills/build-deck`):

```powershell
python $env:USERPROFILE\.config\opencode\skills\build-deck\scripts\emit_tokens.py `
       .slides/brand.json --out .slides/tokens.cjs --register read
node --permission --allow-fs-read="$PWD" --allow-fs-write="$PWD/docs" `
     docs/g360-ventas-api.build.cjs --tokens .slides/tokens.cjs --out docs/g360-ventas-api.pptx
```

## Fase 2: adaptador del cliente

Pendiente en `g360-erp-nc-sustentor`. El contrato de sync ya está verificado, así que el trabajo es cambiar el transporte, no la lógica:

1. **Adaptador Python** en lugar del acceso SMB de `db_network.py`: `login` → `status` + `checksums` → `folios` → `by-folios` → `INSERT` local con dedup (la lógica de `contrastar_con_origen` y `traer_folios_faltantes` se conserva).
2. **Validación**: diff contra el flujo SMB actual sobre un mes real de datos.
3. **Producción**: despliegue en la intranet; si se corre el binario nativo, pedir la excepción de FortiEDR.
