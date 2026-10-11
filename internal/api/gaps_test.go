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
	bia.do("GET", bia.wpath("/members"), nil, 200, &list) // qualquer membro lista
	if len(list) != 3 {
		t.Fatalf("%d membros, want 3: %+v", len(list), list)
	}
	// dona primeiro, depois quem edita, depois quem só lê
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
		t.Fatalf("deveria ter saído da divisão padrão: %+v", m)
	}
	ana.do("PATCH", ana.wpath("/members/%d", ana.user.ID), map[string]any{"role": "reader"}, 400, nil) // o papel da dona não muda
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
		t.Fatalf("só o nome deveria mudar (e sem espaços): %+v", w)
	}
	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, &w)
	if w.Name != "Casa nova" || !w.SplittingEnabled {
		t.Fatalf("só a divisão deveria mudar: %+v", w)
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
		t.Fatalf("a carteira nova deveria ter as 20 categorias padrão, tem %d", len(cats))
	}
}

// ---- catálogo: caminhos que ficaram de fora ----

func TestDeleteUnusedPaymentMethodAndEmptyIcon(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	var p paymentMethodDTO
	ana.do("POST", ana.wpath("/payment-methods"), map[string]any{"name": "Cheque", "icon": "🧾"}, 201, &p)
	ana.do("DELETE", ana.wpath("/payment-methods/%d", p.ID), nil, 204, nil) // sem uso: apaga de verdade
	ana.do("PUT", ana.wpath("/payment-methods/%d", p.ID), map[string]any{"name": "x", "icon": "x"}, 404, nil)

	var res struct {
		Error apiError `json:"error"`
	}
	ana.do("POST", ana.wpath("/payment-methods"), map[string]any{"name": "Vale", "icon": ""}, 400, &res)
	if res.Error.Fields["icon"] == "" {
		t.Errorf("faltou erro no ícone: %+v", res.Error)
	}
	ana.do("POST", ana.wpath("/payment-methods"), map[string]any{"name": "Vale", "icon": "um ícone muito mais comprido que o limite de bytes"}, 400, nil)
}

// ---- recorrência com divisão ----

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
		t.Fatalf("splits da recorrência: %+v", rec.Splits)
	}
	// os lançamentos gerados herdam a divisão
	for _, tx := range listRange(ana, start) {
		if len(tx.Splits) != 2 || tx.Splits[0].ShareBP != 7000 {
			t.Fatalf("o lançamento gerado perdeu a divisão: %+v", tx)
		}
	}
	// trocar a divisão na recorrência substitui as partes
	ana.do("PUT", ana.wpath("/recurrences/%d", rec.ID), map[string]any{
		"kind": "expense", "amountCents": 10000, "description": "Internet", "categoryId": rec.CategoryID,
		"dayOfMonth": 1, "startDate": start,
		"splits": []map[string]any{{"userId": ana.user.ID, "shareBp": 5000}, {"userId": bia.user.ID, "shareBp": 5000}},
	}, 200, &rec)
	if rec.Splits[0].ShareBP != 5000 {
		t.Fatalf("splits depois do PUT: %+v", rec.Splits)
	}
}
