package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

// routeWrap wraps a handler with the wallet middleware at a minimum role.
type routeWrap = func(role, handlerFunc) http.Handler

// catalogRoutes registers the category and payment method routes of a wallet.
func (s *Server) catalogRoutes(mux *http.ServeMux, wallet routeWrap) {
	const base = "/api/wallets/{walletID}"
	mux.Handle("GET "+base+"/categories", wallet(roleReader, s.listCategories))
	mux.Handle("POST "+base+"/categories", wallet(roleEditor, s.createCategory))
	mux.Handle("PUT "+base+"/categories/{id}", wallet(roleEditor, s.updateCategory))
	mux.Handle("DELETE "+base+"/categories/{id}", wallet(roleEditor, s.deleteCategory))
	mux.Handle("GET "+base+"/payment-methods", wallet(roleReader, s.listPaymentMethods))
	mux.Handle("POST "+base+"/payment-methods", wallet(roleEditor, s.createPaymentMethod))
	mux.Handle("PUT "+base+"/payment-methods/{id}", wallet(roleEditor, s.updatePaymentMethod))
	mux.Handle("DELETE "+base+"/payment-methods/{id}", wallet(roleEditor, s.deletePaymentMethod))
}

var _colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// validName requires a name of 1 to 40 characters.
func validName(fe fieldErrors, field, v string) {
	if v == "" || len([]rune(v)) > 40 {
		fe.add(field, "obrigatório, até 40 caracteres")
	}
}

// validIcon requires an icon of at most 32 bytes.
func validIcon(fe fieldErrors, v string) {
	if v == "" || len(v) > 32 {
		fe.add("icon", "obrigatório")
	}
}

// ---- categories ----

// categoryDTO is a category as the API returns it.
type categoryDTO struct {
	ID           int64   `json:"id"`
	Kind         string  `json:"kind"`
	Name         string  `json:"name"`
	Icon         string  `json:"icon"`
	Color        string  `json:"color"`
	Essentiality *string `json:"essentiality"`
	SortOrder    int     `json:"sortOrder"`
	Archived     bool    `json:"archived"`
}

// categoryInput is the body of a category create or update.
type categoryInput struct {
	Kind         string  `json:"kind"`
	Name         string  `json:"name"`
	Icon         string  `json:"icon"`
	Color        string  `json:"color"`
	Essentiality *string `json:"essentiality"`
	SortOrder    *int    `json:"sortOrder"`
	Archived     *bool   `json:"archived"`
}

// validate trims the text fields and checks every field, returning the field errors.
func (in *categoryInput) validate() error {
	in.Name, in.Icon = strings.TrimSpace(in.Name), strings.TrimSpace(in.Icon)
	fe := fieldErrors{}
	if in.Kind != "expense" && in.Kind != "income" {
		fe.add("kind", "deve ser expense ou income")
	}
	validName(fe, "name", in.Name)
	validIcon(fe, in.Icon)
	if !_colorRe.MatchString(in.Color) {
		fe.add("color", "use o formato #RRGGBB")
	}
	switch {
	case in.Kind == "expense":
		if in.Essentiality == nil || (*in.Essentiality != "essential" && *in.Essentiality != "nonessential" && *in.Essentiality != "investment") {
			fe.add("essentiality", "obrigatória para gastos: essential, nonessential ou investment")
		}
	case in.Kind == "income" && in.Essentiality != nil:
		fe.add("essentiality", "não se aplica a entradas")
	}
	return fe.err()
}

const categoryCols = `id, kind, name, icon, color, essentiality, sort_order, archived_at IS NOT NULL`

// scanCategory reads a row selected with categoryCols.
func scanCategory(row interface{ Scan(...any) error }) (categoryDTO, error) {
	var c categoryDTO
	err := row.Scan(&c.ID, &c.Kind, &c.Name, &c.Icon, &c.Color, &c.Essentiality, &c.SortOrder, &c.Archived)
	return c, err
}

// categoryByID returns category id of walletID.
func (s *Server) categoryByID(ctx context.Context, q querier, walletID, id int64) (categoryDTO, error) {
	return scanCategory(q.QueryRowContext(ctx, `SELECT `+categoryCols+` FROM categories WHERE id = ? AND wallet_id = ?`, id, walletID))
}

