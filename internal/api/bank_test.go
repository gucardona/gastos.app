package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ofxRows builds an OFX 1.x (SGML) statement from "YYYYMMDD|amount|fitid|memo" rows.
func ofxRows(rows ...string) []byte {
	var b strings.Builder
	b.WriteString("OFXHEADER:100\nCHARSET:1252\n\n<OFX><BANKTRANLIST>\n")
	for _, r := range rows {
		f := strings.SplitN(r, "|", 4)
		fmt.Fprintf(&b, "<STMTTRN><TRNTYPE>X<DTPOSTED>%s<TRNAMT>%s<FITID>%s<MEMO>%s</STMTTRN>\n", f[0], f[1], f[2], f[3])
	}
	b.WriteString("</BANKTRANLIST></OFX>\n")
	return []byte(b.String())
}

var monthStatement = ofxRows(
	"20261001|-48.90|F1|COMPRA MERCADO DO BAIRRO",
	"20261005|8500.00|F2|CREDITO SALARIO",
	"20261006|-25.00|F3|UBER *TRIP 123",
)

type matchRef struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	Date        string `json:"date"`
}

type inboxItem struct {
	ID                   int64     `json:"id"`
	BankAccountID        int64     `json:"bankAccountId"`
	AccountName          string    `json:"accountName"`
	Date                 string    `json:"date"`
	AmountCents          int64     `json:"amountCents"`
	Description          string    `json:"description"`
	Status               string    `json:"status"`
	SuggestedCategoryID  *int64    `json:"suggestedCategoryId"`
	SuggestedDescription *string   `json:"suggestedDescription"`
	SuggestedMatch       *matchRef `json:"suggestedMatch"`
}

type importSummary struct {
	Imported   int `json:"imported"`
	Duplicates int `json:"duplicates"`
}

func newAccount(t *testing.T, c *client, name string) bankAccountDTO {
	t.Helper()
	var a bankAccountDTO
	c.do("POST", c.wpath("/bank-accounts"), map[string]any{"name": name}, 201, &a)
	return a
}

func importFile(t *testing.T, c *client, accountID int64, content []byte, wantStatus int) importSummary {
	t.Helper()
	status, raw := c.upload(c.wpath("/bank-accounts/%d/import", accountID), map[string]string{"format": "ofx"}, "extrato.ofx", content)
	if status != wantStatus {
		t.Fatalf("import: status %d, want %d; body: %s", status, wantStatus, raw)
	}
	var out importSummary
	_ = json.Unmarshal(raw, &out)
	return out
}

func pending(t *testing.T, c *client) []inboxItem {
	t.Helper()
	var list []inboxItem
	c.do("GET", c.wpath("/inbox"), nil, 200, &list)
	return list
}

func findByDescription(t *testing.T, list []inboxItem, contains string) inboxItem {
	t.Helper()
	for _, it := range list {
		if strings.Contains(it.Description, contains) {
			return it
		}
	}
	t.Fatalf("no inbox item containing %q in %+v", contains, list)
	return inboxItem{}
}

func categoryByName(t *testing.T, c *client, kind, name string) int64 {
	t.Helper()
	var cats []categoryDTO
	c.do("GET", c.wpath("/categories"), nil, 200, &cats)
	for _, cat := range cats {
		if cat.Kind == kind && cat.Name == name {
			return cat.ID
		}
	}
	t.Fatalf("no %s category named %q", kind, name)
	return 0
}

// ---- bank accounts ----

