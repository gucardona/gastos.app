package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gastos/internal/bankimport"
)

func (s *Server) bankRoutes(mux *http.ServeMux, wallet routeWrap) {
	const base = "/api/wallets/{walletID}"
	mux.Handle("GET "+base+"/bank-accounts", wallet(roleReader, s.listBankAccounts))
	mux.Handle("POST "+base+"/bank-accounts", wallet(roleEditor, s.createBankAccount))
	mux.Handle("PUT "+base+"/bank-accounts/{id}", wallet(roleEditor, s.updateBankAccount))
	mux.Handle("DELETE "+base+"/bank-accounts/{id}", wallet(roleEditor, s.deleteBankAccount))
	mux.Handle("POST "+base+"/bank-accounts/{id}/import", wallet(roleEditor, s.importStatement))

	mux.Handle("GET "+base+"/inbox", wallet(roleReader, s.listInbox))
	mux.Handle("POST "+base+"/inbox/accept-suggested", wallet(roleEditor, s.acceptSuggested))
	mux.Handle("POST "+base+"/inbox/{id}/accept", wallet(roleEditor, s.acceptInboxItem))
	mux.Handle("POST "+base+"/inbox/{id}/ignore", wallet(roleEditor, s.ignoreInboxItem))
	mux.Handle("POST "+base+"/inbox/{id}/match", wallet(roleEditor, s.matchInboxItem))

	mux.Handle("GET "+base+"/rules", wallet(roleReader, s.listRules))
	mux.Handle("POST "+base+"/rules", wallet(roleEditor, s.createRule))
	mux.Handle("DELETE "+base+"/rules/{id}", wallet(roleEditor, s.deleteRule))
}

// ---- bank accounts ----

type bankAccountDTO struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	PaymentMethodID *int64 `json:"paymentMethodId"`
	PaidByUserID    int64  `json:"paidByUserId"`
	Enabled         bool   `json:"enabled"`
	PendingCount    int    `json:"pendingCount"`
}

type bankAccountInput struct {
	Name            string `json:"name"`
	PaymentMethodID *int64 `json:"paymentMethodId"`
	PaidByUserID    int64  `json:"paidByUserId"`
	Enabled         *bool  `json:"enabled"`
}

const bankAccountCols = `ba.id, ba.name, ba.payment_method_id, ba.paid_by_user_id, ba.enabled,
	(SELECT COUNT(*) FROM bank_transactions bt WHERE bt.bank_account_id = ba.id AND bt.status = 'pending')`

func scanBankAccount(row interface{ Scan(...any) error }) (bankAccountDTO, error) {
	var a bankAccountDTO
	err := row.Scan(&a.ID, &a.Name, &a.PaymentMethodID, &a.PaidByUserID, &a.Enabled, &a.PendingCount)
	return a, err
}

func (s *Server) bankAccountByID(ctx context.Context, q querier, walletID, id int64) (bankAccountDTO, error) {
	return scanBankAccount(q.QueryRowContext(ctx,
		`SELECT `+bankAccountCols+` FROM bank_accounts ba WHERE ba.id = ? AND ba.wallet_id = ?`, id, walletID))
}

func (in *bankAccountInput) validate(ctx context.Context, q querier, walletID int64) error {
	in.Name = strings.TrimSpace(in.Name)
	fe := fieldErrors{}
	if in.Name == "" || len([]rune(in.Name)) > 60 {
		fe.add("name", "obrigatório, até 60 caracteres")
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
	if in.PaidByUserID != 0 {
		var ok bool
		if err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM wallet_members WHERE wallet_id = ? AND user_id = ?)`,
			walletID, in.PaidByUserID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			fe.add("paidByUserId", "não é membro da carteira")
		}
	}
	return fe.err()
}

func (s *Server) listBankAccounts(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT `+bankAccountCols+` FROM bank_accounts ba WHERE ba.wallet_id = ? ORDER BY ba.name, ba.id`, walletIDFrom(r.Context()))
	if err != nil {
		return err
	}
	defer rows.Close()
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
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) createBankAccount(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), walletIDFrom(r.Context()), userIDFrom(r.Context())
	var in bankAccountInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if err := in.validate(ctx, s.db, walletID); err != nil {
		return err
	}
	if in.PaidByUserID == 0 {
		in.PaidByUserID = userID
	}
	var id int64
	if err := s.db.QueryRowContext(ctx,
		`INSERT INTO bank_accounts (wallet_id, name, payment_method_id, paid_by_user_id) VALUES (?, ?, ?, ?) RETURNING id`,
		walletID, in.Name, in.PaymentMethodID, in.PaidByUserID).Scan(&id); err != nil {
		return err
	}
	a, err := s.bankAccountByID(ctx, s.db, walletID, id)
	if err != nil {
		return err
	}
	s.mutated(r)
	writeJSON(w, http.StatusCreated, a)
	return nil
}

