package api

import (
	"context"
	"net/http"
	"sort"
	"time"

	"gastos/internal/money"
	"gastos/internal/recurrence"
)

func (s *Server) summaryRoutes(mux *http.ServeMux, wallet routeWrap) {
	mux.Handle("GET /api/wallets/{walletID}/summary", wallet(roleReader, s.summary))
}

type categoryTotal struct {
	CategoryID  int64  `json:"categoryId"`
	Kind        string `json:"kind"`
	Cents       int64  `json:"cents"`
	BudgetCents *int64 `json:"budgetCents"`
}

type monthTotals struct {
	IncomeCents  int64 `json:"incomeCents"`
	ExpenseCents int64 `json:"expenseCents"`
}

type memberBalance struct {
	UserID   int64 `json:"userId"`
	NetCents int64 `json:"netCents"`
}

type summaryDTO struct {
	Month             string           `json:"month"`
	IncomeCents       int64            `json:"incomeCents"`
	ExpenseCents      int64            `json:"expenseCents"`
	BalanceCents      int64            `json:"balanceCents"`
	DailyAverageCents int64            `json:"dailyAverageCents"`
	ByCategory        []categoryTotal  `json:"byCategory"`
	ByEssentiality    map[string]int64 `json:"byEssentiality"`
	Previous          monthTotals      `json:"previous"`
	// Only present when splitting is enabled on the wallet.
	MemberBalances       any `json:"memberBalances,omitempty"`
	SuggestedSettlements any `json:"suggestedSettlements,omitempty"`
	// Projection of the month's recurring entries still to come; the totals above stay realized only.
	UpcomingIncomeCents  int64      `json:"upcomingIncomeCents"`
	UpcomingExpenseCents int64      `json:"upcomingExpenseCents"`
	Upcoming             []upcoming `json:"upcoming"`
}

type upcoming struct {
	RecurrenceID    int64  `json:"recurrenceId"`
	Date            string `json:"date"`
	Kind            string `json:"kind"`
	AmountCents     int64  `json:"amountCents"`
	Description     string `json:"description"`
	CategoryID      *int64 `json:"categoryId"`
	PaymentMethodID *int64 `json:"paymentMethodId"`
}

func monthRange(month string) (first, last string, ok bool) {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return "", "", false
	}
	return t.Format(dateLayout), t.AddDate(0, 1, -1).Format(dateLayout), true
}

func (s *Server) totals(ctx context.Context, walletID int64, from, to string) (monthTotals, error) {
	var m monthTotals
	rows, err := s.db.QueryContext(ctx,
		`SELECT kind, SUM(amount_cents) FROM transactions WHERE wallet_id = ? AND date BETWEEN ? AND ? GROUP BY kind`,
		walletID, from, to)
	if err != nil {
		return m, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var sum int64
		if err := rows.Scan(&kind, &sum); err != nil {
			return m, err
		}
		if kind == "income" {
			m.IncomeCents = sum
		} else {
			m.ExpenseCents = sum
		}
	}
	return m, rows.Err()
}