func TestBankAccountCRUD(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")

	var methods []paymentMethodDTO
	ana.do("GET", ana.wpath("/payment-methods"), nil, 200, &methods)
	credit := methods[1].ID

	var a bankAccountDTO
	ana.do("POST", ana.wpath("/bank-accounts"), map[string]any{"name": "  Nubank cartão  ", "paymentMethodId": credit}, 201, &a)
	if a.Name != "Nubank cartão" || a.PaidByUserID != ana.user.ID || a.PaymentMethodID == nil || *a.PaymentMethodID != credit || !a.Enabled {
		t.Fatalf("created: %+v", a)
	}

	ana.do("PUT", ana.wpath("/bank-accounts/%d", a.ID), map[string]any{"name": "Cartão", "paymentMethodId": nil, "enabled": false}, 200, &a)
	if a.Name != "Cartão" || a.PaymentMethodID != nil || a.Enabled {
		t.Fatalf("updated: %+v", a)
	}

	var list []bankAccountDTO
	ana.do("GET", ana.wpath("/bank-accounts"), nil, 200, &list)
	if len(list) != 1 || list[0].ID != a.ID {
		t.Fatalf("list: %+v", list)
	}
	ana.do("DELETE", ana.wpath("/bank-accounts/%d", a.ID), nil, 204, nil)
	ana.do("DELETE", ana.wpath("/bank-accounts/%d", a.ID), nil, 404, nil)
}

func TestBankAccountValidation(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com") // not a member of Ana's wallet

	ana.do("POST", ana.wpath("/bank-accounts"), map[string]any{"name": "   "}, 400, nil)
	ana.do("POST", ana.wpath("/bank-accounts"), map[string]any{"name": strings.Repeat("x", 61)}, 400, nil)
	ana.do("POST", ana.wpath("/bank-accounts"), map[string]any{"name": "Conta", "paidByUserId": bia.user.ID}, 400, nil)

	var biaMethods []paymentMethodDTO
	bia.do("GET", bia.wpath("/payment-methods"), nil, 200, &biaMethods)
	ana.do("POST", ana.wpath("/bank-accounts"), map[string]any{"name": "Conta", "paymentMethodId": biaMethods[0].ID}, 400, nil) // another wallet's method
}

func TestBankAccountsArePrivateToTheWallet(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	acc := newAccount(t, ana, "Conta da Ana")

	bia.do("GET", bia.wpath("/bank-accounts"), nil, 200, nil) // her own wallet: empty
	var list []bankAccountDTO
	bia.do("GET", bia.wpath("/bank-accounts"), nil, 200, &list)
	if len(list) != 0 {
		t.Fatalf("Bia sees %+v", list)
	}
	// Ana's account id through Bia's wallet path is not found, and neither is an import into it
	bia.do("PUT", bia.wpath("/bank-accounts/%d", acc.ID), map[string]any{"name": "x"}, 404, nil)
	if status, _ := bia.upload(bia.wpath("/bank-accounts/%d/import", acc.ID), map[string]string{"format": "ofx"}, "x.ofx", monthStatement); status != 404 {
		t.Fatalf("import into another wallet's account: status %d, want 404", status)
	}
	// and Ana's wallet is invisible to Bia
	bia.walletID = ana.walletID
	bia.do("GET", bia.wpath("/bank-accounts"), nil, 404, nil)
}

func TestReadersCanLookButNotChangeBankData(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "reader")
	acc := newAccount(t, ana, "Conta")
	importFile(t, ana, acc.ID, monthStatement, 200)
	item := pending(t, ana)[0]

	bia.do("GET", bia.wpath("/bank-accounts"), nil, 200, nil)
	bia.do("GET", bia.wpath("/inbox"), nil, 200, nil)
	bia.do("GET", bia.wpath("/rules"), nil, 200, nil)
	bia.do("POST", bia.wpath("/bank-accounts"), map[string]any{"name": "x"}, 403, nil)
	bia.do("POST", bia.wpath("/inbox/%d/ignore", item.ID), nil, 403, nil)
	bia.do("POST", bia.wpath("/inbox/accept-suggested"), nil, 403, nil)
	bia.do("POST", bia.wpath("/rules"), map[string]any{"pattern": "uber", "categoryId": firstCategory(t, ana, "expense")}, 403, nil)
	if status, _ := bia.upload(bia.wpath("/bank-accounts/%d/import", acc.ID), map[string]string{"format": "ofx"}, "x.ofx", monthStatement); status != 403 {
		t.Fatalf("a reader importing: status %d, want 403", status)
	}
}

