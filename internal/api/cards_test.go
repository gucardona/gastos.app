package api

import (
	"context"
	"encoding/json"
	"testing"
)

type testCard struct {
	ClosingDay      *int   `json:"closingDay"`
	DueDay          *int   `json:"dueDay"`
	Issuer          string `json:"issuer"`
	CardForm        string `json:"cardForm"`
	Last4           string `json:"last4"`
	BestPurchaseDay int    `json:"bestPurchaseDay"`
}

type methodDTO struct {
	ID       int64     `json:"id"`
	Name     string    `json:"name"`
	Archived bool      `json:"archived"`
	Card     *testCard `json:"card"`
}

func card2027(c *client) methodDTO {
	c.app.t.Helper()
	var m methodDTO
	c.do("POST", c.wpath("/payment-methods"), map[string]any{"name": "Itaú Black", "icon": "💳",
		"card": map[string]any{"closingDay": 20, "dueDay": 27, "issuer": " Itaú Mastercard ", "cardForm": "virtual", "last4": " 1234 "}}, 201, &m)
	return m
}

func TestCreateCard(t *testing.T) {
	c := newTestApp(t).register("Ana", "ana@x.com")
	m := card2027(c)
	want := testCard{Issuer: "Itaú Mastercard", CardForm: "virtual", Last4: "1234", BestPurchaseDay: 21}
	check := func(m methodDTO) {
		t.Helper()
		if m.Card == nil || *m.Card.ClosingDay != 20 || *m.Card.DueDay != 27 || m.Card.Issuer != want.Issuer ||
			m.Card.CardForm != want.CardForm || m.Card.Last4 != want.Last4 || m.Card.BestPurchaseDay != 21 {
			t.Fatalf("card = %+v", m.Card)
		}
	}
	check(m)
	var list []methodDTO
	c.do("GET", c.wpath("/payment-methods"), nil, 200, &list)
	for _, p := range list {
		if p.ID == m.ID {
			check(p)
		} else if p.Card != nil {
			t.Errorf("%s is not a card: %+v", p.Name, p.Card)
		}
	}
}

func TestCardFieldsValidation(t *testing.T) {
	c := newTestApp(t).register("Ana", "ana@x.com")
	for field, card := range map[string]map[string]any{
		"closingDay": {"dueDay": 27},
		"dueDay":     {"closingDay": 20, "dueDay": 32},
		"last4":      {"closingDay": 20, "dueDay": 27, "last4": "12345"},
		"cardForm":   {"closingDay": 20, "dueDay": 27, "cardForm": "plastic"},
	} {
		var e struct {
			Error struct {
				Fields map[string]string `json:"fields"`
			} `json:"error"`
		}
		c.do("POST", c.wpath("/payment-methods"), map[string]any{"name": "X " + field, "icon": "💳", "card": card}, 400, &e)
		if e.Error.Fields[field] == "" {
			t.Errorf("%v: want an error on %s, got %v", card, field, e.Error.Fields)
		}
	}
	var e struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	c.do("POST", c.wpath("/payment-methods"), map[string]any{"name": "Y", "icon": "💳", "card": map[string]any{"closingDay": 1, "dueDay": 2, "last4": "12a4"}}, 400, &e)
	if e.Error.Fields["last4"] == "" {
		t.Errorf("12a4: %v", e.Error.Fields)
	}
}

func TestCardSurvivesArchiveAndTurnsOff(t *testing.T) {
	c := newTestApp(t).register("Ana", "ana@x.com")
	m := card2027(c)
	path := c.wpath("/payment-methods/%d", m.ID)
	// archiving sends only name, icon, sortOrder and archived: the card stays
	c.do("PUT", path, map[string]any{"name": m.Name, "icon": "💳", "sortOrder": 0, "archived": true}, 200, &m)
	if m.Card == nil || *m.Card.ClosingDay != 20 {
		t.Fatalf("archive dropped the card: %+v", m.Card)
	}
	c.do("PUT", path, map[string]any{"name": m.Name, "icon": "💳", "archived": false, "card": nil}, 200, &m)
	if m.Card != nil {
		t.Fatalf("card: null should turn it into a plain method, got %+v", m.Card)
	}
}

