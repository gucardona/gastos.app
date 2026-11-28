// Package api serves the JSON API under /api, the web app and /healthz.
package api

import (
	"database/sql"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"gastos/internal/api/assistant"
	"gastos/internal/api/auth"
	"gastos/internal/api/bank"
	"gastos/internal/api/httpx"
	"gastos/internal/api/ledger"
	"gastos/internal/api/wallet"
	"gastos/internal/config"
	"gastos/internal/events"
	"gastos/internal/recurrence"
)

// Server holds what the HTTP handlers share: the database, the settings and the background services.
type Server struct {
	*httpx.Env
	wallet *wallet.Handler
	ledger *ledger.Handler
	bank   *bank.Handler
	auth   *auth.Handler

	assistant *assistant.Handler
}

// New returns a Server ready to route requests.
func New(db *sql.DB, cfg config.Config, bus *events.Bus, mat *recurrence.Materializer, log *slog.Logger) *Server {
	aiURL := cfg.AnthropicURL
	if aiURL == "" {
		aiURL = config.DefaultAnthropicURL
	}
	env := &httpx.Env{DB: db, Cfg: cfg, Bus: bus, Mat: mat, Log: log, Now: time.Now}
	w := &wallet.Handler{Env: env}
	ai := assistant.New(env, aiURL, httpx.NewRateLimiter(20, time.Minute))
	return &Server{Env: env, wallet: w, ledger: &ledger.Handler{Env: env}, bank: &bank.Handler{Env: env},
		auth:      auth.New(env, httpx.NewRateLimiter(10, 15*time.Minute), w, ai),
		assistant: ai}
}

// Assistant returns the query assistant handler. It exists for the test harness.
func (s *Server) Assistant() *assistant.Handler { return s.assistant }

// Routes returns the handler for every route: the API, /healthz and the web app served from spa,
// wrapped in logging, panic recovery and the security headers.
func (s *Server) Routes(spa fs.FS) http.Handler {
	mux := http.NewServeMux()
	public := s.Handle
	user := func(h httpx.HandlerFunc) http.Handler { return s.auth.RequireUser(s.Handle(h)) }
	inWallet := func(minRole httpx.Role, h httpx.HandlerFunc) http.Handler {
		return s.auth.RequireUser(s.wallet.RequireWallet(minRole, s.Handle(h)))
	}

	s.auth.Routes(mux, public, user)
	mux.Handle("GET /api/events", user(s.events))

	s.assistant.Routes(mux, user)

	s.wallet.Routes(mux, user, inWallet)

	s.ledger.Routes(mux, user, inWallet)
	s.bank.Routes(mux, inWallet)

	mux.HandleFunc("GET /healthz", s.healthz)
	mux.Handle("/api/", public(func(http.ResponseWriter, *http.Request) error { return httpx.ErrNotFound }))
	mux.Handle("/", spaHandler(spa))

	return s.logRequests(s.recoverPanic(s.securityHeaders(s.devCORS(s.sameOriginWrites(mux)))))
}

// healthz answers "ok" while the database responds, and 503 otherwise.
func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if err := s.DB.PingContext(r.Context()); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ok"))
}