// ---- importing ----

func TestImportOFXFillsTheInbox(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta corrente")

	if got := importFile(t, ana, acc.ID, monthStatement, 200); got.Imported != 3 || got.Duplicates != 0 {
		t.Fatalf("first import: %+v", got)
	}
	items := pending(t, ana)
	if len(items) != 3 {
		t.Fatalf("%d pending, want 3", len(items))
	}
	// newest first, signed amounts as in the file, with the account's name
	if items[0].Date != "2026-10-06" || items[2].Date != "2026-10-01" {
		t.Fatalf("order: %+v", items)
	}
	salary := findByDescription(t, items, "SALARIO")
	if salary.AmountCents != 850000 || salary.AccountName != "Conta corrente" || salary.Status != "pending" || salary.BankAccountID != acc.ID {
		t.Fatalf("salary line: %+v", salary)
	}
	if findByDescription(t, items, "MERCADO").AmountCents != -4890 {
		t.Fatal("an expense must keep its negative sign")
	}
}

func TestReimportingTheSameFileChangesNothing(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	importFile(t, ana, acc.ID, monthStatement, 200)
	if got := importFile(t, ana, acc.ID, monthStatement, 200); got.Imported != 0 || got.Duplicates != 3 {
		t.Fatalf("second import: %+v, want 0 new and 3 duplicates", got)
	}
	if n := len(pending(t, ana)); n != 3 {
		t.Fatalf("%d pending after re-importing, want 3", n)
	}
	// an overlapping export only brings the new lines
	later := ofxRows("20261006|-25.00|F3|UBER *TRIP 123", "20261010|-10.00|F4|CAFE")
	if got := importFile(t, ana, acc.ID, later, 200); got.Imported != 1 || got.Duplicates != 1 {
		t.Fatalf("overlapping import: %+v", got)
	}
}

func TestTheSameLineOnTwoAccountsIsKeptOnBoth(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	a, b := newAccount(t, ana, "A"), newAccount(t, ana, "B")
	importFile(t, ana, a.ID, monthStatement, 200)
	if got := importFile(t, ana, b.ID, monthStatement, 200); got.Imported != 3 {
		t.Fatalf("the second account: %+v", got)
	}
}

func TestImportRejectsBadFiles(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	path := ana.wpath("/bank-accounts/%d/import", acc.ID)
	post := func(fields map[string]string, content []byte) (int, string) {
		status, raw := ana.upload(path, fields, "f.txt", content)
		return status, string(raw)
	}

	if status, body := post(map[string]string{"format": "ofx"}, []byte("just some text")); status != 400 || !strings.Contains(body, "OFX") {
		t.Fatalf("not an OFX: %d %s", status, body)
	}
	if status, _ := post(map[string]string{"format": "ofx"}, []byte("<OFX></OFX>")); status != 400 {
		t.Fatalf("an OFX with nothing in it: status %d", status)
	}
	if status, _ := post(map[string]string{}, monthStatement); status != 400 {
		t.Fatalf("no format: status %d", status)
	}
	if status, _ := post(map[string]string{"format": "pdf"}, monthStatement); status != 400 {
		t.Fatalf("unknown format: status %d", status)
	}
	badLine := ofxRows("20261340|-1.00|X|BAD DATE")
	if status, body := post(map[string]string{"format": "ofx"}, badLine); status != 400 || !strings.Contains(body, "#1") {
		t.Fatalf("a bad date should name the transaction: %d %s", status, body)
	}
	huge := append([]byte("<OFX>"), make([]byte, 6<<20)...)
	if status, _ := post(map[string]string{"format": "ofx"}, huge); status != 413 {
		t.Fatalf("a huge file: status %d, want 413", status)
	}
	if n := len(pending(t, ana)); n != 0 {
		t.Fatalf("%d lines in the inbox after only failed imports", n)
	}

	// a plain JSON post to the import endpoint is not a file upload
	ana.do("POST", path, map[string]any{"format": "ofx"}, 400, nil)
}

