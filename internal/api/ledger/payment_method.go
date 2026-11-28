package ledger

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"gastos/internal/api/httpx"
)

// paymentMethodDTO is a payment method as the API returns it, with the card details when it is a card.
type paymentMethodDTO struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Icon      string   `json:"icon"`
	SortOrder int      `json:"sortOrder"`
	Archived  bool     `json:"archived"`
	Card      *cardDTO `json:"card"` // null: not a credit card
}

// paymentMethodInput is the body of a payment method create or update.
type paymentMethodInput struct {
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	SortOrder *int   `json:"sortOrder"`
	Archived  *bool  `json:"archived"`
	// Card: absent keeps the current card fields (archive and restore send none); null makes it a plain method.
	Card json.RawMessage `json:"card"`
	card *cardInput      // Card decoded by validate when it is an object
}

// validate trims the text fields and checks every field, returning the field errors.
func (in *paymentMethodInput) validate() error {
	in.Name, in.Icon = strings.TrimSpace(in.Name), strings.TrimSpace(in.Icon)
	fe := httpx.FieldErrors{}
	validName(fe, "name", in.Name)
	validIcon(fe, in.Icon)
	in.validateCard(fe)
	return fe.Err()
}

// scanPaymentMethod reads a row selected with paymentMethodCols.
func scanPaymentMethod(row interface{ Scan(...any) error }) (paymentMethodDTO, error) {
	var p paymentMethodDTO
	var c cardScan
	err := row.Scan(append([]any{&p.ID, &p.Name, &p.Icon, &p.SortOrder, &p.Archived}, c.dest()...)...)
	p.Card = c.toDTO()
	return p, err
}

// paymentMethodCols are the columns scanPaymentMethod reads.
const paymentMethodCols = `id, name, icon, sort_order, archived_at IS NOT NULL, ` + cardCols

// paymentMethodByID returns payment method id of walletID.
func (s *Handler) paymentMethodByID(ctx context.Context, walletID, id int64) (paymentMethodDTO, error) {
	return scanPaymentMethod(s.DB.QueryRowContext(ctx,
		`SELECT `+paymentMethodCols+` FROM payment_methods WHERE id = ? AND wallet_id = ?`, id, walletID))
}

// listPaymentMethods handles GET /payment-methods: every method of the wallet, archived ones included.
func (s *Handler) listPaymentMethods(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.DB.QueryContext(r.Context(),
		`SELECT `+paymentMethodCols+` FROM payment_methods WHERE wallet_id = ? ORDER BY sort_order, id`, httpx.WalletIDFrom(r.Context()))
	if err != nil {
		return err
	}
	defer rows.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	out := []paymentMethodDTO{}
	for rows.Next() {
		p, err := scanPaymentMethod(rows)
		if err != nil {
			return err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// createPaymentMethod handles POST /payment-methods.
func (s *Handler) createPaymentMethod(w http.ResponseWriter, r *http.Request) error {
	var in paymentMethodInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	if err := in.validate(); err != nil {
		return err
	}
	walletID := httpx.WalletIDFrom(r.Context())
	var id int64
	err := s.DB.QueryRowContext(r.Context(), `
		INSERT INTO payment_methods (wallet_id, name, icon, sort_order, closing_day, due_day, issuer, card_form, last4)
		VALUES (?, ?, ?, (SELECT COALESCE(MAX(sort_order), -1) + 1 FROM payment_methods WHERE wallet_id = ?), ?, ?, ?, ?, ?)
		RETURNING id`, append([]any{walletID, in.Name, in.Icon, walletID}, in.cardArgs()...)...).Scan(&id)
	if err != nil {
		if httpx.IsUnique(err) {
			return httpx.ErrConflict("Já existe uma forma de pagamento com esse nome")
		}
		return err
	}
	p, err := s.paymentMethodByID(r.Context(), walletID, id)
	if err != nil {
		return err
	}
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusCreated, p)
	return nil
}

// updatePaymentMethod handles PUT /payment-methods/{id}.
func (s *Handler) updatePaymentMethod(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in paymentMethodInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	if err := in.validate(); err != nil {
		return err
	}
	walletID := httpx.WalletIDFrom(r.Context())
	cur, err := s.paymentMethodByID(r.Context(), walletID, id)
	if err != nil {
		return err
	}
	sortOrder := cur.SortOrder
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	}
	archived := cur.Archived
	if in.Archived != nil {
		archived = *in.Archived
	}
	if _, err := s.DB.ExecContext(r.Context(), `
		UPDATE payment_methods SET name = ?, icon = ?, sort_order = ?,
		       archived_at = CASE WHEN ? THEN COALESCE(archived_at, strftime('%Y-%m-%dT%H:%M:%SZ','now')) ELSE NULL END
		WHERE id = ? AND wallet_id = ?`, in.Name, in.Icon, sortOrder, archived, id, walletID); err != nil {
		if httpx.IsUnique(err) {
			return httpx.ErrConflict("Já existe uma forma de pagamento com esse nome")
		}
		return err
	}
	if len(in.Card) > 0 { // absent: the card fields stay as they are
		if _, err := s.DB.ExecContext(r.Context(), `
			UPDATE payment_methods SET closing_day = ?, due_day = ?, issuer = ?, card_form = ?, last4 = ?
			WHERE id = ? AND wallet_id = ?`, append(in.cardArgs(), id, walletID)...); err != nil {
			return err
		}
	}
	p, err := s.paymentMethodByID(r.Context(), walletID, id)
	if err != nil {
		return err
	}
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusOK, p)
	return nil
}

// deletePaymentMethod handles DELETE /payment-methods/{id}. A method still used by a transaction or
// a recurrence is archived instead.
func (s *Handler) deletePaymentMethod(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	walletID := httpx.WalletIDFrom(r.Context())
	cur, err := s.paymentMethodByID(r.Context(), walletID, id)
	if err != nil {
		return err
	}
	var inUse bool
	if err := s.DB.QueryRowContext(r.Context(), `
		SELECT EXISTS(SELECT 1 FROM transactions WHERE payment_method_id = ?)
		    OR EXISTS(SELECT 1 FROM recurrences WHERE payment_method_id = ?)`, id, id).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		if _, err := s.DB.ExecContext(r.Context(), `
			UPDATE payment_methods SET archived_at = COALESCE(archived_at, strftime('%Y-%m-%dT%H:%M:%SZ','now')) WHERE id = ?`, id); err != nil {
			return err
		}
		cur.Archived = true
		s.Mutated(r)
		httpx.WriteJSON(w, http.StatusOK, cur)
		return nil
	}
	if _, err := s.DB.ExecContext(r.Context(), `DELETE FROM payment_methods WHERE id = ? AND wallet_id = ?`, id, walletID); err != nil {
		return err
	}
	s.Mutated(r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
