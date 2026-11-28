package ledger_test

import (
	"testing"

	"gastos/internal/api/apitest"
	"gastos/internal/api/ledger"
	"gastos/internal/money"
)

func TestSettlementCRUD(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	body := func(amount int) map[string]any {
		return map[string]any{"fromUserId": bia.User.ID, "toUserId": ana.User.ID, "amountCents": amount, "date": "2026-10-05", "note": "Pix"}
	}

	ana.Do("POST", ana.WPath("/settlements"), body(5000), 400, nil) // splitting disabled
	ana.Do("PATCH", ana.WPath(""), map[string]any{"splittingEnabled": true}, 200, nil)

	var s ledger.SettlementDTO
	ana.Do("POST", ana.WPath("/settlements"), body(5000), 201, &s)
	if s.CreatedBy != ana.User.ID || s.AmountCents != 5000 {
		t.Fatalf("%+v", s)
	}
	ana.Do("PUT", ana.WPath("/settlements/%d", s.ID), body(7000), 200, &s)
	if s.AmountCents != 7000 {
		t.Fatalf("%+v", s)
	}

	var list []ledger.SettlementDTO
	ana.Do("GET", ana.WPath("/settlements?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 1 {
		t.Fatalf("%+v", list)
	}
	ana.Do("DELETE", ana.WPath("/settlements/%d", s.ID), nil, 204, nil)
	ana.Do("DELETE", ana.WPath("/settlements/%d", s.ID), nil, 404, nil)
}

func TestSettlementValidation(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	ana.Do("PATCH", ana.WPath(""), map[string]any{"splittingEnabled": true}, 200, nil)

	post := func(from, to int64, amount int, want int) {
		t.Helper()
		ana.Do("POST", ana.WPath("/settlements"), map[string]any{
			"fromUserId": from, "toUserId": to, "amountCents": amount, "date": "2026-10-05",
		}, want, nil)
	}
	post(ana.User.ID, ana.User.ID, 100, 400) // same person
	post(ana.User.ID, bia.User.ID, 100, 400) // Bia is not a member
	apitest.ShareWallet(ana, bia, "reader")
	post(bia.User.ID, ana.User.ID, 0, 400) // zero amount
	post(bia.User.ID, ana.User.ID, 100, 201)
}

func TestSettlementDatesAndAmountsAreBounded(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	ana.Do("PATCH", ana.WPath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	apitest.ShareWallet(ana, bia, "reader")
	st := func(date string, amount int64) map[string]any {
		return map[string]any{"fromUserId": bia.User.ID, "toUserId": ana.User.ID, "amountCents": amount, "date": date}
	}
	if got := fieldError(ana, "POST", ana.WPath("/settlements"), st("0202-01-01", 100), "date"); got != "data fora do intervalo aceito" {
		t.Errorf("date: %q", got)
	}
	if got := fieldError(ana, "POST", ana.WPath("/settlements"), st("2026-10-01", money.MaxCents+1), "amountCents"); got == "" {
		t.Error("an amount above R$ 1 billion was accepted")
	}
	ana.Do("POST", ana.WPath("/settlements"), st("2026-10-01", money.MaxCents), 201, nil)
}
