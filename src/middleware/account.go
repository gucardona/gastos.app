package middleware

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"gastos/src/db"
)

const (
	AccountIDKey   ctxKey = "accountID"
	AccountRoleKey ctxKey = "accountRole"
)

func Account(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountIDValue := strings.TrimSpace(r.Header.Get("X-Account-ID"))
		if accountIDValue == "" {
			writeAuthError(w, "missing account id", http.StatusBadRequest)
			return
		}

		accountID, err := strconv.ParseInt(accountIDValue, 10, 64)
		if err != nil || accountID <= 0 {
			writeAuthError(w, "invalid account id", http.StatusBadRequest)
			return
		}

		userID := UserIDFromContext(r.Context())
		if userID <= 0 {
			writeAuthError(w, "invalid user context", http.StatusUnauthorized)
			return
		}

		var role string
		err = db.DB.QueryRow(`
			SELECT role
			FROM account_members
			WHERE account_id = ? AND user_id = ?
		`, accountID, userID).Scan(&role)
		if err != nil {
			if err == sql.ErrNoRows {
				writeAuthError(w, "account access denied", http.StatusForbidden)
				return
			}
			writeAuthError(w, "account lookup failed", http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), AccountIDKey, accountID)
		ctx = context.WithValue(ctx, AccountRoleKey, role)
		next(w, r.WithContext(ctx))
	}
}

func AccountIDFromContext(ctx context.Context) int64 {
	accountID, _ := ctx.Value(AccountIDKey).(int64)
	return accountID
}

func AccountRoleFromContext(ctx context.Context) string {
	role, _ := ctx.Value(AccountRoleKey).(string)
	return role
}
