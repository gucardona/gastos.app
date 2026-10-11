package api

import "testing"

// ---- GET /members ----

func TestListMembers(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	caio := app.register("Caio", "caio@example.com")
	shareWallet(ana, bia, "editor")
	shareWallet(ana, caio, "reader")

	var list []memberDTO
	bia.do("GET", bia.wpath("/members"), nil, 200, &list) // any member can list
	if len(list) != 3 {
		t.Fatalf("%d membros, want 3: %+v", len(list), list)
	}
	// owner first, then editors, then readers
	if list[0].Role != "owner" || list[1].Role != "editor" || list[2].Role != "reader" {
		t.Fatalf("ordem inesperada: %+v", list)
	}
	if list[0].Email != "ana@example.com" || !list[0].SplitsByDefault {
		t.Fatalf("campos do primeiro membro: %+v", list[0])
	}
}

func TestListMembersHiddenFromOutsiders(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	zeca := app.register("Zeca", "zeca@example.com")
	zeca.walletID = ana.walletID
	zeca.do("GET", zeca.wpath("/members"), nil, 404, nil)
}

func TestUpdateMemberSplitsByDefault(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")

	var m memberDTO
	ana.do("PATCH", ana.wpath("/members/%d", bia.user.ID), map[string]any{"splitsByDefault": false}, 200, &m)
	if m.SplitsByDefault {
		t.Fatalf("should have left the default split: %+v", m)
	}
	ana.do("PATCH", ana.wpath("/members/%d", ana.user.ID), map[string]any{"role": "reader"}, 400, nil) // the owner's role cannot change
	ana.do("PATCH", ana.wpath("/members/%d", bia.user.ID), map[string]any{"role": "owner"}, 400, nil)
	ana.do("PATCH", ana.wpath("/members/9999"), map[string]any{"role": "reader"}, 404, nil)
}

// ---- PATCH /wallets ----

func TestUpdateWallet(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	var w walletDTO

	ana.do("PATCH", ana.wpath(""), map[string]any{"name": "  Casa nova  "}, 200, &w)
	if w.Name != "Casa nova" || w.SplittingEnabled {
		t.Fatalf("only the name should change (and be trimmed): %+v", w)
	}
	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, &w)
	if w.Name != "Casa nova" || !w.SplittingEnabled {
		t.Fatalf("only splitting should change: %+v", w)
	}

	var res struct {
		Error apiError `json:"error"`
	}
	ana.do("PATCH", ana.wpath(""), map[string]any{}, 400, nil)
	ana.do("PATCH", ana.wpath(""), map[string]any{"name": "   "}, 400, &res)
	if res.Error.Fields["name"] == "" {
		t.Errorf("faltou erro de campo: %+v", res.Error)
	}
	ana.do("PATCH", ana.wpath(""), map[string]any{"nome": "x"}, 400, nil) // campo desconhecido
}

func TestCreateWalletValidation(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	ana.do("POST", "/api/wallets", map[string]any{"name": ""}, 400, nil)
	var w walletDTO
	ana.do("POST", "/api/wallets", map[string]any{"name": "Viagem"}, 201, &w)
	var cats []categoryDTO
	ana.do("GET", "/api/wallets/"+itoa(w.ID)+"/categories", nil, 200, &cats)
	if len(cats) != 20 {
		t.Fatalf("the new wallet should have the 20 default categories, has %d", len(cats))
	}
}

// ---- catalog: paths that were left out ----

func TestDeleteUnusedPaymentMethodAndEmptyIcon(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	var p paymentMethodDTO
	ana.do("POST", ana.wpath("/payment-methods"), map[string]any{"name": "Cheque", "icon": "🧾"}, 201, &p)
	ana.do("DELETE", ana.wpath("/payment-methods/%d", p.ID), nil, 204, nil) // unused: really deletes
	ana.do("PUT", ana.wpath("/payment-methods/%d", p.ID), map[string]any{"name": "x", "icon": "x"}, 404, nil)

	var res struct {
		Error apiError `json:"error"`
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
