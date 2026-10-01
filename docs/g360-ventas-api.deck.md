---
deck: g360-ventas-api
audience: IT/Seguridad y el equipo de desarrollo de CIPSA. Conocen la base canónica de ventas y deciden dos cosas: la excepción del binario en FortiEDR y el despliegue del API en la intranet.
register: read
structure: tapa y cierre sobre tinta (azul marino), evidencia sobre papel claro entre medio; el acento teal se gasta solo en el elemento que gana cada slide
motif: cadena de tres nodos (productor, API, cliente) unida por flechas finas, repetida en las slides de flujo
---

## Slide 1
layout: title
Canvas: ink
Title: g360-ventas-api
Subtitle: La base canónica de ventas, en solo lectura por HTTP a la intranet
Brief: La tapa presenta el proyecto con la cadena de tres nodos dibujada a la derecha; el título conduce la mirada. Referencia: portada institucional sobria, sin logos ni fotografías.
Notes: Contexto para el lector: el productor de la base canónica es la app Tauri de ventas y esta API la sirve por HTTP. El deck documenta qué expone, cómo se verificó y cómo se despliega.

## Slide 2
layout: title-content
Title: Hoy el cliente depende de copias manuales
Body: db_network.py lee la BD completa por SMB o por USB en zip: 2,8M de filas por pasada, sin control de frescura y con el archivo productor expuesto al acceso por red.
Notes: El problema actual del cliente ERP: cada sincronización lee todo y el equipo no sabe qué cambió desde la última pasada. La API resuelve las dos cosas con checksums y folios.

## Slide 3
layout: composed
Title: La cadena, de extremo a extremo
Brief: Diagrama de flujo de tres paneles con flechas; el panel del API lleva el acento porque es el sujeto del deck. Referencia: el motivo de la cadena.
Block: freeform
panel muted outline ink at cols 1-4 rows 2-6
text h1 ink at cols 1-4 rows 3-4 | Tauri (Rust)
text caption ink at cols 1-4 rows 4-5 | escritor único de historial.db
panel accent at cols 5-8 rows 2-6
text h1 paper at cols 5-8 rows 3-4 | API Go
text caption paper at cols 5-8 rows 4-5 | read-only :8090
panel muted outline ink at cols 9-12 rows 2-6
text h1 ink at cols 9-12 rows 3-4 | Cliente ERP
text caption ink at cols 9-12 rows 4-5 | diff de folios y checksums
arrow ink at cols 4-5 rows 3-4
arrow ink at cols 8-9 rows 3-4
Notes: Un solo escritor (Tauri), una API de solo lectura, y los clientes que consultan por HTTP. La BD nunca se comparte por red como archivo.

## Slide 4
layout: composed
Title: Sync incremental en cinco pasos
Brief: Proceso numerado de cinco pasos; el paso 3 y el 4 son los que ahorran la lectura completa. Referencia: flujo de db_network.py reemplazado por HTTP.
Block: process
Login | credenciales intranet, token HMAC de 24 h
Status y checksums | qué meses cambiaron desde la última sync
Folios | folios del rango, consulta liviana
By-folios | filas completas de los faltantes, máx 500 por request
Insert local | dedup por folio con la lógica existente del cliente
Notes: El cliente ya conoce este flujo: es el mismo contrato de contraster y traer folios faltantes que hoy hace por SMB, ahora por HTTP con token.

## Slide 5
layout: composed
Title: 13 endpoints, tres grupos
Brief: Tres tarjetas con el grupo de sync como tarjeta héroe; los números son los counts reales del código. Referencia: tabla de endpoints del README.
Block: card-grid
! Sync incremental | 6 endpoints: status, checksums, day-checksums, folios, by-folios, contrast
Auth | 2 endpoints: login, health
Modelo completo | 5 endpoints: model, data, query, stats, export
Notes: El grupo de sync alimenta al cliente en producción. El modelo completo permite trabajar offline con allowlist. La base canónica se descarga por export.

## Slide 6
layout: composed
Title: Verificado contra producción
Brief: Fila de stats con los números de la verificación del 2026-10-01; el 201/201 conduce porque es el contrato del sync. Referencia: smoke test y paridad de checksums.
Block: stat-row
2,8M | filas servidas
13/13 | tests de humo
201/201 | checksums vs productor
8s | status en frío
Notes: Verificación del 1 de octubre de 2026 contra la BD real: smoke test de 13 endpoints, paridad del checksum live contra la tabla mes_checksums del productor, y status en 8 segundos sobre el snapshot en ext4.

## Slide 7
layout: composed
Title: Despliegue en esta máquina
Brief: Comparación que resuelve: WSL2 funciona, el nativo está bloqueado; el ganador lleva el acento. Referencia: hallazgo de FortiEDR del diagnóstico.
Block: comparison
! WSL2 + forwarder | funciona: API Go en Ubuntu, snapshot en ext4 y forwarder Python en el 8090, sin admin
Windows nativo | bloqueado: FortiEDR impide el listen() de binarios Go sin cadena de firma confiable
Notes: FortiEDR bloquea el listen() de binarios Go no firmados en Windows. Por eso el API corre en WSL2 y un forwarder Python expone el puerto a la intranet. La alternativa con admin es netsh portproxy más regla de firewall.

## Slide 8
layout: title-content
Title: Seguridad por diseño
Body: La conexión SQLite abre en mode=ro con query_only: escribir es imposible. La whitelist expone 24 objetos y deja fuera audit_log, sync_log y sync_months. El login compara credenciales en constant-time y emite un token HMAC con expiración.
Notes: Tres capas: la conexión no puede escribir, la whitelist limita qué se ve, y el token limita quién entra. Los valores van siempre parametrizados y los identificadores se validan contra la whitelist.

## Slide 9
layout: composed
Title: Fase 2: el cliente deja el SMB
Brief: Proceso de tres pasos con la validación como paso que marca el giro; es la continuación natural del contrato ya verificado. Referencia: db_network.py del cliente.
Block: process
Adaptador Python | reemplaza db_network.py: login, checksums, folios, by-folios
Validación | diff contra SMB en un mes real de datos
Producción | despliegue en la intranet y excepción FortiEDR si se corre nativo
Notes: Fase 1 completa: API verificada y desplegada. Fase 2 reemplaza el transporte del cliente; el contrato de sync ya está probado, así que el adaptador solo cambia el transporte.

## Slide 10
layout: statement
Canvas: ink
Statement: Una sola base canónica, servida por HTTP a toda la intranet.
Brief: Cierre sobre tinta con la cadena de tres nodos como marca fantasma; la frase es el cierre factual del deck. Referencia: motivo de la cadena.
Notes: Cierre: el archivo productor deja de circular como copia; los clientes se ponen al día por HTTP contra una sola fuente. El siguiente paso es el adaptador del cliente (fase 2).
