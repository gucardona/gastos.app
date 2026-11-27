package api

import (
	"net/http"

	"gastos/internal/api/httpx"
)

// budgetRoutes registers the budget routes of a wallet.
func (s *Server) budgetRoutes(mux *http.ServeMux, wallet httpx.RouteWrap) {
	const base = "/api/wallets/{walletID}/budgets"
	mux.Handle("GET "+base, wallet(httpx.RoleReader, s.listBudgets))
	mux.Handle("PUT "+base+"/{categoryID}", wallet(httpx.RoleEditor, s.putBudget))
	mux.Handle("DELETE "+base+"/{categoryID}", wallet(httpx.RoleEditor, s.deleteBudget))
}

// budgetDTO is a monthly spending limit for a category.
type budgetDTO struct {
	CategoryID int64 `json:"categoryId"`
	LimitCents int64 `json:"limitCents"`
}

// listBudgets handles GET /budgets.
func (s *Server) listBudgets(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.DB.QueryContext(r.Context(),
		`SELECT category_id, limit_cents FROM budgets WHERE wallet_id = ? ORDER BY category_id`, httpx.WalletIDFrom(r.Context()))
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []budgetDTO{}
	for rows.Next() {
		var b budgetDTO
		if err := rows.Scan(&b.CategoryID, &b.LimitCents); err != nil {
			return err
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// putBudget handles PUT /budgets/{categoryID}: creates or replaces the category's limit.
func (s *Server) putBudget(w http.ResponseWriter, r *http.Request) error {
	catID, err := httpx.PathID(r, "categoryID")
	if err != nil {
		return err
	}
	var in struct {
		LimitCents int64 `json:"limitCents"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	if in.LimitCents <= 0 {
		return httpx.ValidationError(map[string]string{"limitCents": "deve ser maior que zero"})
	}
	walletID := httpx.WalletIDFrom(r.Context())
	cat, err := s.categoryByID(r.Context(), s.DB, walletID, catID)
	if err != nil {
		return err
	}
	if cat.Kind != "expense" || cat.Archived {
		return httpx.ValidationError(map[string]string{"categoryId": "orçamento só vale para categorias de gasto ativas"})
	}
	if _, err := s.DB.ExecContext(r.Context(), `
		INSERT INTO budgets (wallet_id, category_id, limit_cents) VALUES (?, ?, ?)
		ON CONFLICT (wallet_id, category_id) DO UPDATE SET limit_cents = excluded.limit_cents`,
		walletID, catID, in.LimitCents); err != nil {
		return err
	}
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusOK, budgetDTO{CategoryID: catID, LimitCents: in.LimitCents})
	return nil
}

// deleteBudget handles DELETE /budgets/{categoryID}.
func (s *Server) deleteBudget(w http.ResponseWriter, r *http.Request) error {
	catID, err := httpx.PathID(r, "categoryID")
	if err != nil {
		return err
	}
	res, err := s.DB.ExecContext(r.Context(), `DELETE FROM budgets WHERE wallet_id = ? AND category_id = ?`,
		httpx.WalletIDFrom(r.Context()), catID)
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
