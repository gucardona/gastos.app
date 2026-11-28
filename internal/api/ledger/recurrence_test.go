package ledger_test

import (
	"testing"
	"time"

	"gastos/internal/api/apitest"
	"gastos/internal/api/httpx"
	"gastos/internal/api/ledger"
	"gastos/internal/money"
)

func TestRecurrenceMaterializesImmediately(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	start := apitest.MonthsAgoFirst(t, app, 2)

	var rec ledger.RecurrenceDTO
	ana.Do("POST", ana.WPath("/recurrences"), map[string]any{
		"kind": "expense", "amountCents": 9990, "description": "Streaming", "categoryId": apitest.FirstCategory(t, ana, "expense"),
		"dayOfMonth": 1, "startDate": start,
	}, 201, &rec)
	if !rec.Enabled || rec.NextDate == nil {
		t.Fatalf("recurrence: %+v", rec)
	}
	list := apitest.ListRange(ana, start)
	if len(list) != 3 {
		t.Fatalf("%d transactions, want 3: %+v", len(list), list)
	}
	for _, tx := range list {
		if tx.RecurrenceID == nil || *tx.RecurrenceID != rec.ID || tx.AmountCents != 9990 {
			t.Fatalf("unexpected transaction: %+v", tx)
		}
	}
}

func TestRecurrenceUpdateKeepsOldTransactionsAndDeleteDetaches(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	start := apitest.MonthsAgoFirst(t, app, 1)
	cat := apitest.FirstCategory(t, ana, "expense")
	body := func(amount int) map[string]any {
		return map[string]any{"kind": "expense", "amountCents": amount, "description": "Aluguel", "categoryId": cat,
			"dayOfMonth": 1, "startDate": start}
	}
	var rec ledger.RecurrenceDTO
	ana.Do("POST", ana.WPath("/recurrences"), body(100000), 201, &rec)
	ana.Do("PUT", ana.WPath("/recurrences/%d", rec.ID), body(120000), 200, &rec)
	if rec.AmountCents != 120000 {
		t.Fatalf("%+v", rec)
	}
	for _, tx := range apitest.ListRange(ana, start) {
		if tx.AmountCents != 100000 {
			t.Fatalf("an old transaction changed: %+v", tx)
		}
	}
	ana.Do("DELETE", ana.WPath("/recurrences/%d", rec.ID), nil, 204, nil)
	list := apitest.ListRange(ana, start)
	if len(list) != 2 {
		t.Fatalf("%d transactions, want 2", len(list))
	}
	for _, tx := range list {
		if tx.RecurrenceID != nil {
			t.Fatalf("should have been detached: %+v", tx)
		}
	}
}

func TestRecurrenceDisabledDoesNotMaterialize(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	start := apitest.MonthsAgoFirst(t, app, 1)
	var rec ledger.RecurrenceDTO
	ana.Do("POST", ana.WPath("/recurrences"), map[string]any{
		"kind": "income", "amountCents": 500000, "description": "Salário", "categoryId": apitest.FirstCategory(t, ana, "income"),
		"dayOfMonth": 1, "startDate": start, "enabled": false,
	}, 201, &rec)
	if rec.Enabled || rec.NextDate != nil {
		t.Fatalf("%+v", rec)
	}
	if n := len(apitest.ListRange(ana, start)); n != 0 {
		t.Fatalf("%d transactions, want 0", n)
	}
}

func TestRecurrenceSettlement(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	start := apitest.MonthsAgoFirst(t, app, 1)
	body := map[string]any{"kind": "settlement", "amountCents": 30000, "paidByUserId": bia.User.ID, "toUserId": ana.User.ID,
		"dayOfMonth": 1, "startDate": start}

	ana.Do("POST", ana.WPath("/recurrences"), body, 400, nil) // splitting disabled
	ana.Do("PATCH", ana.WPath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	ana.Do("POST", ana.WPath("/recurrences"), body, 201, nil)

	var list []ledger.SettlementDTO
	to := ledger.MustDate(start).AddDate(0, 0, 365).Format(httpx.DateLayout)
	ana.Do("GET", ana.WPath("/settlements?from=%s&to=%s", start, to), nil, 200, &list)
	if len(list) != 2 || list[0].FromUserID != bia.User.ID || list[0].ToUserID != ana.User.ID {
		t.Fatalf("settlements: %+v", list)
	}
}

func TestRecurrenceValidation(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	cat := apitest.FirstCategory(t, ana, "expense")
	var res struct {
		Error httpx.APIError `json:"error"`
	}
	ana.Do("POST", ana.WPath("/recurrences"), map[string]any{
		"kind": "expense", "amountCents": 100, "categoryId": cat, "dayOfMonth": 32, "startDate": "2026-01-01", "endDate": "2025-01-01",
	}, 400, &res)
	for _, f := range []string{"dayOfMonth", "endDate"} {
		if res.Error.Fields[f] == "" {
			t.Errorf("missing error on %s: %+v", f, res.Error.Fields)
		}
	}
	ana.Do("POST", ana.WPath("/recurrences"), map[string]any{"kind": "settlement", "amountCents": 100, "dayOfMonth": 1, "startDate": "2026-01-01"}, 400, nil)
	ana.Do("POST", ana.WPath("/recurrences"), map[string]any{"kind": "expense", "amountCents": 100, "dayOfMonth": 1, "startDate": "2026-01-01"}, 400, nil)
}

func TestRecurrenceReaderCannotWrite(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "reader")
	bia.Do("GET", bia.WPath("/recurrences"), nil, 200, nil)
	bia.Do("POST", bia.WPath("/recurrences"), map[string]any{
		"kind": "expense", "amountCents": 100, "categoryId": apitest.FirstCategory(t, ana, "expense"), "dayOfMonth": 1, "startDate": "2026-01-01",
	}, 403, nil)
}

