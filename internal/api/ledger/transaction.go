package ledger

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"gastos/internal/api/httpx"
	"gastos/internal/money"
)

// Transaction is a transaction as the API returns it.
type Transaction struct {
	ID              int64   `json:"id"`
	Kind            string  `json:"kind"`
	AmountCents     int64   `json:"amountCents"`
	Description     string  `json:"description"`
	CategoryID      int64   `json:"categoryId"`
	PaymentMethodID *int64  `json:"paymentMethodId"`
	Date            string  `json:"date"`
	PaidByUserID    int64   `json:"paidByUserId"`
	CreatedBy       int64   `json:"createdBy"`
	RecurrenceID    *int64  `json:"recurrenceId"`
	InvoiceDue      *string `json:"invoiceDue"` // due date of the card invoice it falls in; its month is the month it counts in
	Source          string  `json:"source"`     // manual, recurring or bank
	Splits          []Split `json:"splits"`
}

// TransactionInput is the body of a transaction create or update.
type TransactionInput struct {
	Kind            string  `json:"kind"`
	AmountCents     int64   `json:"amountCents"`
	Description     string  `json:"description"`
	CategoryID      int64   `json:"categoryId"`
	PaymentMethodID *int64  `json:"paymentMethodId"`
	Date            string  `json:"date"`
	PaidByUserID    int64   `json:"paidByUserId"`
	Splits          []Split `json:"splits"`
	AllowDuplicate  bool    `json:"allowDuplicate"` // the person saw the duplicate warning and saves anyway
	InvoiceDue      *string `json:"-"`              // set when it is saved (see stampInvoice); never read from a request body
}

// Stored dates must fall in this range; reads accept any valid date, so older data stays readable.
const minDate, maxDate = "2000-01-01", "2100-12-31"

// dateProblem returns the field error for a date about to be stored, or "" when it is fine.
func dateProblem(s string) string {
	switch {
	case !httpx.ValidDate(s):
		return "data inválida"
	case s < minDate || s > maxDate: // YYYY-MM-DD compares as text
		return "data fora do intervalo aceito"
	}
	return ""
}

// amountProblem returns the field error for an amount about to be stored, or "" when it is fine.
func amountProblem(cents int64) string {
	switch {
	case cents <= 0:
		return "deve ser maior que zero"
	case cents > money.MaxCents:
		return "deve ser no máximo R$ 1 bilhão"
	}
	return ""
}

// validate normalizes the input and checks references within the wallet.
// currentCategoryID allows keeping an archived category the transaction already had (0 on create).
func (in *TransactionInput) validate(ctx context.Context, q httpx.Querier, walletID, userID, currentCategoryID int64) error {
	in.Description = strings.TrimSpace(in.Description)
	if in.PaidByUserID == 0 {
		in.PaidByUserID = userID
	}
	fe := httpx.FieldErrors{}
	if in.Kind != "expense" && in.Kind != "income" {
		fe.Add("kind", "deve ser expense ou income")
	}
	if msg := amountProblem(in.AmountCents); msg != "" {
		fe.Add("amountCents", msg)
	}
	if len([]rune(in.Description)) > 200 {
		fe.Add("description", "use até 200 caracteres")
	}
	if msg := dateProblem(in.Date); msg != "" {
		fe.Add("date", msg)
	}

	var catKind string
	var archived bool
	err := q.QueryRowContext(ctx, `SELECT kind, archived_at IS NOT NULL FROM categories WHERE id = ? AND wallet_id = ?`,
		in.CategoryID, walletID).Scan(&catKind, &archived)
	switch {
	case err == sql.ErrNoRows:
		fe.Add("categoryId", "categoria não encontrada")
	case err != nil:
		return err
	case catKind != in.Kind:
		fe.Add("categoryId", "categoria não é do tipo "+in.Kind)
	case archived && in.CategoryID != currentCategoryID:
		fe.Add("categoryId", "categoria arquivada")
	}

	if in.PaymentMethodID != nil {
		var ok bool
		if err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM payment_methods WHERE id = ? AND wallet_id = ?)`,
			*in.PaymentMethodID, walletID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			fe.Add("paymentMethodId", "forma de pagamento não encontrada")
		}
	}

	var isMember bool
	if err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM wallet_members WHERE wallet_id = ? AND user_id = ?)`,
		walletID, in.PaidByUserID).Scan(&isMember); err != nil {
		return err
	}
	if !isMember {
		fe.Add("paidByUserId", "não é membro da carteira")
	}

	if err := fe.Err(); err != nil {
		return err
	}
	splits, err := resolveSplits(ctx, q, walletID, in.Kind, in.Splits)
	if err != nil {
		return err
	}
	in.Splits = splits
	return nil
}

