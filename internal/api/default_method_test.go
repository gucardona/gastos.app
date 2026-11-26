package api

import "testing"

type memberMethod struct {
	UserID                 int64  `json:"userId"`
	DefaultPaymentMethodID *int64 `json:"defaultPaymentMethodId"`
}

func defaultMethodOf(t *testing.T, c *client, userID int64) *int64 {
	t.Helper()
	var list []memberMethod
	c.do("GET", c.wpath("/members"), nil, 200, &list)
	for _, m := range list {
		if m.UserID == userID {
			return m.DefaultPaymentMethodID
		}
	}
	t.Fatalf("member %d not listed: %+v", userID, list)
	return nil
}

// Each member chooses their own default; an editor may, a reader may not, and only a method of the wallet is accepted.
func TestDefaultPaymentMethodIsPerMember(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")
	var methods []methodDTO
	ana.do("GET", ana.wpath("/payment-methods"), nil, 200, &methods)
	pick := methods[1].ID

	if defaultMethodOf(t, ana, ana.user.ID) != nil {
		t.Fatal("no default until someone chooses")
	}
	var m memberMethod
	bia.do("PUT", bia.wpath("/default-payment-method"), map[string]any{"paymentMethodId": pick}, 200, &m)
	if m.DefaultPaymentMethodID == nil || *m.DefaultPaymentMethodID != pick {
		t.Fatalf("set: %+v", m)
	}
	if got := defaultMethodOf(t, ana, bia.user.ID); got == nil || *got != pick || defaultMethodOf(t, ana, ana.user.ID) != nil {
		t.Fatal("only Bia's default changed")
	}

	other := app.register("Cleo", "cleo@example.com")
	var theirs []methodDTO
	other.do("GET", other.wpath("/payment-methods"), nil, 200, &theirs)
	bia.do("PUT", bia.wpath("/default-payment-method"), map[string]any{"paymentMethodId": theirs[0].ID}, 400, nil)

	bia.do("PUT", bia.wpath("/default-payment-method"), map[string]any{"paymentMethodId": nil}, 200, &m)
	if m.DefaultPaymentMethodID != nil {
		t.Fatalf("cleared: %+v", m)
	}

	shareWallet(ana, other, "reader")
	other.do("PUT", other.wpath("/default-payment-method"), map[string]any{"paymentMethodId": pick}, 403, nil)
}
