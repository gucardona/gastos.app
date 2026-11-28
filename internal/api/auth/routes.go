// Package auth serves sign-up, sessions, the signed-in user's profile and password, and the invite codes.
package auth

import (
	"net/http"

	"gastos/internal/api/assistant"
	"gastos/internal/api/httpx"
	"gastos/internal/api/wallet"
)

// Handler serves the auth routes. It holds the login and sign-up rate limiter, the wallet handler that lists
// the user's wallets and the assistant handler that tells whether the assistant is on.
type Handler struct {
	*httpx.Env
	limiter *httpx.RateLimiter
	Wallet  *wallet.Handler
	AI      *assistant.Handler
}

// New returns a Handler sharing env, with the rate limiter for logins and sign-ups set.
func New(env *httpx.Env, limiter *httpx.RateLimiter, w *wallet.Handler, ai *assistant.Handler) *Handler {
	return &Handler{Env: env, limiter: limiter, Wallet: w, AI: ai}
}

// Routes registers the sign-up, session and profile routes on mux. public serves the routes that need no
// session and user wraps the others.
func (s *Handler) Routes(mux *http.ServeMux, public, user func(httpx.HandlerFunc) http.Handler) {
	mux.Handle("GET /api/auth/registration", public(s.registrationInfo))
	mux.Handle("POST /api/auth/register", public(s.register))
	mux.Handle("POST /api/auth/login", public(s.login))
	mux.Handle("POST /api/auth/logout", user(s.logout))
	mux.Handle("GET /api/me", user(s.me))
	mux.Handle("PATCH /api/me", user(s.rename))
	mux.Handle("PUT /api/me/password", user(s.changePassword))
}