// purchase posts a transaction on the given method and returns it.
func purchase(c *client, kind, date string, method int64) transaction {
	c.app.t.Helper()
	var tx transaction
	c.do("POST", c.wpath("/transactions"), map[string]any{"kind": kind, "amountCents": 8700, "description": "Jantar",
		"categoryId": firstCategory(c.app.t, c, kind), "paymentMethodId": method, "date": date, "paidByUserId": c.user.ID}, 201, &tx)
	return tx
}

func due(tx transaction) string {
	if tx.InvoiceDue == nil {
		return "<nil>"
	}
	return *tx.InvoiceDue
}

func TestCardPurchaseGetsInvoiceDue(t *testing.T) {
	c := newTestApp(t).register("Ana", "ana@x.com")
	card := card2027(c)
	for _, x := range []struct{ kind, date, want string }{
		{"expense", "2026-11-22", "2026-12-27"},
		{"expense", "2026-11-20", "2026-11-27"},
		{"income", "2026-11-22", "2026-12-27"}, // a refund credits the same invoice
	} {
		if got := due(purchase(c, x.kind, x.date, card.ID)); got != x.want {
			t.Errorf("%s on %s: invoiceDue %s, want %s", x.kind, x.date, got, x.want)
		}
	}
	var list []transaction
	c.do("GET", c.wpath("/transactions?from=2026-11-01&to=2026-11-30"), nil, 200, &list)
	if len(list) != 3 || due(list[0]) == "<nil>" {
		t.Fatalf("the list carries invoiceDue too: %+v", list)
	}
}

func TestPlainMethodStampsNothing(t *testing.T) {
	c := newTestApp(t).register("Ana", "ana@x.com")
	var methods []methodDTO
	c.do("GET", c.wpath("/payment-methods"), nil, 200, &methods)
	if got := due(purchase(c, "expense", "2026-11-22", methods[0].ID)); got != "<nil>" {
		t.Fatalf("invoiceDue %s on %s", got, methods[0].Name)
	}
}

func TestEditKeepsInvoiceWhenDateAndMethodUnchanged(t *testing.T) {
	c := newTestApp(t).register("Ana", "ana@x.com")
	card := card2027(c)
	tx := purchase(c, "expense", "2026-11-22", card.ID)
	c.do("PUT", c.wpath("/payment-methods/%d", card.ID), map[string]any{"name": card.Name, "icon": "💳",
		"card": map[string]any{"closingDay": 25, "dueDay": 27}}, 200, nil)
	body := map[string]any{"kind": "expense", "amountCents": 9900, "description": "Jantar e vinho", "categoryId": tx.CategoryID,
		"paymentMethodId": card.ID, "date": "2026-11-22", "paidByUserId": c.user.ID}
	c.do("PUT", c.wpath("/transactions/%d", tx.ID), body, 200, &tx)
	if due(tx) != "2026-12-27" {
		t.Fatalf("an edit of the amount moved the purchase: %s", due(tx))
	}
	body["date"] = "2026-11-24" // a new date follows the card's current rule: 24 <= 25
	c.do("PUT", c.wpath("/transactions/%d", tx.ID), body, 200, &tx)
	if due(tx) != "2026-11-27" {
		t.Fatalf("after changing the date: %s, want 2026-11-27", due(tx))
	}
	var methods []methodDTO
	c.do("GET", c.wpath("/payment-methods"), nil, 200, &methods)
	body["paymentMethodId"] = methods[0].ID // Pix
	c.do("PUT", c.wpath("/transactions/%d", tx.ID), body, 200, &tx)
	if due(tx) != "<nil>" {
		t.Fatalf("moving to a plain method keeps invoiceDue %s", due(tx))
	}
}

