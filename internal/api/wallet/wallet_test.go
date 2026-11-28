package wallet_test

import (
	"testing"

	"gastos/internal/api/apitest"
	"gastos/internal/api/httpx"
	"gastos/internal/api/wallet"
)

// recurrenceDTO, bankAccountDTO and categoryDTO decode only the id of what these tests create or list.
type (
	recurrenceDTO struct {
		ID int64 `json:"id"`
	}
	bankAccountDTO struct {
		ID int64 `json:"id"`
	}
	categoryDTO struct {
		ID int64 `json:"id"`
	}
)

func TestRegisterSeedsWallet(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	var cats, methods []map[string]any
	ana.Do("GET", ana.WPath("/categories"), nil, 200, &cats)
	ana.Do("GET", ana.WPath("/payment-methods"), nil, 200, &methods)
	if len(cats) != 20 || len(methods) != 8 {
		t.Fatalf("%d categorias (want 20), %d formas de pagamento (want 8)", len(cats), len(methods))
	}
}

func TestWalletIsolation(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	bia.WalletID = ana.WalletID
	bia.Do("GET", bia.WPath("/categories"), nil, 404, nil)
}

func TestRoles(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "reader")

	newCat := map[string]any{"kind": "expense", "name": "Pets", "icon": "🐶", "color": "#112233", "essentiality": "nonessential"}
	bia.Do("GET", bia.WPath("/categories"), nil, 200, nil)
	bia.Do("POST", bia.WPath("/categories"), newCat, 403, nil)

	ana.Do("PATCH", ana.WPath("/members/%d", bia.User.ID), map[string]any{"role": "editor"}, 200, nil)
	bia.Do("POST", bia.WPath("/categories"), newCat, 201, nil)
	bia.Do("PATCH", bia.WPath(""), map[string]any{"name": "Hackeada"}, 403, nil)
}

func TestMemberLeave(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")

	bia.Do("DELETE", bia.WPath("/members/%d", ana.User.ID), nil, 403, nil) // an editor/reader cannot remove someone else
	ana.Do("DELETE", ana.WPath("/members/%d", ana.User.ID), nil, 400, nil) // the owner cannot leave
	bia.Do("DELETE", bia.WPath("/members/%d", bia.User.ID), nil, 204, nil)
	bia.Do("GET", bia.WPath("/categories"), nil, 404, nil)
}

func TestAddMemberErrors(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")

	ana.Do("POST", ana.WPath("/members"), map[string]string{"email": "ninguem@example.com", "role": "editor"}, 404, nil)
	ana.Do("POST", ana.WPath("/members"), map[string]string{"email": bia.User.Email, "role": "owner"}, 400, nil)
	ana.Do("POST", ana.WPath("/members"), map[string]string{"email": bia.User.Email, "role": "editor"}, 201, nil)
	ana.Do("POST", ana.WPath("/members"), map[string]string{"email": bia.User.Email, "role": "reader"}, 409, nil)
}

func TestWalletCRUD(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")

	var created wallet.WalletDTO
	ana.Do("POST", "/api/wallets", map[string]any{"name": "Casa", "splittingEnabled": true}, 201, &created)
	if created.Role != "owner" || !created.SplittingEnabled || created.OwnerName != "Ana" {
		t.Fatalf("created wallet: %+v", created)
	}

	var list []wallet.WalletDTO
	ana.Do("GET", "/api/wallets", nil, 200, &list)
	if len(list) != 2 {
		t.Fatalf("%d carteiras, want 2", len(list))
	}

	ana.Do("DELETE", ana.WPath(""), nil, 204, nil)
	ana.Do("DELETE", "/api/wallets/"+apitest.Itoa(created.ID), nil, 204, nil)
	ana.Do("GET", "/api/wallets", nil, 200, &list)
	if len(list) != 0 {
		t.Fatalf("%d wallets after deleting, want 0", len(list))
	}
}

