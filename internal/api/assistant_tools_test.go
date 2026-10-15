package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type toolFixture struct {
	app          *testApp
	ana, bia     *client
	a, b, c      int64 // a, b: Ana's wallets; c: Bia's, never handed to the tools
	catA, catB   string
	walletsAB    []int64
	walletsOnlyA []int64
}

func catIDByName(t *testing.T, c *client, walletID int64, kind, name string) int64 {
	t.Helper()
	var cats []categoryDTO
	c.do("GET", "/api/wallets/"+itoa(walletID)+"/categories", nil, 200, &cats)
	for _, cat := range cats {
		if cat.Kind == kind && cat.Name == name {
			return cat.ID
		}
	}
	t.Fatalf("category %q not found", name)
	return 0
}

func newToolFixture(t *testing.T) *toolFixture {
	t.Helper()
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	var w walletDTO
	ana.do("POST", "/api/wallets", map[string]any{"name": "Casa"}, 201, &w)
	f := &toolFixture{app: app, ana: ana, bia: bia, a: ana.walletID, b: w.ID, c: bia.walletID}

	var cats []categoryDTO
	ana.do("GET", ana.wpath("/categories"), nil, 200, &cats)
	var exp, inc []categoryDTO
	for _, c := range cats {
		if c.Kind == "expense" {
			exp = append(exp, c)
		} else {
			inc = append(inc, c)
		}
	}
	x, y := exp[0], exp[1]
	post := func(cl *client, wallet int64, kind, name string, cents int, desc, date string) {
		t.Helper()
		cl.do("POST", "/api/wallets/"+itoa(wallet)+"/transactions", map[string]any{
			"kind": kind, "amountCents": cents, "description": desc, "date": date,
			"categoryId": catIDByName(t, cl, wallet, kind, name),
		}, 201, nil)
	}
	f.catA, f.catB = x.Name, y.Name
	post(ana, f.a, "expense", x.Name, 1000, "Compra Mercado", "2026-10-05") // Monday
	post(ana, f.a, "expense", x.Name, 3000, "Padaria pao", "2026-10-06")
	post(ana, f.a, "expense", y.Name, 500, "Cinema", "2026-10-12") // next Monday
	post(ana, f.a, "income", inc[0].Name, 200000, "Salario", "2026-10-01")
	post(ana, f.b, "expense", x.Name, 2000, "Luz", "2026-10-07")
	post(bia, f.c, "expense", x.Name, 99999, "Segredo", "2026-10-05")
	f.walletsAB = []int64{f.a, f.b}
	f.walletsOnlyA = []int64{f.a}
	return f
}