// updateBankAccount replaces the name and payment method; paidByUserId and enabled are kept when omitted.
func (s *Server) updateBankAccount(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), walletIDFrom(r.Context())
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in bankAccountInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	cur, err := s.bankAccountByID(ctx, s.db, walletID, id)
	if err != nil {
		return err
	}
	if err := in.validate(ctx, s.db, walletID); err != nil {
		return err
	}
	if in.PaidByUserID == 0 {
		in.PaidByUserID = cur.PaidByUserID
	}
	enabled := cur.Enabled
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE bank_accounts SET name = ?, payment_method_id = ?, paid_by_user_id = ?, enabled = ? WHERE id = ? AND wallet_id = ?`,
		in.Name, in.PaymentMethodID, in.PaidByUserID, enabled, id, walletID); err != nil {
		return err
	}
	a, err := s.bankAccountByID(ctx, s.db, walletID, id)
	if err != nil {
		return err
	}
	s.mutated(r)
	writeJSON(w, http.StatusOK, a)
	return nil
}

func (s *Server) deleteBankAccount(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(r.Context(), `DELETE FROM bank_accounts WHERE id = ? AND wallet_id = ?`, id, walletIDFrom(r.Context()))
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

// ---- rules ----

type ruleDTO struct {
	ID         int64   `json:"id"`
	Pattern    string  `json:"pattern"`
	CategoryID int64   `json:"categoryId"`
	RenameTo   *string `json:"renameTo"`
	Priority   int     `json:"priority"`
}

// loadRules returns the wallet's rules by the kind of their category, so an expense rule never classifies a credit.
// Rules pointing at archived categories are left out: accepting them would be refused anyway.
func loadRules(ctx context.Context, q querier, walletID int64) (map[string][]bankimport.Rule, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT r.id, r.pattern, r.category_id, COALESCE(r.rename_to, ''), r.priority, c.kind
		FROM categorization_rules r JOIN categories c ON c.id = r.category_id
		WHERE r.wallet_id = ? AND c.archived_at IS NULL`, walletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]bankimport.Rule{}
	for rows.Next() {
		var r bankimport.Rule
		var kind string
		if err := rows.Scan(&r.ID, &r.Pattern, &r.CategoryID, &r.RenameTo, &r.Priority, &kind); err != nil {
			return nil, err
		}
		out[kind] = append(out[kind], r)
	}
	return out, rows.Err()
}

func kindOf(amountCents int64) string {
	if amountCents > 0 {
		return "income"
	}
	return "expense"
}

// reclassify recomputes the suggested category of every pending line from the current rules and returns how many changed.
func reclassify(ctx context.Context, tx *sql.Tx, walletID int64) (int, error) {
	rules, err := loadRules(ctx, tx, walletID)
	if err != nil {
		return 0, err
	}
	type line struct {
		id, amount int64
		desc       string
		cat        sql.NullInt64
		rename     sql.NullString
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT bt.id, bt.amount_cents, bt.description, bt.suggested_category_id, bt.suggested_description
		FROM bank_transactions bt JOIN bank_accounts ba ON ba.id = bt.bank_account_id
		WHERE ba.wallet_id = ? AND bt.status = 'pending'`, walletID)
	if err != nil {
		return 0, err
	}
	var lines []line
	for rows.Next() {
		var l line
		if err := rows.Scan(&l.id, &l.amount, &l.desc, &l.cat, &l.rename); err != nil {
			rows.Close()
			return 0, err
		}
		lines = append(lines, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	changed := 0
	for _, l := range lines {
		sug := bankimport.Classify(bankimport.Entry{Description: l.desc, AmountCents: l.amount}, rules[kindOf(l.amount)], nil)
		if l.cat.Int64 == sug.CategoryID && l.rename.String == sug.Description {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE bank_transactions SET suggested_category_id = NULLIF(?, 0), suggested_description = NULLIF(?, '') WHERE id = ?`,
			sug.CategoryID, sug.Description, l.id); err != nil {
			return 0, err
		}
		changed++
	}
	return changed, nil
}

