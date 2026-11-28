package ledger_test

import (
	"context"
	"testing"

	"gastos/internal/api/apitest"
	"gastos/internal/api/httpx"
	"gastos/internal/api/ledger"
)

// dupBody builds a manual expense body for the duplicate checks.
func dupBody(c *apitest.Client, method int64, extra map[string]any) map[string]any {
	b := map[string]any{"kind": "expense", "amountCents": 5000, "description": "Pix", "categoryId": apitest.FirstCategory(c.App().T(), c, "expense"),
		"paymentMethodId": method, "date": "2026-11-20", "paidByUserId": c.User.ID}
	for k, v := range extra {
		b[k] = v
	}
	return b
}

// errBody posts body and returns the status and the error of the answer.
func errBody(c *apitest.Client, body map[string]any) (int, httpx.APIError) {
	var out struct{ Error httpx.APIError }
	st := c.DoRaw("POST", c.WPath("/transactions"), body, &out)
	return st, out.Error
}

func TestDuplicateRightAfterIsQuestioned(t *testing.T) {
	c := apitest.New(t).Register("Ana", "ana@x.com")
	m := apitest.Card2027(c).ID
	c.Do("POST", c.WPath("/transactions"), dupBody(c, m, nil), 201, nil)

	st, e := errBody(c, dupBody(c, m, map[string]any{"description": "pix pro joão"})) // the description never counts
	if st != 409 || e.Code != "duplicate" || e.Message != `Você lançou um igual há poucos minutos: "Pix".` {
		t.Fatalf("got %d %+v", st, e)
	}
	c.Do("POST", c.WPath("/transactions"), dupBody(c, m, map[string]any{"allowDuplicate": true}), 201, nil)

	// any difference in amount, date, method, category or kind is a different transaction
	for _, extra := range []map[string]any{{"amountCents": 5001}, {"date": "2026-11-21"}, {"paymentMethodId": nil}, {"kind": "income", "categoryId": apitest.FirstCategory(t, c, "income")}} {
		c.Do("POST", c.WPath("/transactions"), dupBody(c, m, extra), 201, nil)
	}
}

func TestSameEntryHoursLaterBySamePersonPasses(t *testing.T) {
	c := apitest.New(t).Register("Ana", "ana@x.com")
	c.Do("POST", c.WPath("/transactions"), dupBody(c, 0, map[string]any{"paymentMethodId": nil}), 201, nil)
	if _, err := c.App().Server().DB.ExecContext(context.Background(), `UPDATE transactions SET created_at = strftime('%Y-%m-%dT%H:%M:%SZ','now','-16 minutes')`); err != nil {
		t.Fatal(err)
	}
	c.Do("POST", c.WPath("/transactions"), dupBody(c, 0, map[string]any{"paymentMethodId": nil}), 201, nil)
}

func TestSameEntryByTheOtherMemberThatDayIsQuestioned(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@x.com")
	bia := app.Register("Bia Souza", "bia@x.com")
	ana.Do("POST", ana.WPath("/members"), map[string]any{"email": "bia@x.com", "role": "editor"}, 201, nil)
	bia.WalletID = ana.WalletID
	ana.Do("POST", ana.WPath("/transactions"), dupBody(ana, 0, map[string]any{"paymentMethodId": nil, "description": ""}), 201, nil)
	if _, err := app.Server().DB.ExecContext(context.Background(), `UPDATE transactions SET created_at = strftime('%Y-%m-%dT%H:%M:%SZ','now','-8 hours')`); err != nil {
		t.Fatal(err)
	}
	st, e := errBody(bia, dupBody(bia, 0, map[string]any{"paymentMethodId": nil}))
	if st != 409 || e.Message != "Ana já lançou um igual neste dia." {
		t.Fatalf("got %d %+v", st, e)
	}
}

func TestUpdateIsNeverQuestioned(t *testing.T) {
	c := apitest.New(t).Register("Ana", "ana@x.com")
	c.Do("POST", c.WPath("/transactions"), dupBody(c, 0, map[string]any{"paymentMethodId": nil}), 201, nil)
	var tx ledger.Transaction
	c.Do("POST", c.WPath("/transactions"), dupBody(c, 0, map[string]any{"paymentMethodId": nil, "amountCents": 4000}), 201, &tx)
	c.Do("PUT", c.WPath("/transactions/%d", tx.ID), dupBody(c, 0, map[string]any{"paymentMethodId": nil}), 200, nil)
}
