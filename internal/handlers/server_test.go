package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"g360-ventas-api/internal/auth"
	"g360-ventas-api/internal/config"
	"g360-ventas-api/internal/db"
)

const testSchema = `CREATE TABLE ventas (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	id_articulo TEXT NOT NULL, original_sku TEXT, nom_articulo TEXT,
	id_linea TEXT NOT NULL, nom_linea TEXT,
	id_grupo TEXT, nom_grupo TEXT,
	id_tipo TEXT, nom_tipo TEXT,
	id_familia TEXT, nom_familia TEXT,
	id_cliente TEXT NOT NULL, doc_cliente TEXT, nom_cliente TEXT,
	tpo_doc TEXT NOT NULL, serie_doc TEXT, nro_doc TEXT,
	referencia TEXT, moneda TEXT DEFAULT 'Soles',
	cantidad REAL NOT NULL, soles REAL NOT NULL, dolares REAL, precio_unitario REAL,
	cantidad_fae REAL,
	anho INTEGER NOT NULL, mes INTEGER NOT NULL,
	fecha_orig TEXT NOT NULL,
	fecha_ref TEXT, fecha_venc TEXT,
	cod_sucursal TEXT, nom_sucursal TEXT,
	departamento TEXT, provincia TEXT, distrito TEXT,
	id_vendedor TEXT, nom_vendedor TEXT,
	id_pedido TEXT, ord_compra TEXT,
	file_source TEXT, mes_ref TEXT NOT NULL,
	capturado_en TEXT DEFAULT (datetime('now')),
	tipo_operacion TEXT DEFAULT 'venta',
	factura_ref_serie TEXT, factura_ref_nro TEXT,
	folio_unico TEXT, id_ubigeo TEXT, canal_distribucion TEXT,
	nom_condicion_pago TEXT, estado_linea TEXT, division TEXT,
	fec_cargo TEXT, id_guia TEXT
);
CREATE VIEW vw_dim_articulo AS SELECT id_articulo, nom_articulo FROM ventas GROUP BY id_articulo;
`

func seed(t *testing.T, path string) {
	t.Helper()
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Exec(testSchema); err != nil {
		t.Fatal(err)
	}

	ins := `INSERT INTO ventas (id_articulo, nom_articulo, id_linea, id_cliente, tpo_doc, serie_doc, nro_doc,
		cantidad, soles, dolares, anho, mes, fecha_orig, mes_ref, folio_unico, nom_vendedor, ord_compra)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`
	rows := [][]any{
		{"02211", "VINIFAN", "01", "00068414", "F01", "201", "100", 2.0, 100.0, 0.0, 2026, 9, "2026-09-29", "2026-09", "F01/201/100", "V1", "OC-1"},
		{"02212", "OTRO", "01", "00068415", "F01", "201", "100", 1.0, 200.0, 0.0, 2026, 9, "2026-09-29", "2026-09", "F01/201/100", "V1", ""},
		{"02211", "VINIFAN", "01", "00068414", "F01", "201", "101", 1.0, 50.5, 0.0, 2026, 9, "2026-09-30", "2026-09", "F01/201/101", "V1", "OC-2"},
		{"02211", "VINIFAN", "01", "00068416", "F01", "201", "102", 1.0, 10.0, 1.5, 2026, 10, "2026-10-01", "2026-10", "F01/201/102", "V2", ""},
		{"02212", "OTRO", "02", "00068416", "F01", "201", "102", 2.0, 20.0, 3.0, 2026, 10, "2026-10-01", "2026-10", "F01/201/102", "V2", ""},
		{"02213", "TERCERO", "02", "00068417", "F01", "201", "103", 3.0, 30.0, 4.5, 2026, 10, "2026-10-01", "2026-10", "F01/201/103", "V2", "OC-3"},
	}
	for _, r := range rows {
		if _, err := conn.Exec(ins, r...); err != nil {
			t.Fatal(err)
		}
	}
}

