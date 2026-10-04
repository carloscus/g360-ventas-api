package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"g360-ventas-api/internal/db"
)

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func validDate(v string) bool { return dateRe.MatchString(v) }

const checksumSQL = `SELECT mes_ref,
	COUNT(*) as filas,
	ROUND(SUM(soles),2) as soles,
	ROUND(SUM(COALESCE(dolares,0)),2) as dolares,
	ROUND(SUM(cantidad),2) as cantidad,
	printf('%08x-%08x-%08x-%08x',
		COUNT(*),
		CAST(ROUND(SUM(soles)*100) AS INTEGER) & 0xFFFFFFFF,
		CAST(ROUND(SUM(COALESCE(dolares,0)*100),0) AS INTEGER) & 0xFFFFFFFF,
		CAST(ROUND(SUM(cantidad)*100) AS INTEGER) & 0xFFFFFFFF) as checksum
	FROM ventas GROUP BY mes_ref ORDER BY mes_ref`

type cacheEntry struct {
	at   time.Time
	data any
}

type cachedResult struct {
	mu    sync.Mutex
	ttl   time.Duration
	entry *cacheEntry
}

func (c *cachedResult) get() (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entry != nil && time.Since(c.entry.at) < c.ttl {
		return c.entry.data, true
	}
	return nil, false
}

func (c *cachedResult) set(v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entry = &cacheEntry{at: time.Now(), data: v}
}

