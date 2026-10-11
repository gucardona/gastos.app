package recurrence

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"gastos/internal/events"
)

type Materializer struct {
	db  *sql.DB
	bus *events.Bus
	loc *time.Location
	log *slog.Logger
	now func() time.Time
}

func NewMaterializer(db *sql.DB, bus *events.Bus, loc *time.Location, log *slog.Logger) *Materializer {
	return &Materializer{db: db, bus: bus, loc: loc, log: log, now: time.Now}
}

type template struct {
	id, walletID, amount, paidBy, createdBy int64
	kind, description, startDate, endDate   string
	categoryID, paymentMethodID, toUserID   sql.NullInt64
	day                                     int
	splits                                  []splitRow
}

type splitRow struct{ userID, shareBP int64 }

// Run materializes the occurrences due up to today (in the configured time zone).
// walletID == 0 processes every wallet. It is idempotent and safe under concurrency,
// thanks to the unique indexes on (recurrence_id, occurrence).
func (m *Materializer) Run(ctx context.Context, walletID int64) error {
	templates, err := m.load(ctx, walletID)
	if err != nil {
		return err
	}
	today := m.now().In(m.loc).Format(dateLayout)
	changed := map[int64]bool{}

	for _, t := range templates {
		occ, err := Occurrences(t.day, t.startDate, t.endDate, today)
		if err != nil {
			m.log.Error("invalid recurrence", "id", t.id, "err", err)
			continue
		}
		for _, o := range occ {
			inserted, err := m.insert(ctx, t, o)
			if err != nil {
				return err
			}
			if inserted {
				changed[t.walletID] = true
			}
		}
	}
	for wid := range changed {
		m.bus.Notify(wid, 0)
	}
	return nil
}

func (m *Materializer) load(ctx context.Context, walletID int64) ([]template, error) {
	q := `SELECT id, wallet_id, kind, amount_cents, description, category_id, payment_method_id,
	             paid_by_user_id, to_user_id, day_of_month, start_date, COALESCE(end_date, ''), created_by
	      FROM recurrences WHERE enabled = 1`
	var args []any
	if walletID != 0 {
		q += ` AND wallet_id = ?`
		args = append(args, walletID)
	}
	rows, err := m.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []template
	for rows.Next() {
		var t template
		if err := rows.Scan(&t.id, &t.walletID, &t.kind, &t.amount, &t.description, &t.categoryID, &t.paymentMethodID,
			&t.paidBy, &t.toUserID, &t.day, &t.startDate, &t.endDate, &t.createdBy); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	for i := range out {
		srows, err := m.db.QueryContext(ctx,
			`SELECT user_id, share_bp FROM recurrence_splits WHERE recurrence_id = ? ORDER BY user_id`, out[i].id)
		if err != nil {
			return nil, err
		}
		for srows.Next() {
			var s splitRow
			if err := srows.Scan(&s.userID, &s.shareBP); err != nil {
				srows.Close()
				return nil, err
			}
			out[i].splits = append(out[i].splits, s)
		}
		srows.Close()
		if err := srows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// insert writes an occurrence; it returns false if it already existed.
func (m *Materializer) insert(ctx context.Context, t template, o Occurrence) (bool, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	if t.kind == "settlement" {
		var id int64
		err := tx.QueryRowContext(ctx, `
			INSERT INTO settlements (wallet_id, from_user_id, to_user_id, amount_cents, date, note, recurrence_id, occurrence, created_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (recurrence_id, occurrence) WHERE recurrence_id IS NOT NULL DO NOTHING
			RETURNING id`,
			t.walletID, t.paidBy, t.toUserID.Int64, t.amount, o.Date, t.description, t.id, o.Month, t.createdBy,
		).Scan(&id)
		if err == sql.ErrNoRows {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return true, tx.Commit()
	}

	var id int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO transactions (wallet_id, kind, amount_cents, description, category_id, payment_method_id, date,
		                          paid_by_user_id, created_by, recurrence_id, occurrence)
		VALUES (?, ?, ?, COALESCE(NULLIF(?, ''), (SELECT name FROM categories WHERE id = ?)), ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (recurrence_id, occurrence) WHERE recurrence_id IS NOT NULL DO NOTHING
		RETURNING id`,
		t.walletID, t.kind, t.amount, t.description, t.categoryID, t.categoryID, t.paymentMethodID, o.Date,
		t.paidBy, t.createdBy, t.id, o.Month,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, s := range t.splits {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO transaction_splits (transaction_id, user_id, share_bp) VALUES (?, ?, ?)`,
			id, s.userID, s.shareBP); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

// Loop runs Run immediately and then at every interval, until ctx is cancelled.
func (m *Materializer) Loop(ctx context.Context, every time.Duration) {
	run := func() {
		if err := m.Run(ctx, 0); err != nil && ctx.Err() == nil {
			m.log.Error("materialize recurrences", "err", err)
		}
	}
	run()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}