func TestImportIsAllOrNothing(t *testing.T) {
	// the second transaction is broken: the first one must not be left behind
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	file := ofxRows("20261001|-10.00|OK|FINE", "20261002|abc|BAD|BROKEN")
	importFile(t, ana, acc.ID, file, 400)
	if n := len(pending(t, ana)); n != 0 {
		t.Fatalf("%d lines imported from a broken file, want 0", n)
	}
}

func TestImportCSVWithAColumnMapping(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Cartão")
	csv := []byte("Data;Estabelecimento;Valor\n01/10/2026;PADARIA;48,90\n02/10/2026;ESTORNO LOJA;-30,00\n")
	fields := map[string]string{"format": "csv", "dateColumn": "1", "descriptionColumn": "2", "amountColumn": "3", "hasHeader": "true", "invertSign": "true"}
	status, raw := ana.upload(ana.wpath("/bank-accounts/%d/import", acc.ID), fields, "fatura.csv", csv)
	if status != 200 {
		t.Fatalf("status %d: %s", status, raw)
	}
	items := pending(t, ana)
	if len(items) != 2 || findByDescription(t, items, "PADARIA").AmountCents != -4890 || findByDescription(t, items, "ESTORNO").AmountCents != 3000 {
		t.Fatalf("items: %+v", items)
	}

	// the mapping is validated
	for name, bad := range map[string]map[string]string{
		"missing columns":   {"format": "csv"},
		"same column twice": {"format": "csv", "dateColumn": "1", "descriptionColumn": "1", "amountColumn": "3"},
		"not a number":      {"format": "csv", "dateColumn": "a", "descriptionColumn": "2", "amountColumn": "3"},
	} {
		if status, _ := ana.upload(ana.wpath("/bank-accounts/%d/import", acc.ID), bad, "f.csv", csv); status != 400 {
			t.Errorf("%s: status %d, want 400", name, status)
		}
	}
}

func TestImportIntoADisabledAccountIsRefused(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	ana.do("PUT", ana.wpath("/bank-accounts/%d", acc.ID), map[string]any{"name": "Conta", "enabled": false}, 200, nil)
	importFile(t, ana, acc.ID, monthStatement, 409)
}

// ---- accepting, ignoring and linking ----

func TestAcceptTurnsALineIntoATransaction(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	var methods []paymentMethodDTO
	ana.do("GET", ana.wpath("/payment-methods"), nil, 200, &methods)
	acc := newAccount(t, ana, "Conta")
	ana.do("PUT", ana.wpath("/bank-accounts/%d", acc.ID), map[string]any{"name": "Conta", "paymentMethodId": methods[0].ID, "enabled": true}, 200, nil)
	importFile(t, ana, acc.ID, monthStatement, 200)
	item := findByDescription(t, pending(t, ana), "MERCADO")
	cat := categoryByName(t, ana, "expense", "Mercado")

	var tx transaction
	ana.do("POST", ana.wpath("/inbox/%d/accept", item.ID), map[string]any{"categoryId": cat, "description": "Mercado do bairro"}, 201, &tx)
	if tx.Kind != "expense" || tx.AmountCents != 4890 || tx.Date != "2026-10-01" || tx.Description != "Mercado do bairro" ||
		tx.CategoryID != cat || tx.Source != "bank" || tx.PaidByUserID != ana.user.ID || tx.PaymentMethodID == nil || *tx.PaymentMethodID != methods[0].ID {
		t.Fatalf("transaction: %+v", tx)
	}
	// it is a real transaction now
	var list []transaction
	ana.do("GET", ana.wpath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 1 || list[0].ID != tx.ID {
		t.Fatalf("transactions: %+v", list)
	}
	// and no longer pending; accepting twice is a conflict
	if n := len(pending(t, ana)); n != 2 {
		t.Fatalf("%d pending, want 2", n)
	}
	ana.do("POST", ana.wpath("/inbox/%d/accept", item.ID), map[string]any{"categoryId": cat}, 409, nil)
}

func TestAcceptAnIncomeLine(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	importFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)
	item := findByDescription(t, pending(t, ana), "SALARIO")
	var tx transaction
	ana.do("POST", ana.wpath("/inbox/%d/accept", item.ID), map[string]any{"categoryId": categoryByName(t, ana, "income", "Salário")}, 201, &tx)
	if tx.Kind != "income" || tx.AmountCents != 850000 || tx.Description != "CREDITO SALARIO" {
		t.Fatalf("transaction: %+v", tx)
	}
}

