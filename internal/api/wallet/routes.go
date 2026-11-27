// Package wallet serves wallets and their members: the list, create, update and delete of wallets,
// membership and roles, each member's default payment method, and the access check for /api/wallets/{walletID}.
package wallet

import (
	"net/http"

	"gastos/internal/api/httpx"
)

// Handler serves the wallet and member routes.
type Handler struct {
	*httpx.Env
}

// Routes registers the wallet and member routes on mux. user wraps a route that needs a session and
// inWallet one that also needs membership of the wallet in the path.
func (s *Handler) Routes(mux *http.ServeMux, user func(httpx.HandlerFunc) http.Handler, inWallet httpx.RouteWrap) {
	mux.Handle("GET /api/wallets", user(s.listWallets))
	mux.Handle("POST /api/wallets", user(s.createWallet))
	mux.Handle("PATCH /api/wallets/{walletID}", inWallet(httpx.RoleOwner, s.updateWallet))
	mux.Handle("DELETE /api/wallets/{walletID}", inWallet(httpx.RoleOwner, s.deleteWallet))
	mux.Handle("GET /api/wallets/{walletID}/members", inWallet(httpx.RoleReader, s.listMembers))
	mux.Handle("POST /api/wallets/{walletID}/members", inWallet(httpx.RoleOwner, s.addMember))
	mux.Handle("PATCH /api/wallets/{walletID}/members/{userID}", inWallet(httpx.RoleOwner, s.updateMember))
	mux.Handle("PUT /api/wallets/{walletID}/default-payment-method", inWallet(httpx.RoleEditor, s.setDefaultPaymentMethod))
	mux.Handle("DELETE /api/wallets/{walletID}/members/{userID}", inWallet(httpx.RoleReader, s.removeMember)) // owner or the member themselves: checked in the handler
}
