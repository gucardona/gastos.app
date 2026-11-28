package bank_test

import (
	"strings"
	"testing"
	"time"

	"gastos/internal/api/apitest"
	"gastos/internal/api/ledger"
)

func TestAcceptTurnsALineIntoATransaction(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	var methods []apitest.PaymentMethod
	ana.Do("GET", ana.WPath("/payment-methods"), nil, 200, &methods)
	acc := newAccount(t, ana, "Conta")
	ana.Do("PUT", ana.WPath("/bank-accounts/%d", acc.ID), map[string]any{"name": "Conta", "paymentMethodId": methods[0].ID, "enabled": true}, 200, nil)
	apitest.ImportFile(t, ana, acc.ID, monthStatement, 200)
	item := findByDescription(t, pending(t, ana), "MERCADO")
	cat := categoryByName(t, ana, "expense", "Mercado")

	var tx ledger.Transaction
	ana.Do("POST", ana.WPath("/inbox/%d/accept", item.ID), map[string]any{"categoryId": cat, "description": "Mercado do bairro"}, 201, &tx)
	if tx.Kind != "expense" || tx.AmountCents != 4890 || tx.Date != "2026-10-01" || tx.Description != "Mercado do bairro" ||
		tx.CategoryID != cat || tx.Source != "bank" || tx.PaidByUserID != ana.User.ID || tx.PaymentMethodID == nil || *tx.PaymentMethodID != methods[0].ID {
		t.Fatalf("transaction: %+v", tx)
	}
	// it is a real transaction now
	var list []ledger.Transaction
	ana.Do("GET", ana.WPath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 1 || list[0].ID != tx.ID {
		t.Fatalf("transactions: %+v", list)
	}
	// and no longer pending; accepting twice is a conflict
	if n := len(pending(t, ana)); n != 2 {
		t.Fatalf("%d pending, want 2", n)
	}
	ana.Do("POST", ana.WPath("/inbox/%d/accept", item.ID), map[string]any{"categoryId": cat}, 409, nil)
}

func TestAcceptAnIncomeLine(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	apitest.ImportFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)
	item := findByDescription(t, pending(t, ana), "SALARIO")
	var tx ledger.Transaction
	ana.Do("POST", ana.WPath("/inbox/%d/accept", item.ID), map[string]any{"categoryId": categoryByName(t, ana, "income", "Salário")}, 201, &tx)
	if tx.Kind != "income" || tx.AmountCents != 850000 || tx.Description != "CREDITO SALARIO" {
		t.Fatalf("transaction: %+v", tx)
	}
}

func TestAcceptValidationLeavesTheLinePending(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	apitest.ImportFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)
	item := findByDescription(t, pending(t, ana), "MERCADO")

	// an income category for an expense line, and no category at all
	ana.Do("POST", ana.WPath("/inbox/%d/accept", item.ID), map[string]any{"categoryId": categoryByName(t, ana, "income", "Salário")}, 400, nil)
	ana.Do("POST", ana.WPath("/inbox/%d/accept", item.ID), map[string]any{}, 400, nil)
	if n := len(pending(t, ana)); n != 3 {
		t.Fatalf("%d pending, want 3: a refused accept must not consume the line", n)
	}
	var list []ledger.Transaction
	ana.Do("GET", ana.WPath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 0 {
		t.Fatalf("a transaction was created anyway: %+v", list)
	}
}

func TestAcceptInASplittingWalletAppliesTheDefaultSplit(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	ana.Do("PATCH", ana.WPath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	apitest.ImportFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)
	item := findByDescription(t, pending(t, ana), "MERCADO")

	var tx ledger.Transaction
	ana.Do("POST", ana.WPath("/inbox/%d/accept", item.ID), map[string]any{"categoryId": categoryByName(t, ana, "expense", "Mercado")}, 201, &tx)
	if len(tx.Splits) != 2 || tx.Splits[0].AmountCents+tx.Splits[1].AmountCents != 4890 {
		t.Fatalf("splits: %+v", tx.Splits)
	}
}

func TestIgnoreAndTheLineStaysGone(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	apitest.ImportFile(t, ana, acc.ID, monthStatement, 200)
	item := findByDescription(t, pending(t, ana), "UBER")

	ana.Do("POST", ana.WPath("/inbox/%d/ignore", item.ID), nil, 204, nil)
	ana.Do("POST", ana.WPath("/inbox/%d/ignore", item.ID), nil, 409, nil)
	if n := len(pending(t, ana)); n != 2 {
		t.Fatalf("%d pending, want 2", n)
	}
	// re-importing the file does not bring an ignored line back
	if got := apitest.ImportFile(t, ana, acc.ID, monthStatement, 200); got.Imported != 0 {
		t.Fatalf("re-import: %+v", got)
	}
	if n := len(pending(t, ana)); n != 2 {
		t.Fatalf("%d pending after re-import, want 2", n)
	}
}

func TestDeletingAnAcceptedTransactionDoesNotBringTheLineBack(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	apitest.ImportFile(t, ana, acc.ID, monthStatement, 200)
	item := findByDescription(t, pending(t, ana), "MERCADO")
	var tx ledger.Transaction
	ana.Do("POST", ana.WPath("/inbox/%d/accept", item.ID), map[string]any{"categoryId": categoryByName(t, ana, "expense", "Mercado")}, 201, &tx)
	ana.Do("DELETE", ana.WPath("/transactions/%d", tx.ID), nil, 204, nil)

	if got := apitest.ImportFile(t, ana, acc.ID, monthStatement, 200); got.Imported != 0 || got.Duplicates != 3 {
		t.Fatalf("re-import after deleting: %+v", got)
	}
	if findAny(pending(t, ana), "MERCADO") {
		t.Fatal("the deleted transaction's line came back to the inbox")
	}
}

