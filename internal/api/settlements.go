package api

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"
)

func (s *Server) settlementRoutes(mux *http.ServeMux, wallet routeWrap) {
	const base = "/api/wallets/{walletID}/settlements"
	mux.Handle("GET "+base, wallet(roleReader, s.listSettlements))
	mux.Handle("POST "+base, wallet(roleEditor, s.createSettlement))
	mux.Handle("PUT "+base+"/{id}", wallet(roleEditor, s.updateSettlement))
	mux.Handle("DELETE "+base+"/{id}", wallet(roleEditor, s.deleteSettlement))
}

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

type settlementInput struct {
	FromUserID  int64  `json:"fromUserId"`
	ToUserID    int64  `json:"toUserId"`
	AmountCents int64  `json:"amountCents"`
	Date        string `json:"date"`
	Note        string `json:"note"`
}

// checkSettlementMembers requires splitting enabled on the wallet and both users to be members.
func checkSettlementMembers(ctx context.Context, q querier, walletID, from, to int64) error {
	var splitting bool
	if err := q.QueryRowContext(ctx, `SELECT splitting_enabled FROM wallets WHERE id = ?`, walletID).Scan(&splitting); err != nil {
		return err
	}
	if !splitting {
		return errBadRequest("Acertos exigem a divisão de gastos ativa na carteira")
	}
	var n int
	if err := q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM wallet_members WHERE wallet_id = ? AND user_id IN (?, ?)`, walletID, from, to).Scan(&n); err != nil {
		return err
	}
	if n != 2 {
		return errValidation(map[string]string{"toUserId": "ambos precisam ser membros da carteira"})
	}
	return nil
}

func (in *settlementInput) validate(ctx context.Context, q querier, walletID int64) error {
	in.Note = strings.TrimSpace(in.Note)
	fe := fieldErrors{}
	if in.AmountCents <= 0 {
		fe.add("amountCents", "deve ser maior que zero")
	}
	if !validDate(in.Date) {
		fe.add("date", "data inválida")
	}
	if len([]rune(in.Note)) > 200 {
		fe.add("note", "até 200 caracteres")
	}
	if in.FromUserID <= 0 || in.ToUserID <= 0 || in.FromUserID == in.ToUserID {
		fe.add("toUserId", "quem envia e quem recebe devem ser diferentes")
	}
	if err := fe.err(); err != nil {
		return err
	}
	return checkSettlementMembers(ctx, q, walletID, in.FromUserID, in.ToUserID)
}

func (s *Server) listSettlements(w http.ResponseWriter, r *http.Request) error {
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if !validDate(from) || !validDate(to) {
		return errValidation(map[string]string{"from": "obrigatório (YYYY-MM-DD)", "to": "obrigatório (YYYY-MM-DD)"})
	}
	if f, _ := time.Parse(dateLayout, from); f.AddDate(0, 0, 366).Before(mustDate(to)) {
		return errValidation(map[string]string{"to": "intervalo máximo de 366 dias"})
	}
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT id, from_user_id, to_user_id, amount_cents, date, note, recurrence_id, created_by
		FROM settlements WHERE wallet_id = ? AND date BETWEEN ? AND ? ORDER BY date DESC, id DESC`,
		walletIDFrom(r.Context()), from, to)
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
	writeJSON(w, http.StatusOK, out)
	return nil
}

func mustDate(s string) time.Time { t, _ := time.Parse(dateLayout, s); return t }

func (s *Server) settlementByID(ctx context.Context, walletID, id int64) (settlementDTO, error) {
	var d settlementDTO
	err := s.db.QueryRowContext(ctx, `
		SELECT id, from_user_id, to_user_id, amount_cents, date, note, recurrence_id, created_by
		FROM settlements WHERE id = ? AND wallet_id = ?`, id, walletID).
		Scan(&d.ID, &d.FromUserID, &d.ToUserID, &d.AmountCents, &d.Date, &d.Note, &d.RecurrenceID, &d.CreatedBy)
	return d, err
}

func (s *Server) createSettlement(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), walletIDFrom(r.Context()), userIDFrom(r.Context())
	var in settlementInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if err := in.validate(ctx, s.db, walletID); err != nil {
		return err
	}
	var id int64
	if err := s.db.QueryRowContext(ctx, `
		INSERT INTO settlements (wallet_id, from_user_id, to_user_id, amount_cents, date, note, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		walletID, in.FromUserID, in.ToUserID, in.AmountCents, in.Date, in.Note, userID).Scan(&id); err != nil {
		return err
	}
	d, err := s.settlementByID(ctx, walletID, id)
	if err != nil {
		return err
	}
	s.mutated(r)
	writeJSON(w, http.StatusCreated, d)
	return nil
}

func (s *Server) updateSettlement(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), walletIDFrom(r.Context())
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in settlementInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if _, err := s.settlementByID(ctx, walletID, id); err != nil {
		return err
	}
	if err := in.validate(ctx, s.db, walletID); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE settlements SET from_user_id = ?, to_user_id = ?, amount_cents = ?, date = ?, note = ?
		WHERE id = ? AND wallet_id = ?`, in.FromUserID, in.ToUserID, in.AmountCents, in.Date, in.Note, id, walletID); err != nil {
		return err
	}
	d, err := s.settlementByID(ctx, walletID, id)
	if err != nil {
		return err
	}
	s.mutated(r)
	writeJSON(w, http.StatusOK, d)
	return nil
}

func (s *Server) deleteSettlement(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(r.Context(), `DELETE FROM settlements WHERE id = ? AND wallet_id = ?`, id, walletIDFrom(r.Context()))
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

var _ = sql.ErrNoRows
