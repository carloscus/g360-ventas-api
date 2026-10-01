package query

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type clause struct {
	col string
	op  string
	arg string
}

type orderTerm struct {
	col  string
	desc bool
}

type Options struct {
	Select []string
	Where  []clause
	Order  []orderTerm
	Limit  int
	Offset int
}

var reserved = map[string]bool{
	"select": true, "order": true, "limit": true, "offset": true, "token": true,
}

var ops = map[string]string{
	"eq": "=", "neq": "<>", "gt": ">", "gte": ">=", "lt": "<", "lte": "<=",
	"like": "LIKE", "in": "IN",
}

func Parse(params url.Values, columns []string, maxLimit int) (*Options, error) {
	colSet := make(map[string]bool, len(columns))
	for _, c := range columns {
		colSet[c] = true
	}

	opts := &Options{Select: columns, Limit: 100}

	if s := params.Get("select"); s != "" && s != "*" {
		var sel []string
		for _, c := range strings.Split(s, ",") {
			c = strings.TrimSpace(c)
			if !colSet[c] {
				return nil, fmt.Errorf("columna desconocida en select: %s", c)
			}
			sel = append(sel, c)
		}
		if len(sel) > 0 {
			opts.Select = sel
		}
	}

	if o := params.Get("order"); o != "" {
		for _, term := range strings.Split(o, ",") {
			term = strings.TrimSpace(term)
			col, dir, found := strings.Cut(term, ".")
			if !found {
				dir = "asc"
			}
			if !colSet[col] {
				return nil, fmt.Errorf("columna desconocida en order: %s", col)
			}
			switch strings.ToLower(dir) {
			case "asc":
				opts.Order = append(opts.Order, orderTerm{col: col})
			case "desc":
				opts.Order = append(opts.Order, orderTerm{col: col, desc: true})
			default:
				return nil, fmt.Errorf("direccion invalida en order: %s", dir)
			}
		}
	}

	if v := params.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("limit invalido: %s", v)
		}
		opts.Limit = n
	}
	if opts.Limit > maxLimit {
		opts.Limit = maxLimit
	}
	if v := params.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("offset invalido: %s", v)
		}
		opts.Offset = n
	}

	keys := make([]string, 0, len(params))
	for key := range params {
		if !reserved[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, raw := range params[key] {
			c, err := parseFilter(key, raw, colSet)
			if err != nil {
				return nil, err
			}
			opts.Where = append(opts.Where, c)
		}
	}
	return opts, nil
}

func parseFilter(col, raw string, colSet map[string]bool) (clause, error) {
	if !colSet[col] {
		return clause{}, fmt.Errorf("columna desconocida: %s", col)
	}
	op, val, found := strings.Cut(raw, ".")
	if !found {
		return clause{}, fmt.Errorf("filtro invalido (se esperaba op.valor): %s", raw)
	}
	sqlOp, ok := ops[op]
	if !ok {
		return clause{}, fmt.Errorf("operador no soportado: %s", op)
	}
	if op == "in" {
		val = strings.TrimPrefix(val, "(")
		val = strings.TrimSuffix(val, ")")
		if val == "" {
			return clause{}, fmt.Errorf("in. requiere valores: %s", raw)
		}
	} else if val == "" && op != "like" {
		return clause{}, fmt.Errorf("valor vacio en filtro: %s", raw)
	}
	if op == "like" {
		val = strings.ReplaceAll(val, "*", "%")
	}
	return clause{col: col, op: sqlOp, arg: val}, nil
}

func (o *Options) SQL(table string, quote func(string) (string, error)) (string, []any, error) {
	var b strings.Builder
	var args []any

	b.WriteString("SELECT ")
	for i, c := range o.Select {
		if i > 0 {
			b.WriteString(", ")
		}
		qc, err := quote(c)
		if err != nil {
			return "", nil, err
		}
		b.WriteString(qc)
	}
	qt, err := quote(table)
	if err != nil {
		return "", nil, err
	}
	b.WriteString(" FROM ")
	b.WriteString(qt)

	if len(o.Where) > 0 {
		b.WriteString(" WHERE ")
		for i, w := range o.Where {
			if i > 0 {
				b.WriteString(" AND ")
			}
			col, err := quote(w.col)
			if err != nil {
				return "", nil, err
			}
			if w.op == "IN" {
				parts := strings.Split(w.arg, ",")
				b.WriteString(col)
				b.WriteString(" IN (")
				for j, p := range parts {
					if j > 0 {
						b.WriteString(", ")
					}
					b.WriteString("?")
					args = append(args, strings.TrimSpace(p))
				}
				b.WriteString(")")
				continue
			}
			b.WriteString(col)
			b.WriteString(" ")
			b.WriteString(w.op)
			b.WriteString(" ?")
			args = append(args, w.arg)
		}
	}

	if len(o.Order) > 0 {
		b.WriteString(" ORDER BY ")
		for i, t := range o.Order {
			if i > 0 {
				b.WriteString(", ")
			}
			col, err := quote(t.col)
			if err != nil {
				return "", nil, err
			}
			b.WriteString(col)
			if t.desc {
				b.WriteString(" DESC")
			}
		}
	}

	b.WriteString(" LIMIT ?")
	args = append(args, o.Limit)
	if o.Offset > 0 {
		b.WriteString(" OFFSET ?")
		args = append(args, o.Offset)
	}
	return b.String(), args, nil
}
