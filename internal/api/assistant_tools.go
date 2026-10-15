package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"gastos/internal/anthropic"
)

type chartPoint struct {
	Label string `json:"label"`
	Value int64  `json:"value"`
}

type chartDTO struct {
	Kind   string       `json:"kind"` // bars | line | pie
	Title  string       `json:"title"`
	Points []chartPoint `json:"points"`
}

const (
	maxChartPoints = 24
	maxRangeYears  = 5
	maxSearchLen   = 80
	maxTitleLen    = 80
	maxLabelLen    = 60
)

// The schema fragments are shared by the tools that take a period.
const periodProps = `"de":{"type":"string","description":"Data inicial (YYYY-MM-DD), inclusive."},"ate":{"type":"string","description":"Data final (YYYY-MM-DD), inclusive."}`

func schema(props string, required ...string) json.RawMessage {
	req, _ := json.Marshal(required)
	return json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{` + props + `},"required":` + string(req) + `}`)
}

func assistantTools() []anthropic.Tool {
	return []anthropic.Tool{
		{
			Name:        "resumo_por_categoria",
			Description: "Use para saber quanto foi gasto em cada categoria num período, com a participação de cada uma no total de despesas. Considera só despesas. Bom para perguntas como 'onde mais gastei no mês?'.",
			InputSchema: schema(periodProps, "de", "ate"),
		},
		{
			Name:        "totais_por_periodo",
			Description: "Use para ver receitas e despesas totais de um período, agrupadas por dia, semana (começando na segunda-feira) ou mês. Bom para comparar meses, ver tendência e saldo.",
			InputSchema: schema(periodProps+`,"agrupar":{"type":"string","enum":["dia","semana","mes"],"description":"Agrupamento; padrão: mes."}`, "de", "ate"),
		},
		{
			Name:        "listar_lancamentos",
			Description: "Use para listar lançamentos individuais (despesas e receitas) de um período, do mais recente ao mais antigo, opcionalmente filtrando por categoria ou por texto na descrição. Bom para achar uma compra específica.",
			InputSchema: schema(periodProps+`,"categoria":{"type":"string","description":"Nome exato da categoria (opcional)."},"busca":{"type":"string","maxLength":80,"description":"Texto contido na descrição (opcional)."},"limite":{"type":"integer","minimum":1,"maximum":50,"description":"Máximo de itens; padrão 20."}`, "de", "ate"),
		},
		{
			Name:        "maiores_gastos",
			Description: "Use para ver as maiores despesas individuais de um período, da maior para a menor. Bom para perguntas como 'quais foram meus maiores gastos?'.",
			InputSchema: schema(periodProps+`,"n":{"type":"integer","minimum":1,"maximum":20,"description":"Quantidade de itens; padrão 5."}`, "de", "ate"),
		},
		{
			Name:        "mostrar_grafico",
			Description: "Use para mostrar ao usuário um gráfico (barras, linha ou pizza) com números que você já obteve das outras ferramentas. Não busca dados: você informa os pontos. Valores sempre em centavos (inteiros, não negativos), no máximo 24 pontos.",
			InputSchema: schema(`"tipo":{"type":"string","enum":["bars","line","pie"]},"titulo":{"type":"string","maxLength":80,"description":"Título curto do gráfico."},"pontos":{"type":"array","maxItems":24,"items":{"type":"object","additionalProperties":false,"properties":{"rotulo":{"type":"string","maxLength":60},"valor":{"type":"integer","minimum":0,"description":"Valor em centavos."}},"required":["rotulo","valor"]}}`, "tipo", "titulo", "pontos"),
		},
	}
}

type period struct {
	De  string `json:"de"`
	Ate string `json:"ate"`
}

func (p period) validate() error {
	from, err1 := time.Parse(dateLayout, p.De)
	to, err2 := time.Parse(dateLayout, p.Ate)
	switch {
	case err1 != nil || err2 != nil:
		return inputErr("'de' e 'ate' são obrigatórios no formato YYYY-MM-DD")
	case from.After(to):
		return inputErr("'de' não pode ser depois de 'ate'")
	case to.After(from.AddDate(maxRangeYears, 0, 0)):
		return inputErr("o intervalo máximo é de %d anos", maxRangeYears)
	}
	return nil
}

// toolInputError is a problem with the model's tool input; its message is safe to show the model.
type toolInputError struct{ msg string }

func (e toolInputError) Error() string { return e.msg }

func inputErr(format string, a ...any) error { return toolInputError{fmt.Sprintf(format, a...)} }

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func decodeToolInput(input json.RawMessage, v any) error {
	dec := json.NewDecoder(bytes.NewReader(input))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return inputErr("entrada inválida: %v", err)
	}
	return nil
}

// txItem is one transaction as shown to the model; Carteira is only set when several wallets are in play.
type txItem struct {
	Data      string `json:"data"`
	Tipo      string `json:"tipo,omitempty"`
	Descricao string `json:"descricao"`
	Categoria string `json:"categoria"`
	Centavos  int64  `json:"centavos"`
	Carteira  string `json:"carteira,omitempty"`
}

func (s *Server) runTool(ctx context.Context, walletIDs []int64, name string, input json.RawMessage) (string, *chartDTO, error) {
	if name == "mostrar_grafico" {
		chart, err := chartFromInput(input)
		if err != nil {
			return "", nil, err
		}
		return `{"ok":true}`, chart, nil
	}
	if len(walletIDs) == 0 {
		return "", nil, inputErr("nenhuma carteira disponível")
	}
	var out any
	var err error
	switch name {
	case "resumo_por_categoria":
		out, err = s.toolCategorySummary(ctx, walletIDs, input)
	case "totais_por_periodo":
		out, err = s.toolPeriodTotals(ctx, walletIDs, input)
	case "listar_lancamentos":
		out, err = s.toolListTransactions(ctx, walletIDs, input)
	case "maiores_gastos":
		out, err = s.toolTopExpenses(ctx, walletIDs, input)
	default:
		return "", nil, inputErr("ferramenta desconhecida: %s", name)
	}
	if err != nil {
		return "", nil, err
	}
	b, err := json.Marshal(out)
	return string(b), nil, err
}

func chartFromInput(input json.RawMessage) (*chartDTO, error) {
	var in struct {
		Tipo   string `json:"tipo"`
		Titulo string `json:"titulo"`
		Pontos []struct {
			Rotulo string `json:"rotulo"`
			Valor  int64  `json:"valor"`
		} `json:"pontos"`
	}
	if err := decodeToolInput(input, &in); err != nil {
		return nil, err
	}
	title := strings.TrimSpace(in.Titulo)
	switch {
	case in.Tipo != "bars" && in.Tipo != "line" && in.Tipo != "pie":
		return nil, inputErr("'tipo' deve ser bars, line ou pie")
	case utf8.RuneCountInString(title) > maxTitleLen:
		return nil, inputErr("'titulo' pode ter no máximo %d caracteres", maxTitleLen)
	case title == "":
		return nil, inputErr("'titulo' é obrigatório")
	case len(in.Pontos) == 0 || len(in.Pontos) > maxChartPoints:
		return nil, inputErr("'pontos' deve ter de 1 a %d itens", maxChartPoints)
	}
	c := &chartDTO{Kind: in.Tipo, Title: title, Points: make([]chartPoint, 0, len(in.Pontos))}
	for _, p := range in.Pontos {
		if p.Valor < 0 {
			return nil, inputErr("'valor' não pode ser negativo")
		}
		if utf8.RuneCountInString(p.Rotulo) > maxLabelLen {
			return nil, inputErr("'rotulo' pode ter no máximo %d caracteres", maxLabelLen)
		}
		c.Points = append(c.Points, chartPoint{Label: p.Rotulo, Value: p.Valor})
	}
	return c, nil
}

// walletFilter builds "t.wallet_id IN (...) AND t.date BETWEEN ? AND ?" with its args.
func walletFilter(walletIDs []int64, p period) (string, []any) {
	args := make([]any, 0, len(walletIDs)+2)
	for _, id := range walletIDs {
		args = append(args, id)
	}
	args = append(args, p.De, p.Ate)
	return "t.wallet_id IN (?" + strings.Repeat(",?", len(walletIDs)-1) + ") AND t.date BETWEEN ? AND ?", args
}

func (s *Server) toolCategorySummary(ctx context.Context, walletIDs []int64, input json.RawMessage) (any, error) {
	var in period
	if err := decodeToolInput(input, &in); err != nil {
		return nil, err
	}
	if err := in.validate(); err != nil {
		return nil, err
	}
	where, args := walletFilter(walletIDs, in)
	rows, err := s.db.QueryContext(ctx, `
		SELECT w.name, c.name, SUM(t.amount_cents) AS cents
		FROM transactions t JOIN categories c ON c.id = t.category_id AND c.wallet_id = t.wallet_id JOIN wallets w ON w.id = t.wallet_id
		WHERE t.kind = 'expense' AND `+where+`
		GROUP BY t.wallet_id, c.name ORDER BY cents DESC, c.name`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type item struct {
		Categoria string  `json:"categoria"`
		Centavos  int64   `json:"centavos"`
		Pct       float64 `json:"participacao_pct"`
		Carteira  string  `json:"carteira,omitempty"`
	}
	items := []item{}
	var total int64
	for rows.Next() {
		var it item
		var wallet string
		if err := rows.Scan(&wallet, &it.Categoria, &it.Centavos); err != nil {
			return nil, err
		}
		if len(walletIDs) > 1 {
			it.Carteira = wallet
		}
		total += it.Centavos
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Pct = math.Round(float64(items[i].Centavos)*1000/float64(total)) / 10
	}
	return map[string]any{"total_despesas": total, "categorias": items}, nil
}

func (s *Server) toolPeriodTotals(ctx context.Context, walletIDs []int64, input json.RawMessage) (any, error) {
	var in struct {
		period
		Agrupar string `json:"agrupar"`
	}
	if err := decodeToolInput(input, &in); err != nil {
		return nil, err
	}
	if err := in.validate(); err != nil {
		return nil, err
	}
	var bucket string
	switch in.Agrupar {
	case "dia":
		bucket = "t.date"
	case "semana":
		// 'weekday 0' moves to the coming Sunday, so -6 days lands on that week's Monday.
		bucket = "date(t.date, 'weekday 0', '-6 days')"
	case "mes", "":
		bucket = "strftime('%Y-%m', t.date)"
	default:
		return nil, inputErr("'agrupar' deve ser dia, semana ou mes")
	}
	var sum monthTotals
	for _, id := range walletIDs {
		m, err := s.totals(ctx, id, in.De, in.Ate)
		if err != nil {
			return nil, err
		}
		sum.IncomeCents += m.IncomeCents
		sum.ExpenseCents += m.ExpenseCents
	}
	where, args := walletFilter(walletIDs, in.period)
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+bucket+` AS b,
		  COALESCE(SUM(CASE WHEN t.kind = 'expense' THEN t.amount_cents END), 0),
		  COALESCE(SUM(CASE WHEN t.kind = 'income' THEN t.amount_cents END), 0)
		FROM transactions t WHERE `+where+` GROUP BY b ORDER BY b`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type item struct {
		Periodo  string `json:"periodo"`
		Despesas int64  `json:"despesas"`
		Receitas int64  `json:"receitas"`
	}
	items := []item{}
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.Periodo, &it.Despesas, &it.Receitas); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{
		"despesas": sum.ExpenseCents, "receitas": sum.IncomeCents, "saldo": sum.IncomeCents - sum.ExpenseCents, "periodos": items,
	}, nil
}

// queryTx runs the shared transaction listing; extra is an additional WHERE clause (may be empty).
func (s *Server) queryTx(ctx context.Context, walletIDs []int64, p period, extra string, extraArgs []any, order string, limit int) ([]txItem, error) {
	where, args := walletFilter(walletIDs, p)
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.date, t.kind, t.description, c.name, t.amount_cents, w.name
		FROM transactions t JOIN categories c ON c.id = t.category_id AND c.wallet_id = t.wallet_id JOIN wallets w ON w.id = t.wallet_id
		WHERE `+where+extra+` ORDER BY `+order+` LIMIT ?`, append(append(args, extraArgs...), limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []txItem{}
	for rows.Next() {
		var it txItem
		var wallet string
		if err := rows.Scan(&it.Data, &it.Tipo, &it.Descricao, &it.Categoria, &it.Centavos, &wallet); err != nil {
			return nil, err
		}
		if len(walletIDs) > 1 {
			it.Carteira = wallet
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *Server) toolListTransactions(ctx context.Context, walletIDs []int64, input json.RawMessage) (any, error) {
	var in struct {
		period
		Categoria string `json:"categoria"`
		Busca     string `json:"busca"`
		Limite    *int   `json:"limite"`
	}
	if err := decodeToolInput(input, &in); err != nil {
		return nil, err
	}
	if err := in.validate(); err != nil {
		return nil, err
	}
	limit := 20
	if in.Limite != nil {
		if limit = *in.Limite; limit < 1 || limit > 50 {
			return nil, inputErr("'limite' deve estar entre 1 e 50")
		}
	}
	if utf8.RuneCountInString(in.Busca) > maxSearchLen {
		return nil, inputErr("'busca' pode ter no máximo %d caracteres", maxSearchLen)
	}
	extra, extraArgs := "", []any{}
	if in.Categoria != "" {
		extra += ` AND c.name = ? COLLATE NOCASE`
		extraArgs = append(extraArgs, in.Categoria)
	}
	if in.Busca != "" {
		extra += ` AND t.description LIKE '%' || ? || '%' ESCAPE '\'`
		extraArgs = append(extraArgs, likeEscaper.Replace(in.Busca))
	}
	// One extra row tells the model whether the list was cut.
	items, err := s.queryTx(ctx, walletIDs, in.period, extra, extraArgs, "t.date DESC, t.id DESC", limit+1)
	if err != nil {
		return nil, err
	}
	truncated := len(items) > limit
	if truncated {
		items = items[:limit]
	}
	return map[string]any{"lancamentos": items, "truncado": truncated}, nil
}

func (s *Server) toolTopExpenses(ctx context.Context, walletIDs []int64, input json.RawMessage) (any, error) {
	var in struct {
		period
		N *int `json:"n"`
	}
	if err := decodeToolInput(input, &in); err != nil {
		return nil, err
	}
	if err := in.validate(); err != nil {
		return nil, err
	}
	n := 5
	if in.N != nil {
		if n = *in.N; n < 1 || n > 20 {
			return nil, inputErr("'n' deve estar entre 1 e 20")
		}
	}
	items, err := s.queryTx(ctx, walletIDs, in.period, ` AND t.kind = 'expense'`, nil, "t.amount_cents DESC, t.date DESC, t.id DESC", n)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Tipo = "" // always an expense here
	}
	return map[string]any{"maiores_gastos": items}, nil
}
