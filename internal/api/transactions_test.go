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
		t.Fatalf("created: %+v", created)
	}

	var list []transaction
	ana.do("GET", ana.wpath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 1 || list[0].AmountCents != 12990 {
		t.Fatalf("list: %+v", list)
	}

	ana.do("PUT", ana.wpath("/transactions/%d", created.ID), map[string]any{
		"kind": "expense", "amountCents": 5000, "description": "Feira",
		"categoryId": cat, "date": "2026-10-08",
	}, 200, &created)
	if created.AmountCents != 5000 || created.Description != "Feira" {
		t.Fatalf("updated: %+v", created)
	}

	ana.do("DELETE", ana.wpath("/transactions/%d", created.ID), nil, 204, nil)
	ana.do("DELETE", ana.wpath("/transactions/%d", created.ID), nil, 404, nil)
	ana.do("GET", ana.wpath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 0 {
		t.Fatalf("should be empty: %+v", list)
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
		t.Errorf("no filter: %d, want 3", n)
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
			t.Errorf("missing error on %s: %+v", f, res.Error.Fields)
		}
	}
	ana.do("GET", ana.wpath("/transactions"), nil, 400, nil) // from/to are required
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
		t.Fatalf("Bia's edit changed the author/payer: %+v", tx)
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

	// splitting disabled: splits are not allowed
	ana.do("POST", ana.wpath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": ana.user.ID, "shareBp": 10000}},
	}), 400, nil)

	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, nil)

	// no splits: equal among members with splitsByDefault; 101 cents → 51 + 50
	var tx transaction
	ana.do("POST", ana.wpath("/transactions"), body(nil), 201, &tx)
	if len(tx.Splits) != 2 || tx.Splits[0].AmountCents != 51 || tx.Splits[1].AmountCents != 50 {
		t.Fatalf("default split: %+v", tx.Splits)
	}

	// explicit 70/30
	ana.do("POST", ana.wpath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": ana.user.ID, "shareBp": 7000}, {"userId": bia.user.ID, "shareBp": 3000}},
	}), 201, &tx)
	if tx.Splits[0].AmountCents+tx.Splits[1].AmountCents != 101 {
		t.Fatalf("splits do not add up to the total: %+v", tx.Splits)
	}

	// o GET devolve os mesmos splits
	var list []transaction
	ana.do("GET", ana.wpath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 2 || len(list[0].Splits) != 2 {
		t.Fatalf("list: %+v", list)
	}

	// shares that do not add up to 10000
	ana.do("POST", ana.wpath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": ana.user.ID, "shareBp": 6000}, {"userId": bia.user.ID, "shareBp": 3000}},
	}), 400, nil)
	// not a member
	ana.do("POST", ana.wpath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": 9999, "shareBp": 10000}},
	}), 400, nil)
	// repetido
	ana.do("POST", ana.wpath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": ana.user.ID, "shareBp": 5000}, {"userId": ana.user.ID, "shareBp": 5000}},
	}), 400, nil)
}

func TestTransactionSourceIsReported(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	cat := firstCategory(t, ana, "expense")

	var manual transaction
	ana.do("POST", ana.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 100, "description": "typed", "categoryId": cat, "date": "2026-10-01",
	}, 201, &manual)
	if manual.Source != "manual" {
		t.Fatalf("source of a typed transaction: %q, want manual", manual.Source)
	}
	// an edit keeps where it came from
	ana.do("PUT", ana.wpath("/transactions/%d", manual.ID), map[string]any{
		"kind": "expense", "amountCents": 200, "description": "typed", "categoryId": cat, "date": "2026-10-01",
	}, 200, &manual)
	if manual.Source != "manual" {
		t.Fatalf("source after an edit: %q", manual.Source)
	}

	start := monthsAgoFirst(t, app, 1)
	ana.do("POST", ana.wpath("/recurrences"), map[string]any{
		"kind": "expense", "amountCents": 5000, "description": "Gym", "categoryId": cat, "dayOfMonth": 1, "startDate": start,
	}, 201, nil)
	for _, tx := range listRange(ana, start) {
		if tx.RecurrenceID != nil && tx.Source != "recurring" {
			t.Fatalf("a transaction generated by a recurrence has source %q, want recurring", tx.Source)
		}
	}
}
