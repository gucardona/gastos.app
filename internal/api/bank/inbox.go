package bank

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"gastos/internal/api/httpx"
	"gastos/internal/api/ledger"
)

// matchRefDTO is the transaction an inbox line was matched to.
type matchRefDTO struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	Date        string `json:"date"`
}

// inboxItemDTO is a statement line in the inbox, with its suggested category and description.
type inboxItemDTO struct {
	ID                   int64        `json:"id"`
	BankAccountID        int64        `json:"bankAccountId"`
	AccountName          string       `json:"accountName"`
	Date                 string       `json:"date"`
	AmountCents          int64        `json:"amountCents"`
	Description          string       `json:"description"`
	Status               string       `json:"status"`
	SuggestedCategoryID  *int64       `json:"suggestedCategoryId"`
	SuggestedDescription *string      `json:"suggestedDescription"`
	SuggestedMatch       *matchRefDTO `json:"suggestedMatch"`
}

// listInbox handles GET /inbox?status=pending|accepted|matched|ignored (pending when omitted).
func (s *Handler) listInbox(w http.ResponseWriter, r *http.Request) error {
	status := r.URL.Query().Get("status")
	switch status {
	case "":
		status = "pending"
	case "pending", "accepted", "matched", "ignored":
	default:
		return httpx.ValidationError(map[string]string{"status": "deve ser pending, accepted, matched ou ignored"})
	}
	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT bt.id, bt.bank_account_id, ba.name, bt.date, bt.amount_cents, bt.description, bt.status,
		       bt.suggested_category_id, bt.suggested_description, m.id, m.description, m.date
		FROM bank_transactions bt
		JOIN bank_accounts ba ON ba.id = bt.bank_account_id
		LEFT JOIN transactions m ON m.id = bt.suggested_match_id
		WHERE ba.wallet_id = ? AND bt.status = ?
		ORDER BY bt.date DESC, bt.id DESC`, httpx.WalletIDFrom(r.Context()), status)
	if err != nil {
		return err
	}
	defer rows.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	out := []inboxItemDTO{}
	for rows.Next() {
		var it inboxItemDTO
		var mID sql.NullInt64
		var mDesc, mDate sql.NullString
		if err := rows.Scan(&it.ID, &it.BankAccountID, &it.AccountName, &it.Date, &it.AmountCents, &it.Description, &it.Status,
			&it.SuggestedCategoryID, &it.SuggestedDescription, &mID, &mDesc, &mDate); err != nil {
			return err
		}
		if mID.Valid {
			it.SuggestedMatch = &matchRefDTO{ID: mID.Int64, Description: mDesc.String, Date: mDate.String}
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// pendingLine is an inbox line plus what accepting it needs from its account.
type pendingLine struct {
	id, amount          int64
	date, desc          string
	suggestedCategoryID sql.NullInt64
	suggestedDesc       sql.NullString
	paidBy              int64
	paymentMethodID     *int64
}

// lockPendingLine loads a line of the wallet and requires it to still be pending.
func lockPendingLine(ctx context.Context, tx *sql.Tx, walletID, id int64) (pendingLine, error) {
	var l pendingLine
	var status string
	err := tx.QueryRowContext(ctx, `
		SELECT bt.id, bt.amount_cents, bt.date, bt.description, bt.suggested_category_id, bt.suggested_description,
		       ba.paid_by_user_id, ba.payment_method_id, bt.status
		FROM bank_transactions bt JOIN bank_accounts ba ON ba.id = bt.bank_account_id
		WHERE bt.id = ? AND ba.wallet_id = ?`, id, walletID,
	).Scan(&l.id, &l.amount, &l.date, &l.desc, &l.suggestedCategoryID, &l.suggestedDesc, &l.paidBy, &l.paymentMethodID, &status)
	if err != nil {
		return l, err
	}
	if status != "pending" {
		return l, httpx.ErrConflict("Esta linha já foi tratada")
	}
	return l, nil
}

// acceptLine creates a "bank" transaction from the line and marks the line accepted.
func acceptLine(ctx context.Context, tx *sql.Tx, walletID, userID int64, l pendingLine, categoryID int64, description string) (ledger.Transaction, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		description = l.desc
		if l.suggestedDesc.String != "" {
			description = l.suggestedDesc.String
		}
	}
	if r := []rune(description); len(r) > 200 {
		description = string(r[:200])
	}
	abs := l.amount
	if abs < 0 {
		abs = -abs
	}
	in := ledger.TransactionInput{Kind: kindOf(l.amount), AmountCents: abs, Description: description, CategoryID: categoryID,
		PaymentMethodID: l.paymentMethodID, Date: l.date, PaidByUserID: l.paidBy}
	txID, err := ledger.SaveTransaction(ctx, tx, walletID, userID, &in, "bank")
	if err != nil {
		return ledger.Transaction{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE bank_transactions SET status = 'accepted', transaction_id = ? WHERE id = ?`, txID, l.id); err != nil {
		return ledger.Transaction{}, err
	}
	return ledger.Transaction{ID: txID, Kind: in.Kind, AmountCents: abs, Description: description, CategoryID: categoryID,
		PaymentMethodID: in.PaymentMethodID, Date: in.Date, PaidByUserID: in.PaidByUserID, CreatedBy: userID,
		Source: "bank", InvoiceDue: in.InvoiceDue, Splits: ledger.WithAmounts(abs, in.Splits)}, nil
}

