package api

import (
	"context"
	"encoding/json"
	"testing"

	"gastos/internal/api/ledger"
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

// purchase posts a transaction on the given method and returns it.
func purchase(c *client, kind, date string, method int64) ledger.Transaction {
	c.app.t.Helper()
	var tx ledger.Transaction
	c.do("POST", c.wpath("/transactions"), map[string]any{"kind": kind, "amountCents": 8700, "description": "Jantar",
		"categoryId": firstCategory(c.app.t, c, kind), "paymentMethodId": method, "date": date, "paidByUserId": c.user.ID}, 201, &tx)
	return tx
}

// The cases below call the assistant tools; they move to the assistant package with it.
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
