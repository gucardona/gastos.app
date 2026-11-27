package api

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"gastos/internal/api/httpx"
	"gastos/internal/recurrence"
)

// recurrenceRoutes registers the recurrence routes of a wallet.
func (s *Server) recurrenceRoutes(mux *http.ServeMux, wallet httpx.RouteWrap) {
	const base = "/api/wallets/{walletID}/recurrences"
	mux.Handle("GET "+base, wallet(httpx.RoleReader, s.listRecurrences))
	mux.Handle("POST "+base, wallet(httpx.RoleEditor, s.createRecurrence))
	mux.Handle("PUT "+base+"/{id}", wallet(httpx.RoleEditor, s.updateRecurrence))
	mux.Handle("DELETE "+base+"/{id}", wallet(httpx.RoleEditor, s.deleteRecurrence))
}

// recurrenceDTO is a recurring entry as the API returns it.
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
	Splits          []Split `json:"splits"`
}

// recurrenceInput is the body of a recurrence create or update.
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
	Splits          []Split `json:"splits"`
}

// maxBackfillMonths bounds how far back a new start date may go: each past month is materialized inside the request.
const maxBackfillMonths = 24

// earliestStart is the oldest start date a recurrence may be given now.
func (s *Server) earliestStart() string {
	return s.Now().In(s.Cfg.Location).AddDate(0, -maxBackfillMonths, 0).Format(httpx.DateLayout)
}

// validate checks the input; currentCategoryID allows keeping an archived category that is already in use (0 on create).
// earliestStart ("" = no limit) is the oldest start date accepted.
func (in *recurrenceInput) validate(ctx context.Context, q httpx.Querier, walletID, userID, currentCategoryID int64, earliestStart string) error {
	in.Description = strings.TrimSpace(in.Description)
	if in.EndDate != nil && strings.TrimSpace(*in.EndDate) == "" {
		in.EndDate = nil
	}
	fe := httpx.FieldErrors{}
	if in.DayOfMonth < 1 || in.DayOfMonth > 31 {
		fe.Add("dayOfMonth", "deve estar entre 1 e 31")
	}
	if msg := dateProblem(in.StartDate); msg != "" {
		fe.Add("startDate", msg)
	} else if in.StartDate < earliestStart {
		fe.Add("startDate", "o início pode ser no máximo 24 meses atrás")
	}
	if in.EndDate != nil {
		switch msg := dateProblem(*in.EndDate); {
		case msg != "":
			fe.Add("endDate", msg)
		case httpx.ValidDate(in.StartDate) && *in.EndDate < in.StartDate:
			fe.Add("endDate", "deve ser igual ou posterior ao início")
		}
	}
	if in.PaidByUserID == 0 {
		in.PaidByUserID = userID
	}

	switch in.Kind {
	case "expense", "income":
		if in.CategoryID == nil || in.ToUserID != nil {
			return httpx.ValidationError(map[string]string{"categoryId": "obrigatória para gastos e entradas"})
		}
		ti := TransactionInput{Kind: in.Kind, AmountCents: in.AmountCents, Description: in.Description, CategoryID: *in.CategoryID,
			PaymentMethodID: in.PaymentMethodID, Date: in.StartDate, PaidByUserID: in.PaidByUserID, Splits: in.Splits}
		if err := ti.validate(ctx, q, walletID, userID, currentCategoryID); err != nil {
			var ae *httpx.APIError
			if httpx.AsAPIError(err, &ae) && ae.Fields != nil {
				for k, v := range ae.Fields {
					if k != "date" {
						fe.Add(k, v)
					}
				}
				return fe.Err()
			}
			return err
		}
		in.Splits = ti.Splits
	case "settlement":
		if in.CategoryID != nil || in.PaymentMethodID != nil || len(in.Splits) > 0 {
			fe.Add("kind", "acertos não têm categoria, forma de pagamento nem divisão")
		}
		if in.ToUserID == nil || *in.ToUserID == in.PaidByUserID {
			fe.Add("toUserId", "obrigatório e diferente de quem envia")
		} else if err := checkSettlementMembers(ctx, q, walletID, in.PaidByUserID, *in.ToUserID); err != nil {
			return err
		}
		in.Splits = nil
	default:
		fe.Add("kind", "deve ser expense, income ou settlement")
	}
	if msg := amountProblem(in.AmountCents); msg != "" {
		fe.Add("amountCents", msg)
	}
	return fe.Err()
}

