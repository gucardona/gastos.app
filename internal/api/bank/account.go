package bank

import (
	"context"
	"net/http"
	"strings"

	"gastos/internal/api/httpx"
)

// bankAccountDTO is a bank account as the API returns it, with the number of lines waiting in the inbox.
type bankAccountDTO struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	PaymentMethodID *int64 `json:"paymentMethodId"`
	PaidByUserID    int64  `json:"paidByUserId"`
	Enabled         bool   `json:"enabled"`
	PendingCount    int    `json:"pendingCount"`
}

// bankAccountInput is the body of a bank account create or update.
type bankAccountInput struct {
	Name            string `json:"name"`
	PaymentMethodID *int64 `json:"paymentMethodId"`
	PaidByUserID    int64  `json:"paidByUserId"`
	Enabled         *bool  `json:"enabled"`
}

// bankAccountCols is the column list shared by the bank account queries, ending with the pending line count.
const bankAccountCols = `ba.id, ba.name, ba.payment_method_id, ba.paid_by_user_id, ba.enabled,
	(SELECT COUNT(*) FROM bank_transactions bt WHERE bt.bank_account_id = ba.id AND bt.status = 'pending')`

// scanBankAccount reads a row selected with bankAccountCols.
func scanBankAccount(row interface{ Scan(...any) error }) (bankAccountDTO, error) {
	var a bankAccountDTO
	err := row.Scan(&a.ID, &a.Name, &a.PaymentMethodID, &a.PaidByUserID, &a.Enabled, &a.PendingCount)
	return a, err
}

// bankAccountByID returns bank account id of walletID.
func (s *Handler) bankAccountByID(ctx context.Context, q httpx.Querier, walletID, id int64) (bankAccountDTO, error) {
	return scanBankAccount(q.QueryRowContext(ctx,
		`SELECT `+bankAccountCols+` FROM bank_accounts ba WHERE ba.id = ? AND ba.wallet_id = ?`, id, walletID))
}

// validate trims the name and checks every field against walletID, returning the field errors.
func (in *bankAccountInput) validate(ctx context.Context, q httpx.Querier, walletID int64) error {
	in.Name = strings.TrimSpace(in.Name)
	fe := httpx.FieldErrors{}
	if in.Name == "" || len([]rune(in.Name)) > 60 {
		fe.Add("name", "obrigatório, até 60 caracteres")
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
	if in.PaidByUserID != 0 {
		var ok bool
		if err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM wallet_members WHERE wallet_id = ? AND user_id = ?)`,
			walletID, in.PaidByUserID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			fe.Add("paidByUserId", "não é membro da carteira")
		}
	}
	return fe.Err()
}

// listBankAccounts handles GET /bank-accounts.
func (s *Handler) listBankAccounts(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.DB.QueryContext(r.Context(),
		`SELECT `+bankAccountCols+` FROM bank_accounts ba WHERE ba.wallet_id = ? ORDER BY ba.name, ba.id`, httpx.WalletIDFrom(r.Context()))
	if err != nil {
		return err
	}
	defer rows.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	out := []bankAccountDTO{}
	for rows.Next() {
		a, err := scanBankAccount(rows)
		if err != nil {
			return err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// createBankAccount handles POST /bank-accounts.
func (s *Handler) createBankAccount(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), httpx.WalletIDFrom(r.Context()), httpx.UserIDFrom(r.Context())
	var in bankAccountInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	if err := in.validate(ctx, s.DB, walletID); err != nil {
		return err
	}
	if in.PaidByUserID == 0 {
		in.PaidByUserID = userID
	}
	var id int64
	if err := s.DB.QueryRowContext(ctx,
		`INSERT INTO bank_accounts (wallet_id, name, payment_method_id, paid_by_user_id) VALUES (?, ?, ?, ?) RETURNING id`,
		walletID, in.Name, in.PaymentMethodID, in.PaidByUserID).Scan(&id); err != nil {
		return err
	}
	a, err := s.bankAccountByID(ctx, s.DB, walletID, id)
	if err != nil {
		return err
	}
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusCreated, a)
	return nil
}

// updateBankAccount replaces the name and payment method; paidByUserId and enabled are kept when omitted.
func (s *Handler) updateBankAccount(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), httpx.WalletIDFrom(r.Context())
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in bankAccountInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	cur, err := s.bankAccountByID(ctx, s.DB, walletID, id)
	if err != nil {
		return err
	}
	if err := in.validate(ctx, s.DB, walletID); err != nil {
		return err
	}
	if in.PaidByUserID == 0 {
		in.PaidByUserID = cur.PaidByUserID
	}
	enabled := cur.Enabled
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	if _, err := s.DB.ExecContext(ctx,
		`UPDATE bank_accounts SET name = ?, payment_method_id = ?, paid_by_user_id = ?, enabled = ? WHERE id = ? AND wallet_id = ?`,
		in.Name, in.PaymentMethodID, in.PaidByUserID, enabled, id, walletID); err != nil {
		return err
	}
	a, err := s.bankAccountByID(ctx, s.DB, walletID, id)
	if err != nil {
		return err
	}
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusOK, a)
	return nil
}

// deleteBankAccount handles DELETE /bank-accounts/{id}.
func (s *Handler) deleteBankAccount(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	res, err := s.DB.ExecContext(r.Context(), `DELETE FROM bank_accounts WHERE id = ? AND wallet_id = ?`, id, httpx.WalletIDFrom(r.Context()))
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
