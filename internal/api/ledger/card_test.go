package ledger_test

import (
	"testing"

	"gastos/internal/api/apitest"
	"gastos/internal/api/ledger"
)

func TestCreateCard(t *testing.T) {
	c := apitest.New(t).Register("Ana", "ana@x.com")
	m := apitest.Card2027(c)
	want := apitest.CardInfo{Issuer: "Itaú Mastercard", CardForm: "virtual", Last4: "1234", BestPurchaseDay: 21}
	check := func(m apitest.PaymentMethod) {
		t.Helper()
		if m.Card == nil || *m.Card.ClosingDay != 20 || *m.Card.DueDay != 27 || m.Card.Issuer != want.Issuer ||
			m.Card.CardForm != want.CardForm || m.Card.Last4 != want.Last4 || m.Card.BestPurchaseDay != 21 {
			t.Fatalf("card = %+v", m.Card)
		}
	}
	check(m)
	var list []apitest.PaymentMethod
	c.Do("GET", c.WPath("/payment-methods"), nil, 200, &list)
	for _, p := range list {
		if p.ID == m.ID {
			check(p)
		} else if p.Card != nil {
			t.Errorf("%s is not a card: %+v", p.Name, p.Card)
		}
	}
}

func TestCardFieldsValidation(t *testing.T) {
	c := apitest.New(t).Register("Ana", "ana@x.com")
	for field, card := range map[string]map[string]any{
		"closingDay": {"dueDay": 27},
		"dueDay":     {"closingDay": 20, "dueDay": 32},
		"last4":      {"closingDay": 20, "dueDay": 27, "last4": "12345"},
		"cardForm":   {"closingDay": 20, "dueDay": 27, "cardForm": "plastic"},
	} {
		var e struct {
			Error struct {
				Fields map[string]string `json:"fields"`
			} `json:"error"`
		}
		c.Do("POST", c.WPath("/payment-methods"), map[string]any{"name": "X " + field, "icon": "💳", "card": card}, 400, &e)
		if e.Error.Fields[field] == "" {
			t.Errorf("%v: want an error on %s, got %v", card, field, e.Error.Fields)
		}
	}
	var e struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	c.Do("POST", c.WPath("/payment-methods"), map[string]any{"name": "Y", "icon": "💳", "card": map[string]any{"closingDay": 1, "dueDay": 2, "last4": "12a4"}}, 400, &e)
	if e.Error.Fields["last4"] == "" {
		t.Errorf("12a4: %v", e.Error.Fields)
	}
}

func TestCardSurvivesArchiveAndTurnsOff(t *testing.T) {
	c := apitest.New(t).Register("Ana", "ana@x.com")
	m := apitest.Card2027(c)
	path := c.WPath("/payment-methods/%d", m.ID)
	// archiving sends only name, icon, sortOrder and archived: the card stays
	c.Do("PUT", path, map[string]any{"name": m.Name, "icon": "💳", "sortOrder": 0, "archived": true}, 200, &m)
	if m.Card == nil || *m.Card.ClosingDay != 20 {
		t.Fatalf("archive dropped the card: %+v", m.Card)
	}
	c.Do("PUT", path, map[string]any{"name": m.Name, "icon": "💳", "archived": false, "card": nil}, 200, &m)
	if m.Card != nil {
		t.Fatalf("card: null should turn it into a plain method, got %+v", m.Card)
	}
}

// due returns the invoice due date of tx, or "<nil>" when it has none.
func due(tx ledger.Transaction) string {
	if tx.InvoiceDue == nil {
		return "<nil>"
	}
	return *tx.InvoiceDue
}

