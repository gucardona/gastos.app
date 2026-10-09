package api

import "net/http"

func (s *Server) budgetRoutes(mux *http.ServeMux, wallet routeWrap) {
	const base = "/api/wallets/{walletID}/budgets"
	mux.Handle("GET "+base, wallet(roleReader, s.listBudgets))
	mux.Handle("PUT "+base+"/{categoryID}", wallet(roleEditor, s.putBudget))
	mux.Handle("DELETE "+base+"/{categoryID}", wallet(roleEditor, s.deleteBudget))
}

type budgetDTO struct {
	CategoryID int64 `json:"categoryId"`
	LimitCents int64 `json:"limitCents"`
}

func (s *Server) listBudgets(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT category_id, limit_cents FROM budgets WHERE wallet_id = ? ORDER BY category_id`, walletIDFrom(r.Context()))
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
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) putBudget(w http.ResponseWriter, r *http.Request) error {
	catID, err := pathID(r, "categoryID")
	if err != nil {
		return err
	}
	var in struct {
		LimitCents int64 `json:"limitCents"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if in.LimitCents <= 0 {
		return errValidation(map[string]string{"limitCents": "deve ser maior que zero"})
	}
	walletID := walletIDFrom(r.Context())
	cat, err := s.categoryByID(r.Context(), s.db, walletID, catID)
	if err != nil {
		return err
	}
	if cat.Kind != "expense" || cat.Archived {
		return errValidation(map[string]string{"categoryId": "orçamento só vale para categorias de gasto ativas"})
	}
	if _, err := s.db.ExecContext(r.Context(), `
		INSERT INTO budgets (wallet_id, category_id, limit_cents) VALUES (?, ?, ?)
		ON CONFLICT (wallet_id, category_id) DO UPDATE SET limit_cents = excluded.limit_cents`,
		walletID, catID, in.LimitCents); err != nil {
		return err
	}
	s.mutated(r)
	writeJSON(w, http.StatusOK, budgetDTO{CategoryID: catID, LimitCents: in.LimitCents})
	return nil
}

func (s *Server) deleteBudget(w http.ResponseWriter, r *http.Request) error {
	catID, err := pathID(r, "categoryID")
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(r.Context(), `DELETE FROM budgets WHERE wallet_id = ? AND category_id = ?`,
		walletIDFrom(r.Context()), catID)
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