// listTransactions handles GET /transactions.
func (s *Handler) listTransactions(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), httpx.WalletIDFrom(r.Context())
	qs := r.URL.Query()
	from, to, month := qs.Get("from"), qs.Get("to"), qs.Get("month")
	fe := httpx.FieldErrors{}
	// either a month (a card purchase is listed in its invoice's month) or a date range
	if month != "" {
		if _, _, ok := monthRange(month); !ok || from != "" || to != "" {
			fe.Add("month", "use month (YYYY-MM) ou from e to")
		}
	} else {
		if !httpx.ValidDate(from) {
			fe.Add("from", "obrigatório (YYYY-MM-DD)")
		}
		if !httpx.ValidDate(to) {
			fe.Add("to", "obrigatório (YYYY-MM-DD)")
		}
		if len(fe) == 0 {
			f, _ := time.Parse(httpx.DateLayout, from)
			t, _ := time.Parse(httpx.DateLayout, to)
			if t.Before(f) || t.Sub(f) > 366*24*time.Hour {
				fe.Add("to", "intervalo inválido (máx. 366 dias)")
			}
		}
	}
	if err := fe.Err(); err != nil {
		return err
	}

	query := `SELECT id, kind, amount_cents, description, category_id, payment_method_id, date, paid_by_user_id, created_by, recurrence_id, source, invoice_due
	          FROM transactions WHERE wallet_id = ? AND date BETWEEN ? AND ?`
	args := []any{walletID, from, to}
	if month != "" {
		query = strings.Replace(query, "date BETWEEN ? AND ?", "month = ?", 1)
		args = []any{walletID, month}
	}
	if k := qs.Get("kind"); k != "" {
		query += ` AND kind = ?`
		args = append(args, k)
	}
	if c := qs.Get("categoryId"); c != "" {
		query += ` AND category_id = ?`
		args = append(args, c)
	}
	if term := strings.TrimSpace(qs.Get("q")); term != "" {
		query += ` AND description LIKE '%' || ? || '%'`
		args = append(args, term)
	}
	query += ` ORDER BY date DESC, id DESC`

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	list := []Transaction{}
	index := map[int64]int{}
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.Kind, &t.AmountCents, &t.Description, &t.CategoryID, &t.PaymentMethodID,
			&t.Date, &t.PaidByUserID, &t.CreatedBy, &t.RecurrenceID, &t.Source, &t.InvoiceDue); err != nil {
			return err
		}
		t.Splits = []Split{}
		index[t.ID] = len(list)
		list = append(list, t)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	if err := s.attachSplits(ctx, list, index); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, list)
	return nil
}

// attachSplits loads the splits of every transaction in the list with a single query.
func (s *Handler) attachSplits(ctx context.Context, list []Transaction, index map[int64]int) error {
	if len(list) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(list))
	args := make([]any, len(list))
	for i, t := range list {
		args[i] = t.ID
	}
	//nolint:gosec // moved unchanged, see docs/security.md
	rows, err := s.DB.QueryContext(ctx,
		`SELECT transaction_id, user_id, share_bp FROM transaction_splits
		 WHERE transaction_id IN (`+placeholders[:len(placeholders)-1]+`) ORDER BY transaction_id, user_id`, args...)
	if err != nil {
		return err
	}
	defer rows.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	for rows.Next() {
		var txID int64
		var sp Split
		if err := rows.Scan(&txID, &sp.UserID, &sp.ShareBP); err != nil {
			return err
		}
		i := index[txID]
		list[i].Splits = append(list[i].Splits, sp)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range list {
		list[i].Splits = WithAmounts(list[i].AmountCents, list[i].Splits)
	}
	return nil
}

