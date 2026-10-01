package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"g360-ventas-api/internal/auth"
	"g360-ventas-api/internal/config"
	"g360-ventas-api/internal/db"
)

type Server struct {
	Cfg           *config.Config
	Store         *db.Store
	Auth          *auth.Manager
	checksumCache *cachedResult
	statsCache    *cachedResult
}

func New(cfg *config.Config, store *db.Store, am *auth.Manager) *Server {
	return &Server{
		Cfg:           cfg,
		Store:         store,
		Auth:          am,
		checksumCache: &cachedResult{ttl: 60 * time.Second},
		statsCache:    &cachedResult{ttl: 60 * time.Second},
	}
}

func (s *Server) Routes() http.Handler {
	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/status", s.handleStatus)
	protected.HandleFunc("GET /api/checksums", s.handleChecksums)
	protected.HandleFunc("GET /api/day-checksums", s.handleDayChecksums)
	protected.HandleFunc("GET /api/folios", s.handleFolios)
	protected.HandleFunc("GET /api/contrast", s.handleContrast)
	protected.HandleFunc("POST /api/ventas/by-folios", s.handleByFolios)
	protected.HandleFunc("GET /api/model", s.handleModel)
	protected.HandleFunc("GET /api/data/{obj}", s.handleData)
	protected.HandleFunc("GET /api/query/{obj}", s.handleData)
	protected.HandleFunc("GET /api/stats", s.handleStats)
	protected.HandleFunc("GET /api/export/list", s.handleExportList)
	protected.HandleFunc("GET /api/export/base-canonica", s.handleExportDownload)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.Handle("/api/", s.Auth.Require(protected))

	return cors(accessLog(mux))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"api":   "g360-ventas-api",
		"auth":  "POST /api/login {user,password} -> token",
		"rutas": []string{"/api/health", "/api/status", "/api/checksums", "/api/day-checksums", "/api/folios", "/api/contrast", "/api/ventas/by-folios", "/api/model", "/api/data/{obj}", "/api/query/{obj}", "/api/stats", "/api/export/list", "/api/export/base-canonica"},
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		User     string `json:"user"`
		Password string `json:"password"`
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "body JSON invalido")
			return
		}
	} else {
		body.User = r.FormValue("user")
		body.Password = r.FormValue("password")
	}

	token, exp, err := s.Auth.Login(body.User, body.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "credenciales invalidas")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":        token,
		"expires_at":   exp.UTC().Format(time.RFC3339),
		"ttl_segundos": int(s.Cfg.TokenTTL.Seconds()),
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{
		"status": "ok",
		"db":     s.Cfg.DBPath,
		"tiempo": time.Now().UTC().Format(time.RFC3339),
	}
	ctx := r.Context()
	if err := s.Store.Ping(ctx); err != nil {
		out["status"] = "db_no_disponible"
		out["db_error"] = err.Error()
		writeJSON(w, http.StatusServiceUnavailable, out)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
