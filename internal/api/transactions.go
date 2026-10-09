package api

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"gastos/internal/money"
)

const dateLayout = "2006-01-02"

func (s *Server) transactionRoutes(mux *http.ServeMux, wallet routeWrap) {
	const base = "/api/wallets/{walletID}/transactions"
	mux.Handle("GET "+base, wallet(roleReader, s.listTransactions))
	mux.Handle("POST "+base, wallet(roleEditor, s.createTransaction))
	mux.Handle("PUT "+base+"/{id}", wallet(roleEditor, s.updateTransaction))
	mux.Handle("DELETE "+base+"/{id}", wallet(roleEditor, s.deleteTransaction))
}

type split struct {
	UserID      int64 `json:"userId"`
	ShareBP     int64 `json:"shareBp"`
	AmountCents int64 `json:"amountCents"`
}

type transaction struct {
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
	Splits          []split `json:"splits"`
}

type transactionInput struct {
	Kind            string  `json:"kind"`
	AmountCents     int64   `json:"amountCents"`
	Description     string  `json:"description"`
	CategoryID      int64   `json:"categoryId"`
	PaymentMethodID *int64  `json:"paymentMethodId"`
	Date            string  `json:"date"`
	PaidByUserID    int64   `json:"paidByUserId"`
	Splits          []split `json:"splits"`
}

func validDate(s string) bool { _, err := time.Parse(dateLayout, s); return err == nil }

// validate normaliza a entrada e confere as referências dentro da carteira.
// currentCategoryID permite manter uma categoria arquivada que já era da transação (0 na criação).
func (in *transactionInput) validate(ctx context.Context, q querier, walletID, userID, currentCategoryID int64) error {
	in.Description = strings.TrimSpace(in.Description)
	if in.PaidByUserID == 0 {
		in.PaidByUserID = userID
	}
	fe := fieldErrors{}
	if in.Kind != "expense" && in.Kind != "income" {
		fe.add("kind", "deve ser expense ou income")
	}
	if in.AmountCents <= 0 {
		fe.add("amountCents", "deve ser maior que zero")
	}
	if in.Description == "" || len([]rune(in.Description)) > 200 {
		fe.add("description", "obrigatória, até 200 caracteres")
	}
	if !validDate(in.Date) {
		fe.add("date", "data inválida")
	}

	var catKind string
	var archived bool
	err := q.QueryRowContext(ctx, `SELECT kind, archived_at IS NOT NULL FROM categories WHERE id = ? AND wallet_id = ?`,
		in.CategoryID, walletID).Scan(&catKind, &archived)
	switch {
	case err == sql.ErrNoRows:
		fe.add("categoryId", "categoria não encontrada")
	case err != nil:
		return err
	case catKind != in.Kind:
		fe.add("categoryId", "categoria não é do tipo "+in.Kind)
	case archived && in.CategoryID != currentCategoryID:
		fe.add("categoryId", "categoria arquivada")
	}

	if in.PaymentMethodID != nil {
		var ok bool
		if err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM payment_methods WHERE id = ? AND wallet_id = ?)`,
			*in.PaymentMethodID, walletID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			fe.add("paymentMethodId", "forma de pagamento não encontrada")
		}
	}

	var isMember bool
	if err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM wallet_members WHERE wallet_id = ? AND user_id = ?)`,
		walletID, in.PaidByUserID).Scan(&isMember); err != nil {
		return err
	}
	if !isMember {
		fe.add("paidByUserId", "não é membro da carteira")
	}

	if err := fe.err(); err != nil {
		return err
	}
	splits, err := resolveSplits(ctx, q, walletID, in.Kind, in.Splits)
	if err != nil {
		return err
	}
	in.Splits = splits
	return nil
}

