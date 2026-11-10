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

	"gastos/internal/config"
	"gastos/internal/events"
	"gastos/internal/recurrence"
)

type Server struct {
	db      *sql.DB
	cfg     config.Config
	bus     *events.Bus
	mat     *recurrence.Materializer
	log     *slog.Logger
	limiter *rateLimiter
	now     func() time.Time

	anthropicURL string

	chatLimiter  *rateLimiter
	chatMu       sync.Mutex
	chatInFlight map[int64]bool // users with an answer being generated
}

func New(db *sql.DB, cfg config.Config, bus *events.Bus, mat *recurrence.Materializer, log *slog.Logger) *Server {
	aiURL := cfg.AnthropicURL
	if aiURL == "" {
		aiURL = config.DefaultAnthropicURL
	}
	return &Server{db: db, cfg: cfg, bus: bus, mat: mat, log: log, limiter: newRateLimiter(10, 15*time.Minute), now: time.Now, anthropicURL: aiURL,
		chatLimiter: newRateLimiter(20, time.Minute), chatInFlight: map[int64]bool{}}
}

func (s *Server) Routes(spa fs.FS) http.Handler {
	mux := http.NewServeMux()
	public := s.handle
	user := func(h handlerFunc) http.Handler { return s.requireUser(s.handle(h)) }
	wallet := func(min role, h handlerFunc) http.Handler { return s.requireUser(s.requireWallet(min, s.handle(h))) }

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
	mux.Handle("PATCH /api/wallets/{walletID}", wallet(roleOwner, s.updateWallet))
	mux.Handle("DELETE /api/wallets/{walletID}", wallet(roleOwner, s.deleteWallet))
	mux.Handle("GET /api/wallets/{walletID}/members", wallet(roleReader, s.listMembers))
	mux.Handle("POST /api/wallets/{walletID}/members", wallet(roleOwner, s.addMember))
	mux.Handle("PATCH /api/wallets/{walletID}/members/{userID}", wallet(roleOwner, s.updateMember))
	mux.Handle("DELETE /api/wallets/{walletID}/members/{userID}", wallet(roleReader, s.removeMember)) // owner or the member themselves: checked in the handler

	s.catalogRoutes(mux, wallet)
	s.transactionRoutes(mux, wallet)
	s.recurrenceRoutes(mux, wallet)
	s.settlementRoutes(mux, wallet)
	s.budgetRoutes(mux, wallet)
	s.bankRoutes(mux, wallet)
	s.summaryRoutes(mux, wallet)

	mux.HandleFunc("GET /healthz", s.healthz)
	mux.Handle("/api/", public(func(http.ResponseWriter, *http.Request) error { return errNotFound }))
	mux.Handle("/", spaHandler(spa))

	return s.logRequests(s.recoverPanic(s.securityHeaders(s.devCORS(s.sameOriginWrites(mux)))))
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if err := s.db.PingContext(r.Context()); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ok"))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int)        { r.status = code; r.ResponseWriter.WriteHeader(code) }
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter } // keeps Flusher available through ResponseController

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := s.now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		s.log.InfoContext(r.Context(), "http", "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"ms", s.now().Sub(start).Milliseconds())
	})
}

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.log.ErrorContext(r.Context(), "panic", "value", v, "stack", string(debug.Stack()))
				writeJSON(w, 500, map[string]any{"error": &apiError{Code: "internal", Message: "Erro interno"}})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// devCORS allows Vite on localhost:5173 only when GASTOS_DEV=true.
func (s *Server) devCORS(next http.Handler) http.Handler {
	if !s.cfg.Dev {
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
		if s.cfg.CookieSecure {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store") // a handler's own Set replaces it
		}
		next.ServeHTTP(w, r)
	})
}

var errCrossSite = &apiError{Status: 403, Code: "forbidden", Message: "Requisição de outro site bloqueada"}

// sameOriginWrites refuses API writes that the browser says come from another origin, a CSRF barrier for
// the endpoints that take no JSON body. Without the header (old browsers, curl) the request goes through.
func (s *Server) sameOriginWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			site := r.Header.Get("Sec-Fetch-Site")
			ok := site == "" || site == "same-origin" || (s.cfg.Dev && site == "same-site") // dev: Vite on another port
			if !ok && strings.HasPrefix(r.URL.Path, "/api/") {
				s.writeError(w, r, errCrossSite)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