// loadRecurrences returns every recurrence of walletID with its splits.
func (s *Server) loadRecurrences(ctx context.Context, walletID int64) ([]recurrenceDTO, error) {
	rows, err := s.DB.QueryContext(ctx, `
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
		d.Splits = []Split{}
		index[d.ID] = len(list)
		list = append(list, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	srows, err := s.DB.QueryContext(ctx, `
		SELECT recurrence_id, user_id, share_bp FROM recurrence_splits
		WHERE recurrence_id IN (SELECT id FROM recurrences WHERE wallet_id = ?) ORDER BY recurrence_id, user_id`, walletID)
	if err != nil {
		return nil, err
	}
	defer srows.Close()
	for srows.Next() {
		var rid int64
		var sp Split
		if err := srows.Scan(&rid, &sp.UserID, &sp.ShareBP); err != nil {
			return nil, err
		}
		list[index[rid]].Splits = append(list[index[rid]].Splits, sp)
	}
	if err := srows.Err(); err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Splits = WithAmounts(list[i].AmountCents, list[i].Splits)
		list[i].NextDate = s.nextDate(list[i])
	}
	return list, nil
}

// nextDate is the first occurrence after today (nil if disabled or finished).
func (s *Server) nextDate(d recurrenceDTO) *string {
	if !d.Enabled {
		return nil
	}
	tomorrow := s.Now().In(s.Cfg.Location).AddDate(0, 0, 1)
	start := d.StartDate
	if t := tomorrow.Format(httpx.DateLayout); t > start {
		start = t
	}
	end := ""
	if d.EndDate != nil {
		end = *d.EndDate
	}
	until := tomorrow.AddDate(0, 14, 0).Format(httpx.DateLayout)
	occ, err := recurrence.Occurrences(d.DayOfMonth, start, end, until)
	if err != nil || len(occ) == 0 {
		return nil
	}
	return &occ[0].Date
}

// recurrenceByID returns recurrence id of walletID.
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

// listRecurrences handles GET /recurrences.
func (s *Server) listRecurrences(w http.ResponseWriter, r *http.Request) error {
	list, err := s.loadRecurrences(r.Context(), httpx.WalletIDFrom(r.Context()))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, list)
	return nil
}

// replaceRecurrenceSplits replaces the member shares of recurrence id with splits.
func replaceRecurrenceSplits(ctx context.Context, q httpx.Querier, id int64, splits []Split) error {
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

// createRecurrence handles POST /recurrences.
func (s *Server) createRecurrence(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), httpx.WalletIDFrom(r.Context()), httpx.UserIDFrom(r.Context())
	var in recurrenceInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	enabled := in.Enabled == nil || *in.Enabled
	var id int64
	err := s.InTx(ctx, func(tx *sql.Tx) error {
		if err := in.validate(ctx, tx, walletID, userID, 0, s.earliestStart()); err != nil {
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
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusCreated, d)
	return nil
}

// updateRecurrence handles PUT /recurrences/{id}.
func (s *Server) updateRecurrence(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), httpx.WalletIDFrom(r.Context()), httpx.UserIDFrom(r.Context())
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in recurrenceInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	enabled := in.Enabled == nil || *in.Enabled
	err = s.InTx(ctx, func(tx *sql.Tx) error {
		var currentCat sql.NullInt64
		var currentStart string
		if err := tx.QueryRowContext(ctx, `SELECT category_id, start_date FROM recurrences WHERE id = ? AND wallet_id = ?`, id, walletID).
			Scan(&currentCat, &currentStart); err != nil {
			return err
		}
		earliest := s.earliestStart()
		if in.StartDate == currentStart { // keeping the start already stored: those months are materialized already
			earliest = ""
		}
		if err := in.validate(ctx, tx, walletID, userID, currentCat.Int64, earliest); err != nil {
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
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusOK, d)
	return nil
}

// deleteRecurrence handles DELETE /recurrences/{id}.
func (s *Server) deleteRecurrence(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	res, err := s.DB.ExecContext(r.Context(), `DELETE FROM recurrences WHERE id = ? AND wallet_id = ?`, id, httpx.WalletIDFrom(r.Context()))
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

// materialize generates what is already due right away; failures are only logged (the hourly job retries).
func (s *Server) materialize(ctx context.Context, walletID int64) {
	if err := s.Mat.Run(ctx, walletID); err != nil {
		s.Log.ErrorContext(ctx, "materialize recurrences", "wallet", walletID, "err", err)
	}
}
