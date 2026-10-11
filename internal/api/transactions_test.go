package api

import "testing"

func TestTransactionCRUD(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	cat := firstCategory(t, ana, "expense")

	var created transaction
	ana.do("POST", ana.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 12990, "description": "Mercado",
		"categoryId": cat, "date": "2026-10-09",
	}, 201, &created)
	if created.PaidByUserID != ana.user.ID || created.CreatedBy != ana.user.ID || len(created.Splits) != 0 {
		t.Fatalf("criado: %+v", created)
	}

	var list []transaction
	ana.do("GET", ana.wpath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 1 || list[0].AmountCents != 12990 {
		t.Fatalf("lista: %+v", list)
	}

	ana.do("PUT", ana.wpath("/transactions/%d", created.ID), map[string]any{
		"kind": "expense", "amountCents": 5000, "description": "Feira",
		"categoryId": cat, "date": "2026-10-08",
	}, 200, &created)
	if created.AmountCents != 5000 || created.Description != "Feira" {
		t.Fatalf("atualizado: %+v", created)
	}

	ana.do("DELETE", ana.wpath("/transactions/%d", created.ID), nil, 204, nil)
	ana.do("DELETE", ana.wpath("/transactions/%d", created.ID), nil, 404, nil)
	ana.do("GET", ana.wpath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 0 {
		t.Fatalf("deveria estar vazia: %+v", list)
	}
}

func TestTransactionListFilters(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	exp, inc := firstCategory(t, ana, "expense"), firstCategory(t, ana, "income")
	post := func(kind string, cat int64, desc, date string) {
		ana.do("POST", ana.wpath("/transactions"), map[string]any{
			"kind": kind, "amountCents": 100, "description": desc, "categoryId": cat, "date": date,
		}, 201, nil)
	}
	post("expense", exp, "Padaria", "2026-10-02")
	post("expense", exp, "Farmácia", "2026-10-03")
	post("income", inc, "Salário", "2026-10-05")
	post("expense", exp, "Fora do mês", "2026-09-30")

	count := func(query string) int {
		var list []transaction
		ana.do("GET", ana.wpath("/transactions?from=2026-10-01&to=2026-10-31%s", query), nil, 200, &list)
		return len(list)
	}
	if n := count(""); n != 3 {
		t.Errorf("sem filtro: %d, want 3", n)
	}
	if n := count("&kind=income"); n != 1 {
		t.Errorf("kind=income: %d, want 1", n)
	}
	if n := count("&q=pad"); n != 1 {
		t.Errorf("q=pad: %d, want 1", n)
	}
}

func TestTransactionValidation(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	incomeCat := firstCategory(t, ana, "income")
	var res struct {
		Error apiError `json:"error"`
	}
	ana.do("POST", ana.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 0, "description": "", "categoryId": incomeCat, "date": "2026-02-30",
	}, 400, &res)
	for _, f := range []string{"amountCents", "description", "categoryId", "date"} {
		if res.Error.Fields[f] == "" {
			t.Errorf("faltou erro em %s: %+v", f, res.Error.Fields)
		}
	}
	ana.do("GET", ana.wpath("/transactions"), nil, 400, nil) // from/to obrigatórios
	ana.do("GET", ana.wpath("/transactions?from=2020-01-01&to=2026-01-01"), nil, 400, nil)
}

func TestEditorKeepsCreatorAndPayer(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")
	cat := firstCategory(t, ana, "expense")

	var tx transaction
	ana.do("POST", ana.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "Luz", "categoryId": cat, "date": "2026-10-01",
	}, 201, &tx)
	bia.do("PUT", bia.wpath("/transactions/%d", tx.ID), map[string]any{
		"kind": "expense", "amountCents": 1100, "description": "Luz", "categoryId": cat, "date": "2026-10-01",
		"paidByUserId": ana.user.ID,
	}, 200, &tx)
	if tx.CreatedBy != ana.user.ID || tx.PaidByUserID != ana.user.ID {
		t.Fatalf("edição da Bia mudou autoria/pagador: %+v", tx)
	}
}

func TestReaderCannotWrite(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "reader")
	bia.do("POST", bia.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "x", "categoryId": firstCategory(t, ana, "expense"), "date": "2026-10-01",
	}, 403, nil)
}

func TestSplits(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")
	cat := firstCategory(t, ana, "expense")
	body := func(extra map[string]any) map[string]any {
		b := map[string]any{"kind": "expense", "amountCents": 101, "description": "Pizza", "categoryId": cat, "date": "2026-10-01"}
		for k, v := range extra {
			b[k] = v
		}
		return b
	}

	// divisão desligada: splits proibidos
	ana.do("POST", ana.wpath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": ana.user.ID, "shareBp": 10000}},
	}), 400, nil)

	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, nil)

	// sem splits: igual entre membros com splitsByDefault; 101 centavos → 51 + 50
	var tx transaction
	ana.do("POST", ana.wpath("/transactions"), body(nil), 201, &tx)
	if len(tx.Splits) != 2 || tx.Splits[0].AmountCents != 51 || tx.Splits[1].AmountCents != 50 {
		t.Fatalf("split padrão: %+v", tx.Splits)
	}

	// 70/30 explícito
	ana.do("POST", ana.wpath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": ana.user.ID, "shareBp": 7000}, {"userId": bia.user.ID, "shareBp": 3000}},
	}), 201, &tx)
	if tx.Splits[0].AmountCents+tx.Splits[1].AmountCents != 101 {
		t.Fatalf("splits não somam o total: %+v", tx.Splits)
	}

	// o GET devolve os mesmos splits
	var list []transaction
	ana.do("GET", ana.wpath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 2 || len(list[0].Splits) != 2 {
		t.Fatalf("lista: %+v", list)
	}

	// soma diferente de 10000
	ana.do("POST", ana.wpath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": ana.user.ID, "shareBp": 6000}, {"userId": bia.user.ID, "shareBp": 3000}},
	}), 400, nil)
	// não membro
	ana.do("POST", ana.wpath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": 9999, "shareBp": 10000}},
	}), 400, nil)
	// repetido
	ana.do("POST", ana.wpath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": ana.user.ID, "shareBp": 5000}, {"userId": ana.user.ID, "shareBp": 5000}},
	}), 400, nil)
}
