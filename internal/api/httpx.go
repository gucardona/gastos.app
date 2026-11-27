package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

// apiError is an error the client sees as {"error": {...}}, with a message ready to show in pt-BR.
type apiError struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// Error returns the code and the message.
func (e *apiError) Error() string { return e.Code + ": " + e.Message }

var (
	errUnauthenticated = &apiError{Status: 401, Code: "unauthenticated", Message: "Faça login para continuar"}
	errForbidden       = &apiError{Status: 403, Code: "forbidden", Message: "Sem permissão para esta ação"}
	errNotFound        = &apiError{Status: 404, Code: "not_found", Message: "Não encontrado"}
	errRateLimited     = &apiError{Status: 429, Code: "rate_limited", Message: "Muitas tentativas, aguarde alguns minutos"}
)

// errValidation returns a 400 that lists the invalid fields.
func errValidation(fields map[string]string) *apiError {
	return &apiError{Status: 400, Code: "validation", Message: "Dados inválidos", Fields: fields}
}

// errBadRequest returns a 400 with msg.
func errBadRequest(msg string) *apiError {
	return &apiError{Status: 400, Code: "validation", Message: msg}
}

// errConflict returns a 409 with msg.
func errConflict(msg string) *apiError {
	return &apiError{Status: 409, Code: "conflict", Message: msg}
}

// errNotFoundMsg returns a 404 with msg.
func errNotFoundMsg(msg string) *apiError {
	return &apiError{Status: 404, Code: "not_found", Message: msg}
}

// fieldErrors collects validation errors per field (the first error of each field wins).
type fieldErrors map[string]string

// add records msg for field, keeping the first message when a field fails twice.
func (f fieldErrors) add(field, msg string) {
	if _, ok := f[field]; !ok {
		f[field] = msg
	}
}

// err returns a validation error when any field failed, and nil otherwise.
func (f fieldErrors) err() error {
	if len(f) == 0 {
		return nil
	}
	return errValidation(f)
}

// handlerFunc is an HTTP handler that returns its error instead of writing it.
type handlerFunc func(w http.ResponseWriter, r *http.Request) error

// handle adapts h to an http.Handler that writes h's error as a JSON error response.
func (s *Server) handle(h handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			s.writeError(w, r, err)
		}
	})
}

// writeError writes an apiError as is, sql.ErrNoRows as a 404, and logs any other error as a 500
// with a generic message.
func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var ae *apiError
	if errors.As(err, &ae) {
		writeJSON(w, ae.Status, map[string]any{"error": ae})
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 404, map[string]any{"error": errNotFound})
		return
	}
	s.log.ErrorContext(r.Context(), "erro interno", "err", err, "path", r.URL.Path)
	writeJSON(w, 500, map[string]any{"error": &apiError{Code: "internal", Message: "Erro interno"}})
}

// writeJSON writes v as JSON with status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// decode requires JSON (a CSRF barrier together with SameSite=Lax), caps the body at 1 MB and rejects unknown fields.
func decode(w http.ResponseWriter, r *http.Request, dst any) error {
	mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mt != "application/json" {
		return &apiError{Status: 415, Code: "validation", Message: "Content-Type deve ser application/json"}
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return errBadRequest("JSON inválido")
	}
	return nil
}

// querier is satisfied by *sql.DB and *sql.Tx.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// inTx runs fn in a transaction (BEGIN IMMEDIATE through the DSN).
func (s *Server) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// pathID reads the positive integer path value name, or returns a 404.
func pathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, errNotFound
	}
	return id, nil
}

// mutated tells every member of the request's wallet (the one who made the change included, for their other tabs).
func (s *Server) mutated(r *http.Request) {
	s.notifyWallet(r.Context(), walletIDFrom(r.Context()))
}

// asAPIError reports whether err is an *apiError and stores it in target.
func asAPIError(err error, target **apiError) bool { return errors.As(err, target) }

// isUnique reports whether err comes from a UNIQUE constraint.
func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint")
}