func (c *cachedResult) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entry = nil
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	// Cache 30s: el snapshot es statico entre refreshes, no vale la pena
	// re-calcular COUNT(*)+MIN+MAX en cada request (tarda 3-9s en DB de 2.6 GB).
	if v, ok := s.statusCache.get(); ok {
		writeJSON(w, http.StatusOK, v)
		return
	}

	var (
		filas        int64
		desde, hasta sql.NullString
		capturado    sql.NullString
		meses        int64
	)
	err := s.Store.DB().QueryRowContext(r.Context(),
		`SELECT COUNT(*), MIN(fecha_orig), MAX(fecha_orig), MAX(capturado_en), COUNT(DISTINCT mes_ref)
		 FROM ventas`).
		Scan(&filas, &desde, &hasta, &capturado, &meses)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "db no disponible: "+err.Error())
		return
	}

	out := map[string]any{
		"filas":       filas,
		"meses":       meses,
		"generado_en": time.Now().UTC().Format(time.RFC3339),
	}
	if desde.Valid {
		out["fecha_desde"] = desde.String
	}
	if hasta.Valid {
		out["fecha_hasta"] = hasta.String
	}
	if capturado.Valid {
		out["capturado_en_ultimo"] = capturado.String
	}
	s.statusCache.set(out)
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleChecksums(w http.ResponseWriter, r *http.Request) {
	if v, ok := s.checksumCache.get(); ok {
		writeJSON(w, http.StatusOK, v)
		return
	}
	rows, err := s.Store.QueryRows(r.Context(), checksumSQL)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	out := map[string]any{
		"fuente":       "live",
		"formula":      "printf('%08x-%08x-%08x-%08x', filas, soles*100, dolares*100, cantidad*100)",
		"calculado_en": time.Now().UTC().Format(time.RFC3339),
		"meses":        rows,
	}
	s.checksumCache.set(out)
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDayChecksums(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	desde, hasta := q.Get("desde"), q.Get("hasta")
	if !validDate(desde) || !validDate(hasta) {
		writeError(w, http.StatusBadRequest, "desde y hasta requeridos (YYYY-MM-DD)")
		return
	}
	d1, _ := time.Parse("2006-01-02", desde)
	d2, _ := time.Parse("2006-01-02", hasta)
	if d2.Before(d1) {
		writeError(w, http.StatusBadRequest, "hasta anterior a desde")
		return
	}
	if d2.Sub(d1).Hours()/24 > 1500 {
		writeError(w, http.StatusBadRequest, "rango maximo de 1500 dias")
		return
	}

	rows, err := s.Store.QueryRows(r.Context(),
		`SELECT substr(fecha_orig,1,10) as dia,
			COUNT(*) as filas,
			ROUND(SUM(soles),2) as soles,
			ROUND(SUM(cantidad),2) as cantidad,
			printf('%08x-%08x-%08x-%08x',
				COUNT(*),
				CAST(ROUND(SUM(soles)*100) AS INTEGER) & 0xFFFFFFFF,
				CAST(ROUND(SUM(COALESCE(dolares,0)*100),0) AS INTEGER) & 0xFFFFFFFF,
				CAST(ROUND(SUM(cantidad)*100) AS INTEGER) & 0xFFFFFFFF) as checksum
		 FROM ventas WHERE fecha_orig >= ? AND fecha_orig <= ?
		 GROUP BY substr(fecha_orig,1,10) ORDER BY dia`,
		desde, hasta)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"desde": desde, "hasta": hasta, "dias": rows,
		"calculado_en": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleFolios(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	desde, hasta := q.Get("desde"), q.Get("hasta")
	if !validDate(desde) || !validDate(hasta) {
		writeError(w, http.StatusBadRequest, "desde y hasta requeridos (YYYY-MM-DD)")
		return
	}

	rows, err := s.Store.QueryRows(r.Context(),
		`SELECT DISTINCT tpo_doc || serie_doc || nro_doc AS folio
		 FROM ventas
		 WHERE fecha_orig >= ? AND fecha_orig <= ?
		   AND tpo_doc || serie_doc || nro_doc IS NOT NULL
		 ORDER BY folio`,
		desde, hasta)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	folios := make([]string, 0, len(rows))
	for _, row := range rows {
		if f, ok := row["folio"].(string); ok {
			folios = append(folios, f)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"desde": desde, "hasta": hasta, "total": len(folios), "folios": folios,
	})
}

var contrastColumns = []string{
	"id_ubigeo", "estado_linea", "canal_distribucion", "id_guia",
	"nom_condicion_pago", "division", "fec_cargo",
	"ord_compra", "nom_vendedor",
}

func (s *Server) handleContrast(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	desde, hasta := q.Get("desde"), q.Get("hasta")
	if !validDate(desde) || !validDate(hasta) {
		writeError(w, http.StatusBadRequest, "desde y hasta requeridos (YYYY-MM-DD)")
		return
	}
	ctx := r.Context()
	rangeWhere := " fecha_orig >= ? AND fecha_orig <= ?"

	var filas int64
	var soles float64
	err := s.Store.DB().QueryRowContext(ctx,
		"SELECT COUNT(*), ROUND(COALESCE(SUM(soles),0),2) FROM ventas WHERE"+rangeWhere,
		desde, hasta).Scan(&filas, &soles)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	ventasCols := s.Store.VentasColumns()
	colSet := map[string]bool{}
	for _, c := range ventasCols {
		colSet[c] = true
	}
	columnas := map[string]*int64{}
	for _, c := range contrastColumns {
		if !colSet[c] {
			continue
		}
		var n int64
		err := s.Store.DB().QueryRowContext(ctx,
			"SELECT COUNT(*) FROM ventas WHERE"+rangeWhere+" AND TRIM(IFNULL("+c+",'')) <> ''",
			desde, hasta).Scan(&n)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		columnas[c] = &n
	}

	var folios int64
	err = s.Store.DB().QueryRowContext(ctx,
		"SELECT COUNT(DISTINCT tpo_doc || serie_doc || nro_doc) FROM ventas WHERE"+rangeWhere,
		desde, hasta).Scan(&folios)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	colsOut := map[string]int64{}
	for k, v := range columnas {
		colsOut[k] = *v
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"desde": desde, "hasta": hasta,
		"filas": filas, "soles": soles,
		"columnas": colsOut, "folios": folios,
		"generado_en": time.Now().UTC().Format(time.RFC3339),
	})
}

type byFoliosRequest struct {
	Folios []string `json:"folios"`
	Desde  string   `json:"desde"`
	Hasta  string   `json:"hasta"`
}

func (s *Server) handleByFolios(w http.ResponseWriter, r *http.Request) {
	var req byFoliosRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body JSON invalido")
		return
	}
	if len(req.Folios) == 0 {
		writeError(w, http.StatusBadRequest, "folios vacio")
		return
	}
	if len(req.Folios) > s.Cfg.MaxFolios {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("maximo %d folios por request (recibido %d)", s.Cfg.MaxFolios, len(req.Folios)))
		return
	}
	for _, f := range req.Folios {
		if f == "" || len(f) > 64 || strings.ContainsRune(f, 0) {
			writeError(w, http.StatusBadRequest, "folio invalido")
			return
		}
	}
	if (req.Desde == "") != (req.Hasta == "") {
		writeError(w, http.StatusBadRequest, "desde y hasta deben ir juntos")
		return
	}
	if req.Desde != "" && (!validDate(req.Desde) || !validDate(req.Hasta)) {
		writeError(w, http.StatusBadRequest, "formato de fecha invalido (YYYY-MM-DD)")
		return
	}

	cols := s.Store.VentasColumns()
	sel := make([]string, 0, len(cols))
	for _, c := range cols {
		if c == "id" {
			continue
		}
		sel = append(sel, `"`+c+`"`)
	}

	tx, err := s.Store.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	defer tx.Rollback()

	var all []map[string]any
	chunk := 400
	for i := 0; i < len(req.Folios); i += chunk {
		end := i + chunk
		if end > len(req.Folios) {
			end = len(req.Folios)
		}
		part := req.Folios[i:end]

		var b strings.Builder
		args := make([]any, 0, len(part)+2)
		b.WriteString("SELECT " + strings.Join(sel, ", ") + " FROM ventas WHERE (tpo_doc || serie_doc || nro_doc) IN (")
		for j, f := range part {
			if j > 0 {
				b.WriteString(",")
			}
			b.WriteString("?")
			args = append(args, f)
		}
		b.WriteString(")")
		if req.Desde != "" {
			b.WriteString(" AND fecha_orig >= ? AND fecha_orig <= ?")
			args = append(args, req.Desde, req.Hasta)
		}

		rows, err := tx.QueryContext(r.Context(), b.String(), args...)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		page, err := db.ScanAll(rows)
		rows.Close()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		all = append(all, page...)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"solicitados": len(req.Folios),
		"devueltos":   len(all),
		"filas":       all,
	})
}
