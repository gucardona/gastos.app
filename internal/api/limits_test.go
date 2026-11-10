package api

import (
	"testing"
	"time"

	"gastos/internal/money"
)

// fieldError posts body and returns the validation message of field (the request must answer 400).
func fieldError(c *client, method, path string, body map[string]any, field string) string {
	c.app.t.Helper()
	var res struct {
		Error apiError `json:"error"`
	}
	c.do(method, path, body, 400, &res)
	return res.Error.Fields[field]
}

func TestTransactionDatesAndAmountsAreBounded(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	tx := func(date string, amount int64) map[string]any {
		return map[string]any{"kind": "expense", "amountCents": amount, "categoryId": firstCategory(t, ana, "expense"), "date": date}
	}
	for _, d := range []string{"0202-01-01", "1999-12-31", "2101-01-01"} {
		if got := fieldError(ana, "POST", ana.wpath("/transactions"), tx(d, 100), "date"); got != "data fora do intervalo aceito" {
			t.Errorf("date %s: %q", d, got)
		}
	}
	if got := fieldError(ana, "POST", ana.wpath("/transactions"), tx("2026-10-01", money.MaxCents+1), "amountCents"); got == "" {
		t.Error("an amount above R$ 1 billion was accepted")
	}
	ana.do("POST", ana.wpath("/transactions"), tx("2000-01-01", money.MaxCents), 201, nil)
	ana.do("POST", ana.wpath("/transactions"), tx("2100-12-31", 100), 201, nil)
}

func TestSettlementDatesAndAmountsAreBounded(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	shareWallet(ana, bia, "reader")
	st := func(date string, amount int64) map[string]any {
		return map[string]any{"fromUserId": bia.user.ID, "toUserId": ana.user.ID, "amountCents": amount, "date": date}
	}
	if got := fieldError(ana, "POST", ana.wpath("/settlements"), st("0202-01-01", 100), "date"); got != "data fora do intervalo aceito" {
		t.Errorf("date: %q", got)
	}
	if got := fieldError(ana, "POST", ana.wpath("/settlements"), st("2026-10-01", money.MaxCents+1), "amountCents"); got == "" {
		t.Error("an amount above R$ 1 billion was accepted")
	}
	ana.do("POST", ana.wpath("/settlements"), st("2026-10-01", money.MaxCents), 201, nil)
}

func TestRecurrenceDatesAndAmountsAreBounded(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	shareWallet(ana, bia, "editor")
	rec := func(start string, amount int64) map[string]any {
		return map[string]any{"kind": "expense", "amountCents": amount, "categoryId": firstCategory(t, ana, "expense"),
			"dayOfMonth": 1, "startDate": start}
	}
	path := ana.wpath("/recurrences")

	if got := fieldError(ana, "POST", path, rec("0202-01-01", 100), "startDate"); got != "data fora do intervalo aceito" {
		t.Errorf("start 0202: %q", got)
	}
	tooOld := monthsAgoFirst(t, app, 25)
	if got := fieldError(ana, "POST", path, rec(tooOld, 100), "startDate"); got == "" {
		t.Errorf("a start 25 months ago was accepted")
	}
	end := rec(monthsAgoFirst(t, app, 1), 100)
	end["endDate"] = "2101-01-01"
	if got := fieldError(ana, "POST", path, end, "endDate"); got != "data fora do intervalo aceito" {
		t.Errorf("end 2101: %q", got)
	}
	if got := fieldError(ana, "POST", path, rec(monthsAgoFirst(t, app, 1), money.MaxCents+1), "amountCents"); got == "" {
		t.Error("an expense above R$ 1 billion was accepted")
	}
	settle := map[string]any{"kind": "settlement", "amountCents": money.MaxCents + 1, "paidByUserId": bia.user.ID,
		"toUserId": ana.user.ID, "dayOfMonth": 1, "startDate": monthsAgoFirst(t, app, 1)}
	if got := fieldError(ana, "POST", path, settle, "amountCents"); got == "" {
		t.Error("a settlement above R$ 1 billion was accepted")
	}

	var created recurrenceDTO
	limit := time.Now().In(app.s.cfg.Location).AddDate(0, -24, 0).Format(dateLayout) // exactly 24 months ago
	ana.do("POST", path, rec(limit, 100), 201, &created)

	// a recurrence that already started long ago (stored before this rule) can still be edited while its start stays
	old := time.Now().In(app.s.cfg.Location).AddDate(-3, 0, 0).Format(dateLayout)
	if _, err := app.s.db.Exec(`UPDATE recurrences SET start_date = ? WHERE id = ?`, old, created.ID); err != nil {
		t.Fatal(err)
	}
	ana.do("PUT", path+"/"+itoa(created.ID), rec(old, 200), 200, nil)
	if got := fieldError(ana, "PUT", path+"/"+itoa(created.ID), rec(tooOld, 200), "startDate"); got == "" {
		t.Error("moving the start 25 months back was accepted")
	}
}
