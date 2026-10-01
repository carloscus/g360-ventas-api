package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var tableAllowlist = map[string]bool{
	"ventas":         true,
	"dim_cliente":    true,
	"dim_articulo":   true,
	"dim_documento":  true,
	"dim_linea":      true,
	"dim_vendedor":   true,
	"dim_ruc":        true,
	"fact_venta_mes": true,
	"stats_por_mes":  true,
	"mes_checksums":  true,
}

var viewPrefixes = []string{"vw_", "mv_"}

type Column struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	NotNull bool   `json:"notnull"`
	PK      bool   `json:"pk"`
}

type Object struct {
	Nombre   string   `json:"nombre"`
	Tipo     string   `json:"tipo"`
	Columnas []Column `json:"columnas"`
}

type Store struct {
	sql  *sql.DB
	path string

	mu         sync.RWMutex
	allowed    map[string]string
	ventasCols []string
}

func Open(path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(abs), RawQuery: "mode=ro"}
	dsn := u.String() + "&_pragma=query_only(1)"

	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	sqldb.SetMaxOpenConns(4)
	sqldb.SetMaxIdleConns(4)
	sqldb.SetConnMaxIdleTime(time.Minute)

	s := &Store{sql: sqldb, path: abs, allowed: map[string]string{}}
	if err := s.Refresh(context.Background()); err != nil {
		sqldb.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.sql.Close() }

func (s *Store) Path() string { return s.path }

func (s *Store) Ping(ctx context.Context) error { return s.sql.PingContext(ctx) }

func (s *Store) DB() *sql.DB { return s.sql }

func (s *Store) Refresh(ctx context.Context) error {
	rows, err := s.sql.QueryContext(ctx,
		`SELECT name, type FROM sqlite_master WHERE type IN ('table','view') AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return err
	}
	defer rows.Close()

	allowed := map[string]string{}
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			return err
		}
		if tableAllowlist[name] || hasViewPrefix(name) {
			allowed[name] = typ
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	cols, err := s.tableInfo(ctx, "ventas")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(cols))
	for _, c := range cols {
		names = append(names, c.Name)
	}

	s.mu.Lock()
	s.allowed = allowed
	s.ventasCols = names
	s.mu.Unlock()
	return nil
}

func hasViewPrefix(name string) bool {
	for _, p := range viewPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func (s *Store) IsAllowed(name string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.allowed[name]
	return t, ok
}

func (s *Store) Allowed() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.allowed))
	for k, v := range s.allowed {
		out[k] = v
	}
	return out
}

func (s *Store) VentasColumns() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.ventasCols))
	copy(out, s.ventasCols)
	return out
}

func QuoteIdent(name string) (string, error) {
	if strings.ContainsAny(name, "\"';") || strings.Contains(name, "[") {
		return "", fmt.Errorf("identificador invalido: %q", name)
	}
	return `"` + name + `"`, nil
}

func (s *Store) tableInfo(ctx context.Context, name string) ([]Column, error) {
	quoted, err := QuoteIdent(name)
	if err != nil {
		return nil, err
	}
	rows, err := s.sql.QueryContext(ctx, "PRAGMA table_info("+quoted+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []Column
	for rows.Next() {
		var (
			cid, notnull, pk int
			colName, colType string
			dflt             sql.NullString
		)
		if err := rows.Scan(&cid, &colName, &colType, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, Column{Name: colName, Type: colType, NotNull: notnull == 1, PK: pk > 0})
	}
	return cols, rows.Err()
}

func (s *Store) objectColumns(ctx context.Context, name string) ([]Column, error) {
	if _, ok := s.IsAllowed(name); !ok {
		return nil, fmt.Errorf("objeto no permitido: %s", name)
	}
	return s.tableInfo(ctx, name)
}

func (s *Store) Columns(ctx context.Context, name string) ([]Column, error) {
	return s.objectColumns(ctx, name)
}

func (s *Store) Model(ctx context.Context) ([]Object, error) {
	if err := s.Refresh(ctx); err != nil {
		return nil, err
	}
	allowed := s.Allowed()
	names := make([]string, 0, len(allowed))
	for n := range allowed {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]Object, 0, len(names))
	for _, n := range names {
		cols, err := s.objectColumns(ctx, n)
		if err != nil {
			return nil, err
		}
		out = append(out, Object{Nombre: n, Tipo: allowed[n], Columnas: cols})
	}
	return out, nil
}

func (s *Store) QueryRows(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	rows, err := s.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return ScanAll(rows)
}

func ScanAll(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			m[c] = normalize(vals[i])
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func normalize(v any) any {
	switch t := v.(type) {
	case []byte:
		return string(t)
	case time.Time:
		return t.Format(time.RFC3339)
	default:
		return v
	}
}
