package api

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"gastos/internal/recurrence"
)

func (s *Server) recurrenceRoutes(mux *http.ServeMux, wallet routeWrap) {
	const base = "/api/wallets/{walletID}/recurrences"
	mux.Handle("GET "+base, wallet(roleReader, s.listRecurrences))
	mux.Handle("POST "+base, wallet(roleEditor, s.createRecurrence))
	mux.Handle("PUT "+base+"/{id}", wallet(roleEditor, s.updateRecurrence))
	mux.Handle("DELETE "+base+"/{id}", wallet(roleEditor, s.deleteRecurrence))
}

type recurrenceDTO struct {
	ID              int64   `json:"id"`
	Kind            string  `json:"kind"`
	AmountCents     int64   `json:"amountCents"`
	Description     string  `json:"description"`
	CategoryID      *int64  `json:"categoryId"`
	PaymentMethodID *int64  `json:"paymentMethodId"`
	PaidByUserID    int64   `json:"paidByUserId"`
	ToUserID        *int64  `json:"toUserId"`
	DayOfMonth      int     `json:"dayOfMonth"`
	StartDate       string  `json:"startDate"`
	EndDate         *string `json:"endDate"`
	Enabled         bool    `json:"enabled"`
	NextDate        *string `json:"nextDate"`
	Splits          []split `json:"splits"`
}

type recurrenceInput struct {
	Kind            string  `json:"kind"`
	AmountCents     int64   `json:"amountCents"`
	Description     string  `json:"description"`
	CategoryID      *int64  `json:"categoryId"`
	PaymentMethodID *int64  `json:"paymentMethodId"`
	PaidByUserID    int64   `json:"paidByUserId"`
	ToUserID        *int64  `json:"toUserId"`
	DayOfMonth      int     `json:"dayOfMonth"`
	StartDate       string  `json:"startDate"`
	EndDate         *string `json:"endDate"`
	Enabled         *bool   `json:"enabled"`
	Splits          []split `json:"splits"`
}

// validate confere a entrada; currentCategoryID permite manter categoria arquivada já usada (0 na criação).
func (in *recurrenceInput) validate(ctx context.Context, q querier, walletID, userID, currentCategoryID int64) error {
	in.Description = strings.TrimSpace(in.Description)
	if in.EndDate != nil && strings.TrimSpace(*in.EndDate) == "" {
		in.EndDate = nil
	}
	fe := fieldErrors{}
	if in.DayOfMonth < 1 || in.DayOfMonth > 31 {
		fe.add("dayOfMonth", "deve estar entre 1 e 31")
	}
	if !validDate(in.StartDate) {
		fe.add("startDate", "data inválida")
	}
	if in.EndDate != nil {
		switch {
		case !validDate(*in.EndDate):
			fe.add("endDate", "data inválida")
		case validDate(in.StartDate) && *in.EndDate < in.StartDate:
			fe.add("endDate", "deve ser igual ou posterior ao início")
		}
	}
	if in.PaidByUserID == 0 {
		in.PaidByUserID = userID
	}

	switch in.Kind {
	case "expense", "income":
		if in.CategoryID == nil || in.ToUserID != nil {
			return errValidation(map[string]string{"categoryId": "obrigatória para gastos e entradas"})
		}
		desc := in.Description
		if desc == "" {
			desc = "-" // a descrição é opcional em recorrências (vira o nome da categoria)
		}
		ti := transactionInput{Kind: in.Kind, AmountCents: in.AmountCents, Description: desc, CategoryID: *in.CategoryID,
			PaymentMethodID: in.PaymentMethodID, Date: in.StartDate, PaidByUserID: in.PaidByUserID, Splits: in.Splits}
		if err := ti.validate(ctx, q, walletID, userID, currentCategoryID); err != nil {
			var ae *apiError
			if asAPIError(err, &ae) && ae.Fields != nil {
				for k, v := range ae.Fields {
					if k != "date" {
						fe.add(k, v)
					}
				}
				return fe.err()
			}
			return err
		}
		in.Splits = ti.Splits
	case "settlement":
		if in.AmountCents <= 0 {
			fe.add("amountCents", "deve ser maior que zero")
		}
		if in.CategoryID != nil || in.PaymentMethodID != nil || len(in.Splits) > 0 {
			fe.add("kind", "acertos não têm categoria, forma de pagamento nem divisão")
		}
		if in.ToUserID == nil || *in.ToUserID == in.PaidByUserID {
			fe.add("toUserId", "obrigatório e diferente de quem envia")
		} else if err := checkSettlementMembers(ctx, q, walletID, in.PaidByUserID, *in.ToUserID); err != nil {
			return err
		}
		in.Splits = nil
	default:
		fe.add("kind", "deve ser expense, income ou settlement")
	}
	if in.AmountCents <= 0 {
		fe.add("amountCents", "deve ser maior que zero")
	}
	return fe.err()
}