func (s *Server) listRules(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT id, pattern, category_id, rename_to, priority FROM categorization_rules WHERE wallet_id = ? ORDER BY priority DESC, id`,
		walletIDFrom(r.Context()))
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []ruleDTO{}
	for rows.Next() {
		var d ruleDTO
		if err := rows.Scan(&d.ID, &d.Pattern, &d.CategoryID, &d.RenameTo, &d.Priority); err != nil {
			return err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) createRule(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), walletIDFrom(r.Context())
	var in struct {
		Pattern    string  `json:"pattern"`
		CategoryID int64   `json:"categoryId"`
		RenameTo   *string `json:"renameTo"`
		Priority   int     `json:"priority"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	in.Pattern = bankimport.Normalize(in.Pattern)
	rename := ""
	if in.RenameTo != nil {
		rename = strings.TrimSpace(*in.RenameTo)
	}
	fe := fieldErrors{}
	if n := len([]rune(in.Pattern)); n < 2 || n > 60 {
		fe.add("pattern", "de 2 a 60 caracteres")
	}
	if len([]rune(rename)) > 200 {
		fe.add("renameTo", "até 200 caracteres")
	}
	var out struct {
		Rule    ruleDTO `json:"rule"`
		Updated int     `json:"updated"`
	}
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var ok bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM categories WHERE id = ? AND wallet_id = ? AND archived_at IS NULL)`,
			in.CategoryID, walletID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			fe.add("categoryId", "categoria não encontrada")
		}
		if err := fe.err(); err != nil {
			return err
		}
		err := tx.QueryRowContext(ctx, `
			INSERT INTO categorization_rules (wallet_id, pattern, category_id, rename_to, priority)
			VALUES (?, ?, ?, NULLIF(?, ''), ?) RETURNING id, pattern, category_id, rename_to, priority`,
			walletID, in.Pattern, in.CategoryID, rename, in.Priority,
		).Scan(&out.Rule.ID, &out.Rule.Pattern, &out.Rule.CategoryID, &out.Rule.RenameTo, &out.Rule.Priority)
		if isUnique(err) {
			return errConflict("Já existe uma regra com esse texto")
		}
		if err != nil {
			return err
		}
		out.Updated, err = reclassify(ctx, tx, walletID)
		return err
	})
	if err != nil {
		return err
	}
	s.mutated(r)
	writeJSON(w, http.StatusCreated, out)
	return nil
}

func (s *Server) deleteRule(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), walletIDFrom(r.Context())
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM categorization_rules WHERE id = ? AND wallet_id = ?`, id, walletID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errNotFound
		}
		_, err = reclassify(ctx, tx, walletID)
		return err
	})
	if err != nil {
		return err
	}
	s.mutated(r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ---- importing ----

type importResult struct {
	Imported   int `json:"imported"`
	Duplicates int `json:"duplicates"`
}

// importSlack is room for the multipart framing around a file of up to bankimport.MaxFileBytes.
const importSlack = 1 << 20

func (s *Server) importStatement(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), walletIDFrom(r.Context())
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	acc, err := s.bankAccountByID(ctx, s.db, walletID, id)
	if err != nil {
		return err
	}
	if !acc.Enabled {
		return errConflict("Esta conta está desativada")
	}

	r.Body = http.MaxBytesReader(w, r.Body, bankimport.MaxFileBytes+importSlack)
	if err := r.ParseMultipartForm(importSlack); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return errTooLarge
		}
		return errBadRequest("Envie o arquivo do extrato")
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		return errBadRequest("Envie o arquivo do extrato")
	}
	defer file.Close()

	var entries []bankimport.Entry
	switch format := r.FormValue("format"); format {
	case "ofx":
		entries, err = bankimport.ParseOFX(file)
	case "csv":
		var m bankimport.CSVMapping
		if m, err = csvMapping(r); err != nil {
			return err
		}
		entries, err = bankimport.ParseCSV(file, m)
	default:
		return errValidation(map[string]string{"format": "deve ser ofx ou csv"})
	}
	switch {
	case errors.Is(err, bankimport.ErrNotOFX):
		return errBadRequest("Este arquivo não parece ser um OFX.")
	case errors.Is(err, bankimport.ErrNoTransactions):
		return errBadRequest("Nenhuma transação encontrada no arquivo.")
	case errors.Is(err, bankimport.ErrTooLarge):
		return errTooLarge
	case err != nil:
		return errBadRequest("Arquivo inválido: " + err.Error())
	}

	var res importResult
	err = s.inTx(ctx, func(tx *sql.Tx) (err error) {
		res, err = storeEntries(ctx, tx, walletID, id, entries)
		return err
	})
	if err != nil {
		return err
	}
	if res.Imported > 0 {
		s.mutated(r)
	}
	writeJSON(w, http.StatusOK, res)
	return nil
}

