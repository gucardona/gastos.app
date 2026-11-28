package wallet_test

import (
	"testing"

	"gastos/internal/api/apitest"
)

// methodDTO decodes only the id of a payment method.
type methodDTO struct {
	ID int64 `json:"id"`
}

// memberDTO is a wallet member as the API returns it.
type memberDTO struct {
	UserID                 int64  `json:"userId"`
	Name                   string `json:"name"`
	Email                  string `json:"email"`
	Role                   string `json:"role"`
	SplitsByDefault        bool   `json:"splitsByDefault"`
	DefaultShareBP         int    `json:"defaultShareBp"`
	DefaultPaymentMethodID *int64 `json:"defaultPaymentMethodId"`
}
type memberMethod struct {
	UserID                 int64  `json:"userId"`
	DefaultPaymentMethodID *int64 `json:"defaultPaymentMethodId"`
}

func defaultMethodOf(t *testing.T, c *apitest.Client, userID int64) *int64 {
	t.Helper()
	var list []memberMethod
	c.Do("GET", c.WPath("/members"), nil, 200, &list)
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
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	var methods []methodDTO
	ana.Do("GET", ana.WPath("/payment-methods"), nil, 200, &methods)
	pick := methods[1].ID

	if defaultMethodOf(t, ana, ana.User.ID) != nil {
		t.Fatal("no default until someone chooses")
	}
	var m memberMethod
	bia.Do("PUT", bia.WPath("/default-payment-method"), map[string]any{"paymentMethodId": pick}, 200, &m)
	if m.DefaultPaymentMethodID == nil || *m.DefaultPaymentMethodID != pick {
		t.Fatalf("set: %+v", m)
	}
	if got := defaultMethodOf(t, ana, bia.User.ID); got == nil || *got != pick || defaultMethodOf(t, ana, ana.User.ID) != nil {
		t.Fatal("only Bia's default changed")
	}

	other := app.Register("Cleo", "cleo@example.com")
	var theirs []methodDTO
	other.Do("GET", other.WPath("/payment-methods"), nil, 200, &theirs)
	bia.Do("PUT", bia.WPath("/default-payment-method"), map[string]any{"paymentMethodId": theirs[0].ID}, 400, nil)

	bia.Do("PUT", bia.WPath("/default-payment-method"), map[string]any{"paymentMethodId": nil}, 200, &m)
	if m.DefaultPaymentMethodID != nil {
		t.Fatalf("cleared: %+v", m)
	}

	apitest.ShareWallet(ana, other, "reader")
	other.Do("PUT", other.WPath("/default-payment-method"), map[string]any{"paymentMethodId": pick}, 403, nil)
}

func TestListMembers(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	caio := app.Register("Caio", "caio@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	apitest.ShareWallet(ana, caio, "reader")

	var list []memberDTO
	bia.Do("GET", bia.WPath("/members"), nil, 200, &list) // any member can list
	if len(list) != 3 {
		t.Fatalf("%d membros, want 3: %+v", len(list), list)
	}
	// owner first, then editors, then readers
	if list[0].Role != "owner" || list[1].Role != "editor" || list[2].Role != "reader" {
		t.Fatalf("unexpected order: %+v", list)
	}
	if list[0].Email != "ana@example.com" || !list[0].SplitsByDefault {
		t.Fatalf("first member's fields: %+v", list[0])
	}
}

func TestListMembersHiddenFromOutsiders(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	zeca := app.Register("Zeca", "zeca@example.com")
	zeca.WalletID = ana.WalletID
	zeca.Do("GET", zeca.WPath("/members"), nil, 404, nil)
}

func TestUpdateMemberSplitsByDefault(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")

	var m memberDTO
	ana.Do("PATCH", ana.WPath("/members/%d", bia.User.ID), map[string]any{"splitsByDefault": false}, 200, &m)
	if m.SplitsByDefault {
		t.Fatalf("should have left the default split: %+v", m)
	}
	ana.Do("PATCH", ana.WPath("/members/%d", ana.User.ID), map[string]any{"role": "reader"}, 400, nil) // the owner's role cannot change
	ana.Do("PATCH", ana.WPath("/members/%d", bia.User.ID), map[string]any{"role": "owner"}, 400, nil)
	ana.Do("PATCH", ana.WPath("/members/9999"), map[string]any{"role": "reader"}, 404, nil)
}
