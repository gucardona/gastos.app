package api

import "testing"

// shareWallet adds other to owner's wallet with the given role and points other.walletID at it.
func shareWallet(owner, other *client, role string) {
	owner.app.t.Helper()
	owner.do("POST", owner.wpath("/members"), map[string]string{"email": other.user.Email, "role": role}, 201, nil)
	other.walletID = owner.walletID
}

func TestRegisterSeedsWallet(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	var cats, methods []map[string]any
	ana.do("GET", ana.wpath("/categories"), nil, 200, &cats)
	ana.do("GET", ana.wpath("/payment-methods"), nil, 200, &methods)
	if len(cats) != 20 || len(methods) != 8 {
		t.Fatalf("%d categorias (want 20), %d formas de pagamento (want 8)", len(cats), len(methods))
	}
}

func TestWalletIsolation(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	bia.walletID = ana.walletID
	bia.do("GET", bia.wpath("/categories"), nil, 404, nil)
}

func TestRoles(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "reader")

	newCat := map[string]any{"kind": "expense", "name": "Pets", "icon": "🐶", "color": "#112233", "essentiality": "nonessential"}
	bia.do("GET", bia.wpath("/categories"), nil, 200, nil)
	bia.do("POST", bia.wpath("/categories"), newCat, 403, nil)

	ana.do("PATCH", ana.wpath("/members/%d", bia.user.ID), map[string]any{"role": "editor"}, 200, nil)
	bia.do("POST", bia.wpath("/categories"), newCat, 201, nil)
	bia.do("PATCH", bia.wpath(""), map[string]any{"name": "Hackeada"}, 403, nil)
}

func TestMemberLeave(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")

	bia.do("DELETE", bia.wpath("/members/%d", ana.user.ID), nil, 403, nil) // an editor/reader cannot remove someone else
	ana.do("DELETE", ana.wpath("/members/%d", ana.user.ID), nil, 400, nil) // the owner cannot leave
	bia.do("DELETE", bia.wpath("/members/%d", bia.user.ID), nil, 204, nil)
	bia.do("GET", bia.wpath("/categories"), nil, 404, nil)
}

func TestAddMemberErrors(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")

	ana.do("POST", ana.wpath("/members"), map[string]string{"email": "ninguem@example.com", "role": "editor"}, 404, nil)
	ana.do("POST", ana.wpath("/members"), map[string]string{"email": bia.user.Email, "role": "owner"}, 400, nil)
	ana.do("POST", ana.wpath("/members"), map[string]string{"email": bia.user.Email, "role": "editor"}, 201, nil)
	ana.do("POST", ana.wpath("/members"), map[string]string{"email": bia.user.Email, "role": "reader"}, 409, nil)
}

func TestWalletCRUD(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")

	var created walletDTO
	ana.do("POST", "/api/wallets", map[string]any{"name": "Casa", "splittingEnabled": true}, 201, &created)
	if created.Role != "owner" || !created.SplittingEnabled || created.OwnerName != "Ana" {
		t.Fatalf("created wallet: %+v", created)
	}

	var list []walletDTO
	ana.do("GET", "/api/wallets", nil, 200, &list)
	if len(list) != 2 {
		t.Fatalf("%d carteiras, want 2", len(list))
	}

	ana.do("DELETE", ana.wpath(""), nil, 204, nil)
	ana.do("DELETE", "/api/wallets/"+itoa(created.ID), nil, 204, nil)
	ana.do("GET", "/api/wallets", nil, 200, &list)
	if len(list) != 0 {
		t.Fatalf("%d wallets after deleting, want 0", len(list))
	}
}

func TestRemovingAMemberNeedsTheirRecurrencesAndBankAccountsChangedFirst(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")
	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	start := monthsAgoFirst(t, app, 0)
	cat := firstCategory(t, ana, "expense")
	remove := func(want int) string {
		t.Helper()
		var res struct {
			Error apiError `json:"error"`
		}
		ana.do("DELETE", ana.wpath("/members/%d", bia.user.ID), nil, want, &res)
		return res.Error.Message
	}

	cases := []map[string]any{
		{"kind": "expense", "amountCents": 100, "categoryId": cat, "dayOfMonth": 1, "startDate": start, "paidByUserId": bia.user.ID},
		{"kind": "expense", "amountCents": 100, "categoryId": cat, "dayOfMonth": 1, "startDate": start,
			"splits": []map[string]any{{"userId": ana.user.ID, "shareBp": 5000}, {"userId": bia.user.ID, "shareBp": 5000}}},
		{"kind": "settlement", "amountCents": 100, "dayOfMonth": 1, "startDate": start, "paidByUserId": ana.user.ID, "toUserId": bia.user.ID},
	}
	for i, body := range cases {
		var rec recurrenceDTO
		ana.do("POST", ana.wpath("/recurrences"), body, 201, &rec)
		if msg := remove(400); msg != "Antes, edite ou exclua as recorrências que envolvem essa pessoa" {
			t.Errorf("case %d: message %q", i, msg)
		}
		body["enabled"] = false // a paused recurrence generates nothing, so it does not block
		ana.do("PUT", ana.wpath("/recurrences/%d", rec.ID), body, 200, nil)
	}

	var acc bankAccountDTO
	ana.do("POST", ana.wpath("/bank-accounts"), map[string]any{"name": "Conta da Bia", "paidByUserId": bia.user.ID}, 201, &acc)
	if msg := remove(400); msg == "" {
		t.Error("removal allowed while a bank account names her as payer")
	}
	ana.do("DELETE", ana.wpath("/bank-accounts/%d", acc.ID), nil, 204, nil)
	ana.do("DELETE", ana.wpath("/members/%d", bia.user.ID), nil, 204, nil)
}
