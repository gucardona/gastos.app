package ledger_test

import (
	"strings"
	"testing"

	"gastos/internal/api/apitest"
	"gastos/internal/api/httpx"
	"gastos/internal/api/ledger"
	"gastos/internal/money"
)

func TestTransactionCRUD(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	cat := apitest.FirstCategory(t, ana, "expense")

	var created ledger.Transaction
	ana.Do("POST", ana.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 12990, "description": "Mercado",
		"categoryId": cat, "date": "2026-10-09",
	}, 201, &created)
	if created.PaidByUserID != ana.User.ID || created.CreatedBy != ana.User.ID || len(created.Splits) != 0 {
		t.Fatalf("created: %+v", created)
	}

	var list []ledger.Transaction
	ana.Do("GET", ana.WPath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 1 || list[0].AmountCents != 12990 {
		t.Fatalf("list: %+v", list)
	}

	ana.Do("PUT", ana.WPath("/transactions/%d", created.ID), map[string]any{
		"kind": "expense", "amountCents": 5000, "description": "Feira",
		"categoryId": cat, "date": "2026-10-08",
	}, 200, &created)
	if created.AmountCents != 5000 || created.Description != "Feira" {
		t.Fatalf("updated: %+v", created)
	}

	ana.Do("DELETE", ana.WPath("/transactions/%d", created.ID), nil, 204, nil)
	ana.Do("DELETE", ana.WPath("/transactions/%d", created.ID), nil, 404, nil)
	ana.Do("GET", ana.WPath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 0 {
		t.Fatalf("should be empty: %+v", list)
	}
}

func TestTransactionListFilters(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	exp, inc := apitest.FirstCategory(t, ana, "expense"), apitest.FirstCategory(t, ana, "income")
	post := func(kind string, cat int64, desc, date string) {
		ana.Do("POST", ana.WPath("/transactions"), map[string]any{
			"kind": kind, "amountCents": 100, "description": desc, "categoryId": cat, "date": date,
		}, 201, nil)
	}
	post("expense", exp, "Padaria", "2026-10-02")
	post("expense", exp, "Farmácia", "2026-10-03")
	post("income", inc, "Salário", "2026-10-05")
	post("expense", exp, "Fora do mês", "2026-09-30")

	count := func(query string) int {
		var list []ledger.Transaction
		ana.Do("GET", ana.WPath("/transactions?from=2026-10-01&to=2026-10-31%s", query), nil, 200, &list)
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
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	incomeCat := apitest.FirstCategory(t, ana, "income")
	var res struct {
		Error httpx.APIError `json:"error"`
	}
	ana.Do("POST", ana.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 0, "description": "", "categoryId": incomeCat, "date": "2026-02-30",
	}, 400, &res)
	for _, f := range []string{"amountCents", "categoryId", "date"} {
		if res.Error.Fields[f] == "" {
			t.Errorf("missing error on %s: %+v", f, res.Error.Fields)
		}
	}
	if res.Error.Fields["description"] != "" {
		t.Errorf("an empty description is allowed, got an error: %+v", res.Error.Fields)
	}
	ana.Do("GET", ana.WPath("/transactions"), nil, 400, nil) // from/to are required
	ana.Do("GET", ana.WPath("/transactions?from=2020-01-01&to=2026-01-01"), nil, 400, nil)
}

// TestInvoiceDueNotAccepted pins that a request body cannot set invoiceDue:
// the server derives it from the card, so the key is an unknown field.
func TestInvoiceDueNotAccepted(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	cat := apitest.FirstCategory(t, ana, "expense")
	var res struct {
		Error httpx.APIError `json:"error"`
	}
	ana.Do("POST", ana.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "Luz", "categoryId": cat,
		"date": "2026-10-01", "invoiceDue": "2026-12-10",
	}, 400, &res)
	if res.Error.Code != "validation" {
		t.Fatalf("code = %q, want validation", res.Error.Code)
	}
}

func TestEditorKeepsCreatorAndPayer(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	cat := apitest.FirstCategory(t, ana, "expense")

	var tx ledger.Transaction
	ana.Do("POST", ana.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "Luz", "categoryId": cat, "date": "2026-10-01",
	}, 201, &tx)
	bia.Do("PUT", bia.WPath("/transactions/%d", tx.ID), map[string]any{
		"kind": "expense", "amountCents": 1100, "description": "Luz", "categoryId": cat, "date": "2026-10-01",
		"paidByUserId": ana.User.ID,
	}, 200, &tx)
	if tx.CreatedBy != ana.User.ID || tx.PaidByUserID != ana.User.ID {
		t.Fatalf("Bia's edit changed the author/payer: %+v", tx)
	}
}

func TestReaderCannotWrite(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "reader")
	bia.Do("POST", bia.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "x", "categoryId": apitest.FirstCategory(t, ana, "expense"), "date": "2026-10-01",
	}, 403, nil)
}

