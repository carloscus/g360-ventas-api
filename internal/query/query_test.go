package query

import (
	"net/url"
	"strings"
	"testing"
)

func cols() []string { return []string{"id_articulo", "soles", "mes_ref", "nom_articulo"} }

func quote(name string) (string, error) { return `"` + name + `"`, nil }

func TestParseEqLikeIn(t *testing.T) {
	params := url.Values{}
	params.Set("id_articulo", "eq.02211")
	params.Set("nom_articulo", "like.%VINIFAN%")
	params.Set("mes_ref", "in.(2026-09,2026-10)")
	params.Set("order", "mes_ref.desc")
	params.Set("limit", "50")
	params.Set("select", "id_articulo,soles")

	opts, err := Parse(params, cols(), 5000)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(opts.Where) != 3 {
		t.Fatalf("esperaba 3 filtros, got %d", len(opts.Where))
	}
	if opts.Limit != 50 {
		t.Errorf("limit = %d", opts.Limit)
	}
	if len(opts.Select) != 2 {
		t.Errorf("select = %v", opts.Select)
	}
	if len(opts.Order) != 1 || !opts.Order[0].desc {
		t.Errorf("order = %v", opts.Order)
	}

	q, args := opts.SQLMust("ventas", quote)
	if !strings.Contains(q, `"id_articulo" = ?`) {
		t.Errorf("sql sin eq: %s", q)
	}
	if !strings.Contains(q, `"nom_articulo" LIKE ?`) {
		t.Errorf("sql sin like: %s", q)
	}
	if !strings.Contains(q, `"mes_ref" IN (?, ?)`) {
		t.Errorf("sql sin in: %s", q)
	}
	if len(args) != 5 {
		t.Errorf("args = %v", args)
	}
	if args[0] != "02211" {
		t.Errorf("eq arg = %v", args[0])
	}
	if args[1] != "2026-09" || args[2] != "2026-10" {
		t.Errorf("args in = %v", args)
	}
	if args[3] != "%VINIFAN%" {
		t.Errorf("like arg = %v", args[3])
	}
	if args[4] != 50 {
		t.Errorf("limit arg = %v", args[4])
	}
}

func TestParseUnknownColumn(t *testing.T) {
	params := url.Values{}
	params.Set("columna_rara", "eq.x")
	if _, err := Parse(params, cols(), 5000); err == nil {
		t.Fatal("esperaba error por columna desconocida")
	}
}

func TestParseUnknownOp(t *testing.T) {
	params := url.Values{}
	params.Set("soles", "regex.x")
	if _, err := Parse(params, cols(), 5000); err == nil {
		t.Fatal("esperaba error por operador desconocido")
	}
}

func TestParseLikeStar(t *testing.T) {
	params := url.Values{}
	params.Set("nom_articulo", "like.VINI*")
	opts, err := Parse(params, cols(), 5000)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if opts.Where[0].arg != "VINI%" {
		t.Errorf("like star = %q", opts.Where[0].arg)
	}
}

func TestLimitCap(t *testing.T) {
	params := url.Values{}
	params.Set("limit", "999999")
	opts, err := Parse(params, cols(), 5000)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if opts.Limit != 5000 {
		t.Errorf("limit cap = %d", opts.Limit)
	}
}

func (o *Options) SQLMust(table string, quote func(string) (string, error)) (string, []any) {
	q, args, err := o.SQL(table, quote)
	if err != nil {
		panic(err)
	}
	return q, args
}