func TestRecurrenceKeepsSplits(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	ana.Do("PATCH", ana.WPath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	start := apitest.MonthsAgoFirst(t, app, 1)

	var rec ledger.RecurrenceDTO
	ana.Do("POST", ana.WPath("/recurrences"), map[string]any{
		"kind": "expense", "amountCents": 10000, "description": "Internet", "categoryId": apitest.FirstCategory(t, ana, "expense"),
		"dayOfMonth": 1, "startDate": start,
		"splits": []map[string]any{{"userId": ana.User.ID, "shareBp": 7000}, {"userId": bia.User.ID, "shareBp": 3000}},
	}, 201, &rec)
	if len(rec.Splits) != 2 || rec.Splits[0].AmountCents != 7000 || rec.Splits[1].AmountCents != 3000 {
		t.Fatalf("recurrence splits: %+v", rec.Splits)
	}
	// generated transactions inherit the split
	for _, tx := range apitest.ListRange(ana, start) {
		if len(tx.Splits) != 2 || tx.Splits[0].ShareBP != 7000 {
			t.Fatalf("the generated transaction lost its split: %+v", tx)
		}
	}
	// changing the split on the recurrence replaces the parts
	ana.Do("PUT", ana.WPath("/recurrences/%d", rec.ID), map[string]any{
		"kind": "expense", "amountCents": 10000, "description": "Internet", "categoryId": rec.CategoryID,
		"dayOfMonth": 1, "startDate": start,
		"splits": []map[string]any{{"userId": ana.User.ID, "shareBp": 5000}, {"userId": bia.User.ID, "shareBp": 5000}},
	}, 200, &rec)
	if rec.Splits[0].ShareBP != 5000 {
		t.Fatalf("splits after the PUT: %+v", rec.Splits)
	}
}

func TestRecurrenceDatesAndAmountsAreBounded(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	ana.Do("PATCH", ana.WPath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	apitest.ShareWallet(ana, bia, "editor")
	rec := func(start string, amount int64) map[string]any {
		return map[string]any{"kind": "expense", "amountCents": amount, "categoryId": apitest.FirstCategory(t, ana, "expense"),
			"dayOfMonth": 1, "startDate": start}
	}
	path := ana.WPath("/recurrences")

	if got := fieldError(ana, "POST", path, rec("0202-01-01", 100), "startDate"); got != "data fora do intervalo aceito" {
		t.Errorf("start 0202: %q", got)
	}
	tooOld := apitest.MonthsAgoFirst(t, app, 25)
	if got := fieldError(ana, "POST", path, rec(tooOld, 100), "startDate"); got == "" {
		t.Errorf("a start 25 months ago was accepted")
	}
	end := rec(apitest.MonthsAgoFirst(t, app, 1), 100)
	end["endDate"] = "2101-01-01"
	if got := fieldError(ana, "POST", path, end, "endDate"); got != "data fora do intervalo aceito" {
		t.Errorf("end 2101: %q", got)
	}
	if got := fieldError(ana, "POST", path, rec(apitest.MonthsAgoFirst(t, app, 1), money.MaxCents+1), "amountCents"); got == "" {
		t.Error("an expense above R$ 1 billion was accepted")
	}
	settle := map[string]any{"kind": "settlement", "amountCents": money.MaxCents + 1, "paidByUserId": bia.User.ID,
		"toUserId": ana.User.ID, "dayOfMonth": 1, "startDate": apitest.MonthsAgoFirst(t, app, 1)}
	if got := fieldError(ana, "POST", path, settle, "amountCents"); got == "" {
		t.Error("a settlement above R$ 1 billion was accepted")
	}

	var created ledger.RecurrenceDTO
	limit := time.Now().In(app.Server().Cfg.Location).AddDate(0, -24, 0).Format(httpx.DateLayout) // exactly 24 months ago
	ana.Do("POST", path, rec(limit, 100), 201, &created)

	// a recurrence that already started long ago (stored before this rule) can still be edited while its start stays
	old := time.Now().In(app.Server().Cfg.Location).AddDate(-3, 0, 0).Format(httpx.DateLayout)
	if _, err := app.Server().DB.Exec(`UPDATE recurrences SET start_date = ? WHERE id = ?`, old, created.ID); err != nil {
		t.Fatal(err)
	}
	ana.Do("PUT", path+"/"+apitest.Itoa(created.ID), rec(old, 200), 200, nil)
	if got := fieldError(ana, "PUT", path+"/"+apitest.Itoa(created.ID), rec(tooOld, 200), "startDate"); got == "" {
		t.Error("moving the start 25 months back was accepted")
	}
}
