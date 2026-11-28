package ledger

import (
	"context"
	"database/sql"
	"net/http"
	"regexp"
	"strings"

	"gastos/internal/api/httpx"
)

// _colorRe matches a #rrggbb color.
var _colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// validName requires a name of 1 to 40 characters.
func validName(fe httpx.FieldErrors, field, v string) {
	if v == "" || len([]rune(v)) > 40 {
		fe.Add(field, "obrigatório, até 40 caracteres")
	}
}

// validIcon requires an icon of at most 32 bytes.
func validIcon(fe httpx.FieldErrors, v string) {
	if v == "" || len(v) > 32 {
		fe.Add("icon", "obrigatório")
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
	fe := httpx.FieldErrors{}
	if in.Kind != "expense" && in.Kind != "income" {
		fe.Add("kind", "deve ser expense ou income")
	}
	validName(fe, "name", in.Name)
	validIcon(fe, in.Icon)
	if !_colorRe.MatchString(in.Color) {
		fe.Add("color", "use o formato #RRGGBB")
	}
	switch {
	case in.Kind == "expense":
		if in.Essentiality == nil || (*in.Essentiality != "essential" && *in.Essentiality != "nonessential" && *in.Essentiality != "investment") {
			fe.Add("essentiality", "obrigatória para gastos: essential, nonessential ou investment")
		}
	case in.Kind == "income" && in.Essentiality != nil:
		fe.Add("essentiality", "não se aplica a entradas")
	}
	return fe.Err()
}

// categoryCols are the columns scanCategory reads.
const categoryCols = `id, kind, name, icon, color, essentiality, sort_order, archived_at IS NOT NULL`

// scanCategory reads a row selected with categoryCols.
func scanCategory(row interface{ Scan(...any) error }) (categoryDTO, error) {
	var c categoryDTO
	err := row.Scan(&c.ID, &c.Kind, &c.Name, &c.Icon, &c.Color, &c.Essentiality, &c.SortOrder, &c.Archived)
	return c, err
}

// categoryByID returns category id of walletID.
func (s *Handler) categoryByID(ctx context.Context, q httpx.Querier, walletID, id int64) (categoryDTO, error) {
	return scanCategory(q.QueryRowContext(ctx, `SELECT `+categoryCols+` FROM categories WHERE id = ? AND wallet_id = ?`, id, walletID))
}

// listCategories handles GET /categories: every category of the wallet, archived ones included.
func (s *Handler) listCategories(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.DB.QueryContext(r.Context(),
		`SELECT `+categoryCols+` FROM categories WHERE wallet_id = ? ORDER BY kind, sort_order, id`, httpx.WalletIDFrom(r.Context()))
	if err != nil {
		return err
	}
	defer rows.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
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
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// createCategory handles POST /categories.
func (s *Handler) createCategory(w http.ResponseWriter, r *http.Request) error {
	var in categoryInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	if err := in.validate(); err != nil {
		return err
	}
	walletID := httpx.WalletIDFrom(r.Context())
	var id int64
	err := s.DB.QueryRowContext(r.Context(), `
		INSERT INTO categories (wallet_id, kind, name, icon, color, essentiality, sort_order)
		VALUES (?, ?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order), -1) + 1 FROM categories WHERE wallet_id = ? AND kind = ?))
		RETURNING id`, walletID, in.Kind, in.Name, in.Icon, in.Color, in.Essentiality, walletID, in.Kind).Scan(&id)
	if err != nil {
		if httpx.IsUnique(err) {
			return httpx.ErrConflict("Já existe uma categoria com esse nome")
		}
		return err
	}
	c, err := s.categoryByID(r.Context(), s.DB, walletID, id)
	if err != nil {
		return err
	}
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusCreated, c)
	return nil
}

// updateCategory handles PUT /categories/{id}.
func (s *Handler) updateCategory(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in categoryInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	walletID := httpx.WalletIDFrom(r.Context())
	cur, err := s.categoryByID(r.Context(), s.DB, walletID, id)
	if err != nil {
		return err
	}
	if in.Kind != cur.Kind {
		return httpx.ValidationError(map[string]string{"kind": "não pode ser alterado"})
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
	_, err = s.DB.ExecContext(r.Context(), `
		UPDATE categories SET name = ?, icon = ?, color = ?, essentiality = ?, sort_order = ?,
		       archived_at = CASE WHEN ? THEN COALESCE(archived_at, strftime('%Y-%m-%dT%H:%M:%SZ','now')) ELSE NULL END
		WHERE id = ? AND wallet_id = ?`,
		in.Name, in.Icon, in.Color, in.Essentiality, sortOrder, archived, id, walletID)
	if err != nil {
		if httpx.IsUnique(err) {
			return httpx.ErrConflict("Já existe uma categoria com esse nome")
		}
		return err
	}
	c, err := s.categoryByID(r.Context(), s.DB, walletID, id)
	if err != nil {
		return err
	}
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusOK, c)
	return nil
}

// ensureNotLastActive prevents ending up with no active category of the kind.
func (s *Handler) ensureNotLastActive(ctx context.Context, walletID int64, kind string) error {
	var n int
	if err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM categories WHERE wallet_id = ? AND kind = ? AND archived_at IS NULL`, walletID, kind).Scan(&n); err != nil {
		return err
	}
	if n <= 1 {
		return httpx.ErrBadRequest("Mantenha ao menos uma categoria ativa desse tipo")
	}
	return nil
}

// deleteCategory handles DELETE /categories/{id}. A category still used by a transaction or a
// recurrence is archived instead, and its budget removed; the last active category of a kind stays.
func (s *Handler) deleteCategory(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	walletID := httpx.WalletIDFrom(r.Context())
	cur, err := s.categoryByID(r.Context(), s.DB, walletID, id)
	if err != nil {
		return err
	}
	if !cur.Archived {
		if err := s.ensureNotLastActive(r.Context(), walletID, cur.Kind); err != nil {
			return err
		}
	}
	var inUse bool
	if err := s.DB.QueryRowContext(r.Context(), `
		SELECT EXISTS(SELECT 1 FROM transactions WHERE category_id = ?)
		    OR EXISTS(SELECT 1 FROM recurrences WHERE category_id = ?)`, id, id).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		if _, err := s.DB.ExecContext(r.Context(), `
			UPDATE categories SET archived_at = COALESCE(archived_at, strftime('%Y-%m-%dT%H:%M:%SZ','now')) WHERE id = ?`, id); err != nil {
			return err
		}
		//nolint:errcheck,gosec // moved unchanged, see docs/security.md
		s.DB.ExecContext(r.Context(), `DELETE FROM budgets WHERE category_id = ?`, id)
		cur.Archived = true
		s.Mutated(r)
		httpx.WriteJSON(w, http.StatusOK, cur)
		return nil
	}
	if _, err := s.DB.ExecContext(r.Context(), `DELETE FROM categories WHERE id = ? AND wallet_id = ?`, id, walletID); err != nil {
		return err
	}
	s.Mutated(r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

var _ = sql.ErrNoRows