func TestAcceptValidationLeavesTheLinePending(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	importFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)
	item := findByDescription(t, pending(t, ana), "MERCADO")

	// an income category for an expense line, and no category at all
	ana.do("POST", ana.wpath("/inbox/%d/accept", item.ID), map[string]any{"categoryId": categoryByName(t, ana, "income", "Salário")}, 400, nil)
	ana.do("POST", ana.wpath("/inbox/%d/accept", item.ID), map[string]any{}, 400, nil)
	if n := len(pending(t, ana)); n != 3 {
		t.Fatalf("%d pending, want 3: a refused accept must not consume the line", n)
	}
	var list []transaction
	ana.do("GET", ana.wpath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 0 {
		t.Fatalf("a transaction was created anyway: %+v", list)
	}
}

func TestAcceptInASplittingWalletAppliesTheDefaultSplit(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")
	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	importFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)
	item := findByDescription(t, pending(t, ana), "MERCADO")

	var tx transaction
	ana.do("POST", ana.wpath("/inbox/%d/accept", item.ID), map[string]any{"categoryId": categoryByName(t, ana, "expense", "Mercado")}, 201, &tx)
	if len(tx.Splits) != 2 || tx.Splits[0].AmountCents+tx.Splits[1].AmountCents != 4890 {
		t.Fatalf("splits: %+v", tx.Splits)
	}
}

func TestIgnoreAndTheLineStaysGone(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	importFile(t, ana, acc.ID, monthStatement, 200)
	item := findByDescription(t, pending(t, ana), "UBER")

	ana.do("POST", ana.wpath("/inbox/%d/ignore", item.ID), nil, 204, nil)
	ana.do("POST", ana.wpath("/inbox/%d/ignore", item.ID), nil, 409, nil)
	if n := len(pending(t, ana)); n != 2 {
		t.Fatalf("%d pending, want 2", n)
	}
	// re-importing the file does not bring an ignored line back
	if got := importFile(t, ana, acc.ID, monthStatement, 200); got.Imported != 0 {
		t.Fatalf("re-import: %+v", got)
	}
	if n := len(pending(t, ana)); n != 2 {
		t.Fatalf("%d pending after re-import, want 2", n)
	}
}

func TestDeletingAnAcceptedTransactionDoesNotBringTheLineBack(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	importFile(t, ana, acc.ID, monthStatement, 200)
	item := findByDescription(t, pending(t, ana), "MERCADO")
	var tx transaction
	ana.do("POST", ana.wpath("/inbox/%d/accept", item.ID), map[string]any{"categoryId": categoryByName(t, ana, "expense", "Mercado")}, 201, &tx)
	ana.do("DELETE", ana.wpath("/transactions/%d", tx.ID), nil, 204, nil)

	if got := importFile(t, ana, acc.ID, monthStatement, 200); got.Imported != 0 || got.Duplicates != 3 {
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
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	importFile(t, ana, acc.ID, monthStatement, 200)
	item := findByDescription(t, pending(t, ana), "MERCADO")

	var typed transaction
	ana.do("POST", ana.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 4890, "description": "Mercado (digitado)", "categoryId": categoryByName(t, ana, "expense", "Mercado"), "date": "2026-10-01",
	}, 201, &typed)

	ana.do("POST", ana.wpath("/inbox/%d/match", item.ID), map[string]any{"transactionId": typed.ID}, 204, nil)
	if findAny(pending(t, ana), "MERCADO") {
		t.Fatal("a linked line must leave the inbox")
	}
	var list []transaction
	ana.do("GET", ana.wpath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 1 {
		t.Fatalf("linking must not create a second transaction: %+v", list)
	}

	// the same transaction cannot be linked to another line
	other := findByDescription(t, pending(t, ana), "UBER")
	ana.do("POST", ana.wpath("/inbox/%d/match", other.ID), map[string]any{"transactionId": typed.ID}, 409, nil)
	// nor a transaction that does not exist, or from another wallet
	ana.do("POST", ana.wpath("/inbox/%d/match", other.ID), map[string]any{"transactionId": 99999}, 404, nil)
	bia := app.register("Bia", "bia@example.com")
	var biaTx transaction
	bia.do("POST", bia.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 100, "description": "x", "categoryId": firstCategory(t, bia, "expense"), "date": "2026-10-01",
	}, 201, &biaTx)
	ana.do("POST", ana.wpath("/inbox/%d/match", other.ID), map[string]any{"transactionId": biaTx.ID}, 404, nil)
}

