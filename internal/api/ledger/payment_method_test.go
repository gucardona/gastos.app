package ledger_test

import (
	"testing"

	"gastos/internal/api/apitest"
	"gastos/internal/api/httpx"
	"gastos/internal/api/ledger"
)

func TestDeleteUnusedPaymentMethodAndEmptyIcon(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	var p ledger.PaymentMethodDTO
	ana.Do("POST", ana.WPath("/payment-methods"), map[string]any{"name": "Cheque", "icon": "🧾"}, 201, &p)
	ana.Do("DELETE", ana.WPath("/payment-methods/%d", p.ID), nil, 204, nil) // unused: really deletes
	ana.Do("PUT", ana.WPath("/payment-methods/%d", p.ID), map[string]any{"name": "x", "icon": "x"}, 404, nil)

	var res struct {
		Error httpx.APIError `json:"error"`
	}
	ana.Do("POST", ana.WPath("/payment-methods"), map[string]any{"name": "Vale", "icon": ""}, 400, &res)
	if res.Error.Fields["icon"] == "" {
		t.Errorf("missing icon error: %+v", res.Error)
	}
	ana.Do("POST", ana.WPath("/payment-methods"), map[string]any{"name": "Vale", "icon": "um ícone muito mais comprido que o limite de bytes"}, 400, nil)
}