var errTooLarge = &apiError{Status: http.StatusRequestEntityTooLarge, Code: "too_large", Message: "Arquivo grande demais (máximo de 5 MB)."}

func csvMapping(r *http.Request) (bankimport.CSVMapping, error) {
	fe := fieldErrors{}
	col := func(field string) int {
		n, err := strconv.Atoi(strings.TrimSpace(r.FormValue(field)))
		if err != nil || n < 1 {
			fe.add(field, "informe o número da coluna (a partir de 1)")
		}
		return n
	}
	m := bankimport.CSVMapping{
		Date: col("dateColumn"), Description: col("descriptionColumn"), Amount: col("amountColumn"),
		HasHeader: r.FormValue("hasHeader") == "true", InvertSign: r.FormValue("invertSign") == "true",
	}
	if err := fe.err(); err != nil {
		return m, err
	}
	if m.Date == m.Description || m.Date == m.Amount || m.Description == m.Amount {
		return m, errBadRequest("Use três colunas diferentes para data, descrição e valor")
	}
	return m, nil
}

// storeEntries inserts the lines that are new, each with a suggested category and a possible existing transaction.
// A transaction is suggested to at most one line, and never one that is already linked to or suggested for another.
func storeEntries(ctx context.Context, tx *sql.Tx, walletID, accountID int64, entries []bankimport.Entry) (importResult, error) {
	var res importResult
	rules, err := loadRules(ctx, tx, walletID)
	if err != nil {
		return res, err
	}
	candidates, err := loadCandidates(ctx, tx, walletID, entries)
	if err != nil {
		return res, err
	}
	for _, e := range entries {
		ins, err := tx.ExecContext(ctx, `
			INSERT INTO bank_transactions (bank_account_id, external_id, date, amount_cents, description)
			VALUES (?, ?, ?, ?, ?) ON CONFLICT (bank_account_id, external_id) DO NOTHING`,
			accountID, e.ExternalID, e.Date, e.AmountCents, e.Description)
		if err != nil {
			return res, err
		}
		if n, _ := ins.RowsAffected(); n == 0 {
			res.Duplicates++
			continue
		}
		res.Imported++

		sug := bankimport.Classify(e, rules[kindOf(e.AmountCents)], candidates)
		if sug.MatchID != 0 {
			for i, c := range candidates {
				if c.ID == sug.MatchID {
					candidates = append(candidates[:i], candidates[i+1:]...)
					break
				}
			}
		}
		if sug.CategoryID == 0 && sug.Description == "" && sug.MatchID == 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE bank_transactions SET suggested_category_id = NULLIF(?, 0), suggested_description = NULLIF(?, ''), suggested_match_id = NULLIF(?, 0)
			WHERE bank_account_id = ? AND external_id = ?`,
			sug.CategoryID, sug.Description, sug.MatchID, accountID, e.ExternalID); err != nil {
			return res, err
		}
	}
	return res, nil
}

// loadCandidates returns the wallet's transactions near the dates in the file that no statement line is tied to yet.
func loadCandidates(ctx context.Context, q querier, walletID int64, entries []bankimport.Entry) ([]bankimport.Candidate, error) {
	lo, hi := entries[0].Date, entries[0].Date
	for _, e := range entries {
		lo, hi = min(lo, e.Date), max(hi, e.Date)
	}
	from, err1 := time.Parse(dateLayout, lo)
	to, err2 := time.Parse(dateLayout, hi)
	if err1 != nil || err2 != nil {
		return nil, nil
	}
	rows, err := q.QueryContext(ctx, `
		SELECT t.id, t.kind, t.amount_cents, t.date FROM transactions t
		WHERE t.wallet_id = ? AND t.date BETWEEN ? AND ?
		  AND NOT EXISTS (SELECT 1 FROM bank_transactions b WHERE b.transaction_id = t.id OR b.suggested_match_id = t.id)`,
		walletID, from.AddDate(0, 0, -3).Format(dateLayout), to.AddDate(0, 0, 3).Format(dateLayout))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []bankimport.Candidate
	for rows.Next() {
		var c bankimport.Candidate
		if err := rows.Scan(&c.ID, &c.Kind, &c.AmountCents, &c.Date); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ---- inbox ----

type matchRefDTO struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	Date        string `json:"date"`
}

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

func (s *Server) listInbox(w http.ResponseWriter, r *http.Request) error {
	status := r.URL.Query().Get("status")
	switch status {
	case "":
		status = "pending"
	case "pending", "accepted", "matched", "ignored":
	default:
		return errValidation(map[string]string{"status": "deve ser pending, accepted, matched ou ignored"})
	}
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT bt.id, bt.bank_account_id, ba.name, bt.date, bt.amount_cents, bt.description, bt.status,
		       bt.suggested_category_id, bt.suggested_description, m.id, m.description, m.date
		FROM bank_transactions bt
		JOIN bank_accounts ba ON ba.id = bt.bank_account_id
		LEFT JOIN transactions m ON m.id = bt.suggested_match_id
		WHERE ba.wallet_id = ? AND bt.status = ?
		ORDER BY bt.date DESC, bt.id DESC`, walletIDFrom(r.Context()), status)
	if err != nil {
		return err
	}
	defer rows.Close()
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
	writeJSON(w, http.StatusOK, out)
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
		return l, errConflict("Esta linha já foi tratada")
	}
	return l, nil
}