// ---- suggestions ----

func TestImportSuggestsAnExistingTransaction(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	var typed transaction
	ana.do("POST", ana.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 4890, "description": "Mercado (digitado)", "categoryId": categoryByName(t, ana, "expense", "Mercado"), "date": "2026-10-02",
	}, 201, &typed)

	importFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)
	item := findByDescription(t, pending(t, ana), "MERCADO")
	if item.SuggestedMatch == nil || item.SuggestedMatch.ID != typed.ID || item.SuggestedMatch.Description != "Mercado (digitado)" || item.SuggestedMatch.Date != "2026-10-02" {
		t.Fatalf("suggested match: %+v", item.SuggestedMatch)
	}
	if findByDescription(t, pending(t, ana), "UBER").SuggestedMatch != nil {
		t.Fatal("a line with nothing similar must have no suggested match")
	}
}

func TestOneTransactionIsSuggestedToOnlyOneLine(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	var typed transaction
	ana.do("POST", ana.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "Almoço", "categoryId": firstCategory(t, ana, "expense"), "date": "2026-10-01",
	}, 201, &typed)
	// two identical lines: only one of them can be that lunch
	importFile(t, ana, newAccount(t, ana, "Conta").ID, ofxRows("20261001|-10.00|A|ALMOCO", "20261001|-10.00|B|ALMOCO"), 200)
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
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	var typed transaction
	ana.do("POST", ana.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "Almoço", "categoryId": firstCategory(t, ana, "expense"), "date": "2026-10-01",
	}, 201, &typed)
	importFile(t, ana, acc.ID, ofxRows("20261001|-10.00|A|ALMOCO"), 200)
	ana.do("POST", ana.wpath("/inbox/%d/match", pending(t, ana)[0].ID), map[string]any{"transactionId": typed.ID}, 204, nil)

	importFile(t, ana, acc.ID, ofxRows("20261001|-10.00|B|ALMOCO OUTRO DIA"), 200)
	if m := pending(t, ana)[0].SuggestedMatch; m != nil {
		t.Fatalf("a transaction that is already linked was suggested again: %+v", m)
	}
}

func TestRulesSuggestCategoriesOnImport(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	transport := categoryByName(t, ana, "expense", "Transporte")
	ana.do("POST", ana.wpath("/rules"), map[string]any{"pattern": "uber", "categoryId": transport, "renameTo": "Uber"}, 201, nil)

	importFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)
	uber := findByDescription(t, pending(t, ana), "UBER")
	if uber.SuggestedCategoryID == nil || *uber.SuggestedCategoryID != transport || uber.SuggestedDescription == nil || *uber.SuggestedDescription != "Uber" {
		t.Fatalf("uber line: %+v", uber)
	}
	if findByDescription(t, pending(t, ana), "MERCADO").SuggestedCategoryID != nil {
		t.Fatal("a line no rule matches must have no suggestion")
	}
}

