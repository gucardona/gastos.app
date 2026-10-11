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

type apiError struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func (e *apiError) Error() string { return e.Code + ": " + e.Message }

var (
	errUnauthenticated = &apiError{Status: 401, Code: "unauthenticated", Message: "Faça login para continuar"}
	errForbidden       = &apiError{Status: 403, Code: "forbidden", Message: "Sem permissão para esta ação"}
	errNotFound        = &apiError{Status: 404, Code: "not_found", Message: "Não encontrado"}
	errRateLimited     = &apiError{Status: 429, Code: "rate_limited", Message: "Muitas tentativas, aguarde alguns minutos"}
)

func errValidation(fields map[string]string) *apiError {
	return &apiError{Status: 400, Code: "validation", Message: "Dados inválidos", Fields: fields}
}

func errBadRequest(msg string) *apiError {
	return &apiError{Status: 400, Code: "validation", Message: msg}
}

func errConflict(msg string) *apiError {
	return &apiError{Status: 409, Code: "conflict", Message: msg}
}

func errNotFoundMsg(msg string) *apiError {
	return &apiError{Status: 404, Code: "not_found", Message: msg}
}

// fieldErrors collects validation errors per field (the first error of each field wins).
type fieldErrors map[string]string

func (f fieldErrors) add(field, msg string) {
	if _, ok := f[field]; !ok {
		f[field] = msg
	}
}

func (f fieldErrors) err() error {
	if len(f) == 0 {
		return nil
	}
	return errValidation(f)
}

type handlerFunc func(w http.ResponseWriter, r *http.Request) error

func (s *Server) handle(h handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			s.writeError(w, r, err)
		}
	})
}

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

func pathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, errNotFound
	}
	return id, nil
}

func (s *Server) mutated(r *http.Request) {
	s.bus.Notify(walletIDFrom(r.Context()), userIDFrom(r.Context()))
}

func asAPIError(err error, target **apiError) bool { return errors.As(err, target) }

func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint")
}