type testEnv struct {
	handler http.Handler
	token   string
	dir     string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "historial.db")
	seed(t, dbPath)

	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := &config.Config{
		Addr:      "127.0.0.1:0",
		DBPath:    dbPath,
		ExportDir: filepath.Join(dir, "export"),
		User:      "user1",
		Pass:      "pass1",
		Secret:    "test-secret",
		TokenTTL:  time.Hour,
		MaxFolios: 500,
		MaxLimit:  5000,
	}
	am := auth.New(cfg.Secret, cfg.User, cfg.Pass, cfg.TokenTTL)
	srv := New(cfg, store, am)

	env := &testEnv{handler: srv.Routes(), dir: dir}
	env.token = env.login(t)
	return env
}

func (e *testEnv) do(t *testing.T, method, path string, body io.Reader, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+e.token)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func (e *testEnv) get(t *testing.T, path string) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	rec := e.do(t, http.MethodGet, path, nil, true)
	return rec, rec.Body.Bytes()
}

func (e *testEnv) login(t *testing.T) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"user": "user1", "password": "pass1"})
	rec := e.do(t, http.MethodPost, "/api/login", bytes.NewReader(body), false)
	if rec.Code != 200 {
		t.Fatalf("login status = %d", rec.Code)
	}
	var out struct {
		Token string `json:"token"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Token == "" {
		t.Fatal("token vacio")
	}
	return out.Token
}

func TestHealthNoAuth(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodGet, "/api/health", nil, false)
	if rec.Code != 200 {
		t.Fatalf("health = %d", rec.Code)
	}
}

func TestStatusRequiresAuth(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodGet, "/api/status", nil, false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status sin token = %d", rec.Code)
	}
}

func TestStatus(t *testing.T) {
	env := newTestEnv(t)
	rec, body := env.get(t, "/api/status")
	if rec.Code != 200 {
		t.Fatalf("status = %d: %s", rec.Code, body)
	}
	var out struct {
		Filas      int64  `json:"filas"`
		FechaDesde string `json:"fecha_desde"`
		FechaHasta string `json:"fecha_hasta"`
		Meses      int64  `json:"meses"`
	}
	json.Unmarshal(body, &out)
	if out.Filas != 6 || out.FechaDesde != "2026-09-29" || out.FechaHasta != "2026-10-01" || out.Meses != 2 {
		t.Errorf("status = %+v", out)
	}
}

func TestChecksums(t *testing.T) {
	env := newTestEnv(t)
	rec, body := env.get(t, "/api/checksums")
	if rec.Code != 200 {
		t.Fatalf("checksums = %d: %s", rec.Code, body)
	}
	var out struct {
		Meses []map[string]any `json:"meses"`
	}
	json.Unmarshal(body, &out)
	if len(out.Meses) != 2 {
		t.Fatalf("meses = %d", len(out.Meses))
	}
	m0 := out.Meses[0]
	if m0["mes_ref"] != "2026-09" {
		t.Errorf("mes_ref = %v", m0["mes_ref"])
	}
	if filas, _ := m0["filas"].(float64); filas != 3 {
		t.Errorf("filas 2026-09 = %v", m0["filas"])
	}
	if soles, _ := m0["soles"].(float64); soles != 350.5 {
		t.Errorf("soles 2026-09 = %v", m0["soles"])
	}
	checksum, _ := m0["checksum"].(string)
	if len(checksum) != 35 {
		t.Errorf("checksum = %q", checksum)
	}
}

func TestDayChecksums(t *testing.T) {
	env := newTestEnv(t)
	rec, body := env.get(t, "/api/day-checksums?desde=2026-09-29&hasta=2026-10-01")
	if rec.Code != 200 {
		t.Fatalf("day-checksums = %d: %s", rec.Code, body)
	}
	var out struct {
		Dias []map[string]any `json:"dias"`
	}
	json.Unmarshal(body, &out)
	if len(out.Dias) != 3 {
		t.Fatalf("dias = %d", len(out.Dias))
	}
	if out.Dias[0]["dia"] != "2026-09-29" || out.Dias[2]["dia"] != "2026-10-01" {
		t.Errorf("dias = %v", out.Dias)
	}

	rec, _ = env.get(t, "/api/day-checksums?desde=malo&hasta=2026-10-01")
	if rec.Code != 400 {
		t.Errorf("fecha invalida = %d", rec.Code)
	}
}

func TestFolios(t *testing.T) {
	env := newTestEnv(t)
	rec, body := env.get(t, "/api/folios?desde=2026-09-29&hasta=2026-09-30")
	if rec.Code != 200 {
		t.Fatalf("folios = %d: %s", rec.Code, body)
	}
	var out struct {
		Total  int      `json:"total"`
		Folios []string `json:"folios"`
	}
	json.Unmarshal(body, &out)
	if out.Total != 2 {
		t.Fatalf("total = %d", out.Total)
	}
	if out.Folios[0] != "F01201100" || out.Folios[1] != "F01201101" {
		t.Errorf("folios = %v", out.Folios)
	}
}

func TestByFolios(t *testing.T) {
	env := newTestEnv(t)
	payload, _ := json.Marshal(map[string]any{
		"folios": []string{"F01201100", "F01201102"},
		"desde":  "2026-09-29",
		"hasta":  "2026-10-01",
	})
	rec := env.do(t, http.MethodPost, "/api/ventas/by-folios", bytes.NewReader(payload), true)
	if rec.Code != 200 {
		t.Fatalf("by-folios = %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Devueltos int              `json:"devueltos"`
		Filas     []map[string]any `json:"filas"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Devueltos != 4 {
		t.Fatalf("devueltos = %d", out.Devueltos)
	}
	for _, row := range out.Filas {
		if _, hasID := row["id"]; hasID {
			t.Error("la fila no debe traer id")
		}
		if _, hasArts := row["id_articulo"]; !hasArts {
			t.Error("la fila debe traer id_articulo")
		}
	}
}

