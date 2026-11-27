package wallet

import (
	"net/http"

	"gastos/internal/api/httpx"
)

// RequireWallet requires the user to be a member of the wallet in the path with role >= minRole.
// Non-members get 404 so the wallet's existence is not revealed.
func (s *Handler) RequireWallet(minRole httpx.Role, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		walletID, err := httpx.PathID(r, "walletID")
		if err != nil {
			s.WriteError(w, r, err)
			return
		}
		var roleStr string
		err = s.DB.QueryRowContext(r.Context(),
			`SELECT role FROM wallet_members WHERE wallet_id = ? AND user_id = ?`,
			walletID, httpx.UserIDFrom(r.Context())).Scan(&roleStr)
		if err != nil {
			s.WriteError(w, r, err) // ErrNoRows → 404
			return
		}
		have, _ := httpx.ParseRole(roleStr)
		if have < minRole {
			s.WriteError(w, r, httpx.ErrForbidden)
			return
		}
		ctx := httpx.WithWallet(r.Context(), walletID, have)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
