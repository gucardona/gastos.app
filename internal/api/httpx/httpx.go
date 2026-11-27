// Package httpx holds the HTTP plumbing the API's area packages share: the error type, JSON helpers,
// request context values, roles, the rate limiter and the Env the handlers run in.
package httpx

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gastos/internal/config"
	"gastos/internal/events"
	"gastos/internal/recurrence"
)

// DateLayout is the layout of every stored date (YYYY-MM-DD).
const DateLayout = "2006-01-02"

// Env holds what the HTTP handlers share: the database, the settings and the background services.
type Env struct {
	DB  *sql.DB
	Cfg config.Config
	Bus *events.Bus
	Mat *recurrence.Materializer
	Log *slog.Logger
	Now func() time.Time
}

// HandlerFunc is an HTTP handler that returns its error instead of writing it.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// RouteWrap wraps a handler for a wallet route that needs at least the given role.
type RouteWrap = func(Role, HandlerFunc) http.Handler

// Handle adapts h to an http.Handler that writes h's error as a JSON error response.
func (s *Env) Handle(h HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			s.WriteError(w, r, err)
		}
	})
}

// WriteError writes an APIError as is, sql.ErrNoRows as a 404, and logs any other error as a 500
// with a generic message.
func (s *Env) WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var ae *APIError
	if errors.As(err, &ae) {
		WriteJSON(w, ae.Status, map[string]any{"error": ae})
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		WriteJSON(w, 404, map[string]any{"error": ErrNotFound})
		return
	}
	s.Log.ErrorContext(r.Context(), "erro interno", "err", err, "path", r.URL.Path)
	WriteJSON(w, 500, map[string]any{"error": &APIError{Code: "internal", Message: "Erro interno"}})
}

// WriteJSON writes v as JSON with status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Decode requires JSON (a CSRF barrier together with SameSite=Lax), caps the body at 1 MB and rejects unknown fields.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mt != "application/json" {
		return &APIError{Status: 415, Code: "validation", Message: "Content-Type deve ser application/json"}
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return ErrBadRequest("JSON inválido")
	}
	return nil
}

// Querier is satisfied by *sql.DB and *sql.Tx.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// InTx runs fn in a transaction (BEGIN IMMEDIATE through the DSN).
func (s *Env) InTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	//nolint:errcheck // moved unchanged, see docs/security.md
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// PathID reads the positive integer path value name, or returns a 404.
func PathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, ErrNotFound
	}
	return id, nil
}

// Mutated tells every member of the request's wallet (the one who made the change included, for their other tabs).
func (s *Env) Mutated(r *http.Request) {
	s.NotifyWallet(r.Context(), WalletIDFrom(r.Context()))
}

// ValidDate reports whether s is a valid YYYY-MM-DD date.
func ValidDate(s string) bool { _, err := time.Parse(DateLayout, s); return err == nil }

// IsUnique reports whether err comes from a UNIQUE constraint.
func IsUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint")
}

// NotifyUsers tells every open connection of these users that something changed.
func (s *Env) NotifyUsers(ids ...int64) { s.Bus.Notify(ids...) }

// WalletMembers lists who belongs to the wallet; read it before a delete to still reach the people being removed.
func (s *Env) WalletMembers(ctx context.Context, walletID int64) ([]int64, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT user_id FROM wallet_members WHERE wallet_id = ?`, walletID)
	if err != nil {
		return nil, err
	}
	//nolint:errcheck // moved unchanged, see docs/security.md
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// NotifyCtx outlives the request: the change is saved, so a client hanging up must not stop the notice to the others.
func NotifyCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

// NotifyWallet tells every member of the wallet. The change is already saved, so a failure is only logged.
func (s *Env) NotifyWallet(ctx context.Context, walletID int64) {
	ctx, cancel := NotifyCtx(ctx)
	defer cancel()
	ids, err := s.WalletMembers(ctx, walletID)
	if err != nil {
		s.Log.ErrorContext(ctx, "notify wallet members", "wallet", walletID, "err", err)
		return
	}
	s.NotifyUsers(ids...)
}
