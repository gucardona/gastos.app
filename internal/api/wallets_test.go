package api

import "testing"

// shareWallet adiciona other à carteira de owner com o papel dado e aponta other.walletID para ela.
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

	bia.do("DELETE", bia.wpath("/members/%d", ana.user.ID), nil, 403, nil) // reader/editor não remove outro
	ana.do("DELETE", ana.wpath("/members/%d", ana.user.ID), nil, 400, nil) // owner não sai
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
		t.Fatalf("carteira criada: %+v", created)
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
		t.Fatalf("%d carteiras após apagar, want 0", len(list))
	}
}