func TestByFoliosMaxExcedido(t *testing.T) {
	env := newTestEnv(t)
	folios := make([]string, 501)
	for i := range folios {
		folios[i] = fmt.Sprintf("F01201%04d", i)
	}
	payload, _ := json.Marshal(map[string]any{"folios": folios})
	rec := env.do(t, http.MethodPost, "/api/ventas/by-folios", bytes.NewReader(payload), true)
	if rec.Code != 400 {
		t.Errorf("max folios = %d", rec.Code)
	}
}

func TestContrast(t *testing.T) {
	env := newTestEnv(t)
	rec, body := env.get(t, "/api/contrast?desde=2026-09-29&hasta=2026-09-30")
	if rec.Code != 200 {
		t.Fatalf("contrast = %d: %s", rec.Code, body)
	}
	var out struct {
		Filas    int64            `json:"filas"`
		Soles    float64          `json:"soles"`
		Folios   int64            `json:"folios"`
		Columnas map[string]int64 `json:"columnas"`
	}
	json.Unmarshal(body, &out)
	if out.Filas != 3 || out.Soles != 350.5 || out.Folios != 2 {
		t.Errorf("contrast = %+v", out)
	}
	if _, ok := out.Columnas["ord_compra"]; !ok {
		t.Errorf("falta columna ord_compra: %v", out.Columnas)
	}
}

func TestDataQuery(t *testing.T) {
	env := newTestEnv(t)

	rec, body := env.get(t, "/api/data/ventas?select=id_articulo,soles&order=soles.asc&limit=2")
	if rec.Code != 200 {
		t.Fatalf("data = %d: %s", rec.Code, body)
	}
	var out struct {
		Filas int              `json:"filas"`
		Datos []map[string]any `json:"datos"`
	}
	json.Unmarshal(body, &out)
	if out.Filas != 2 || out.Datos[0]["soles"].(float64) != 10.0 {
		t.Errorf("data = %+v", out)
	}

	rec, _ = env.get(t, "/api/query/vw_dim_articulo?limit=10")
	if rec.Code != 200 {
		t.Errorf("query vista = %d", rec.Code)
	}

	rec, _ = env.get(t, "/api/data/no_existe")
	if rec.Code != 404 {
		t.Errorf("objeto inexistente = %d", rec.Code)
	}

	rec, _ = env.get(t, "/api/data/ventas?select=columna_rara")
	if rec.Code != 400 {
		t.Errorf("select invalido = %d", rec.Code)
	}
}