func (s *Server) summary(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), walletIDFrom(r.Context())
	month := r.URL.Query().Get("month")
	first, last, ok := monthRange(month)
	if !ok {
		return errValidation(map[string]string{"month": "obrigatório (YYYY-MM)"})
	}
	prevFirst, prevLast, _ := monthRange(mustDate(first).AddDate(0, -1, 0).Format("2006-01"))

	cur, err := s.totals(ctx, walletID, first, last)
	if err != nil {
		return err
	}
	prev, err := s.totals(ctx, walletID, prevFirst, prevLast)
	if err != nil {
		return err
	}

	out := summaryDTO{
		Month: month, IncomeCents: cur.IncomeCents, ExpenseCents: cur.ExpenseCents,
		BalanceCents: cur.IncomeCents - cur.ExpenseCents, Previous: prev,
		ByEssentiality: map[string]int64{"essential": 0, "nonessential": 0, "investment": 0},
		ByCategory:     []categoryTotal{},
	}

	now := s.now().In(s.cfg.Location)
	nowMonth := now.Format("2006-01")
	days := int64(0)
	switch {
	case month == nowMonth:
		days = int64(now.Day())
	case month < nowMonth:
		days = int64(mustDate(last).Day())
	}
	if days > 0 {
		out.DailyAverageCents = cur.ExpenseCents / days
	}
	if out.Upcoming, err = s.upcoming(ctx, walletID, first, last, now.Format(dateLayout)); err != nil {
		return err
	}
	for _, u := range out.Upcoming {
		if u.Kind == "income" {
			out.UpcomingIncomeCents += u.AmountCents
		} else {
			out.UpcomingExpenseCents += u.AmountCents
		}
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.kind, COALESCE(SUM(t.amount_cents), 0) AS cents, b.limit_cents
		FROM categories c
		LEFT JOIN transactions t ON t.category_id = c.id AND t.date BETWEEN ? AND ?
		LEFT JOIN budgets b ON b.category_id = c.id
		WHERE c.wallet_id = ?
		GROUP BY c.id, b.limit_cents
		HAVING cents > 0 OR b.limit_cents IS NOT NULL
		ORDER BY cents DESC, c.id`, first, last, walletID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var c categoryTotal
		if err := rows.Scan(&c.CategoryID, &c.Kind, &c.Cents, &c.BudgetCents); err != nil {
			return err
		}
		out.ByCategory = append(out.ByCategory, c)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()

	ess, err := s.db.QueryContext(ctx, `
		SELECT c.essentiality, SUM(t.amount_cents)
		FROM transactions t JOIN categories c ON c.id = t.category_id
		WHERE t.wallet_id = ? AND t.kind = 'expense' AND t.date BETWEEN ? AND ?
		GROUP BY c.essentiality`, walletID, first, last)
	if err != nil {
		return err
	}
	defer ess.Close()
	for ess.Next() {
		var key string
		var sum int64
		if err := ess.Scan(&key, &sum); err != nil {
			return err
		}
		out.ByEssentiality[key] = sum
	}
	if err := ess.Err(); err != nil {
		return err
	}
	ess.Close()

	var splitting bool
	if err := s.db.QueryRowContext(ctx, `SELECT splitting_enabled FROM wallets WHERE id = ?`, walletID).Scan(&splitting); err != nil {
		return err
	}
	if splitting {
		net, order, err := s.memberNet(ctx, walletID)
		if err != nil {
			return err
		}
		balances := make([]memberBalance, 0, len(order))
		for _, id := range order {
			balances = append(balances, memberBalance{id, net[id]})
		}
		out.MemberBalances = balances
		out.SuggestedSettlements = suggestSettlements(net)
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// upcoming projects the month's occurrences of enabled expense/income recurrences that fall after today
// (today's are materialized already), leaving out those already present as a transaction or deleted by a person.
func (s *Server) upcoming(ctx context.Context, walletID int64, first, last, today string) ([]upcoming, error) {
	out := []upcoming{}
	if last <= today {
		return out, nil
	}
	month := first[:7]
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.kind, r.amount_cents, COALESCE(NULLIF(r.description, ''), c.name, ''), r.category_id, r.payment_method_id,
		       r.day_of_month, r.start_date, COALESCE(r.end_date, '')
		FROM recurrences r LEFT JOIN categories c ON c.id = r.category_id
		WHERE r.wallet_id = ? AND r.enabled = 1 AND r.kind IN ('expense', 'income')
		  AND NOT EXISTS (SELECT 1 FROM transactions t WHERE t.recurrence_id = r.id AND t.occurrence = ?)
		  AND NOT EXISTS (SELECT 1 FROM recurrence_skips k WHERE k.recurrence_id = r.id AND k.occurrence = ?)`,
		walletID, month, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var u upcoming
		var day int
		var start, end string
		if err := rows.Scan(&u.RecurrenceID, &u.Kind, &u.AmountCents, &u.Description, &u.CategoryID, &u.PaymentMethodID,
			&day, &start, &end); err != nil {
			return nil, err
		}
		occ, err := recurrence.Occurrences(day, start, end, last)
		if err != nil || len(occ) == 0 {
			continue // an invalid recurrence is logged by the materializer
		}
		if o := occ[len(occ)-1]; o.Date > today && o.Date >= first {
			u.Date = o.Date
			out = append(out, u)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].RecurrenceID < out[j].RecurrenceID
	})
	return out, nil
}

// memberNet computes each member's balance over the wallet's entire history:
// whoever paid a split expense is credited the total; each participant is debited their share
// (allocated in cents, the same as the transaction list shows); settlements move balance from the sender to the receiver.
func (s *Server) memberNet(ctx context.Context, walletID int64) (map[int64]int64, []int64, error) {
	net := map[int64]int64{}
	var order []int64

	mrows, err := s.db.QueryContext(ctx, `SELECT user_id FROM wallet_members WHERE wallet_id = ? ORDER BY user_id`, walletID)
	if err != nil {
		return nil, nil, err
	}
	defer mrows.Close()
	for mrows.Next() {
		var id int64
		if err := mrows.Scan(&id); err != nil {
			return nil, nil, err
		}
		net[id] = 0
		order = append(order, id)
	}
	if err := mrows.Err(); err != nil {
		return nil, nil, err
	}
	mrows.Close()

	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.amount_cents, t.paid_by_user_id, ts.user_id, ts.share_bp
		FROM transactions t JOIN transaction_splits ts ON ts.transaction_id = t.id
		WHERE t.wallet_id = ? AND t.kind = 'expense' ORDER BY t.id, ts.user_id`, walletID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var curID, curAmount, curPayer int64
	var users, shares []int64
	flush := func() {
		if curID == 0 {
			return
		}
		net[curPayer] += curAmount
		for i, part := range money.Allocate(curAmount, shares) {
			net[users[i]] -= part
		}
		users, shares = users[:0], shares[:0]
	}
	for rows.Next() {
		var id, amount, payer, uid, bp int64
		if err := rows.Scan(&id, &amount, &payer, &uid, &bp); err != nil {
			return nil, nil, err
		}
		if id != curID {
			flush()
			curID, curAmount, curPayer = id, amount, payer
		}
		users, shares = append(users, uid), append(shares, bp)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	flush()
	rows.Close()

	srows, err := s.db.QueryContext(ctx, `SELECT from_user_id, to_user_id, amount_cents FROM settlements WHERE wallet_id = ?`, walletID)
	if err != nil {
		return nil, nil, err
	}
	defer srows.Close()
	for srows.Next() {
		var from, to, amount int64
		if err := srows.Scan(&from, &to, &amount); err != nil {
			return nil, nil, err
		}
		net[from] += amount
		net[to] -= amount
	}
	return net, order, srows.Err()
}