func (s *Server) loadRecurrences(ctx context.Context, walletID int64) ([]recurrenceDTO, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, amount_cents, description, category_id, payment_method_id, paid_by_user_id, to_user_id,
		       day_of_month, start_date, end_date, enabled
		FROM recurrences WHERE wallet_id = ? ORDER BY enabled DESC, day_of_month, id`, walletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []recurrenceDTO{}
	index := map[int64]int{}
	for rows.Next() {
		var d recurrenceDTO
		if err := rows.Scan(&d.ID, &d.Kind, &d.AmountCents, &d.Description, &d.CategoryID, &d.PaymentMethodID,
			&d.PaidByUserID, &d.ToUserID, &d.DayOfMonth, &d.StartDate, &d.EndDate, &d.Enabled); err != nil {
			return nil, err
		}
		d.Splits = []split{}
		index[d.ID] = len(list)
		list = append(list, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	srows, err := s.db.QueryContext(ctx, `
		SELECT recurrence_id, user_id, share_bp FROM recurrence_splits
		WHERE recurrence_id IN (SELECT id FROM recurrences WHERE wallet_id = ?) ORDER BY recurrence_id, user_id`, walletID)
	if err != nil {
		return nil, err
	}
	defer srows.Close()
	for srows.Next() {
		var rid int64
		var sp split
		if err := srows.Scan(&rid, &sp.UserID, &sp.ShareBP); err != nil {
			return nil, err
		}
		list[index[rid]].Splits = append(list[index[rid]].Splits, sp)
	}
	if err := srows.Err(); err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Splits = withAmounts(list[i].AmountCents, list[i].Splits)
		list[i].NextDate = s.nextDate(list[i])
	}
	return list, nil
}

// nextDate é a primeira ocorrência depois de hoje (nil se desabilitada ou encerrada).
func (s *Server) nextDate(d recurrenceDTO) *string {
	if !d.Enabled {
		return nil
	}
	tomorrow := s.now().In(s.cfg.Location).AddDate(0, 0, 1)
	start := d.StartDate
	if t := tomorrow.Format(dateLayout); t > start {
		start = t
	}
	end := ""
	if d.EndDate != nil {
		end = *d.EndDate
	}
	until := tomorrow.AddDate(0, 14, 0).Format(dateLayout)
	occ, err := recurrence.Occurrences(d.DayOfMonth, start, end, until)
	if err != nil || len(occ) == 0 {
		return nil
	}
	return &occ[0].Date
}

func (s *Server) recurrenceByID(ctx context.Context, walletID, id int64) (recurrenceDTO, error) {
	list, err := s.loadRecurrences(ctx, walletID)
	if err != nil {
		return recurrenceDTO{}, err
	}
	for _, d := range list {
		if d.ID == id {
			return d, nil
		}
	}
	return recurrenceDTO{}, sql.ErrNoRows
}

func (s *Server) listRecurrences(w http.ResponseWriter, r *http.Request) error {
	list, err := s.loadRecurrences(r.Context(), walletIDFrom(r.Context()))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, list)
	return nil
}

func replaceRecurrenceSplits(ctx context.Context, q querier, id int64, splits []split) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM recurrence_splits WHERE recurrence_id = ?`, id); err != nil {
		return err
	}
	for _, sp := range splits {
		if _, err := q.ExecContext(ctx, `INSERT INTO recurrence_splits (recurrence_id, user_id, share_bp) VALUES (?, ?, ?)`,
			id, sp.UserID, sp.ShareBP); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) createRecurrence(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), walletIDFrom(r.Context()), userIDFrom(r.Context())
	var in recurrenceInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	enabled := in.Enabled == nil || *in.Enabled
	var id int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if err := in.validate(ctx, tx, walletID, userID, 0); err != nil {
			return err
		}
		err := tx.QueryRowContext(ctx, `
			INSERT INTO recurrences (wallet_id, kind, amount_cents, description, category_id, payment_method_id, paid_by_user_id,
			                         to_user_id, day_of_month, start_date, end_date, enabled, created_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			walletID, in.Kind, in.AmountCents, in.Description, in.CategoryID, in.PaymentMethodID, in.PaidByUserID,
			in.ToUserID, in.DayOfMonth, in.StartDate, in.EndDate, enabled, userID).Scan(&id)
		if err != nil {
			return err
		}
		return replaceRecurrenceSplits(ctx, tx, id, in.Splits)
	})
	if err != nil {
		return err
	}
	s.materialize(ctx, walletID)
	d, err := s.recurrenceByID(ctx, walletID, id)
	if err != nil {
		return err
	}
	s.mutated(r)
	writeJSON(w, http.StatusCreated, d)
	return nil
}

func (s *Server) updateRecurrence(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), walletIDFrom(r.Context()), userIDFrom(r.Context())
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in recurrenceInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	enabled := in.Enabled == nil || *in.Enabled
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		var currentCat sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT category_id FROM recurrences WHERE id = ? AND wallet_id = ?`, id, walletID).
			Scan(&currentCat); err != nil {
			return err
		}
		if err := in.validate(ctx, tx, walletID, userID, currentCat.Int64); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE recurrences SET kind = ?, amount_cents = ?, description = ?, category_id = ?, payment_method_id = ?,
			       paid_by_user_id = ?, to_user_id = ?, day_of_month = ?, start_date = ?, end_date = ?, enabled = ?
			WHERE id = ? AND wallet_id = ?`,
			in.Kind, in.AmountCents, in.Description, in.CategoryID, in.PaymentMethodID, in.PaidByUserID, in.ToUserID,
			in.DayOfMonth, in.StartDate, in.EndDate, enabled, id, walletID); err != nil {
			return err
		}
		return replaceRecurrenceSplits(ctx, tx, id, in.Splits)
	})
	if err != nil {
		return err
	}
	s.materialize(ctx, walletID)
	d, err := s.recurrenceByID(ctx, walletID, id)
	if err != nil {
		return err
	}
	s.mutated(r)
	writeJSON(w, http.StatusOK, d)
	return nil
}

func (s *Server) deleteRecurrence(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(r.Context(), `DELETE FROM recurrences WHERE id = ? AND wallet_id = ?`, id, walletIDFrom(r.Context()))
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

// materialize gera na hora o que já é devido; falhas só são logadas (o job horário tenta de novo).
func (s *Server) materialize(ctx context.Context, walletID int64) {
	if err := s.mat.Run(ctx, walletID); err != nil {
		s.log.ErrorContext(ctx, "materializar recorrências", "wallet", walletID, "err", err)
	}
}