// acceptLine creates a "bank" transaction from the line and marks the line accepted.
func acceptLine(ctx context.Context, tx *sql.Tx, walletID, userID int64, l pendingLine, categoryID int64, description string) (transaction, error) {
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
	in := transactionInput{Kind: kindOf(l.amount), AmountCents: abs, Description: description, CategoryID: categoryID,
		PaymentMethodID: l.paymentMethodID, Date: l.date, PaidByUserID: l.paidBy}
	txID, err := saveTransaction(ctx, tx, walletID, userID, &in, "bank")
	if err != nil {
		return transaction{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE bank_transactions SET status = 'accepted', transaction_id = ? WHERE id = ?`, txID, l.id); err != nil {
		return transaction{}, err
	}
	return transaction{ID: txID, Kind: in.Kind, AmountCents: abs, Description: description, CategoryID: categoryID,
		PaymentMethodID: in.PaymentMethodID, Date: in.Date, PaidByUserID: in.PaidByUserID, CreatedBy: userID,
		Source: "bank", Splits: withAmounts(abs, in.Splits)}, nil
}

func (s *Server) acceptInboxItem(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), walletIDFrom(r.Context()), userIDFrom(r.Context())
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		CategoryID  int64  `json:"categoryId"`
		Description string `json:"description"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	var out transaction
	err = s.inTx(ctx, func(tx *sql.Tx) error {
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
	s.mutated(r)
	writeJSON(w, http.StatusCreated, out)
	return nil
}

func (s *Server) ignoreInboxItem(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), walletIDFrom(r.Context())
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := lockPendingLine(ctx, tx, walletID, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE bank_transactions SET status = 'ignored' WHERE id = ?`, id)
		return err
	})
	if err != nil {
		return err
	}
	s.mutated(r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// matchInboxItem says "this line is that transaction I already typed in", so nothing is duplicated.
func (s *Server) matchInboxItem(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), walletIDFrom(r.Context())
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		TransactionID int64 `json:"transactionId"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
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
			return errNotFoundMsg("Transação não encontrada")
		}
		if linked {
			return errConflict("Esta transação já está ligada a outra linha do extrato")
		}
		_, err := tx.ExecContext(ctx, `UPDATE bank_transactions SET status = 'matched', transaction_id = ? WHERE id = ?`, in.TransactionID, id)
		return err
	})
	if err != nil {
		return err
	}
	s.mutated(r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// acceptSuggested accepts every pending line that has a suggested category and no look-alike transaction
// (those need a person to choose between linking and creating). Lines that fail validation stay pending.
func (s *Server) acceptSuggested(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), walletIDFrom(r.Context()), userIDFrom(r.Context())
	accepted := 0
	err := s.inTx(ctx, func(tx *sql.Tx) error {
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
				rows.Close()
				return err
			}
			lines = append(lines, l)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, l := range lines {
			if _, err := acceptLine(ctx, tx, walletID, userID, l, l.suggestedCategoryID.Int64, ""); err != nil {
				var ae *apiError
				if asAPIError(err, &ae) && ae.Status == http.StatusBadRequest {
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
		s.mutated(r)
	}
	writeJSON(w, http.StatusOK, map[string]int{"accepted": accepted})
	return nil
}
