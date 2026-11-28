package bank

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gastos/internal/api/httpx"
	"gastos/internal/bankimport"
)

// importResult reports how many statement lines an import added and how many it skipped as duplicates.
type importResult struct {
	Imported   int `json:"imported"`
	Duplicates int `json:"duplicates"`
}

// importSlack is room for the multipart framing around a file of up to bankimport.MaxFileBytes.
const importSlack = 1 << 20

// importStatement handles POST /bank-accounts/{id}/import: an OFX or CSV statement whose new lines go
// to the inbox.
func (s *Handler) importStatement(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), httpx.WalletIDFrom(r.Context())
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	acc, err := s.bankAccountByID(ctx, s.DB, walletID, id)
	if err != nil {
		return err
	}
	if !acc.Enabled {
		return httpx.ErrConflict("Esta conta está desativada")
	}

	r.Body = http.MaxBytesReader(w, r.Body, bankimport.MaxFileBytes+importSlack)
	if err := r.ParseMultipartForm(importSlack); err != nil { //nolint:gosec // moved unchanged, see docs/security.md
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return errTooLarge
		}
		return httpx.ErrBadRequest("Envie o arquivo do extrato")
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		return httpx.ErrBadRequest("Envie o arquivo do extrato")
	}
	defer file.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md

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
		return httpx.ValidationError(map[string]string{"format": "deve ser ofx ou csv"})
	}
	switch {
	case errors.Is(err, bankimport.ErrNotOFX):
		return httpx.ErrBadRequest("Este arquivo não parece ser um OFX.")
	case errors.Is(err, bankimport.ErrNoTransactions):
		return httpx.ErrBadRequest("Nenhuma transação encontrada no arquivo.")
	case errors.Is(err, bankimport.ErrTooLarge):
		return errTooLarge
	case err != nil:
		return httpx.ErrBadRequest("Arquivo inválido: " + err.Error())
	}

	var res importResult
	err = s.InTx(ctx, func(tx *sql.Tx) (err error) {
		res, err = storeEntries(ctx, tx, walletID, id, entries)
		return err
	})
	if err != nil {
		return err
	}
	if res.Imported > 0 {
		s.Mutated(r)
	}
	httpx.WriteJSON(w, http.StatusOK, res)
	return nil
}

// errTooLarge is the answer for a statement upload over the size cap.
var errTooLarge = &httpx.APIError{Status: http.StatusRequestEntityTooLarge, Code: "too_large", Message: "Arquivo grande demais (máximo de 5 MB)."}

// csvMapping reads from the form which CSV columns hold the date, the amount and the description.
func csvMapping(r *http.Request) (bankimport.CSVMapping, error) {
	fe := httpx.FieldErrors{}
	col := func(field string) int {
		n, err := strconv.Atoi(strings.TrimSpace(r.FormValue(field)))
		if err != nil || n < 1 {
			fe.Add(field, "informe o número da coluna (a partir de 1)")
		}
		return n
	}
	m := bankimport.CSVMapping{
		Date: col("dateColumn"), Description: col("descriptionColumn"), Amount: col("amountColumn"),
		HasHeader: r.FormValue("hasHeader") == "true", InvertSign: r.FormValue("invertSign") == "true",
	}
	if err := fe.Err(); err != nil {
		return m, err
	}
	if m.Date == m.Description || m.Date == m.Amount || m.Description == m.Amount {
		return m, httpx.ErrBadRequest("Use três colunas diferentes para data, descrição e valor")
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
func loadCandidates(ctx context.Context, q httpx.Querier, walletID int64, entries []bankimport.Entry) ([]bankimport.Candidate, error) {
	lo, hi := entries[0].Date, entries[0].Date
	for _, e := range entries {
		lo, hi = min(lo, e.Date), max(hi, e.Date)
	}
	from, err1 := time.Parse(httpx.DateLayout, lo)
	to, err2 := time.Parse(httpx.DateLayout, hi)
	if err1 != nil || err2 != nil {
		return nil, nil
	}
	rows, err := q.QueryContext(ctx, `
		SELECT t.id, t.kind, t.amount_cents, t.date FROM transactions t
		WHERE t.wallet_id = ? AND t.date BETWEEN ? AND ?
		  AND NOT EXISTS (SELECT 1 FROM bank_transactions b WHERE b.transaction_id = t.id OR b.suggested_match_id = t.id)`,
		walletID, from.AddDate(0, 0, -3).Format(httpx.DateLayout), to.AddDate(0, 0, 3).Format(httpx.DateLayout))
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
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
