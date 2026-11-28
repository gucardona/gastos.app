package ledger

import (
	"context"

	"gastos/internal/api/httpx"
	"gastos/internal/money"
)

// Split is one member's share of a transaction, in basis points and in cents.
type Split struct {
	UserID      int64 `json:"userId"`
	ShareBP     int64 `json:"shareBp"`
	AmountCents int64 `json:"amountCents"`
}

// resolveSplits applies the spec's split rules and returns the final splits (without amountCents).
func resolveSplits(ctx context.Context, q httpx.Querier, walletID int64, kind string, requested []Split) ([]Split, error) {
	var splitting bool
	if err := q.QueryRowContext(ctx, `SELECT splitting_enabled FROM wallets WHERE id = ?`, walletID).Scan(&splitting); err != nil {
		return nil, err
	}
	if !splitting || kind != "expense" {
		if len(requested) > 0 {
			return nil, httpx.ValidationError(map[string]string{"splits": "divisão não se aplica"})
		}
		return nil, nil
	}

	rows, err := q.QueryContext(ctx, `SELECT user_id, splits_by_default, COALESCE(default_share_bp, 0) FROM wallet_members WHERE wallet_id = ? ORDER BY user_id`, walletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	members := map[int64]bool{}
	var defaults, defaultShares []int64
	allShared := true
	for rows.Next() {
		var id, share int64
		var byDefault bool
		if err := rows.Scan(&id, &byDefault, &share); err != nil {
			return nil, err
		}
		members[id] = true
		if byDefault {
			defaults = append(defaults, id)
			defaultShares = append(defaultShares, share)
			allShared = allShared && share > 0
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(requested) == 0 {
		if len(defaults) == 0 {
			return nil, nil
		}
		// Every flagged member's default share, normalized to 100%; equal split if any of them has none.
		shares := money.EqualShares(len(defaults))
		if allShared {
			shares = money.Allocate(money.FullShare, defaultShares)
		}
		var out []Split
		for i, id := range defaults {
			if shares[i] > 0 { // a tiny share can round to 0%: that member does not take part
				out = append(out, Split{UserID: id, ShareBP: shares[i]})
			}
		}
		return out, nil
	}

	seen := map[int64]bool{}
	var total int64
	for _, sp := range requested {
		switch {
		case !members[sp.UserID]:
			return nil, httpx.ValidationError(map[string]string{"splits": "participante não é membro da carteira"})
		case seen[sp.UserID]:
			return nil, httpx.ValidationError(map[string]string{"splits": "participante repetido"})
		case sp.ShareBP < 1 || sp.ShareBP > money.FullShare:
			return nil, httpx.ValidationError(map[string]string{"splits": "parte deve estar entre 0,01% e 100%"})
		}
		seen[sp.UserID] = true
		total += sp.ShareBP
	}
	if total != money.FullShare {
		return nil, httpx.ValidationError(map[string]string{"splits": "as partes devem somar 100%"})
	}
	out := make([]Split, len(requested))
	for i, sp := range requested {
		out[i] = Split{UserID: sp.UserID, ShareBP: sp.ShareBP}
	}
	return out, nil
}

// WithAmounts fills in each split's amount from total and its share, without losing a cent.
func WithAmounts(total int64, splits []Split) []Split {
	shares := make([]int64, len(splits))
	for i, sp := range splits {
		shares[i] = sp.ShareBP
	}
	amounts := money.Allocate(total, shares)
	out := make([]Split, len(splits))
	for i, sp := range splits {
		out[i] = Split{UserID: sp.UserID, ShareBP: sp.ShareBP, AmountCents: amounts[i]}
	}
	return out
}

// replaceTransactionSplits replaces the member shares of transaction txID with splits.
func replaceTransactionSplits(ctx context.Context, q httpx.Querier, txID int64, splits []Split) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM transaction_splits WHERE transaction_id = ?`, txID); err != nil {
		return err
	}
	for _, sp := range splits {
		if _, err := q.ExecContext(ctx, `INSERT INTO transaction_splits (transaction_id, user_id, share_bp) VALUES (?, ?, ?)`,
			txID, sp.UserID, sp.ShareBP); err != nil {
			return err
		}
	}
	return nil
}