func TestARuleForAnExpenseCategoryIsNotSuggestedForIncome(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	ana.do("POST", ana.wpath("/rules"), map[string]any{"pattern": "estorno", "categoryId": categoryByName(t, ana, "expense", "Lazer")}, 201, nil)
	importFile(t, ana, newAccount(t, ana, "Conta").ID, ofxRows("20261001|30.00|R|ESTORNO LOJA"), 200)
	if got := pending(t, ana)[0]; got.SuggestedCategoryID != nil {
		t.Fatalf("an expense category was suggested for a credit: %+v", got)
	}
}

func TestAcceptSuggestedOnlyTakesLinesThatAreSafe(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	transport := categoryByName(t, ana, "expense", "Transporte")
	ana.do("POST", ana.wpath("/rules"), map[string]any{"pattern": "uber", "categoryId": transport, "renameTo": "Uber"}, 201, nil)
	ana.do("POST", ana.wpath("/rules"), map[string]any{"pattern": "mercado", "categoryId": categoryByName(t, ana, "expense", "Mercado")}, 201, nil)
	// the market purchase was already typed in: that line needs a person to decide (link or not)
	ana.do("POST", ana.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 4890, "description": "Mercado (digitado)", "categoryId": categoryByName(t, ana, "expense", "Mercado"), "date": "2026-10-01",
	}, 201, nil)
	importFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)

	var out struct {
		Accepted int `json:"accepted"`
	}
	ana.do("POST", ana.wpath("/inbox/accept-suggested"), nil, 200, &out)
	if out.Accepted != 1 {
		t.Fatalf("accepted %d, want only the Uber line", out.Accepted)
	}
	left := pending(t, ana)
	if len(left) != 2 || findAny(left, "UBER") {
		t.Fatalf("pending after accept-suggested: %+v", left)
	}
	var list []transaction
	ana.do("GET", ana.wpath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	for _, tx := range list {
		if tx.Source == "bank" && (tx.Description != "Uber" || tx.CategoryID != transport || tx.AmountCents != 2500) {
			t.Fatalf("the accepted transaction: %+v", tx)
		}
	}
}

// ---- rules ----

func TestRulesCRUDAndValidation(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	cat := categoryByName(t, ana, "expense", "Transporte")

	var created struct {
		Rule ruleDTO `json:"rule"`
	}
	ana.do("POST", ana.wpath("/rules"), map[string]any{"pattern": "  Uber  ", "categoryId": cat}, 201, &created)
	r := created.Rule
	if r.Pattern != "uber" || r.CategoryID != cat {
		t.Fatalf("created: %+v", r) // patterns are stored normalized so "UBER", "Uber" and "über" are one rule
	}
	ana.do("POST", ana.wpath("/rules"), map[string]any{"pattern": "UBER", "categoryId": cat}, 409, nil)

	for name, body := range map[string]map[string]any{
		"empty pattern":    {"pattern": "  ", "categoryId": cat},
		"one character":    {"pattern": "u", "categoryId": cat},
		"very long":        {"pattern": strings.Repeat("a", 61), "categoryId": cat},
		"unknown category": {"pattern": "loja", "categoryId": 99999},
	} {
		if status := ana.status("POST", ana.wpath("/rules"), body); status != 400 {
			t.Errorf("%s: status %d, want 400", name, status)
		}
	}
	ana.do("POST", ana.wpath("/rules"), map[string]any{"pattern": "salario", "categoryId": categoryByName(t, ana, "income", "Salário")}, 201, nil) // income rules are fine

	var list []ruleDTO
	ana.do("GET", ana.wpath("/rules"), nil, 200, &list)
	if len(list) != 2 {
		t.Fatalf("rules: %+v", list)
	}
	ana.do("DELETE", ana.wpath("/rules/%d", r.ID), nil, 204, nil)
	ana.do("DELETE", ana.wpath("/rules/%d", r.ID), nil, 404, nil)
}

