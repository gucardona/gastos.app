// Package api serves the JSON API under /api, the web app and /healthz.
package api

import (
	"database/sql"
	"io/fs"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"gastos/internal/api/httpx"
	"gastos/internal/config"
	"gastos/internal/events"
	"gastos/internal/recurrence"
)

// Server holds what the HTTP handlers share: the database, the settings and the background services.
type Server struct {
	*httpx.Env
	limiter *httpx.RateLimiter

	anthropicURL string

	chatLimiter  *httpx.RateLimiter
	chatMu       sync.Mutex
	chatInFlight map[int64]bool // users with an answer being generated
}

// New returns a Server ready to route requests.
func New(db *sql.DB, cfg config.Config, bus *events.Bus, mat *recurrence.Materializer, log *slog.Logger) *Server {
	aiURL := cfg.AnthropicURL
	if aiURL == "" {
		aiURL = config.DefaultAnthropicURL
	}
	env := &httpx.Env{DB: db, Cfg: cfg, Bus: bus, Mat: mat, Log: log, Now: time.Now}
	return &Server{Env: env, limiter: httpx.NewRateLimiter(10, 15*time.Minute), anthropicURL: aiURL,
		chatLimiter: httpx.NewRateLimiter(20, time.Minute), chatInFlight: map[int64]bool{}}
}

// Routes returns the handler for every route: the API, /healthz and the web app served from spa,
// wrapped in logging, panic recovery and the security headers.
func (s *Server) Routes(spa fs.FS) http.Handler {
	mux := http.NewServeMux()
	public := s.Handle
	user := func(h httpx.HandlerFunc) http.Handler { return s.RequireUser(s.Handle(h)) }
	inWallet := func(minRole httpx.Role, h httpx.HandlerFunc) http.Handler {
		return s.RequireUser(s.RequireWallet(minRole, s.Handle(h)))
	}

	mux.Handle("GET /api/auth/registration", public(s.registrationInfo))
	mux.Handle("POST /api/auth/register", public(s.register))
	mux.Handle("POST /api/auth/login", public(s.login))
	mux.Handle("POST /api/auth/logout", user(s.logout))
	mux.Handle("GET /api/me", user(s.me))
	mux.Handle("GET /api/events", user(s.events))
	mux.Handle("PATCH /api/me", user(s.rename))
	mux.Handle("PUT /api/me/password", user(s.changePassword))
	mux.Handle("GET /api/me/ai", user(s.getAI))
	mux.Handle("PUT /api/me/ai", user(s.putAI))
	mux.Handle("DELETE /api/me/ai/key", user(s.deleteAIKey))
	mux.Handle("POST /api/chat", user(s.postChat))
	mux.Handle("GET /api/chat", user(s.getChat))
	mux.Handle("DELETE /api/chat", user(s.deleteChat))

	mux.Handle("GET /api/wallets", user(s.listWallets))
	mux.Handle("POST /api/wallets", user(s.createWallet))
	mux.Handle("PATCH /api/wallets/{walletID}", inWallet(httpx.RoleOwner, s.updateWallet))
	mux.Handle("DELETE /api/wallets/{walletID}", inWallet(httpx.RoleOwner, s.deleteWallet))
	mux.Handle("GET /api/wallets/{walletID}/members", inWallet(httpx.RoleReader, s.listMembers))
	mux.Handle("POST /api/wallets/{walletID}/members", inWallet(httpx.RoleOwner, s.addMember))
	mux.Handle("PATCH /api/wallets/{walletID}/members/{userID}", inWallet(httpx.RoleOwner, s.updateMember))
	mux.Handle("PUT /api/wallets/{walletID}/default-payment-method", inWallet(httpx.RoleEditor, s.setDefaultPaymentMethod))
	mux.Handle("DELETE /api/wallets/{walletID}/members/{userID}", inWallet(httpx.RoleReader, s.removeMember)) // owner or the member themselves: checked in the handler

	s.catalogRoutes(mux, inWallet)
	s.transactionRoutes(mux, inWallet)
	s.recurrenceRoutes(mux, inWallet)
	s.settlementRoutes(mux, inWallet)
	s.budgetRoutes(mux, inWallet)
	s.bankRoutes(mux, inWallet)
	s.summaryRoutes(mux, inWallet)

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

// statusRecorder remembers the status code a handler wrote, for the request log.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader records code and passes it on.
func (r *statusRecorder) WriteHeader(code int) { r.status = code; r.ResponseWriter.WriteHeader(code) }

// Unwrap returns the wrapped writer, so http.ResponseController can still flush through it.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter } // keeps Flusher available through ResponseController

// logRequests logs the method, path, status and duration of every request.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := s.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		s.Log.InfoContext(r.Context(), "http", "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"ms", s.Now().Sub(start).Milliseconds())
	})
}

// recoverPanic turns a panic in a handler into a logged 500 instead of a dropped connection.
func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.Log.ErrorContext(r.Context(), "panic", "value", v, "stack", string(debug.Stack()))
				httpx.WriteJSON(w, 500, map[string]any{"error": &httpx.APIError{Code: "internal", Message: "Erro interno"}})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// devCORS allows Vite on localhost:5173 only when GASTOS_DEV=true.
func (s *Server) devCORS(next http.Handler) http.Handler {
	if !s.Cfg.Dev {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") == "http://localhost:5173" {
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// csp suits the built SPA: same-origin scripts, styles and bundled fonts; React sets inline style attributes.
const csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; " +
	"font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

// securityHeaders goes on every response; API responses are not cached unless the handler says otherwise (SSE does).
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", csp)
		if s.Cfg.CookieSecure {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store") // a handler's own Set replaces it
		}
		next.ServeHTTP(w, r)
	})
}

var errCrossSite = &httpx.APIError{Status: 403, Code: "forbidden", Message: "Requisição de outro site bloqueada"}

// sameOriginWrites refuses API writes that the browser says come from another origin, a CSRF barrier for
// the endpoints that take no JSON body. Without the header (old browsers, curl) the request goes through.
func (s *Server) sameOriginWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			site := r.Header.Get("Sec-Fetch-Site")
			ok := site == "" || site == "same-origin" || (s.Cfg.Dev && site == "same-site") // dev: Vite on another port
			if !ok && strings.HasPrefix(r.URL.Path, "/api/") {
				s.WriteError(w, r, errCrossSite)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
