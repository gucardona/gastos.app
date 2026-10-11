package api

import "testing"

func firstCategory(t *testing.T, c *client, kind string) int64 {
	t.Helper()
	var cats []categoryDTO
	c.do("GET", c.wpath("/categories"), nil, 200, &cats)
	for _, cat := range cats {
		if cat.Kind == kind && !cat.Archived {
			return cat.ID
		}
	}
	t.Fatalf("no %s category", kind)
	return 0
}

func TestCategoryCRUD(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")

	var c categoryDTO
	ana.do("POST", ana.wpath("/categories"), map[string]any{
		"kind": "expense", "name": "Pets", "icon": "🐶", "color": "#112233", "essentiality": "nonessential",
	}, 201, &c)
	if c.ID == 0 || c.SortOrder != 15 {
		t.Fatalf("criada: %+v", c)
	}

	ana.do("PUT", ana.wpath("/categories/%d", c.ID), map[string]any{
		"kind": "expense", "name": "Bichos", "icon": "🐱", "color": "#445566", "essentiality": "essential", "sortOrder": 0,
	}, 200, &c)
	if c.Name != "Bichos" || c.SortOrder != 0 || *c.Essentiality != "essential" {
		t.Fatalf("atualizada: %+v", c)
	}

	ana.do("DELETE", ana.wpath("/categories/%d", c.ID), nil, 204, nil)
	ana.do("PUT", ana.wpath("/categories/%d", c.ID), map[string]any{
		"kind": "expense", "name": "X", "icon": "x", "color": "#445566", "essentiality": "essential",
	}, 404, nil)
}

func TestCategoryValidation(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	var res struct {
		Error apiError `json:"error"`
	}
	ana.do("POST", ana.wpath("/categories"), map[string]any{
		"kind": "expense", "name": "Pets", "icon": "🐶", "color": "vermelho", "essentiality": "essential",
	}, 400, &res)
	if res.Error.Fields["color"] == "" {
		t.Errorf("faltou erro de cor: %+v", res.Error)
	}
	ana.do("POST", ana.wpath("/categories"), map[string]any{
		"kind": "income", "name": "Pets", "icon": "🐶", "color": "#112233", "essentiality": "essential",
	}, 400, nil)
	ana.do("POST", ana.wpath("/categories"), map[string]any{
		"kind": "expense", "name": "Pets", "icon": "🐶", "color": "#112233",
	}, 400, nil)
	ana.do("POST", ana.wpath("/categories"), map[string]any{
		"kind": "expense", "name": "Mercado", "icon": "🛒", "color": "#112233", "essentiality": "essential",
	}, 409, nil)
}

func TestCategoryInUseIsArchived(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	cat := firstCategory(t, ana, "expense")
	var tx transaction
	ana.do("POST", ana.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "x", "categoryId": cat, "date": "2026-10-01",
	}, 201, &tx)

	var c categoryDTO
	ana.do("DELETE", ana.wpath("/categories/%d", cat), nil, 200, &c)
	if !c.Archived {
		t.Fatalf("should be archived: %+v", c)
	}
	var list []transaction
	ana.do("GET", ana.wpath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 1 || list[0].CategoryID != cat {
		t.Fatalf("the transaction should keep its category: %+v", list)
	}
	// an archived category does not accept new transactions
	ana.do("POST", ana.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "y", "categoryId": cat, "date": "2026-10-02",
	}, 400, nil)
	// but it can be unarchived
	ana.do("PUT", ana.wpath("/categories/%d", cat), map[string]any{
		"kind": "expense", "name": c.Name, "icon": c.Icon, "color": c.Color, "essentiality": *c.Essentiality, "archived": false,
	}, 200, &c)
	if c.Archived {
		t.Fatal("should have been unarchived")
	}
}

func TestCannotRemoveLastIncomeCategory(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	var cats []categoryDTO
	ana.do("GET", ana.wpath("/categories"), nil, 200, &cats)
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
		ana.do("DELETE", ana.wpath("/categories/%d", id), nil, want, nil)
	}
}

func TestPaymentMethods(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")

	var p paymentMethodDTO
	ana.do("POST", ana.wpath("/payment-methods"), map[string]any{"name": "Cheque", "icon": "🧾"}, 201, &p)
	ana.do("POST", ana.wpath("/payment-methods"), map[string]any{"name": "Cheque", "icon": "🧾"}, 409, nil)
	ana.do("POST", ana.wpath("/payment-methods"), map[string]any{"name": "", "icon": "🧾"}, 400, nil)
	ana.do("PUT", ana.wpath("/payment-methods/%d", p.ID), map[string]any{"name": "Cheque 2", "icon": "🧾"}, 200, &p)
	if p.Name != "Cheque 2" {
		t.Fatalf("%+v", p)
	}

	// em uso: arquiva
	cat := firstCategory(t, ana, "expense")
	ana.do("POST", ana.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "x", "categoryId": cat, "paymentMethodId": p.ID, "date": "2026-10-01",
	}, 201, nil)
	ana.do("DELETE", ana.wpath("/payment-methods/%d", p.ID), nil, 200, &p)
	if !p.Archived {
		t.Fatalf("should be archived: %+v", p)
	}
}
