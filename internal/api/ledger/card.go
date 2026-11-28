package ledger

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"strings"

	"gastos/internal/api/httpx"
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

// cardInput is the card part of a payment method body: closing and due days, issuer, form, last 4 digits.
type cardInput struct {
	ClosingDay *int   `json:"closingDay"`
	DueDay     *int   `json:"dueDay"`
	Issuer     string `json:"issuer"`
	CardForm   string `json:"cardForm"`
	Last4      string `json:"last4"`
}

// _last4Re matches exactly four digits.
var _last4Re = regexp.MustCompile(`^[0-9]{4}$`)

// cardCols follow paymentMethodCols' base columns; scanCard reads them.
const cardCols = `closing_day, due_day, COALESCE(issuer, ''), COALESCE(card_form, ''), COALESCE(last4, '')`

// validateCard decodes in.Card (absent or null: nothing to set) into in.card, checking each field.
func (in *paymentMethodInput) validateCard(fe httpx.FieldErrors) {
	if len(in.Card) == 0 || string(in.Card) == "null" {
		return
	}
	var c cardInput
	dec := json.NewDecoder(bytes.NewReader(in.Card))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		fe.Add("card", "cartão inválido")
		return
	}
	c.Issuer, c.Last4 = strings.TrimSpace(c.Issuer), strings.TrimSpace(c.Last4)
	for _, d := range []struct {
		field string
		day   *int
	}{{"closingDay", c.ClosingDay}, {"dueDay", c.DueDay}} {
		if d.day == nil {
			fe.Add(d.field, "informe o fechamento e o vencimento")
		} else if *d.day < 1 || *d.day > 31 {
			fe.Add(d.field, "dia entre 1 e 31")
		}
	}
	if len([]rune(c.Issuer)) > 60 {
		fe.Add("issuer", "use até 60 caracteres")
	}
	if c.CardForm != "" && c.CardForm != "physical" && c.CardForm != "virtual" {
		fe.Add("cardForm", "físico ou virtual")
	}
	if c.Last4 != "" && !_last4Re.MatchString(c.Last4) {
		fe.Add("last4", "use os 4 últimos dígitos")
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

// nullIfEmpty returns nil for an empty string, so it is stored as NULL.
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

// dest returns the scan destinations of the card columns, in paymentMethodCols order.
func (c *cardScan) dest() []any { return []any{&c.closing, &c.due, &c.issuer, &c.cardForm, &c.last4} }

// toDTO returns the card, or nil when the method is not a credit card.
func (c *cardScan) toDTO() *cardDTO {
	if !c.closing.Valid || !c.due.Valid {
		return nil
	}
	closing := int(c.closing.Int64)
	return &cardDTO{ClosingDay: closing, DueDay: int(c.due.Int64), Issuer: c.issuer, CardForm: c.cardForm, Last4: c.last4,
		BestPurchaseDay: billing.BestPurchaseDay(closing)}
}

// invoiceDueFor returns the due date of the invoice a purchase on date falls in when the method is a credit card
// (archived or not), and NULL otherwise.
func invoiceDueFor(ctx context.Context, q httpx.Querier, paymentMethodID *int64, date string) (sql.NullString, error) {
	if paymentMethodID == nil {
		return sql.NullString{}, nil
	}
	var closing, dueDay sql.NullInt64
	err := q.QueryRowContext(ctx, `SELECT closing_day, due_day FROM payment_methods WHERE id = ?`, *paymentMethodID).Scan(&closing, &dueDay)
	if err == sql.ErrNoRows {
		return sql.NullString{}, nil // a missing method is reported by validate
	}
	if err != nil || !closing.Valid || !dueDay.Valid {
		return sql.NullString{}, err
	}
	d, err := billing.InvoiceDue(date, int(closing.Int64), int(dueDay.Int64))
	return sql.NullString{String: d, Valid: err == nil}, err
}

// stampInvoice sets in.InvoiceDue from its payment method and date.
func (in *TransactionInput) stampInvoice(ctx context.Context, q httpx.Querier) error {
	d, err := invoiceDueFor(ctx, q, in.PaymentMethodID, in.Date)
	in.InvoiceDue = nil
	if d.Valid {
		in.InvoiceDue = &d.String
	}
	return err
}

// sameID reports whether a and b are both nil or point to equal ids.
func sameID(a, b *int64) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }
