package api

import "testing"

func TestSettlementCRUD(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")
	body := func(amount int) map[string]any {
		return map[string]any{"fromUserId": bia.user.ID, "toUserId": ana.user.ID, "amountCents": amount, "date": "2026-10-05", "note": "Pix"}
	}

	ana.do("POST", ana.wpath("/settlements"), body(5000), 400, nil) // splitting disabled
	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, nil)

	var s settlementDTO
	ana.do("POST", ana.wpath("/settlements"), body(5000), 201, &s)
	if s.CreatedBy != ana.user.ID || s.AmountCents != 5000 {
		t.Fatalf("%+v", s)
	}
	ana.do("PUT", ana.wpath("/settlements/%d", s.ID), body(7000), 200, &s)
	if s.AmountCents != 7000 {
		t.Fatalf("%+v", s)
	}

	var list []settlementDTO
	ana.do("GET", ana.wpath("/settlements?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 1 {
		t.Fatalf("%+v", list)
	}
	ana.do("DELETE", ana.wpath("/settlements/%d", s.ID), nil, 204, nil)
	ana.do("DELETE", ana.wpath("/settlements/%d", s.ID), nil, 404, nil)
}

func TestSettlementValidation(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, nil)

	post := func(from, to int64, amount int, want int) {
		t.Helper()
		ana.do("POST", ana.wpath("/settlements"), map[string]any{
			"fromUserId": from, "toUserId": to, "amountCents": amount, "date": "2026-10-05",
		}, want, nil)
	}
	post(ana.user.ID, ana.user.ID, 100, 400) // same person
	post(ana.user.ID, bia.user.ID, 100, 400) // Bia is not a member
	shareWallet(ana, bia, "reader")
	post(bia.user.ID, ana.user.ID, 0, 400) // zero amount
	post(bia.user.ID, ana.user.ID, 100, 201)
}
