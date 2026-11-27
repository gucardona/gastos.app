package api

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"gastos/internal/api/httpx"
)

// settlementRoutes registers the settlement routes of a wallet.
func (s *Server) settlementRoutes(mux *http.ServeMux, wallet httpx.RouteWrap) {
	const base = "/api/wallets/{walletID}/settlements"
	mux.Handle("GET "+base, wallet(httpx.RoleReader, s.listSettlements))
	mux.Handle("POST "+base, wallet(httpx.RoleEditor, s.createSettlement))
	mux.Handle("PUT "+base+"/{id}", wallet(httpx.RoleEditor, s.updateSettlement))
	mux.Handle("DELETE "+base+"/{id}", wallet(httpx.RoleEditor, s.deleteSettlement))
}

// settlementDTO is a recorded payment between two members, as the API returns it.
type settlementDTO struct {
	ID           int64  `json:"id"`
	FromUserID   int64  `json:"fromUserId"`
	ToUserID     int64  `json:"toUserId"`
	AmountCents  int64  `json:"amountCents"`
	Date         string `json:"date"`
	Note         string `json:"note"`
	RecurrenceID *int64 `json:"recurrenceId"`
	CreatedBy    int64  `json:"createdBy"`
}

// settlementInput is the body of a settlement create or update.
type settlementInput struct {
	FromUserID  int64  `json:"fromUserId"`
	ToUserID    int64  `json:"toUserId"`
	AmountCents int64  `json:"amountCents"`
	Date        string `json:"date"`
	Note        string `json:"note"`
}

// checkSettlementMembers requires splitting enabled on the wallet and both users to be members.
func checkSettlementMembers(ctx context.Context, q httpx.Querier, walletID, from, to int64) error {
	var splitting bool
	if err := q.QueryRowContext(ctx, `SELECT splitting_enabled FROM wallets WHERE id = ?`, walletID).Scan(&splitting); err != nil {
		return err
	}
	if !splitting {
		return httpx.ErrBadRequest("Acertos exigem a divisão de gastos ativa na carteira")
	}
	var n int
	if err := q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM wallet_members WHERE wallet_id = ? AND user_id IN (?, ?)`, walletID, from, to).Scan(&n); err != nil {
		return err
	}
	if n != 2 {
		return httpx.ValidationError(map[string]string{"toUserId": "ambos precisam ser membros da carteira"})
	}
	return nil
}

// validate checks every field against walletID, returning the field errors.
func (in *settlementInput) validate(ctx context.Context, q httpx.Querier, walletID int64) error {
	in.Note = strings.TrimSpace(in.Note)
	fe := httpx.FieldErrors{}
	if msg := amountProblem(in.AmountCents); msg != "" {
		fe.Add("amountCents", msg)
	}
	if msg := dateProblem(in.Date); msg != "" {
		fe.Add("date", msg)
	}
	if len([]rune(in.Note)) > 200 {
		fe.Add("note", "até 200 caracteres")
	}
	if in.FromUserID <= 0 || in.ToUserID <= 0 || in.FromUserID == in.ToUserID {
		fe.Add("toUserId", "quem envia e quem recebe devem ser diferentes")
	}
	if err := fe.Err(); err != nil {
		return err
	}
	return checkSettlementMembers(ctx, q, walletID, in.FromUserID, in.ToUserID)
}

// listSettlements handles GET /settlements, optionally between the from and to dates.
func (s *Server) listSettlements(w http.ResponseWriter, r *http.Request) error {
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if !httpx.ValidDate(from) || !httpx.ValidDate(to) {
		return httpx.ValidationError(map[string]string{"from": "obrigatório (YYYY-MM-DD)", "to": "obrigatório (YYYY-MM-DD)"})
	}
	if f, _ := time.Parse(httpx.DateLayout, from); f.AddDate(0, 0, 366).Before(mustDate(to)) {
		return httpx.ValidationError(map[string]string{"to": "intervalo máximo de 366 dias"})
	}
	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT id, from_user_id, to_user_id, amount_cents, date, note, recurrence_id, created_by
		FROM settlements WHERE wallet_id = ? AND date BETWEEN ? AND ? ORDER BY date DESC, id DESC`,
		httpx.WalletIDFrom(r.Context()), from, to)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []settlementDTO{}
	for rows.Next() {
		var d settlementDTO
		if err := rows.Scan(&d.ID, &d.FromUserID, &d.ToUserID, &d.AmountCents, &d.Date, &d.Note, &d.RecurrenceID, &d.CreatedBy); err != nil {
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

// mustDate parses a date already validated as YYYY-MM-DD.
func mustDate(s string) time.Time { t, _ := time.Parse(httpx.DateLayout, s); return t }

// settlementByID returns settlement id of walletID.
func (s *Server) settlementByID(ctx context.Context, walletID, id int64) (settlementDTO, error) {
	var d settlementDTO
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, from_user_id, to_user_id, amount_cents, date, note, recurrence_id, created_by
		FROM settlements WHERE id = ? AND wallet_id = ?`, id, walletID).
		Scan(&d.ID, &d.FromUserID, &d.ToUserID, &d.AmountCents, &d.Date, &d.Note, &d.RecurrenceID, &d.CreatedBy)
	return d, err
}

// createSettlement handles POST /settlements.
func (s *Server) createSettlement(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), httpx.WalletIDFrom(r.Context()), httpx.UserIDFrom(r.Context())
	var in settlementInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	if err := in.validate(ctx, s.DB, walletID); err != nil {
		return err
	}
	var id int64
	if err := s.DB.QueryRowContext(ctx, `
		INSERT INTO settlements (wallet_id, from_user_id, to_user_id, amount_cents, date, note, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		walletID, in.FromUserID, in.ToUserID, in.AmountCents, in.Date, in.Note, userID).Scan(&id); err != nil {
		return err
	}
	d, err := s.settlementByID(ctx, walletID, id)
	if err != nil {
		return err
	}
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusCreated, d)
	return nil
}

// updateSettlement handles PUT /settlements/{id}.
func (s *Server) updateSettlement(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), httpx.WalletIDFrom(r.Context())
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in settlementInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	if _, err := s.settlementByID(ctx, walletID, id); err != nil {
		return err
	}
	if err := in.validate(ctx, s.DB, walletID); err != nil {
		return err
	}
	if _, err := s.DB.ExecContext(ctx, `
		UPDATE settlements SET from_user_id = ?, to_user_id = ?, amount_cents = ?, date = ?, note = ?
		WHERE id = ? AND wallet_id = ?`, in.FromUserID, in.ToUserID, in.AmountCents, in.Date, in.Note, id, walletID); err != nil {
		return err
	}
	d, err := s.settlementByID(ctx, walletID, id)
	if err != nil {
		return err
	}
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusOK, d)
	return nil
}

// deleteSettlement handles DELETE /settlements/{id}.
func (s *Server) deleteSettlement(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	res, err := s.DB.ExecContext(r.Context(), `DELETE FROM settlements WHERE id = ? AND wallet_id = ?`, id, httpx.WalletIDFrom(r.Context()))
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

var _ = sql.ErrNoRows
