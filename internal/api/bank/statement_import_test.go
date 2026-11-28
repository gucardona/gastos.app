package bank_test

import (
	"strings"
	"testing"

	"gastos/internal/api/apitest"
)

func TestImportOFXFillsTheInbox(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta corrente")

	if got := apitest.ImportFile(t, ana, acc.ID, monthStatement, 200); got.Imported != 3 || got.Duplicates != 0 {
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
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	apitest.ImportFile(t, ana, acc.ID, monthStatement, 200)
	if got := apitest.ImportFile(t, ana, acc.ID, monthStatement, 200); got.Imported != 0 || got.Duplicates != 3 {
		t.Fatalf("second import: %+v, want 0 new and 3 duplicates", got)
	}
	if n := len(pending(t, ana)); n != 3 {
		t.Fatalf("%d pending after re-importing, want 3", n)
	}
	// an overlapping export only brings the new lines
	later := apitest.OfxRows("20261006|-25.00|F3|UBER *TRIP 123", "20261010|-10.00|F4|CAFE")
	if got := apitest.ImportFile(t, ana, acc.ID, later, 200); got.Imported != 1 || got.Duplicates != 1 {
		t.Fatalf("overlapping import: %+v", got)
	}
}

func TestTheSameLineOnTwoAccountsIsKeptOnBoth(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	a, b := newAccount(t, ana, "A"), newAccount(t, ana, "B")
	apitest.ImportFile(t, ana, a.ID, monthStatement, 200)
	if got := apitest.ImportFile(t, ana, b.ID, monthStatement, 200); got.Imported != 3 {
		t.Fatalf("the second account: %+v", got)
	}
}

func TestImportRejectsBadFiles(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	path := ana.WPath("/bank-accounts/%d/import", acc.ID)
	post := func(fields map[string]string, content []byte) (int, string) {
		status, raw := ana.Upload(path, fields, "f.txt", content)
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
	badLine := apitest.OfxRows("20261340|-1.00|X|BAD DATE")
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
	ana.Do("POST", path, map[string]any{"format": "ofx"}, 400, nil)
}

func TestImportIsAllOrNothing(t *testing.T) {
	// the second transaction is broken: the first one must not be left behind
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	file := apitest.OfxRows("20261001|-10.00|OK|FINE", "20261002|abc|BAD|BROKEN")
	apitest.ImportFile(t, ana, acc.ID, file, 400)
	if n := len(pending(t, ana)); n != 0 {
		t.Fatalf("%d lines imported from a broken file, want 0", n)
	}
}

func TestImportCSVWithAColumnMapping(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Cartão")
	csv := []byte("Data;Estabelecimento;Valor\n01/10/2026;PADARIA;48,90\n02/10/2026;ESTORNO LOJA;-30,00\n")
	fields := map[string]string{"format": "csv", "dateColumn": "1", "descriptionColumn": "2", "amountColumn": "3", "hasHeader": "true", "invertSign": "true"}
	status, raw := ana.Upload(ana.WPath("/bank-accounts/%d/import", acc.ID), fields, "fatura.csv", csv)
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
		if status, _ := ana.Upload(ana.WPath("/bank-accounts/%d/import", acc.ID), bad, "f.csv", csv); status != 400 {
			t.Errorf("%s: status %d, want 400", name, status)
		}
	}
}

func TestImportIntoADisabledAccountIsRefused(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	acc := newAccount(t, ana, "Conta")
	ana.Do("PUT", ana.WPath("/bank-accounts/%d", acc.ID), map[string]any{"name": "Conta", "enabled": false}, 200, nil)
	apitest.ImportFile(t, ana, acc.ID, monthStatement, 409)
}

// ---- accepting, ignoring and linking ----