func TestRemovingAMemberNeedsTheirRecurrencesAndBankAccountsChangedFirst(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	ana.Do("PATCH", ana.WPath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	start := apitest.MonthsAgoFirst(t, app, 0)
	cat := apitest.FirstCategory(t, ana, "expense")
	remove := func(want int) string {
		t.Helper()
		var res struct {
			Error httpx.APIError `json:"error"`
		}
		ana.Do("DELETE", ana.WPath("/members/%d", bia.User.ID), nil, want, &res)
		return res.Error.Message
	}

	cases := []map[string]any{
		{"kind": "expense", "amountCents": 100, "categoryId": cat, "dayOfMonth": 1, "startDate": start, "paidByUserId": bia.User.ID},
		{"kind": "expense", "amountCents": 100, "categoryId": cat, "dayOfMonth": 1, "startDate": start,
			"splits": []map[string]any{{"userId": ana.User.ID, "shareBp": 5000}, {"userId": bia.User.ID, "shareBp": 5000}}},
		{"kind": "settlement", "amountCents": 100, "dayOfMonth": 1, "startDate": start, "paidByUserId": ana.User.ID, "toUserId": bia.User.ID},
	}
	for i, body := range cases {
		var rec recurrenceDTO
		ana.Do("POST", ana.WPath("/recurrences"), body, 201, &rec)
		if msg := remove(400); msg != "Antes, edite ou exclua as recorrências que envolvem essa pessoa" {
			t.Errorf("case %d: message %q", i, msg)
		}
		body["enabled"] = false // a paused recurrence generates nothing, so it does not block
		ana.Do("PUT", ana.WPath("/recurrences/%d", rec.ID), body, 200, nil)
	}

	var acc bankAccountDTO
	ana.Do("POST", ana.WPath("/bank-accounts"), map[string]any{"name": "Conta da Bia", "paidByUserId": bia.User.ID}, 201, &acc)
	if msg := remove(400); msg == "" {
		t.Error("removal allowed while a bank account names her as payer")
	}
	ana.Do("DELETE", ana.WPath("/bank-accounts/%d", acc.ID), nil, 204, nil)
	ana.Do("DELETE", ana.WPath("/members/%d", bia.User.ID), nil, 204, nil)
}

func TestUpdateWallet(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	var w wallet.WalletDTO

	ana.Do("PATCH", ana.WPath(""), map[string]any{"name": "  Casa nova  "}, 200, &w)
	if w.Name != "Casa nova" || w.SplittingEnabled {
		t.Fatalf("only the name should change (and be trimmed): %+v", w)
	}
	ana.Do("PATCH", ana.WPath(""), map[string]any{"splittingEnabled": true}, 200, &w)
	if w.Name != "Casa nova" || !w.SplittingEnabled {
		t.Fatalf("only splitting should change: %+v", w)
	}

	var res struct {
		Error httpx.APIError `json:"error"`
	}
	ana.Do("PATCH", ana.WPath(""), map[string]any{}, 400, nil)
	ana.Do("PATCH", ana.WPath(""), map[string]any{"name": "   "}, 400, &res)
	if res.Error.Fields["name"] == "" {
		t.Errorf("missing field error: %+v", res.Error)
	}
	ana.Do("PATCH", ana.WPath(""), map[string]any{"nome": "x"}, 400, nil) // campo desconhecido
}

func TestCreateWalletValidation(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	ana.Do("POST", "/api/wallets", map[string]any{"name": ""}, 400, nil)
	var w wallet.WalletDTO
	ana.Do("POST", "/api/wallets", map[string]any{"name": "Viagem"}, 201, &w)
	var cats []categoryDTO
	ana.Do("GET", "/api/wallets/"+apitest.Itoa(w.ID)+"/categories", nil, 200, &cats)
	if len(cats) != 20 {
		t.Fatalf("the new wallet should have the 20 default categories, has %d", len(cats))
	}
}