// resolveSplits aplica as regras de divisão da spec e devolve os splits finais (sem amountCents).
func resolveSplits(ctx context.Context, q querier, walletID int64, kind string, requested []split) ([]split, error) {
	var splitting bool
	if err := q.QueryRowContext(ctx, `SELECT splitting_enabled FROM wallets WHERE id = ?`, walletID).Scan(&splitting); err != nil {
		return nil, err
	}
	if !splitting || kind != "expense" {
		if len(requested) > 0 {
			return nil, errValidation(map[string]string{"splits": "divisão não se aplica"})
		}
		return nil, nil
	}

	rows, err := q.QueryContext(ctx, `SELECT user_id, splits_by_default FROM wallet_members WHERE wallet_id = ? ORDER BY user_id`, walletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := map[int64]bool{}
	var defaults []int64
	for rows.Next() {
		var id int64
		var byDefault bool
		if err := rows.Scan(&id, &byDefault); err != nil {
			return nil, err
		}
		members[id] = true
		if byDefault {
			defaults = append(defaults, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(requested) == 0 {
		if len(defaults) == 0 {
			return nil, nil
		}
		shares := money.EqualShares(len(defaults))
		out := make([]split, len(defaults))
		for i, id := range defaults {
			out[i] = split{UserID: id, ShareBP: shares[i]}
		}
		return out, nil
	}

	seen := map[int64]bool{}
	var total int64
	for _, sp := range requested {
		switch {
		case !members[sp.UserID]:
			return nil, errValidation(map[string]string{"splits": "participante não é membro da carteira"})
		case seen[sp.UserID]:
			return nil, errValidation(map[string]string{"splits": "participante repetido"})
		case sp.ShareBP < 1 || sp.ShareBP > money.FullShare:
			return nil, errValidation(map[string]string{"splits": "parte deve estar entre 0,01% e 100%"})
		}
		seen[sp.UserID] = true
		total += sp.ShareBP
	}
	if total != money.FullShare {
		return nil, errValidation(map[string]string{"splits": "as partes devem somar 100%"})
	}
	out := make([]split, len(requested))
	for i, sp := range requested {
		out[i] = split{UserID: sp.UserID, ShareBP: sp.ShareBP}
	}
	return out, nil
}

func withAmounts(total int64, splits []split) []split {
	shares := make([]int64, len(splits))
	for i, sp := range splits {
		shares[i] = sp.ShareBP
	}
	amounts := money.Allocate(total, shares)
	out := make([]split, len(splits))
	for i, sp := range splits {
		out[i] = split{UserID: sp.UserID, ShareBP: sp.ShareBP, AmountCents: amounts[i]}
	}
	return out
}

func replaceTransactionSplits(ctx context.Context, q querier, txID int64, splits []split) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM transaction_splits WHERE transaction_id = ?`, txID); err != nil {
		return err
	}
	for _, sp := range splits {
		if _, err := q.ExecContext(ctx, `INSERT INTO transaction_splits (transaction_id, user_id, share_bp) VALUES (?, ?, ?)`,
			txID, sp.UserID, sp.ShareBP); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) listTransactions(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), walletIDFrom(r.Context())
	qs := r.URL.Query()
	from, to := qs.Get("from"), qs.Get("to")
	fe := fieldErrors{}
	if !validDate(from) {
		fe.add("from", "obrigatório (YYYY-MM-DD)")
	}
	if !validDate(to) {
		fe.add("to", "obrigatório (YYYY-MM-DD)")
	}
	if len(fe) == 0 {
		f, _ := time.Parse(dateLayout, from)
		t, _ := time.Parse(dateLayout, to)
		if t.Before(f) || t.Sub(f) > 366*24*time.Hour {
			fe.add("to", "intervalo inválido (máx. 366 dias)")
		}
	}
	if err := fe.err(); err != nil {
		return err
	}

	query := `SELECT id, kind, amount_cents, description, category_id, payment_method_id, date, paid_by_user_id, created_by, recurrence_id
	          FROM transactions WHERE wallet_id = ? AND date BETWEEN ? AND ?`
	args := []any{walletID, from, to}
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

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	list := []transaction{}
	index := map[int64]int{}
	for rows.Next() {
		var t transaction
		if err := rows.Scan(&t.ID, &t.Kind, &t.AmountCents, &t.Description, &t.CategoryID, &t.PaymentMethodID,
			&t.Date, &t.PaidByUserID, &t.CreatedBy, &t.RecurrenceID); err != nil {
			return err
		}
		t.Splits = []split{}
		index[t.ID] = len(list)
		list = append(list, t)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	if err := s.attachSplits(ctx, list, index); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, list)
	return nil
}

// attachSplits carrega os splits de todas as transações da lista com uma única query.
func (s *Server) attachSplits(ctx context.Context, list []transaction, index map[int64]int) error {
	if len(list) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(list))
	args := make([]any, len(list))
	for i, t := range list {
		args[i] = t.ID
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT transaction_id, user_id, share_bp FROM transaction_splits
		 WHERE transaction_id IN (`+placeholders[:len(placeholders)-1]+`) ORDER BY transaction_id, user_id`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var txID int64
		var sp split
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
		list[i].Splits = withAmounts(list[i].AmountCents, list[i].Splits)
	}
	return nil
}

func (s *Server) createTransaction(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), walletIDFrom(r.Context()), userIDFrom(r.Context())
	var in transactionInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	var id int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if err := in.validate(ctx, tx, walletID, userID, 0); err != nil {
			return err
		}
		err := tx.QueryRowContext(ctx, `
			INSERT INTO transactions (wallet_id, kind, amount_cents, description, category_id, payment_method_id, date, paid_by_user_id, created_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			walletID, in.Kind, in.AmountCents, in.Description, in.CategoryID, in.PaymentMethodID, in.Date, in.PaidByUserID, userID,
		).Scan(&id)
		if err != nil {
			return err
		}
		return replaceTransactionSplits(ctx, tx, id, in.Splits)
	})
	if err != nil {
		return err
	}
	out := transaction{ID: id, Kind: in.Kind, AmountCents: in.AmountCents, Description: in.Description,
		CategoryID: in.CategoryID, PaymentMethodID: in.PaymentMethodID, Date: in.Date,
		PaidByUserID: in.PaidByUserID, CreatedBy: userID, Splits: withAmounts(in.AmountCents, in.Splits)}
	s.mutated(r)
	writeJSON(w, http.StatusCreated, out)
	return nil
}

func (s *Server) updateTransaction(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), walletIDFrom(r.Context()), userIDFrom(r.Context())
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in transactionInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	var createdBy int64
	var recurrenceID *int64
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		var currentCat int64
		if err := tx.QueryRowContext(ctx,
			`SELECT category_id, created_by, recurrence_id FROM transactions WHERE id = ? AND wallet_id = ?`, id, walletID,
		).Scan(&currentCat, &createdBy, &recurrenceID); err != nil {
			return err // sql.ErrNoRows → 404 via writeError
		}
		if err := in.validate(ctx, tx, walletID, userID, currentCat); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE transactions SET kind = ?, amount_cents = ?, description = ?, category_id = ?, payment_method_id = ?,
			       date = ?, paid_by_user_id = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
			WHERE id = ? AND wallet_id = ?`,
			in.Kind, in.AmountCents, in.Description, in.CategoryID, in.PaymentMethodID, in.Date, in.PaidByUserID, id, walletID,
		); err != nil {
			return err
		}
		return replaceTransactionSplits(ctx, tx, id, in.Splits)
	})
	if err != nil {
		return err
	}
	out := transaction{ID: id, Kind: in.Kind, AmountCents: in.AmountCents, Description: in.Description,
		CategoryID: in.CategoryID, PaymentMethodID: in.PaymentMethodID, Date: in.Date, PaidByUserID: in.PaidByUserID,
		CreatedBy: createdBy, RecurrenceID: recurrenceID, Splits: withAmounts(in.AmountCents, in.Splits)}
	s.mutated(r)
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) deleteTransaction(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(r.Context(), `DELETE FROM transactions WHERE id = ? AND wallet_id = ?`, id, walletIDFrom(r.Context()))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errNotFound
	}
	s.mutated(r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