func TestCardPurchaseGetsInvoiceDue(t *testing.T) {
	c := apitest.New(t).Register("Ana", "ana@x.com")
	card := apitest.Card2027(c)
	for _, x := range []struct{ kind, date, want string }{
		{"expense", "2026-11-22", "2026-12-27"},
		{"expense", "2026-11-20", "2026-11-27"},
		{"income", "2026-11-22", "2026-12-27"}, // a refund credits the same invoice
	} {
		if got := due(apitest.Purchase(c, x.kind, x.date, card.ID)); got != x.want {
			t.Errorf("%s on %s: invoiceDue %s, want %s", x.kind, x.date, got, x.want)
		}
	}
	var list []ledger.Transaction
	c.Do("GET", c.WPath("/transactions?from=2026-11-01&to=2026-11-30"), nil, 200, &list)
	if len(list) != 3 || due(list[0]) == "<nil>" {
		t.Fatalf("the list carries invoiceDue too: %+v", list)
	}
}

func TestPlainMethodStampsNothing(t *testing.T) {
	c := apitest.New(t).Register("Ana", "ana@x.com")
	var methods []apitest.PaymentMethod
	c.Do("GET", c.WPath("/payment-methods"), nil, 200, &methods)
	if got := due(apitest.Purchase(c, "expense", "2026-11-22", methods[0].ID)); got != "<nil>" {
		t.Fatalf("invoiceDue %s on %s", got, methods[0].Name)
	}
}

func TestEditKeepsInvoiceWhenDateAndMethodUnchanged(t *testing.T) {
	c := apitest.New(t).Register("Ana", "ana@x.com")
	card := apitest.Card2027(c)
	tx := apitest.Purchase(c, "expense", "2026-11-22", card.ID)
	c.Do("PUT", c.WPath("/payment-methods/%d", card.ID), map[string]any{"name": card.Name, "icon": "💳",
		"card": map[string]any{"closingDay": 25, "dueDay": 27}}, 200, nil)
	body := map[string]any{"kind": "expense", "amountCents": 9900, "description": "Jantar e vinho", "categoryId": tx.CategoryID,
		"paymentMethodId": card.ID, "date": "2026-11-22", "paidByUserId": c.User.ID}
	c.Do("PUT", c.WPath("/transactions/%d", tx.ID), body, 200, &tx)
	if due(tx) != "2026-12-27" {
		t.Fatalf("an edit of the amount moved the purchase: %s", due(tx))
	}
	body["date"] = "2026-11-24" // a new date follows the card's current rule: 24 <= 25
	c.Do("PUT", c.WPath("/transactions/%d", tx.ID), body, 200, &tx)
	if due(tx) != "2026-11-27" {
		t.Fatalf("after changing the date: %s, want 2026-11-27", due(tx))
	}
	var methods []apitest.PaymentMethod
	c.Do("GET", c.WPath("/payment-methods"), nil, 200, &methods)
	body["paymentMethodId"] = methods[0].ID // Pix
	c.Do("PUT", c.WPath("/transactions/%d", tx.ID), body, 200, &tx)
	if due(tx) != "<nil>" {
		t.Fatalf("moving to a plain method keeps invoiceDue %s", due(tx))
	}
}

func TestBankAcceptOnCardAccount(t *testing.T) {
	c := apitest.New(t).Register("Ana", "ana@x.com")
	card := apitest.Card2027(c)
	acc := apitest.NewAccount(t, c, "Cartão")
	c.Do("PUT", c.WPath("/bank-accounts/%d", acc.ID), map[string]any{"name": "Cartão", "paymentMethodId": card.ID, "enabled": true}, 200, nil)
	apitest.ImportFile(t, c, acc.ID, apitest.OfxRows("20261122|-48.90|C1|RESTAURANTE"), 200)
	item := apitest.FindByDescription(t, apitest.Pending(t, c), "RESTAURANTE")
	var tx ledger.Transaction
	c.Do("POST", c.WPath("/inbox/%d/accept", item.ID), map[string]any{"categoryId": apitest.FirstCategory(t, c, "expense")}, 201, &tx)
	if due(tx) != "2026-12-27" {
		t.Fatalf("accepted line: invoiceDue %s", due(tx))
	}
}
