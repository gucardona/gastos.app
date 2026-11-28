package ledger_test

import (
	"testing"

	"gastos/internal/api/apitest"
	"gastos/internal/api/httpx"
	"gastos/internal/api/ledger"
)

func TestCategoryCRUD(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")

	var c ledger.CategoryDTO
	ana.Do("POST", ana.WPath("/categories"), map[string]any{
		"kind": "expense", "name": "Pets", "icon": "🐶", "color": "#112233", "essentiality": "nonessential",
	}, 201, &c)
	if c.ID == 0 || c.SortOrder != 15 {
		t.Fatalf("created: %+v", c)
	}

	ana.Do("PUT", ana.WPath("/categories/%d", c.ID), map[string]any{
		"kind": "expense", "name": "Bichos", "icon": "🐱", "color": "#445566", "essentiality": "essential", "sortOrder": 0,
	}, 200, &c)
	if c.Name != "Bichos" || c.SortOrder != 0 || *c.Essentiality != "essential" {
		t.Fatalf("updated: %+v", c)
	}

	ana.Do("DELETE", ana.WPath("/categories/%d", c.ID), nil, 204, nil)
	ana.Do("PUT", ana.WPath("/categories/%d", c.ID), map[string]any{
		"kind": "expense", "name": "X", "icon": "x", "color": "#445566", "essentiality": "essential",
	}, 404, nil)
}

func TestCategoryValidation(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	var res struct {
		Error httpx.APIError `json:"error"`
	}
	ana.Do("POST", ana.WPath("/categories"), map[string]any{
		"kind": "expense", "name": "Pets", "icon": "🐶", "color": "vermelho", "essentiality": "essential",
	}, 400, &res)
	if res.Error.Fields["color"] == "" {
		t.Errorf("missing color error: %+v", res.Error)
	}
	ana.Do("POST", ana.WPath("/categories"), map[string]any{
		"kind": "income", "name": "Pets", "icon": "🐶", "color": "#112233", "essentiality": "essential",
	}, 400, nil)
	ana.Do("POST", ana.WPath("/categories"), map[string]any{
		"kind": "expense", "name": "Pets", "icon": "🐶", "color": "#112233",
	}, 400, nil)
	ana.Do("POST", ana.WPath("/categories"), map[string]any{
		"kind": "expense", "name": "Mercado", "icon": "🛒", "color": "#112233", "essentiality": "essential",
	}, 409, nil)
}

func TestCategoryInUseIsArchived(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	cat := apitest.FirstCategory(t, ana, "expense")
	var tx ledger.Transaction
	ana.Do("POST", ana.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "x", "categoryId": cat, "date": "2026-10-01",
	}, 201, &tx)

	var c ledger.CategoryDTO
	ana.Do("DELETE", ana.WPath("/categories/%d", cat), nil, 200, &c)
	if !c.Archived {
		t.Fatalf("should be archived: %+v", c)
	}
	var list []ledger.Transaction
	ana.Do("GET", ana.WPath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 1 || list[0].CategoryID != cat {
		t.Fatalf("the transaction should keep its category: %+v", list)
	}
	// an archived category does not accept new transactions
	ana.Do("POST", ana.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "y", "categoryId": cat, "date": "2026-10-02",
	}, 400, nil)
	// but it can be unarchived
	ana.Do("PUT", ana.WPath("/categories/%d", cat), map[string]any{
		"kind": "expense", "name": c.Name, "icon": c.Icon, "color": c.Color, "essentiality": *c.Essentiality, "archived": false,
	}, 200, &c)
	if c.Archived {
		t.Fatal("should have been unarchived")
	}
}

func TestCannotRemoveLastIncomeCategory(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	var cats []ledger.CategoryDTO
	ana.Do("GET", ana.WPath("/categories"), nil, 200, &cats)
	var incomes []int64
	for _, c := range cats {
		if c.Kind == "income" {
			incomes = append(incomes, c.ID)
		}
	}
	for i, id := range incomes {
		want := 204
		if i == len(incomes)-1 {
			want = 400
		}
		ana.Do("DELETE", ana.WPath("/categories/%d", id), nil, want, nil)
	}
}

func TestPaymentMethods(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")

	var p ledger.PaymentMethodDTO
	ana.Do("POST", ana.WPath("/payment-methods"), map[string]any{"name": "Cheque", "icon": "🧾"}, 201, &p)
	ana.Do("POST", ana.WPath("/payment-methods"), map[string]any{"name": "Cheque", "icon": "🧾"}, 409, nil)
	ana.Do("POST", ana.WPath("/payment-methods"), map[string]any{"name": "", "icon": "🧾"}, 400, nil)
	ana.Do("PUT", ana.WPath("/payment-methods/%d", p.ID), map[string]any{"name": "Cheque 2", "icon": "🧾"}, 200, &p)
	if p.Name != "Cheque 2" {
		t.Fatalf("%+v", p)
	}

	// em uso: arquiva
	cat := apitest.FirstCategory(t, ana, "expense")
	ana.Do("POST", ana.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "x", "categoryId": cat, "paymentMethodId": p.ID, "date": "2026-10-01",
	}, 201, nil)
	ana.Do("DELETE", ana.WPath("/payment-methods/%d", p.ID), nil, 200, &p)
	if !p.Archived {
		t.Fatalf("should be archived: %+v", p)
	}
}