func findAny(list []inboxItem, contains string) bool {
	for _, it := range list {
		if strings.Contains(it.Description, contains) {
			return true
		}
	}
	return false
}

func TestLinkALineToAnExistingTransaction(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	apitest.ImportFile(t, ana, acc.ID, monthStatement, 200)
	item := findByDescription(t, pending(t, ana), "MERCADO")

	var typed ledger.Transaction
	ana.Do("POST", ana.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 4890, "description": "Mercado (digitado)", "categoryId": categoryByName(t, ana, "expense", "Mercado"), "date": "2026-10-01",
	}, 201, &typed)

	ana.Do("POST", ana.WPath("/inbox/%d/match", item.ID), map[string]any{"transactionId": typed.ID}, 204, nil)
	if findAny(pending(t, ana), "MERCADO") {
		t.Fatal("a linked line must leave the inbox")
	}
	var list []ledger.Transaction
	ana.Do("GET", ana.WPath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 1 {
		t.Fatalf("linking must not create a second transaction: %+v", list)
	}

	// the same transaction cannot be linked to another line
	other := findByDescription(t, pending(t, ana), "UBER")
	ana.Do("POST", ana.WPath("/inbox/%d/match", other.ID), map[string]any{"transactionId": typed.ID}, 409, nil)
	// nor a transaction that does not exist, or from another wallet
	ana.Do("POST", ana.WPath("/inbox/%d/match", other.ID), map[string]any{"transactionId": 99999}, 404, nil)
	bia := app.Register("Bia", "bia@example.com")
	var biaTx ledger.Transaction
	bia.Do("POST", bia.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 100, "description": "x", "categoryId": apitest.FirstCategory(t, bia, "expense"), "date": "2026-10-01",
	}, 201, &biaTx)
	ana.Do("POST", ana.WPath("/inbox/%d/match", other.ID), map[string]any{"transactionId": biaTx.ID}, 404, nil)
}

// ---- suggestions ----

func TestImportSuggestsAnExistingTransaction(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	var typed ledger.Transaction
	ana.Do("POST", ana.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 4890, "description": "Mercado (digitado)", "categoryId": categoryByName(t, ana, "expense", "Mercado"), "date": "2026-10-02",
	}, 201, &typed)

	apitest.ImportFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)
	item := findByDescription(t, pending(t, ana), "MERCADO")
	if item.SuggestedMatch == nil || item.SuggestedMatch.ID != typed.ID || item.SuggestedMatch.Description != "Mercado (digitado)" || item.SuggestedMatch.Date != "2026-10-02" {
		t.Fatalf("suggested match: %+v", item.SuggestedMatch)
	}
	if findByDescription(t, pending(t, ana), "UBER").SuggestedMatch != nil {
		t.Fatal("a line with nothing similar must have no suggested match")
	}
}

func TestOneTransactionIsSuggestedToOnlyOneLine(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	var typed ledger.Transaction
	ana.Do("POST", ana.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "Almoço", "categoryId": apitest.FirstCategory(t, ana, "expense"), "date": "2026-10-01",
	}, 201, &typed)
	// two identical lines: only one of them can be that lunch
	apitest.ImportFile(t, ana, newAccount(t, ana, "Conta").ID, apitest.OfxRows("20261001|-10.00|A|ALMOCO", "20261001|-10.00|B|ALMOCO"), 200)
	n := 0
	for _, it := range pending(t, ana) {
		if it.SuggestedMatch != nil {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d lines got the same suggested transaction, want exactly 1", n)
	}
}

func TestAlreadyLinkedTransactionsAreNotSuggestedAgain(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	var typed ledger.Transaction
	ana.Do("POST", ana.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "Almoço", "categoryId": apitest.FirstCategory(t, ana, "expense"), "date": "2026-10-01",
	}, 201, &typed)
	apitest.ImportFile(t, ana, acc.ID, apitest.OfxRows("20261001|-10.00|A|ALMOCO"), 200)
	ana.Do("POST", ana.WPath("/inbox/%d/match", pending(t, ana)[0].ID), map[string]any{"transactionId": typed.ID}, 204, nil)

	apitest.ImportFile(t, ana, acc.ID, apitest.OfxRows("20261001|-10.00|B|ALMOCO OUTRO DIA"), 200)
	if m := pending(t, ana)[0].SuggestedMatch; m != nil {
		t.Fatalf("a transaction that is already linked was suggested again: %+v", m)
	}
}

func TestInboxOnlyListsPendingByDefaultAndCanFilter(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	apitest.ImportFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)
	ana.Do("POST", ana.WPath("/inbox/%d/ignore", findByDescription(t, pending(t, ana), "UBER").ID), nil, 204, nil)

	var ignored []inboxItem
	ana.Do("GET", ana.WPath("/inbox?status=ignored"), nil, 200, &ignored)
	if len(ignored) != 1 || ignored[0].Status != "ignored" {
		t.Fatalf("ignored: %+v", ignored)
	}
	ana.Do("GET", ana.WPath("/inbox?status=nonsense"), nil, 400, nil)
}

func TestInboxChangesNotifyOtherMembers(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	stream := apitest.OpenStream(t, bia)

	apitest.ImportFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)
	deadline := time.After(2 * time.Second)
	for {
		select {
		case l := <-stream:
			if l == `data: {"type":"changed"}` {
				return
			}
		case <-deadline:
			t.Fatal("event did not arrive")
		}
	}
}