func TestCreatingARuleReclassifiesPendingLines(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	importFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)
	if findByDescription(t, pending(t, ana), "UBER").SuggestedCategoryID != nil {
		t.Fatal("no rule yet, so no suggestion")
	}

	transport := categoryByName(t, ana, "expense", "Transporte")
	var res struct {
		Rule    ruleDTO `json:"rule"`
		Updated int     `json:"updated"`
	}
	ana.do("POST", ana.wpath("/rules"), map[string]any{"pattern": "uber", "categoryId": transport}, 201, &res)
	if res.Updated != 1 || res.Rule.Pattern != "uber" {
		t.Fatalf("response: %+v", res)
	}
	if got := findByDescription(t, pending(t, ana), "UBER").SuggestedCategoryID; got == nil || *got != transport {
		t.Fatalf("the pending line was not reclassified: %v", got)
	}
}

func TestDeletingACategoryUsedByARuleRemovesTheRule(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	var c categoryDTO
	ana.do("POST", ana.wpath("/categories"), map[string]any{"kind": "expense", "name": "Pets", "icon": "🐶", "color": "#112233", "essentiality": "nonessential"}, 201, &c)
	ana.do("POST", ana.wpath("/rules"), map[string]any{"pattern": "petshop", "categoryId": c.ID}, 201, nil)
	ana.do("DELETE", ana.wpath("/categories/%d", c.ID), nil, 204, nil) // unused: really deleted
	var list []ruleDTO
	ana.do("GET", ana.wpath("/rules"), nil, 200, &list)
	if len(list) != 0 {
		t.Fatalf("a rule pointing at a deleted category survived: %+v", list)
	}
}

// ---- the inbox as a whole ----

func TestInboxOnlyListsPendingByDefaultAndCanFilter(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	importFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)
	ana.do("POST", ana.wpath("/inbox/%d/ignore", findByDescription(t, pending(t, ana), "UBER").ID), nil, 204, nil)

	var ignored []inboxItem
	ana.do("GET", ana.wpath("/inbox?status=ignored"), nil, 200, &ignored)
	if len(ignored) != 1 || ignored[0].Status != "ignored" {
		t.Fatalf("ignored: %+v", ignored)
	}
	ana.do("GET", ana.wpath("/inbox?status=nonsense"), nil, 400, nil)
}

func TestPendingCountOnTheAccount(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	importFile(t, ana, acc.ID, monthStatement, 200)
	var list []bankAccountDTO
	ana.do("GET", ana.wpath("/bank-accounts"), nil, 200, &list)
	if list[0].PendingCount != 3 {
		t.Fatalf("pending count %d, want 3", list[0].PendingCount)
	}
	ana.do("POST", ana.wpath("/inbox/%d/ignore", pending(t, ana)[0].ID), nil, 204, nil)
	ana.do("GET", ana.wpath("/bank-accounts"), nil, 200, &list)
	if list[0].PendingCount != 2 {
		t.Fatalf("pending count %d, want 2", list[0].PendingCount)
	}
}

func TestDeletingABankAccountKeepsTheTransactionsItCreated(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	importFile(t, ana, acc.ID, monthStatement, 200)
	var tx transaction
	ana.do("POST", ana.wpath("/inbox/%d/accept", findByDescription(t, pending(t, ana), "MERCADO").ID), map[string]any{"categoryId": categoryByName(t, ana, "expense", "Mercado")}, 201, &tx)

	ana.do("DELETE", ana.wpath("/bank-accounts/%d", acc.ID), nil, 204, nil)
	if n := len(pending(t, ana)); n != 0 {
		t.Fatalf("%d pending left from a deleted account", n)
	}
	var list []transaction
	ana.do("GET", ana.wpath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 1 || list[0].ID != tx.ID {
		t.Fatalf("the accepted transaction must survive its account: %+v", list)
	}
}

func TestInboxChangesNotifyOtherMembers(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")
	stream := openStream(t, bia)
	time.Sleep(100 * time.Millisecond) // make sure the subscription on the bus is in place

	importFile(t, ana, newAccount(t, ana, "Conta").ID, monthStatement, 200)
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