// listCategories handles GET /categories: every category of the wallet, archived ones included.
func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT `+categoryCols+` FROM categories WHERE wallet_id = ? ORDER BY kind, sort_order, id`, walletIDFrom(r.Context()))
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []categoryDTO{}
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// createCategory handles POST /categories.
func (s *Server) createCategory(w http.ResponseWriter, r *http.Request) error {
	var in categoryInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if err := in.validate(); err != nil {
		return err
	}
	walletID := walletIDFrom(r.Context())
	var id int64
	err := s.db.QueryRowContext(r.Context(), `
		INSERT INTO categories (wallet_id, kind, name, icon, color, essentiality, sort_order)
		VALUES (?, ?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order), -1) + 1 FROM categories WHERE wallet_id = ? AND kind = ?))
		RETURNING id`, walletID, in.Kind, in.Name, in.Icon, in.Color, in.Essentiality, walletID, in.Kind).Scan(&id)
	if err != nil {
		if isUnique(err) {
			return errConflict("Já existe uma categoria com esse nome")
		}
		return err
	}
	c, err := s.categoryByID(r.Context(), s.db, walletID, id)
	if err != nil {
		return err
	}
	s.mutated(r)
	writeJSON(w, http.StatusCreated, c)
	return nil
}

// updateCategory handles PUT /categories/{id}.
func (s *Server) updateCategory(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in categoryInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	walletID := walletIDFrom(r.Context())
	cur, err := s.categoryByID(r.Context(), s.db, walletID, id)
	if err != nil {
		return err
	}
	if in.Kind != cur.Kind {
		return errValidation(map[string]string{"kind": "não pode ser alterado"})
	}
	if err := in.validate(); err != nil {
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
	if archived && !cur.Archived {
		if err := s.ensureNotLastActive(r.Context(), walletID, cur.Kind); err != nil {
			return err
		}
	}
	_, err = s.db.ExecContext(r.Context(), `
		UPDATE categories SET name = ?, icon = ?, color = ?, essentiality = ?, sort_order = ?,
		       archived_at = CASE WHEN ? THEN COALESCE(archived_at, strftime('%Y-%m-%dT%H:%M:%SZ','now')) ELSE NULL END
		WHERE id = ? AND wallet_id = ?`,
		in.Name, in.Icon, in.Color, in.Essentiality, sortOrder, archived, id, walletID)
	if err != nil {
		if isUnique(err) {
			return errConflict("Já existe uma categoria com esse nome")
		}
		return err
	}
	c, err := s.categoryByID(r.Context(), s.db, walletID, id)
	if err != nil {
		return err
	}
	s.mutated(r)
	writeJSON(w, http.StatusOK, c)
	return nil
}

// ensureNotLastActive prevents ending up with no active category of the kind.
func (s *Server) ensureNotLastActive(ctx context.Context, walletID int64, kind string) error {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM categories WHERE wallet_id = ? AND kind = ? AND archived_at IS NULL`, walletID, kind).Scan(&n); err != nil {
		return err
	}
	if n <= 1 {
		return errBadRequest("Mantenha ao menos uma categoria ativa desse tipo")
	}
	return nil
}

// deleteCategory handles DELETE /categories/{id}. A category still used by a transaction or a
// recurrence is archived instead, and its budget removed; the last active category of a kind stays.
func (s *Server) deleteCategory(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	walletID := walletIDFrom(r.Context())
	cur, err := s.categoryByID(r.Context(), s.db, walletID, id)
	if err != nil {
		return err
	}
	if !cur.Archived {
		if err := s.ensureNotLastActive(r.Context(), walletID, cur.Kind); err != nil {
			return err
		}
	}
	var inUse bool
	if err := s.db.QueryRowContext(r.Context(), `
		SELECT EXISTS(SELECT 1 FROM transactions WHERE category_id = ?)
		    OR EXISTS(SELECT 1 FROM recurrences WHERE category_id = ?)`, id, id).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		if _, err := s.db.ExecContext(r.Context(), `
			UPDATE categories SET archived_at = COALESCE(archived_at, strftime('%Y-%m-%dT%H:%M:%SZ','now')) WHERE id = ?`, id); err != nil {
			return err
		}
		s.db.ExecContext(r.Context(), `DELETE FROM budgets WHERE category_id = ?`, id)
		cur.Archived = true
		s.mutated(r)
		writeJSON(w, http.StatusOK, cur)
		return nil
	}
	if _, err := s.db.ExecContext(r.Context(), `DELETE FROM categories WHERE id = ? AND wallet_id = ?`, id, walletID); err != nil {
		return err
	}
	s.mutated(r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ---- payment methods ----

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
	fe := fieldErrors{}
	validName(fe, "name", in.Name)
	validIcon(fe, in.Icon)
	in.validateCard(fe)
	return fe.err()
}

// scanPaymentMethod reads a row selected with paymentMethodCols.
func scanPaymentMethod(row interface{ Scan(...any) error }) (paymentMethodDTO, error) {
	var p paymentMethodDTO
	var c cardScan
	err := row.Scan(append([]any{&p.ID, &p.Name, &p.Icon, &p.SortOrder, &p.Archived}, c.dest()...)...)
	p.Card = c.toDTO()
	return p, err
}