func TestSplits(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	cat := apitest.FirstCategory(t, ana, "expense")
	body := func(extra map[string]any) map[string]any {
		b := map[string]any{"kind": "expense", "amountCents": 101, "description": "Pizza", "categoryId": cat, "date": "2026-10-01", "allowDuplicate": true}
		for k, v := range extra {
			b[k] = v
		}
		return b
	}

	// splitting disabled: splits are not allowed
	ana.Do("POST", ana.WPath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": ana.User.ID, "shareBp": 10000}},
	}), 400, nil)

	ana.Do("PATCH", ana.WPath(""), map[string]any{"splittingEnabled": true}, 200, nil)

	// no splits: equal among members with splitsByDefault; 101 cents → 51 + 50
	var tx ledger.Transaction
	ana.Do("POST", ana.WPath("/transactions"), body(nil), 201, &tx)
	if len(tx.Splits) != 2 || tx.Splits[0].AmountCents != 51 || tx.Splits[1].AmountCents != 50 {
		t.Fatalf("default split: %+v", tx.Splits)
	}

	// explicit 70/30
	ana.Do("POST", ana.WPath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": ana.User.ID, "shareBp": 7000}, {"userId": bia.User.ID, "shareBp": 3000}},
	}), 201, &tx)
	if tx.Splits[0].AmountCents+tx.Splits[1].AmountCents != 101 {
		t.Fatalf("splits do not add up to the total: %+v", tx.Splits)
	}

	// o GET devolve os mesmos splits
	var list []ledger.Transaction
	ana.Do("GET", ana.WPath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 2 || len(list[0].Splits) != 2 {
		t.Fatalf("list: %+v", list)
	}

	// shares that do not add up to 10000
	ana.Do("POST", ana.WPath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": ana.User.ID, "shareBp": 6000}, {"userId": bia.User.ID, "shareBp": 3000}},
	}), 400, nil)
	// not a member
	ana.Do("POST", ana.WPath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": 9999, "shareBp": 10000}},
	}), 400, nil)
	// repetido
	ana.Do("POST", ana.WPath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": ana.User.ID, "shareBp": 5000}, {"userId": ana.User.ID, "shareBp": 5000}},
	}), 400, nil)
}

func TestTransactionSourceIsReported(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	cat := apitest.FirstCategory(t, ana, "expense")

	var manual ledger.Transaction
	ana.Do("POST", ana.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 100, "description": "typed", "categoryId": cat, "date": "2026-10-01",
	}, 201, &manual)
	if manual.Source != "manual" {
		t.Fatalf("source of a typed transaction: %q, want manual", manual.Source)
	}
	// an edit keeps where it came from
	ana.Do("PUT", ana.WPath("/transactions/%d", manual.ID), map[string]any{
		"kind": "expense", "amountCents": 200, "description": "typed", "categoryId": cat, "date": "2026-10-01",
	}, 200, &manual)
	if manual.Source != "manual" {
		t.Fatalf("source after an edit: %q", manual.Source)
	}

	start := apitest.MonthsAgoFirst(t, app, 1)
	ana.Do("POST", ana.WPath("/recurrences"), map[string]any{
		"kind": "expense", "amountCents": 5000, "description": "Gym", "categoryId": cat, "dayOfMonth": 1, "startDate": start,
	}, 201, nil)
	for _, tx := range apitest.ListRange(ana, start) {
		if tx.RecurrenceID != nil && tx.Source != "recurring" {
			t.Fatalf("a transaction generated by a recurrence has source %q, want recurring", tx.Source)
		}
	}
}

func TestDescriptionIsOptional(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	cat := apitest.FirstCategory(t, ana, "expense")
	body := func(desc string) map[string]any {
		return map[string]any{"kind": "expense", "amountCents": 100, "description": desc, "categoryId": cat, "date": "2026-10-01", "allowDuplicate": true}
	}
	var created struct {
		ID          int64  `json:"id"`
		Description string `json:"description"`
	}
	ana.Do("POST", ana.WPath("/transactions"), body(""), 201, &created)
	if created.Description != "" {
		t.Fatalf("description should stay empty, got %q", created.Description)
	}
	ana.Do("POST", ana.WPath("/transactions"), body("   "), 201, &created) // only spaces counts as empty
	if created.Description != "" {
		t.Fatalf("spaces should be trimmed to empty, got %q", created.Description)
	}
	ana.Do("POST", ana.WPath("/transactions"), body(strings.Repeat("x", 201)), 400, nil) // the cap stays
	// clearing the description of an existing transaction
	ana.Do("POST", ana.WPath("/transactions"), body("Feira"), 201, &created)
	var edited struct {
		Description string `json:"description"`
	}
	ana.Do("PUT", ana.WPath("/transactions/%d", created.ID), body(""), 200, &edited)
	if edited.Description != "" {
		t.Fatalf("editing to empty: %q", edited.Description)
	}
}

func fieldError(c *apitest.Client, method, path string, body map[string]any, field string) string {
	c.App().T().Helper()
	var res struct {
		Error httpx.APIError `json:"error"`
	}
	c.Do(method, path, body, 400, &res)
	return res.Error.Fields[field]
}

func TestTransactionDatesAndAmountsAreBounded(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	tx := func(date string, amount int64) map[string]any {
		return map[string]any{"kind": "expense", "amountCents": amount, "categoryId": apitest.FirstCategory(t, ana, "expense"), "date": date}
	}
	for _, d := range []string{"0202-01-01", "1999-12-31", "2101-01-01"} {
		if got := fieldError(ana, "POST", ana.WPath("/transactions"), tx(d, 100), "date"); got != "data fora do intervalo aceito" {
			t.Errorf("date %s: %q", d, got)
		}
	}
	if got := fieldError(ana, "POST", ana.WPath("/transactions"), tx("2026-10-01", money.MaxCents+1), "amountCents"); got == "" {
		t.Error("an amount above R$ 1 billion was accepted")
	}
	ana.Do("POST", ana.WPath("/transactions"), tx("2000-01-01", money.MaxCents), 201, nil)
	ana.Do("POST", ana.WPath("/transactions"), tx("2100-12-31", 100), 201, nil)
}