// acceptInboxItem handles POST /inbox/{id}/accept: the line becomes a transaction.
func (s *Handler) acceptInboxItem(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), httpx.WalletIDFrom(r.Context()), httpx.UserIDFrom(r.Context())
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		CategoryID  int64  `json:"categoryId"`
		Description string `json:"description"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	var out ledger.Transaction
	err = s.InTx(ctx, func(tx *sql.Tx) error {
		l, err := lockPendingLine(ctx, tx, walletID, id)
		if err != nil {
			return err
		}
		out, err = acceptLine(ctx, tx, walletID, userID, l, in.CategoryID, in.Description)
		return err
	})
	if err != nil {
		return err
	}
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusCreated, out)
	return nil
}

// ignoreInboxItem handles POST /inbox/{id}/ignore.
func (s *Handler) ignoreInboxItem(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), httpx.WalletIDFrom(r.Context())
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	err = s.InTx(ctx, func(tx *sql.Tx) error {
		if _, err := lockPendingLine(ctx, tx, walletID, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE bank_transactions SET status = 'ignored' WHERE id = ?`, id)
		return err
	})
	if err != nil {
		return err
	}
	s.Mutated(r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// matchInboxItem says "this line is that transaction I already typed in", so nothing is duplicated.
func (s *Handler) matchInboxItem(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), httpx.WalletIDFrom(r.Context())
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		TransactionID int64 `json:"transactionId"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	err = s.InTx(ctx, func(tx *sql.Tx) error {
		if _, err := lockPendingLine(ctx, tx, walletID, id); err != nil {
			return err
		}
		var exists, linked bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM transactions WHERE id = ? AND wallet_id = ?),
			       EXISTS(SELECT 1 FROM bank_transactions WHERE transaction_id = ?)`,
			in.TransactionID, walletID, in.TransactionID).Scan(&exists, &linked); err != nil {
			return err
		}
		if !exists {
			return httpx.ErrNotFoundMsg("Transação não encontrada")
		}
		if linked {
			return httpx.ErrConflict("Esta transação já está ligada a outra linha do extrato")
		}
		_, err := tx.ExecContext(ctx, `UPDATE bank_transactions SET status = 'matched', transaction_id = ? WHERE id = ?`, in.TransactionID, id)
		return err
	})
	if err != nil {
		return err
	}
	s.Mutated(r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// acceptSuggested accepts every pending line that has a suggested category and no look-alike transaction
// (those need a person to choose between linking and creating). Lines that fail validation stay pending.
func (s *Handler) acceptSuggested(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), httpx.WalletIDFrom(r.Context()), httpx.UserIDFrom(r.Context())
	accepted := 0
	err := s.InTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT bt.id, bt.amount_cents, bt.date, bt.description, bt.suggested_category_id, bt.suggested_description,
			       ba.paid_by_user_id, ba.payment_method_id
			FROM bank_transactions bt JOIN bank_accounts ba ON ba.id = bt.bank_account_id
			WHERE ba.wallet_id = ? AND bt.status = 'pending' AND bt.suggested_category_id IS NOT NULL AND bt.suggested_match_id IS NULL
			ORDER BY bt.date, bt.id`, walletID)
		if err != nil {
			return err
		}
		var lines []pendingLine
		for rows.Next() {
			var l pendingLine
			if err := rows.Scan(&l.id, &l.amount, &l.date, &l.desc, &l.suggestedCategoryID, &l.suggestedDesc, &l.paidBy, &l.paymentMethodID); err != nil {
				rows.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
				return err
			}
			lines = append(lines, l)
		}
		rows.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
		if err := rows.Err(); err != nil {
			return err
		}
		for _, l := range lines {
			if _, err := acceptLine(ctx, tx, walletID, userID, l, l.suggestedCategoryID.Int64, ""); err != nil {
				var ae *httpx.APIError
				if httpx.AsAPIError(err, &ae) && ae.Status == http.StatusBadRequest {
					continue // validation happens before any write, so skipping leaves nothing behind
				}
				return err
			}
			accepted++
		}
		return nil
	})
	if err != nil {
		return err
	}
	if accepted > 0 {
		s.Mutated(r)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]int{"accepted": accepted})
	return nil
}