func (f *toolFixture) run(t *testing.T, wallets []int64, name, input string) (map[string]any, *chartDTO) {
	t.Helper()
	text, chart, err := f.app.s.runTool(context.Background(), wallets, name, json.RawMessage(input))
	if err != nil {
		t.Fatalf("%s(%s): %v", name, input, err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("%s: result is not JSON: %v: %s", name, err, text)
	}
	return out, chart
}

const octRange = `"de":"2026-10-01","ate":"2026-10-31"`

func TestResumoPorCategoria(t *testing.T) {
	f := newToolFixture(t)
	out, _ := f.run(t, f.walletsOnlyA, "resumo_por_categoria", `{`+octRange+`}`)
	if out["total_despesas"] != float64(4500) {
		t.Fatalf("total: %v", out)
	}
	cats := out["categorias"].([]any)
	if len(cats) != 2 {
		t.Fatalf("categories: %v", cats)
	}
	first := cats[0].(map[string]any)
	if first["categoria"] != f.catA || first["centavos"] != float64(4000) || first["participacao_pct"] != 88.9 {
		t.Errorf("first: %v", first)
	}
	if _, has := first["carteira"]; has {
		t.Errorf("single wallet must not name the wallet: %v", first)
	}
}

func TestTotaisPorPeriodoGroups(t *testing.T) {
	f := newToolFixture(t)
	periods := func(group string) map[string][2]float64 {
		out, _ := f.run(t, f.walletsAB, "totais_por_periodo", `{`+octRange+`,"agrupar":"`+group+`"}`)
		if out["despesas"] != float64(6500) || out["receitas"] != float64(200000) || out["saldo"] != float64(193500) {
			t.Fatalf("totals: %v", out)
		}
		m := map[string][2]float64{}
		for _, p := range out["periodos"].([]any) {
			p := p.(map[string]any)
			m[p["periodo"].(string)] = [2]float64{p["despesas"].(float64), p["receitas"].(float64)}
		}
		return m
	}
	if m := periods("mes"); len(m) != 1 || m["2026-10"] != [2]float64{6500, 200000} {
		t.Errorf("month: %v", m)
	}
	m := periods("semana")
	if len(m) != 3 || m["2026-09-28"] != [2]float64{0, 200000} || m["2026-10-05"] != [2]float64{6000, 0} || m["2026-10-12"] != [2]float64{500, 0} {
		t.Errorf("week: %v", m)
	}
	if m := periods("dia"); len(m) != 5 || m["2026-10-06"] != [2]float64{3000, 0} {
		t.Errorf("day: %v", m)
	}
}

func TestListarLancamentosFilters(t *testing.T) {
	f := newToolFixture(t)
	list := func(extra string) []any {
		out, _ := f.run(t, f.walletsAB, "listar_lancamentos", `{`+octRange+extra+`}`)
		return out["lancamentos"].([]any)
	}
	if got := list(``); len(got) != 5 {
		t.Errorf("all: %d", len(got))
	}
	if got := list(`,"categoria":"` + strings.ToUpper(f.catB) + `"`); len(got) != 1 {
		t.Errorf("category: %v", got)
	}
	if got := list(`,"busca":"padaria"`); len(got) != 1 || got[0].(map[string]any)["centavos"] != float64(3000) {
		t.Errorf("text: %v", got)
	}
	f.ana.do("POST", "/api/wallets/"+itoa(f.a)+"/transactions", map[string]any{
		"kind": "expense", "amountCents": 100, "description": "Desconto 10%", "date": "2026-10-08",
		"categoryId": catIDByName(t, f.ana, f.a, "expense", f.catA),
	}, 201, nil)
	if got := list(`,"busca":"%"`); len(got) != 1 || got[0].(map[string]any)["descricao"] != "Desconto 10%" {
		t.Errorf("literal percent: %v", got)
	}
	if got := list(`,"busca":"_"`); len(got) != 0 {
		t.Errorf("underscore must not be a wildcard: %v", got)
	}
	out, _ := f.run(t, f.walletsAB, "listar_lancamentos", `{`+octRange+`,"limite":2}`)
	if len(out["lancamentos"].([]any)) != 2 || out["truncado"] != true {
		t.Errorf("limit: %v", out)
	}
}

func TestMaioresGastosOrder(t *testing.T) {
	f := newToolFixture(t)
	out, _ := f.run(t, f.walletsAB, "maiores_gastos", `{`+octRange+`,"n":2}`)
	got := out["maiores_gastos"].([]any)
	if len(got) != 2 || got[0].(map[string]any)["centavos"] != float64(3000) || got[1].(map[string]any)["centavos"] != float64(2000) {
		t.Errorf("order: %v", got)
	}
}

func TestMostrarGraficoReturnsChart(t *testing.T) {
	f := newToolFixture(t)
	_, chart := f.run(t, f.walletsAB, "mostrar_grafico",
		`{"tipo":"pie","titulo":"Gastos","pontos":[{"rotulo":"Mercado","valor":1050},{"rotulo":"Luz","valor":2}]}`)
	if chart == nil || chart.Kind != "pie" || chart.Title != "Gastos" || len(chart.Points) != 2 || chart.Points[0].Value != 1050 {
		t.Fatalf("chart: %+v", chart)
	}
	var many []string
	for i := 0; i < 25; i++ {
		many = append(many, `{"rotulo":"p","valor":1}`)
	}
	for name, in := range map[string]string{
		"bad kind":    `{"tipo":"radar","titulo":"x","pontos":[{"rotulo":"a","valor":1}]}`,
		"too many":    `{"tipo":"bars","titulo":"x","pontos":[` + strings.Join(many, ",") + `]}`,
		"empty title": `{"tipo":"bars","titulo":"  ","pontos":[{"rotulo":"a","valor":1}]}`,
		"no points":   `{"tipo":"bars","titulo":"x","pontos":[]}`,
		"negative":    `{"tipo":"line","titulo":"x","pontos":[{"rotulo":"a","valor":-1}]}`,
		"fractional":  `{"tipo":"bars","titulo":"x","pontos":[{"rotulo":"a","valor":1.5}]}`,
		"long title":  `{"tipo":"bars","titulo":"` + strings.Repeat("t", 81) + `","pontos":[{"rotulo":"a","valor":1}]}`,
		"long label":  `{"tipo":"bars","titulo":"x","pontos":[{"rotulo":"` + strings.Repeat("r", 61) + `","valor":1}]}`,
	} {
		if _, c, err := f.app.s.runTool(context.Background(), f.walletsAB, "mostrar_grafico", json.RawMessage(in)); err == nil || c != nil {
			t.Errorf("%s: want error, got chart=%v err=%v", name, c, err)
		}
	}
}

func TestToolsOnlySeeGivenWallets(t *testing.T) {
	f := newToolFixture(t)
	for _, name := range []string{"resumo_por_categoria", "totais_por_periodo", "listar_lancamentos", "maiores_gastos"} {
		text, _, err := f.app.s.runTool(context.Background(), f.walletsAB, name, json.RawMessage(`{`+octRange+`}`))
		if err != nil || strings.Contains(text, "99999") || strings.Contains(text, "Segredo") {
			t.Errorf("%s leaked another wallet: %v %s", name, err, text)
		}
	}
}

func TestToolRejectsBadInput(t *testing.T) {
	f := newToolFixture(t)
	for name, c := range map[string][2]string{
		"inverted":     {"listar_lancamentos", `{"de":"2026-10-31","ate":"2026-10-01"}`},
		"over 5 years": {"listar_lancamentos", `{"de":"2020-01-01","ate":"2026-10-01"}`},
		"bad date":     {"listar_lancamentos", `{"de":"2026-13-01","ate":"2026-10-01"}`},
		"missing ate":  {"listar_lancamentos", `{"de":"2026-10-01"}`},
		"limit 0":      {"listar_lancamentos", `{` + octRange + `,"limite":0}`},
		"limit 51":     {"listar_lancamentos", `{` + octRange + `,"limite":51}`},
		"wallet_id":    {"listar_lancamentos", `{` + octRange + `,"wallet_id":3}`},
		"invalid json": {"listar_lancamentos", `{"de":`},
		"long search":  {"listar_lancamentos", `{` + octRange + `,"busca":"` + strings.Repeat("a", 81) + `"}`},
		"bad grouping": {"totais_por_periodo", `{` + octRange + `,"agrupar":"ano"}`},
		"n 0":          {"maiores_gastos", `{` + octRange + `,"n":0}`},
		"n 21":         {"maiores_gastos", `{` + octRange + `,"n":21}`},
		"unknown tool": {"apagar_tudo", `{}`},
	} {
		if _, _, err := f.app.s.runTool(context.Background(), f.walletsAB, c[0], json.RawMessage(c[1])); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestEmptyPeriodIsNotAnError(t *testing.T) {
	f := newToolFixture(t)
	for _, name := range []string{"resumo_por_categoria", "totais_por_periodo", "listar_lancamentos", "maiores_gastos"} {
		f.run(t, f.walletsAB, name, `{"de":"2020-01-01","ate":"2020-01-31"}`)
	}
	out, _ := f.run(t, f.walletsAB, "listar_lancamentos", `{"de":"2020-01-01","ate":"2020-01-31"}`)
	if len(out["lancamentos"].([]any)) != 0 {
		t.Errorf("want empty list, got %v", out)
	}
}

func TestToolNamesWalletWhenMany(t *testing.T) {
	f := newToolFixture(t)
	out, _ := f.run(t, f.walletsAB, "maiores_gastos", `{`+octRange+`,"n":1}`)
	if out["maiores_gastos"].([]any)[0].(map[string]any)["carteira"] != "Pessoal" {
		t.Errorf("many wallets: %v", out)
	}
	out, _ = f.run(t, f.walletsOnlyA, "maiores_gastos", `{`+octRange+`,"n":1}`)
	if _, has := out["maiores_gastos"].([]any)[0].(map[string]any)["carteira"]; has {
		t.Errorf("one wallet: %v", out)
	}
}
