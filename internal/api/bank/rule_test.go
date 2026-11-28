package bank_test

import (
	"strings"
	"testing"

	"gastos/internal/api/apitest"
	"gastos/internal/api/bank"
	"gastos/internal/api/ledger"
)

func TestRulesSuggestCategoriesOnImport(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	transport := categoryByName(t, ana, "expense", "Transporte")
	ana.Do("POST", ana.WPath("/rules"), map[string]any{"pattern": "uber", "categoryId": transport, "renameTo": "Uber"}, 201, nil)

	apitest.ImportFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)
	uber := findByDescription(t, pending(t, ana), "UBER")
	if uber.SuggestedCategoryID == nil || *uber.SuggestedCategoryID != transport || uber.SuggestedDescription == nil || *uber.SuggestedDescription != "Uber" {
		t.Fatalf("uber line: %+v", uber)
	}
	if findByDescription(t, pending(t, ana), "MERCADO").SuggestedCategoryID != nil {
		t.Fatal("a line no rule matches must have no suggestion")
	}
}

func TestARuleForAnExpenseCategoryIsNotSuggestedForIncome(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	ana.Do("POST", ana.WPath("/rules"), map[string]any{"pattern": "estorno", "categoryId": categoryByName(t, ana, "expense", "Lazer")}, 201, nil)
	apitest.ImportFile(t, ana, newAccount(t, ana, "Conta").ID, apitest.OfxRows("20261001|30.00|R|ESTORNO LOJA"), 200)
	if got := pending(t, ana)[0]; got.SuggestedCategoryID != nil {
		t.Fatalf("an expense category was suggested for a credit: %+v", got)
	}
}

func TestAcceptSuggestedOnlyTakesLinesThatAreSafe(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	transport := categoryByName(t, ana, "expense", "Transporte")
	ana.Do("POST", ana.WPath("/rules"), map[string]any{"pattern": "uber", "categoryId": transport, "renameTo": "Uber"}, 201, nil)
	ana.Do("POST", ana.WPath("/rules"), map[string]any{"pattern": "mercado", "categoryId": categoryByName(t, ana, "expense", "Mercado")}, 201, nil)
	// the market purchase was already typed in: that line needs a person to decide (link or not)
	ana.Do("POST", ana.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 4890, "description": "Mercado (digitado)", "categoryId": categoryByName(t, ana, "expense", "Mercado"), "date": "2026-10-01",
	}, 201, nil)
	apitest.ImportFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)

	var out struct {
		Accepted int `json:"accepted"`
	}
	ana.Do("POST", ana.WPath("/inbox/accept-suggested"), nil, 200, &out)
	if out.Accepted != 1 {
		t.Fatalf("accepted %d, want only the Uber line", out.Accepted)
	}
	left := pending(t, ana)
	if len(left) != 2 || findAny(left, "UBER") {
		t.Fatalf("pending after accept-suggested: %+v", left)
	}
	var list []ledger.Transaction
	ana.Do("GET", ana.WPath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	for _, tx := range list {
		if tx.Source == "bank" && (tx.Description != "Uber" || tx.CategoryID != transport || tx.AmountCents != 2500) {
			t.Fatalf("the accepted transaction: %+v", tx)
		}
	}
}

// ---- rules ----

func TestRulesCRUDAndValidation(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	cat := categoryByName(t, ana, "expense", "Transporte")

	var created struct {
		Rule bank.RuleDTO `json:"rule"`
	}
	ana.Do("POST", ana.WPath("/rules"), map[string]any{"pattern": "  Uber  ", "categoryId": cat}, 201, &created)
	r := created.Rule
	if r.Pattern != "uber" || r.CategoryID != cat {
		t.Fatalf("created: %+v", r) // patterns are stored normalized so "UBER", "Uber" and "über" are one rule
	}
	ana.Do("POST", ana.WPath("/rules"), map[string]any{"pattern": "UBER", "categoryId": cat}, 409, nil)

	for name, body := range map[string]map[string]any{
		"empty pattern":    {"pattern": "  ", "categoryId": cat},
		"one character":    {"pattern": "u", "categoryId": cat},
		"very long":        {"pattern": strings.Repeat("a", 61), "categoryId": cat},
		"unknown category": {"pattern": "loja", "categoryId": 99999},
	} {
		if status := ana.Status("POST", ana.WPath("/rules"), body); status != 400 {
			t.Errorf("%s: status %d, want 400", name, status)
		}
	}
	ana.Do("POST", ana.WPath("/rules"), map[string]any{"pattern": "salario", "categoryId": categoryByName(t, ana, "income", "Salário")}, 201, nil) // income rules are fine

	var list []bank.RuleDTO
	ana.Do("GET", ana.WPath("/rules"), nil, 200, &list)
	if len(list) != 2 {
		t.Fatalf("rules: %+v", list)
	}
	ana.Do("DELETE", ana.WPath("/rules/%d", r.ID), nil, 204, nil)
	ana.Do("DELETE", ana.WPath("/rules/%d", r.ID), nil, 404, nil)
}

func TestCreatingARuleReclassifiesPendingLines(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	apitest.ImportFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)
	if findByDescription(t, pending(t, ana), "UBER").SuggestedCategoryID != nil {
		t.Fatal("no rule yet, so no suggestion")
	}

	transport := categoryByName(t, ana, "expense", "Transporte")
	var res struct {
		Rule    bank.RuleDTO `json:"rule"`
		Updated int          `json:"updated"`
	}
	ana.Do("POST", ana.WPath("/rules"), map[string]any{"pattern": "uber", "categoryId": transport}, 201, &res)
	if res.Updated != 1 || res.Rule.Pattern != "uber" {
		t.Fatalf("response: %+v", res)
	}
	if got := findByDescription(t, pending(t, ana), "UBER").SuggestedCategoryID; got == nil || *got != transport {
		t.Fatalf("the pending line was not reclassified: %v", got)
	}
}

func TestDeletingACategoryUsedByARuleRemovesTheRule(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	var c category
	ana.Do("POST", ana.WPath("/categories"), map[string]any{"kind": "expense", "name": "Pets", "icon": "🐶", "color": "#112233", "essentiality": "nonessential"}, 201, &c)
	ana.Do("POST", ana.WPath("/rules"), map[string]any{"pattern": "petshop", "categoryId": c.ID}, 201, nil)
	ana.Do("DELETE", ana.WPath("/categories/%d", c.ID), nil, 204, nil) // unused: really deleted
	var list []bank.RuleDTO
	ana.Do("GET", ana.WPath("/rules"), nil, 200, &list)
	if len(list) != 0 {
		t.Fatalf("a rule pointing at a deleted category survived: %+v", list)
	}
}

// ---- the inbox as a whole ----
