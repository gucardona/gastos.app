package bank_test

import (
	"strings"
	"testing"

	"gastos/internal/api/apitest"
	"gastos/internal/api/bank"
	"gastos/internal/api/ledger"
)

func TestBankAccountCRUD(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")

	var methods []apitest.PaymentMethod
	ana.Do("GET", ana.WPath("/payment-methods"), nil, 200, &methods)
	credit := methods[1].ID

	var a bank.BankAccountDTO
	ana.Do("POST", ana.WPath("/bank-accounts"), map[string]any{"name": "  Nubank cartão  ", "paymentMethodId": credit}, 201, &a)
	if a.Name != "Nubank cartão" || a.PaidByUserID != ana.User.ID || a.PaymentMethodID == nil || *a.PaymentMethodID != credit || !a.Enabled {
		t.Fatalf("created: %+v", a)
	}

	ana.Do("PUT", ana.WPath("/bank-accounts/%d", a.ID), map[string]any{"name": "Cartão", "paymentMethodId": nil, "enabled": false}, 200, &a)
	if a.Name != "Cartão" || a.PaymentMethodID != nil || a.Enabled {
		t.Fatalf("updated: %+v", a)
	}

	var list []bank.BankAccountDTO
	ana.Do("GET", ana.WPath("/bank-accounts"), nil, 200, &list)
	if len(list) != 1 || list[0].ID != a.ID {
		t.Fatalf("list: %+v", list)
	}
	ana.Do("DELETE", ana.WPath("/bank-accounts/%d", a.ID), nil, 204, nil)
	ana.Do("DELETE", ana.WPath("/bank-accounts/%d", a.ID), nil, 404, nil)
}

func TestBankAccountValidation(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com") // not a member of Ana's wallet

	ana.Do("POST", ana.WPath("/bank-accounts"), map[string]any{"name": "   "}, 400, nil)
	ana.Do("POST", ana.WPath("/bank-accounts"), map[string]any{"name": strings.Repeat("x", 61)}, 400, nil)
	ana.Do("POST", ana.WPath("/bank-accounts"), map[string]any{"name": "Conta", "paidByUserId": bia.User.ID}, 400, nil)

	var biaMethods []apitest.PaymentMethod
	bia.Do("GET", bia.WPath("/payment-methods"), nil, 200, &biaMethods)
	ana.Do("POST", ana.WPath("/bank-accounts"), map[string]any{"name": "Conta", "paymentMethodId": biaMethods[0].ID}, 400, nil) // another wallet's method
}

func TestBankAccountsArePrivateToTheWallet(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	acc := newAccount(t, ana, "Conta da Ana")

	bia.Do("GET", bia.WPath("/bank-accounts"), nil, 200, nil) // her own wallet: empty
	var list []bank.BankAccountDTO
	bia.Do("GET", bia.WPath("/bank-accounts"), nil, 200, &list)
	if len(list) != 0 {
		t.Fatalf("Bia sees %+v", list)
	}
	// Ana's account id through Bia's wallet path is not found, and neither is an import into it
	bia.Do("PUT", bia.WPath("/bank-accounts/%d", acc.ID), map[string]any{"name": "x"}, 404, nil)
	if status, _ := bia.Upload(bia.WPath("/bank-accounts/%d/import", acc.ID), map[string]string{"format": "ofx"}, "x.ofx", monthStatement); status != 404 {
		t.Fatalf("import into another wallet's account: status %d, want 404", status)
	}
	// and Ana's wallet is invisible to Bia
	bia.WalletID = ana.WalletID
	bia.Do("GET", bia.WPath("/bank-accounts"), nil, 404, nil)
}

func TestReadersCanLookButNotChangeBankData(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "reader")
	acc := newAccount(t, ana, "Conta")
	apitest.ImportFile(t, ana, acc.ID, monthStatement, 200)
	item := pending(t, ana)[0]

	bia.Do("GET", bia.WPath("/bank-accounts"), nil, 200, nil)
	bia.Do("GET", bia.WPath("/inbox"), nil, 200, nil)
	bia.Do("GET", bia.WPath("/rules"), nil, 200, nil)
	bia.Do("POST", bia.WPath("/bank-accounts"), map[string]any{"name": "x"}, 403, nil)
	bia.Do("POST", bia.WPath("/inbox/%d/ignore", item.ID), nil, 403, nil)
	bia.Do("POST", bia.WPath("/inbox/accept-suggested"), nil, 403, nil)
	bia.Do("POST", bia.WPath("/rules"), map[string]any{"pattern": "uber", "categoryId": apitest.FirstCategory(t, ana, "expense")}, 403, nil)
	if status, _ := bia.Upload(bia.WPath("/bank-accounts/%d/import", acc.ID), map[string]string{"format": "ofx"}, "x.ofx", monthStatement); status != 403 {
		t.Fatalf("a reader importing: status %d, want 403", status)
	}
}

// ---- importing ----

func TestPendingCountOnTheAccount(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	apitest.ImportFile(t, ana, acc.ID, monthStatement, 200)
	var list []bank.BankAccountDTO
	ana.Do("GET", ana.WPath("/bank-accounts"), nil, 200, &list)
	if list[0].PendingCount != 3 {
		t.Fatalf("pending count %d, want 3", list[0].PendingCount)
	}
	ana.Do("POST", ana.WPath("/inbox/%d/ignore", pending(t, ana)[0].ID), nil, 204, nil)
	ana.Do("GET", ana.WPath("/bank-accounts"), nil, 200, &list)
	if list[0].PendingCount != 2 {
		t.Fatalf("pending count %d, want 2", list[0].PendingCount)
	}
}

func TestDeletingABankAccountKeepsTheTransactionsItCreated(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	apitest.ImportFile(t, ana, acc.ID, monthStatement, 200)
	var tx ledger.Transaction
	ana.Do("POST", ana.WPath("/inbox/%d/accept", findByDescription(t, pending(t, ana), "MERCADO").ID), map[string]any{"categoryId": categoryByName(t, ana, "expense", "Mercado")}, 201, &tx)

	ana.Do("DELETE", ana.WPath("/bank-accounts/%d", acc.ID), nil, 204, nil)
	if n := len(pending(t, ana)); n != 0 {
		t.Fatalf("%d pending left from a deleted account", n)
	}
	var list []ledger.Transaction
	ana.Do("GET", ana.WPath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 1 || list[0].ID != tx.ID {
		t.Fatalf("the accepted transaction must survive its account: %+v", list)
	}
}