func TestModel(t *testing.T) {
	env := newTestEnv(t)
	rec, body := env.get(t, "/api/model")
	if rec.Code != 200 {
		t.Fatalf("model = %d: %s", rec.Code, body)
	}
	var out struct {
		Total   int `json:"total"`
		Objetos []struct {
			Nombre string `json:"nombre"`
			Tipo   string `json:"tipo"`
		} `json:"objetos"`
	}
	json.Unmarshal(body, &out)
	found := map[string]string{}
	for _, o := range out.Objetos {
		found[o.Nombre] = o.Tipo
	}
	if found["ventas"] != "table" {
		t.Errorf("ventas = %q", found["ventas"])
	}
	if found["vw_dim_articulo"] != "view" {
		t.Errorf("vw_dim_articulo = %q", found["vw_dim_articulo"])
	}
}

func TestExportListAndDownload(t *testing.T) {
	env := newTestEnv(t)

	rec, body := env.get(t, "/api/export/list")
	if rec.Code != 200 {
		t.Fatalf("export list = %d: %s", rec.Code, body)
	}
	var out struct {
		Total int `json:"total"`
	}
	json.Unmarshal(body, &out)
	if out.Total != 0 {
		t.Fatalf("total inicial = %d", out.Total)
	}

	exportDir := filepath.Join(env.dir, "export")
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("db-falsa-de-test")
	name := "base_canonica_20261001_120000.db"
	if err := os.WriteFile(filepath.Join(exportDir, name), content, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := []byte(`{"format":"g360-ventas-canonical","total_rows_raw":6}`)
	if err := os.WriteFile(filepath.Join(exportDir, name+".manifest.json"), manifest, 0o644); err != nil {
		t.Fatal(err)
	}

	rec, body = env.get(t, "/api/export/list")
	json.Unmarshal(body, &out)
	if out.Total != 1 {
		t.Fatalf("total = %d", out.Total)
	}

	rec, body = env.get(t, "/api/export/base-canonica")
	if rec.Code != 200 {
		t.Fatalf("download = %d", rec.Code)
	}
	if string(body) != string(content) {
		t.Errorf("contenido = %q", body)
	}

	rec, body = env.get(t, "/api/export/base-canonica?manifest=1")
	if rec.Code != 200 || !strings.Contains(string(body), "canonical") {
		t.Errorf("manifest = %d %s", rec.Code, body)
	}

	rec, _ = env.get(t, "/api/export/base-canonica?name=../../etc/passwd")
	if rec.Code != 404 {
		t.Errorf("traversal = %d", rec.Code)
	}
}

func TestStats(t *testing.T) {
	env := newTestEnv(t)
	rec, body := env.get(t, "/api/stats")
	if rec.Code != 200 {
		t.Fatalf("stats = %d: %s", rec.Code, body)
	}
	var out struct {
		Filas      int64 `json:"filas"`
		Docs       int64 `json:"docs"`
		MesesTotal int   `json:"meses_total"`
	}
	json.Unmarshal(body, &out)
	if out.Filas != 6 || out.Docs != 4 || out.MesesTotal != 2 {
		t.Errorf("stats = %+v", out)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	env := newTestEnv(t)
	body, _ := json.Marshal(map[string]string{"user": "user1", "password": "mala"})
	rec := env.do(t, http.MethodPost, "/api/login", bytes.NewReader(body), false)
	if rec.Code != 401 {
		t.Fatalf("login malo = %d", rec.Code)
	}
}
