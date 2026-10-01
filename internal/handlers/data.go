package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"g360-ventas-api/internal/db"
	"g360-ventas-api/internal/query"
)

func (s *Server) handleModel(w http.ResponseWriter, r *http.Request) {
	objs, err := s.Store.Model(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"objetos":     objs,
		"total":       len(objs),
		"generado_en": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleData(w http.ResponseWriter, r *http.Request) {
	obj := r.PathValue("obj")
	typ, ok := s.Store.IsAllowed(obj)
	if !ok {
		writeError(w, http.StatusNotFound, "objeto no encontrado o no permitido: "+obj)
		return
	}

	cols, err := s.Store.Columns(r.Context(), obj)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	names := make([]string, 0, len(cols))
	for _, c := range cols {
		names = append(names, c.Name)
	}

	opts, err := query.Parse(r.URL.Query(), names, s.Cfg.MaxLimit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	q, args, err := opts.SQL(obj, db.QuoteIdent)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := s.Store.QueryRows(r.Context(), q, args...)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"objeto": obj,
		"tipo":   typ,
		"filas":  len(rows),
		"limit":  opts.Limit,
		"offset": opts.Offset,
		"datos":  rows,
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if v, ok := s.statsCache.get(); ok {
		writeJSON(w, http.StatusOK, v)
		return
	}

	var (
		filas        int64
		docs         int64
		clientes     int64
		desde, hasta sql.NullString
	)
	err := s.Store.DB().QueryRowContext(r.Context(),
		`SELECT COUNT(*), COUNT(DISTINCT folio_unico), COUNT(DISTINCT id_cliente),
		        MIN(fecha_orig), MAX(fecha_orig)
		 FROM ventas`).
		Scan(&filas, &docs, &clientes, &desde, &hasta)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "db no disponible: "+err.Error())
		return
	}

	meses, err := s.Store.QueryRows(r.Context(), checksumSQL)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	out := map[string]any{
		"fuente":      "live",
		"filas":       filas,
		"docs":        docs,
		"clientes":    clientes,
		"meses":       meses,
		"meses_total": len(meses),
		"generado_en": time.Now().UTC().Format(time.RFC3339),
	}
	if desde.Valid {
		out["fecha_desde"] = desde.String
	}
	if hasta.Valid {
		out["fecha_hasta"] = hasta.String
	}
	s.statsCache.set(out)
	writeJSON(w, http.StatusOK, out)
}
