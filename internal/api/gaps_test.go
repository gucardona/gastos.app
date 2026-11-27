package api

import (
	"testing"

	"gastos/internal/api/httpx"
)

// ---- catalog: paths that were left out ----

func TestDeleteUnusedPaymentMethodAndEmptyIcon(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	var p paymentMethodDTO
	ana.do("POST", ana.wpath("/payment-methods"), map[string]any{"name": "Cheque", "icon": "🧾"}, 201, &p)
	ana.do("DELETE", ana.wpath("/payment-methods/%d", p.ID), nil, 204, nil) // unused: really deletes
	ana.do("PUT", ana.wpath("/payment-methods/%d", p.ID), map[string]any{"name": "x", "icon": "x"}, 404, nil)

	var res struct {
		Error httpx.APIError `json:"error"`
	}
	ana.do("POST", ana.wpath("/payment-methods"), map[string]any{"name": "Vale", "icon": ""}, 400, &res)
	if res.Error.Fields["icon"] == "" {
		t.Errorf("missing icon error: %+v", res.Error)
	}
	ana.do("POST", ana.wpath("/payment-methods"), map[string]any{"name": "Vale", "icon": "um ícone muito mais comprido que o limite de bytes"}, 400, nil)
}

// ---- recurrence with splitting ----

func TestRecurrenceKeepsSplits(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")
	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	start := monthsAgoFirst(t, app, 1)

	var rec recurrenceDTO
	ana.do("POST", ana.wpath("/recurrences"), map[string]any{
		"kind": "expense", "amountCents": 10000, "description": "Internet", "categoryId": firstCategory(t, ana, "expense"),
		"dayOfMonth": 1, "startDate": start,
		"splits": []map[string]any{{"userId": ana.user.ID, "shareBp": 7000}, {"userId": bia.user.ID, "shareBp": 3000}},
	}, 201, &rec)
	if len(rec.Splits) != 2 || rec.Splits[0].AmountCents != 7000 || rec.Splits[1].AmountCents != 3000 {
		t.Fatalf("recurrence splits: %+v", rec.Splits)
	}
	// generated transactions inherit the split
	for _, tx := range listRange(ana, start) {
		if len(tx.Splits) != 2 || tx.Splits[0].ShareBP != 7000 {
			t.Fatalf("the generated transaction lost its split: %+v", tx)
		}
	}
	// changing the split on the recurrence replaces the parts
	ana.do("PUT", ana.wpath("/recurrences/%d", rec.ID), map[string]any{
		"kind": "expense", "amountCents": 10000, "description": "Internet", "categoryId": rec.CategoryID,
		"dayOfMonth": 1, "startDate": start,
		"splits": []map[string]any{{"userId": ana.user.ID, "shareBp": 5000}, {"userId": bia.user.ID, "shareBp": 5000}},
	}, 200, &rec)
	if rec.Splits[0].ShareBP != 5000 {
		t.Fatalf("splits after the PUT: %+v", rec.Splits)
	}
}
