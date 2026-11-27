package api

import (
	"context"
	"testing"

	"gastos/internal/api/httpx"
)

func dupBody(c *client, method int64, extra map[string]any) map[string]any {
	b := map[string]any{"kind": "expense", "amountCents": 5000, "description": "Pix", "categoryId": firstCategory(c.app.t, c, "expense"),
		"paymentMethodId": method, "date": "2026-11-20", "paidByUserId": c.user.ID}
	for k, v := range extra {
		b[k] = v
	}
	return b
}

func errBody(c *client, body map[string]any) (int, httpx.APIError) {
	var out struct{ Error httpx.APIError }
	st := c.doRaw("POST", c.wpath("/transactions"), body, &out)
	return st, out.Error
}

func TestDuplicateRightAfterIsQuestioned(t *testing.T) {
	c := newTestApp(t).register("Ana", "ana@x.com")
	m := card2027(c).ID
	c.do("POST", c.wpath("/transactions"), dupBody(c, m, nil), 201, nil)

	st, e := errBody(c, dupBody(c, m, map[string]any{"description": "pix pro joão"})) // the description never counts
	if st != 409 || e.Code != "duplicate" || e.Message != `Você lançou um igual há poucos minutos: "Pix".` {
		t.Fatalf("got %d %+v", st, e)
	}
	c.do("POST", c.wpath("/transactions"), dupBody(c, m, map[string]any{"allowDuplicate": true}), 201, nil)

	// any difference in amount, date, method, category or kind is a different transaction
	for _, extra := range []map[string]any{{"amountCents": 5001}, {"date": "2026-11-21"}, {"paymentMethodId": nil}, {"kind": "income", "categoryId": firstCategory(t, c, "income")}} {
		c.do("POST", c.wpath("/transactions"), dupBody(c, m, extra), 201, nil)
	}
}

func TestSameEntryHoursLaterBySamePersonPasses(t *testing.T) {
	c := newTestApp(t).register("Ana", "ana@x.com")
	c.do("POST", c.wpath("/transactions"), dupBody(c, 0, map[string]any{"paymentMethodId": nil}), 201, nil)
	if _, err := c.app.s.DB.ExecContext(context.Background(), `UPDATE transactions SET created_at = strftime('%Y-%m-%dT%H:%M:%SZ','now','-16 minutes')`); err != nil {
		t.Fatal(err)
	}
	c.do("POST", c.wpath("/transactions"), dupBody(c, 0, map[string]any{"paymentMethodId": nil}), 201, nil)
}

func TestSameEntryByTheOtherMemberThatDayIsQuestioned(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@x.com")
	bia := app.register("Bia Souza", "bia@x.com")
	ana.do("POST", ana.wpath("/members"), map[string]any{"email": "bia@x.com", "role": "editor"}, 201, nil)
	bia.walletID = ana.walletID
	ana.do("POST", ana.wpath("/transactions"), dupBody(ana, 0, map[string]any{"paymentMethodId": nil, "description": ""}), 201, nil)
	if _, err := app.s.DB.ExecContext(context.Background(), `UPDATE transactions SET created_at = strftime('%Y-%m-%dT%H:%M:%SZ','now','-8 hours')`); err != nil {
		t.Fatal(err)
	}
	st, e := errBody(bia, dupBody(bia, 0, map[string]any{"paymentMethodId": nil}))
	if st != 409 || e.Message != "Ana já lançou um igual neste dia." {
		t.Fatalf("got %d %+v", st, e)
	}
}

func TestUpdateIsNeverQuestioned(t *testing.T) {
	c := newTestApp(t).register("Ana", "ana@x.com")
	c.do("POST", c.wpath("/transactions"), dupBody(c, 0, map[string]any{"paymentMethodId": nil}), 201, nil)
	var tx Transaction
	c.do("POST", c.wpath("/transactions"), dupBody(c, 0, map[string]any{"paymentMethodId": nil, "amountCents": 4000}), 201, &tx)
	c.do("PUT", c.wpath("/transactions/%d", tx.ID), dupBody(c, 0, map[string]any{"paymentMethodId": nil}), 200, nil)
}
