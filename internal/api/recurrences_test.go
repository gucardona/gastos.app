package api

import (
	"testing"
	"time"
)

// monthsAgoFirst returns the 1st of the month n months ago, in the app's time zone.
func monthsAgoFirst(t *testing.T, app *testApp, n int) string {
	t.Helper()
	now := time.Now().In(app.s.cfg.Location)
	return time.Date(now.Year(), now.Month()-time.Month(n), 1, 0, 0, 0, 0, time.UTC).Format(dateLayout)
}

func listRange(c *client, from string) []transaction {
	c.app.t.Helper()
	var list []transaction
	to := mustDate(from).AddDate(0, 0, 365).Format(dateLayout)
	c.do("GET", c.wpath("/transactions?from=%s&to=%s", from, to), nil, 200, &list)
	return list
}

func TestRecurrenceMaterializesImmediately(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	start := monthsAgoFirst(t, app, 2)

	var rec recurrenceDTO
	ana.do("POST", ana.wpath("/recurrences"), map[string]any{
		"kind": "expense", "amountCents": 9990, "description": "Streaming", "categoryId": firstCategory(t, ana, "expense"),
		"dayOfMonth": 1, "startDate": start,
	}, 201, &rec)
	if !rec.Enabled || rec.NextDate == nil {
		t.Fatalf("recurrence: %+v", rec)
	}
	list := listRange(ana, start)
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
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	start := monthsAgoFirst(t, app, 1)
	cat := firstCategory(t, ana, "expense")
	body := func(amount int) map[string]any {
		return map[string]any{"kind": "expense", "amountCents": amount, "description": "Aluguel", "categoryId": cat,
			"dayOfMonth": 1, "startDate": start}
	}
	var rec recurrenceDTO
	ana.do("POST", ana.wpath("/recurrences"), body(100000), 201, &rec)
	ana.do("PUT", ana.wpath("/recurrences/%d", rec.ID), body(120000), 200, &rec)
	if rec.AmountCents != 120000 {
		t.Fatalf("%+v", rec)
	}
	for _, tx := range listRange(ana, start) {
		if tx.AmountCents != 100000 {
			t.Fatalf("an old transaction changed: %+v", tx)
		}
	}
	ana.do("DELETE", ana.wpath("/recurrences/%d", rec.ID), nil, 204, nil)
	list := listRange(ana, start)
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
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	start := monthsAgoFirst(t, app, 1)
	var rec recurrenceDTO
	ana.do("POST", ana.wpath("/recurrences"), map[string]any{
		"kind": "income", "amountCents": 500000, "description": "Salário", "categoryId": firstCategory(t, ana, "income"),
		"dayOfMonth": 1, "startDate": start, "enabled": false,
	}, 201, &rec)
	if rec.Enabled || rec.NextDate != nil {
		t.Fatalf("%+v", rec)
	}
	if n := len(listRange(ana, start)); n != 0 {
		t.Fatalf("%d transactions, want 0", n)
	}
}

func TestRecurrenceSettlement(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")
	start := monthsAgoFirst(t, app, 1)
	body := map[string]any{"kind": "settlement", "amountCents": 30000, "paidByUserId": bia.user.ID, "toUserId": ana.user.ID,
		"dayOfMonth": 1, "startDate": start}

	ana.do("POST", ana.wpath("/recurrences"), body, 400, nil) // splitting disabled
	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	ana.do("POST", ana.wpath("/recurrences"), body, 201, nil)

	var list []settlementDTO
	to := mustDate(start).AddDate(0, 0, 365).Format(dateLayout)
	ana.do("GET", ana.wpath("/settlements?from=%s&to=%s", start, to), nil, 200, &list)
	if len(list) != 2 || list[0].FromUserID != bia.user.ID || list[0].ToUserID != ana.user.ID {
		t.Fatalf("acertos: %+v", list)
	}
}

func TestRecurrenceValidation(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	cat := firstCategory(t, ana, "expense")
	var res struct {
		Error apiError `json:"error"`
	}
	ana.do("POST", ana.wpath("/recurrences"), map[string]any{
		"kind": "expense", "amountCents": 100, "categoryId": cat, "dayOfMonth": 32, "startDate": "2026-01-01", "endDate": "2025-01-01",
	}, 400, &res)
	for _, f := range []string{"dayOfMonth", "endDate"} {
		if res.Error.Fields[f] == "" {
			t.Errorf("faltou erro em %s: %+v", f, res.Error.Fields)
		}
	}
	ana.do("POST", ana.wpath("/recurrences"), map[string]any{"kind": "settlement", "amountCents": 100, "dayOfMonth": 1, "startDate": "2026-01-01"}, 400, nil)
	ana.do("POST", ana.wpath("/recurrences"), map[string]any{"kind": "expense", "amountCents": 100, "dayOfMonth": 1, "startDate": "2026-01-01"}, 400, nil)
}

func TestRecurrenceReaderCannotWrite(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "reader")
	bia.do("GET", bia.wpath("/recurrences"), nil, 200, nil)
	bia.do("POST", bia.wpath("/recurrences"), map[string]any{
		"kind": "expense", "amountCents": 100, "categoryId": firstCategory(t, ana, "expense"), "dayOfMonth": 1, "startDate": "2026-01-01",
	}, 403, nil)
}
