package api

import (
	"database/sql"
	"io/fs"
	"log/slog"
	"net/http"
	"runtime/debug"
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
}

func New(db *sql.DB, cfg config.Config, bus *events.Bus, mat *recurrence.Materializer, log *slog.Logger) *Server {
	return &Server{db: db, cfg: cfg, bus: bus, mat: mat, log: log, limiter: newRateLimiter(10, 15*time.Minute), now: time.Now}
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
	s.summaryRoutes(mux, wallet)
	s.eventsRoutes(mux, wallet)

	mux.HandleFunc("GET /healthz", s.healthz)
	mux.Handle("/api/", public(func(http.ResponseWriter, *http.Request) error { return errNotFound }))
	mux.Handle("/", spaHandler(spa))

	return s.logRequests(s.recoverPanic(s.devCORS(mux)))
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
