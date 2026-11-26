package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"regexp"
	"strings"

	"gastos/internal/billing"
)

// cardDTO is what makes a payment method a credit card: purchases up to closingDay fall in the invoice due on dueDay.
type cardDTO struct {
	ClosingDay      int    `json:"closingDay"`
	DueDay          int    `json:"dueDay"`
	Issuer          string `json:"issuer"`
	CardForm        string `json:"cardForm"` // "", physical or virtual
	Last4           string `json:"last4"`    // only the last 4 digits, never the card number
	BestPurchaseDay int    `json:"bestPurchaseDay"`
}

type cardInput struct {
	ClosingDay *int   `json:"closingDay"`
	DueDay     *int   `json:"dueDay"`
	Issuer     string `json:"issuer"`
	CardForm   string `json:"cardForm"`
	Last4      string `json:"last4"`
}

var last4Re = regexp.MustCompile(`^[0-9]{4}$`)

// cardCols follow paymentMethodCols' base columns; scanCard reads them.
const cardCols = `closing_day, due_day, COALESCE(issuer, ''), COALESCE(card_form, ''), COALESCE(last4, '')`

// validateCard decodes in.Card (absent or null: nothing to set) into in.card, checking each field.
func (in *paymentMethodInput) validateCard(fe fieldErrors) {
	if len(in.Card) == 0 || string(in.Card) == "null" {
		return
	}
	var c cardInput
	dec := json.NewDecoder(bytes.NewReader(in.Card))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		fe.add("card", "cartão inválido")
		return
	}
	c.Issuer, c.Last4 = strings.TrimSpace(c.Issuer), strings.TrimSpace(c.Last4)
	for _, d := range []struct {
		field string
		day   *int
	}{{"closingDay", c.ClosingDay}, {"dueDay", c.DueDay}} {
		if d.day == nil {
			fe.add(d.field, "informe o fechamento e o vencimento")
		} else if *d.day < 1 || *d.day > 31 {
			fe.add(d.field, "dia entre 1 e 31")
		}
	}
	if len([]rune(c.Issuer)) > 60 {
		fe.add("issuer", "use até 60 caracteres")
	}
	if c.CardForm != "" && c.CardForm != "physical" && c.CardForm != "virtual" {
		fe.add("cardForm", "físico ou virtual")
	}
	if c.Last4 != "" && !last4Re.MatchString(c.Last4) {
		fe.add("last4", "use os 4 últimos dígitos")
	}
	in.card = &c
}

// cardArgs are the values for closing_day, due_day, issuer, card_form and last4 (all NULL for a plain method).
func (in *paymentMethodInput) cardArgs() []any {
	c := in.card
	if c == nil {
		return []any{nil, nil, nil, nil, nil}
	}
	return []any{*c.ClosingDay, *c.DueDay, nullIfEmpty(c.Issuer), nullIfEmpty(c.CardForm), nullIfEmpty(c.Last4)}
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// cardScan holds the scanned card columns until toDTO turns them into a card (or nil for a plain method).
type cardScan struct {
	closing, due            sql.NullInt64
	issuer, cardForm, last4 string
}

func (c *cardScan) dest() []any { return []any{&c.closing, &c.due, &c.issuer, &c.cardForm, &c.last4} }

func (c *cardScan) toDTO() *cardDTO {
	if !c.closing.Valid || !c.due.Valid {
		return nil
	}
	closing := int(c.closing.Int64)
	return &cardDTO{ClosingDay: closing, DueDay: int(c.due.Int64), Issuer: c.issuer, CardForm: c.cardForm, Last4: c.last4,
		BestPurchaseDay: billing.BestPurchaseDay(closing)}
}