// SaveTransaction validates in and inserts it, with its splits, inside tx. source says where it came from:
// "manual" (typed by a person) or "bank" (accepted from an imported statement).
func SaveTransaction(ctx context.Context, tx *sql.Tx, walletID, userID int64, in *TransactionInput, source string) (int64, error) {
	if err := in.validate(ctx, tx, walletID, userID, 0); err != nil {
		return 0, err
	}
	if err := in.stampInvoice(ctx, tx); err != nil {
		return 0, err
	}
	if source == "manual" && !in.AllowDuplicate {
		if err := checkDuplicate(ctx, tx, walletID, userID, in); err != nil {
			return 0, err
		}
	}
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO transactions (wallet_id, kind, amount_cents, description, category_id, payment_method_id, date, paid_by_user_id, created_by, source, invoice_due)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		walletID, in.Kind, in.AmountCents, in.Description, in.CategoryID, in.PaymentMethodID, in.Date, in.PaidByUserID, userID, source, in.InvoiceDue,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, replaceTransactionSplits(ctx, tx, id, in.Splits)
}

// createTransaction handles POST /transactions.
func (s *Handler) createTransaction(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), httpx.WalletIDFrom(r.Context()), httpx.UserIDFrom(r.Context())
	var in TransactionInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	var id int64
	err := s.InTx(ctx, func(tx *sql.Tx) (err error) {
		id, err = SaveTransaction(ctx, tx, walletID, userID, &in, "manual")
		return err
	})
	if err != nil {
		return err
	}
	out := Transaction{ID: id, Source: "manual", Kind: in.Kind, AmountCents: in.AmountCents, Description: in.Description,
		CategoryID: in.CategoryID, PaymentMethodID: in.PaymentMethodID, Date: in.Date,
		PaidByUserID: in.PaidByUserID, CreatedBy: userID, InvoiceDue: in.InvoiceDue, Splits: WithAmounts(in.AmountCents, in.Splits)}
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusCreated, out)
	return nil
}

// updateTransaction handles PUT /transactions/{id}.
func (s *Handler) updateTransaction(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), httpx.WalletIDFrom(r.Context()), httpx.UserIDFrom(r.Context())
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in TransactionInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	var createdBy int64
	var recurrenceID *int64
	var source string
	err = s.InTx(ctx, func(tx *sql.Tx) error {
		var currentCat int64
		var currentDate string
		var currentMethod *int64
		if err := tx.QueryRowContext(ctx,
			`SELECT category_id, created_by, recurrence_id, source, date, payment_method_id, invoice_due FROM transactions WHERE id = ? AND wallet_id = ?`, id, walletID,
		).Scan(&currentCat, &createdBy, &recurrenceID, &source, &currentDate, &currentMethod, &in.InvoiceDue); err != nil {
			return err // sql.ErrNoRows → 404 through writeError
		}
		if err := in.validate(ctx, tx, walletID, userID, currentCat); err != nil {
			return err
		}
		// the invoice moves only with the date or the method: fixing the amount keeps the purchase where it was
		if in.Date != currentDate || !sameID(in.PaymentMethodID, currentMethod) {
			if err := in.stampInvoice(ctx, tx); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE transactions SET kind = ?, amount_cents = ?, description = ?, category_id = ?, payment_method_id = ?,
			       date = ?, paid_by_user_id = ?, invoice_due = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
			WHERE id = ? AND wallet_id = ?`,
			in.Kind, in.AmountCents, in.Description, in.CategoryID, in.PaymentMethodID, in.Date, in.PaidByUserID, in.InvoiceDue, id, walletID,
		); err != nil {
			return err
		}
		return replaceTransactionSplits(ctx, tx, id, in.Splits)
	})
	if err != nil {
		return err
	}
	out := Transaction{ID: id, Kind: in.Kind, AmountCents: in.AmountCents, Description: in.Description,
		CategoryID: in.CategoryID, PaymentMethodID: in.PaymentMethodID, Date: in.Date, PaidByUserID: in.PaidByUserID,
		CreatedBy: createdBy, RecurrenceID: recurrenceID, Source: source, InvoiceDue: in.InvoiceDue, Splits: WithAmounts(in.AmountCents, in.Splits)}
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// deleteTransaction handles DELETE /transactions/{id}.
func (s *Handler) deleteTransaction(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	res, err := s.DB.ExecContext(r.Context(), `DELETE FROM transactions WHERE id = ? AND wallet_id = ?`, id, httpx.WalletIDFrom(r.Context()))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return httpx.ErrNotFound
	}
	s.Mutated(r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
