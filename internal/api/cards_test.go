package api

import "testing"

type testCard struct {
	ClosingDay      *int   `json:"closingDay"`
	DueDay          *int   `json:"dueDay"`
	Issuer          string `json:"issuer"`
	CardForm        string `json:"cardForm"`
	Last4           string `json:"last4"`
	BestPurchaseDay int    `json:"bestPurchaseDay"`
}

type methodDTO struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Archived bool     `json:"archived"`
	Card     *testCard `json:"card"`
}

func card2027(c *client) methodDTO {
	c.app.t.Helper()
	var m methodDTO
	c.do("POST", c.wpath("/payment-methods"), map[string]any{"name": "Itaú Black", "icon": "💳",
		"card": map[string]any{"closingDay": 20, "dueDay": 27, "issuer": " Itaú Mastercard ", "cardForm": "virtual", "last4": " 1234 "}}, 201, &m)
	return m
}

func TestCreateCard(t *testing.T) {
	c := newTestApp(t).register("Ana", "ana@x.com")
	m := card2027(c)
	want := testCard{Issuer: "Itaú Mastercard", CardForm: "virtual", Last4: "1234", BestPurchaseDay: 21}
	check := func(m methodDTO) {
		t.Helper()
		if m.Card == nil || *m.Card.ClosingDay != 20 || *m.Card.DueDay != 27 || m.Card.Issuer != want.Issuer ||
			m.Card.CardForm != want.CardForm || m.Card.Last4 != want.Last4 || m.Card.BestPurchaseDay != 21 {
			t.Fatalf("card = %+v", m.Card)
		}
	}
	check(m)
	var list []methodDTO
	c.do("GET", c.wpath("/payment-methods"), nil, 200, &list)
	for _, p := range list {
		if p.ID == m.ID {
			check(p)
		} else if p.Card != nil {
			t.Errorf("%s is not a card: %+v", p.Name, p.Card)
		}
	}
}

func TestCardFieldsValidation(t *testing.T) {
	c := newTestApp(t).register("Ana", "ana@x.com")
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
		c.do("POST", c.wpath("/payment-methods"), map[string]any{"name": "X " + field, "icon": "💳", "card": card}, 400, &e)
		if e.Error.Fields[field] == "" {
			t.Errorf("%v: want an error on %s, got %v", card, field, e.Error.Fields)
		}
	}
	var e struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	c.do("POST", c.wpath("/payment-methods"), map[string]any{"name": "Y", "icon": "💳", "card": map[string]any{"closingDay": 1, "dueDay": 2, "last4": "12a4"}}, 400, &e)
	if e.Error.Fields["last4"] == "" {
		t.Errorf("12a4: %v", e.Error.Fields)
	}
}

func TestCardSurvivesArchiveAndTurnsOff(t *testing.T) {
	c := newTestApp(t).register("Ana", "ana@x.com")
	m := card2027(c)
	path := c.wpath("/payment-methods/%d", m.ID)
	// archiving sends only name, icon, sortOrder and archived: the card stays
	c.do("PUT", path, map[string]any{"name": m.Name, "icon": "💳", "sortOrder": 0, "archived": true}, 200, &m)
	if m.Card == nil || *m.Card.ClosingDay != 20 {
		t.Fatalf("archive dropped the card: %+v", m.Card)
	}
	c.do("PUT", path, map[string]any{"name": m.Name, "icon": "💳", "archived": false, "card": nil}, 200, &m)
	if m.Card != nil {
		t.Fatalf("card: null should turn it into a plain method, got %+v", m.Card)
	}
}
