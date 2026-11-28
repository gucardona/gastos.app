package bank

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"gastos/internal/api/httpx"
	"gastos/internal/bankimport"
)

// ruleDTO is a categorization rule as the API returns it.
type ruleDTO struct {
	ID         int64   `json:"id"`
	Pattern    string  `json:"pattern"`
	CategoryID int64   `json:"categoryId"`
	RenameTo   *string `json:"renameTo"`
	Priority   int     `json:"priority"`
}

// loadRules returns the wallet's rules by the kind of their category, so an expense rule never classifies a credit.
// Rules pointing at archived categories are left out: accepting them would be refused anyway.
func loadRules(ctx context.Context, q httpx.Querier, walletID int64) (map[string][]bankimport.Rule, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT r.id, r.pattern, r.category_id, COALESCE(r.rename_to, ''), r.priority, c.kind
		FROM categorization_rules r JOIN categories c ON c.id = r.category_id
		WHERE r.wallet_id = ? AND c.archived_at IS NULL`, walletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
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

// kindOf returns "income" for a positive statement amount and "expense" otherwise.
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
			rows.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
			return 0, err
		}
		lines = append(lines, l)
	}
	rows.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
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

// listRules handles GET /rules, highest priority first.
func (s *Handler) listRules(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.DB.QueryContext(r.Context(),
		`SELECT id, pattern, category_id, rename_to, priority FROM categorization_rules WHERE wallet_id = ? ORDER BY priority DESC, id`,
		httpx.WalletIDFrom(r.Context()))
	if err != nil {
		return err
	}
	defer rows.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
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
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// createRule handles POST /rules.
func (s *Handler) createRule(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), httpx.WalletIDFrom(r.Context())
	var in struct {
		Pattern    string  `json:"pattern"`
		CategoryID int64   `json:"categoryId"`
		RenameTo   *string `json:"renameTo"`
		Priority   int     `json:"priority"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	in.Pattern = bankimport.Normalize(in.Pattern)
	rename := ""
	if in.RenameTo != nil {
		rename = strings.TrimSpace(*in.RenameTo)
	}
	fe := httpx.FieldErrors{}
	if n := len([]rune(in.Pattern)); n < 2 || n > 60 {
		fe.Add("pattern", "de 2 a 60 caracteres")
	}
	if len([]rune(rename)) > 200 {
		fe.Add("renameTo", "até 200 caracteres")
	}
	var out struct {
		Rule    ruleDTO `json:"rule"`
		Updated int     `json:"updated"`
	}
	err := s.InTx(ctx, func(tx *sql.Tx) error {
		var ok bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM categories WHERE id = ? AND wallet_id = ? AND archived_at IS NULL)`,
			in.CategoryID, walletID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			fe.Add("categoryId", "categoria não encontrada")
		}
		if err := fe.Err(); err != nil {
			return err
		}
		err := tx.QueryRowContext(ctx, `
			INSERT INTO categorization_rules (wallet_id, pattern, category_id, rename_to, priority)
			VALUES (?, ?, ?, NULLIF(?, ''), ?) RETURNING id, pattern, category_id, rename_to, priority`,
			walletID, in.Pattern, in.CategoryID, rename, in.Priority,
		).Scan(&out.Rule.ID, &out.Rule.Pattern, &out.Rule.CategoryID, &out.Rule.RenameTo, &out.Rule.Priority)
		if httpx.IsUnique(err) {
			return httpx.ErrConflict("Já existe uma regra com esse texto")
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
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusCreated, out)
	return nil
}

// deleteRule handles DELETE /rules/{id}.
func (s *Handler) deleteRule(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), httpx.WalletIDFrom(r.Context())
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	err = s.InTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM categorization_rules WHERE id = ? AND wallet_id = ?`, id, walletID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return httpx.ErrNotFound
		}
		_, err = reclassify(ctx, tx, walletID)
		return err
	})
	if err != nil {
		return err
	}
	s.Mutated(r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