const paymentMethodCols = `id, name, icon, sort_order, archived_at IS NOT NULL, ` + cardCols

// paymentMethodByID returns payment method id of walletID.
func (s *Server) paymentMethodByID(ctx context.Context, walletID, id int64) (paymentMethodDTO, error) {
	return scanPaymentMethod(s.db.QueryRowContext(ctx,
		`SELECT `+paymentMethodCols+` FROM payment_methods WHERE id = ? AND wallet_id = ?`, id, walletID))
}

// listPaymentMethods handles GET /payment-methods: every method of the wallet, archived ones included.
func (s *Server) listPaymentMethods(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT `+paymentMethodCols+` FROM payment_methods WHERE wallet_id = ? ORDER BY sort_order, id`, walletIDFrom(r.Context()))
	if err != nil {
		return err
	}
	defer rows.Close()
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
	writeJSON(w, http.StatusOK, out)
	return nil
}

// createPaymentMethod handles POST /payment-methods.
func (s *Server) createPaymentMethod(w http.ResponseWriter, r *http.Request) error {
	var in paymentMethodInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if err := in.validate(); err != nil {
		return err
	}
	walletID := walletIDFrom(r.Context())
	var id int64
	err := s.db.QueryRowContext(r.Context(), `
		INSERT INTO payment_methods (wallet_id, name, icon, sort_order, closing_day, due_day, issuer, card_form, last4)
		VALUES (?, ?, ?, (SELECT COALESCE(MAX(sort_order), -1) + 1 FROM payment_methods WHERE wallet_id = ?), ?, ?, ?, ?, ?)
		RETURNING id`, append([]any{walletID, in.Name, in.Icon, walletID}, in.cardArgs()...)...).Scan(&id)
	if err != nil {
		if isUnique(err) {
			return errConflict("Já existe uma forma de pagamento com esse nome")
		}
		return err
	}
	p, err := s.paymentMethodByID(r.Context(), walletID, id)
	if err != nil {
		return err
	}
	s.mutated(r)
	writeJSON(w, http.StatusCreated, p)
	return nil
}

// updatePaymentMethod handles PUT /payment-methods/{id}.
func (s *Server) updatePaymentMethod(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in paymentMethodInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if err := in.validate(); err != nil {
		return err
	}
	walletID := walletIDFrom(r.Context())
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
	if _, err := s.db.ExecContext(r.Context(), `
		UPDATE payment_methods SET name = ?, icon = ?, sort_order = ?,
		       archived_at = CASE WHEN ? THEN COALESCE(archived_at, strftime('%Y-%m-%dT%H:%M:%SZ','now')) ELSE NULL END
		WHERE id = ? AND wallet_id = ?`, in.Name, in.Icon, sortOrder, archived, id, walletID); err != nil {
		if isUnique(err) {
			return errConflict("Já existe uma forma de pagamento com esse nome")
		}
		return err
	}
	if len(in.Card) > 0 { // absent: the card fields stay as they are
		if _, err := s.db.ExecContext(r.Context(), `
			UPDATE payment_methods SET closing_day = ?, due_day = ?, issuer = ?, card_form = ?, last4 = ?
			WHERE id = ? AND wallet_id = ?`, append(in.cardArgs(), id, walletID)...); err != nil {
			return err
		}
	}
	p, err := s.paymentMethodByID(r.Context(), walletID, id)
	if err != nil {
		return err
	}
	s.mutated(r)
	writeJSON(w, http.StatusOK, p)
	return nil
}

// deletePaymentMethod handles DELETE /payment-methods/{id}. A method still used by a transaction or
// a recurrence is archived instead.
func (s *Server) deletePaymentMethod(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	walletID := walletIDFrom(r.Context())
	cur, err := s.paymentMethodByID(r.Context(), walletID, id)
	if err != nil {
		return err
	}
	var inUse bool
	if err := s.db.QueryRowContext(r.Context(), `
		SELECT EXISTS(SELECT 1 FROM transactions WHERE payment_method_id = ?)
		    OR EXISTS(SELECT 1 FROM recurrences WHERE payment_method_id = ?)`, id, id).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		if _, err := s.db.ExecContext(r.Context(), `
			UPDATE payment_methods SET archived_at = COALESCE(archived_at, strftime('%Y-%m-%dT%H:%M:%SZ','now')) WHERE id = ?`, id); err != nil {
			return err
		}
		cur.Archived = true
		s.mutated(r)
		writeJSON(w, http.StatusOK, cur)
		return nil
	}
	if _, err := s.db.ExecContext(r.Context(), `DELETE FROM payment_methods WHERE id = ? AND wallet_id = ?`, id, walletID); err != nil {
		return err
	}
	s.mutated(r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

var _ = sql.ErrNoRows
