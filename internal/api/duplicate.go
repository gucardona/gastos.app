package api

import (
	"context"
	"database/sql"
	"strings"

	"gastos/internal/api/httpx"
)

// duplicateWindow is how recent one's own identical entry must be to look like a double tap rather than a second purchase.
const duplicateWindow = "-15 minutes"

// checkDuplicate refuses a new manual entry identical (kind, amount, date, category, method; never the description) to one
// the same person saved minutes ago, or to one another member saved for that day: the client asks and resends with allowDuplicate.
func checkDuplicate(ctx context.Context, q httpx.Querier, walletID, userID int64, in *transactionInput) error {
	var desc, name string
	var by int64
	err := q.QueryRowContext(ctx, `
		SELECT t.description, t.created_by, u.name FROM transactions t JOIN users u ON u.id = t.created_by
		WHERE t.wallet_id = ? AND t.kind = ? AND t.amount_cents = ? AND t.date = ? AND t.category_id = ? AND t.payment_method_id IS ?
		  AND (t.created_by <> ? OR t.created_at >= strftime('%Y-%m-%dT%H:%M:%SZ', 'now', ?))
		ORDER BY t.created_by = ? DESC, t.id DESC LIMIT 1`,
		walletID, in.Kind, in.AmountCents, in.Date, in.CategoryID, in.PaymentMethodID, userID, duplicateWindow, userID,
	).Scan(&desc, &by, &name)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	msg := strings.Fields(name + " Alguém")[0] + " já lançou um igual neste dia"
	if by == userID {
		msg = "Você lançou um igual há poucos minutos"
	}
	if desc != "" {
		return &httpx.APIError{Status: 409, Code: "duplicate", Message: msg + `: "` + desc + `".`}
	}
	return &httpx.APIError{Status: 409, Code: "duplicate", Message: msg + "."}
}