func TestBankAcceptOnCardAccount(t *testing.T) {
	c := newTestApp(t).register("Ana", "ana@x.com")
	card := card2027(c)
	acc := newAccount(t, c, "Cartão")
	c.do("PUT", c.wpath("/bank-accounts/%d", acc.ID), map[string]any{"name": "Cartão", "paymentMethodId": card.ID, "enabled": true}, 200, nil)
	importFile(t, c, acc.ID, ofxRows("20261122|-48.90|C1|RESTAURANTE"), 200)
	item := findByDescription(t, pending(t, c), "RESTAURANTE")
	var tx transaction
	c.do("POST", c.wpath("/inbox/%d/accept", item.ID), map[string]any{"categoryId": firstCategory(t, c, "expense")}, 201, &tx)
	if due(tx) != "2026-12-27" {
		t.Fatalf("accepted line: invoiceDue %s", due(tx))
	}
}

func TestMonthlyTotalsUseInvoiceMonth(t *testing.T) {
	c := newTestApp(t).register("Ana", "ana@x.com")
	card := card2027(c)
	purchase(c, "expense", "2026-11-22", card.ID) // due 2026-12-27
	run := func(input string) map[string]any {
		t.Helper()
		text, _, err := c.app.s.runTool(context.Background(), []int64{c.walletID}, "totais_por_periodo", json.RawMessage(input))
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(text), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	out := run(`{"de":"2026-11-01","ate":"2026-12-31","agrupar":"mes"}`)
	ps := out["periodos"].([]any)
	if len(ps) != 1 || ps[0].(map[string]any)["periodo"] != "2026-12" || out["despesas"] != float64(8700) {
		t.Fatalf("by month: %v", out)
	}
	// by day it stays on the purchase date
	out = run(`{"de":"2026-11-01","ate":"2026-11-30","agrupar":"dia"}`)
	if ps := out["periodos"].([]any); len(ps) != 1 || ps[0].(map[string]any)["periodo"] != "2026-11-22" || out["despesas"] != float64(8700) {
		t.Fatalf("by day: %v", out)
	}
}

func runTool(t *testing.T, c *client, name, input string) map[string]any {
	t.Helper()
	text, _, err := c.app.s.runTool(context.Background(), []int64{c.walletID}, name, json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// A range that is not whole months keeps filtering by date, for everyone: "até dia 15" means up to the 15th.
func TestPartialRangeKeepsDateFilter(t *testing.T) {
	c := newTestApp(t).register("Ana", "ana@x.com")
	var methods []methodDTO
	c.do("GET", c.wpath("/payment-methods"), nil, 200, &methods)
	purchase(c, "expense", "2026-11-03", methods[0].ID)
	purchase(c, "expense", "2026-11-25", methods[0].ID)
	for _, tool := range []string{"totais_por_periodo", "resumo_por_categoria"} {
		out := runTool(t, c, tool, `{"de":"2026-11-01","ate":"2026-11-15"}`)
		got := out["despesas"]
		if tool == "resumo_por_categoria" {
			got = out["total_despesas"]
		}
		if got != float64(8700) {
			t.Errorf("%s up to the 15th: %v, want 8700", tool, got)
		}
	}
}

// Whole months follow the invoice in every tool, so they agree with the Visão.
func TestWholeMonthToolsFollowInvoice(t *testing.T) {
	c := newTestApp(t).register("Ana", "ana@x.com")
	card := card2027(c)
	purchase(c, "expense", "2026-11-22", card.ID) // due 2026-12-27
	dec := `{"de":"2026-12-01","ate":"2026-12-31"}`
	if out := runTool(t, c, "resumo_por_categoria", dec); out["total_despesas"] != float64(8700) {
		t.Errorf("category summary for December: %v", out)
	}
	if out := runTool(t, c, "resumo_por_categoria", `{"de":"2026-11-01","ate":"2026-11-30"}`); out["total_despesas"] != float64(0) {
		t.Errorf("category summary for November: %v", out)
	}
	if out := runTool(t, c, "listar_lancamentos", dec); len(out["lancamentos"].([]any)) != 1 {
		t.Errorf("listing December: %v", out)
	}
}
