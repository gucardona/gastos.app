package httpx

import "errors"

// APIError is an error the client sees as {"error": {...}}, with a message ready to show in pt-BR.
type APIError struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// Error returns the code and the message.
func (e *APIError) Error() string { return e.Code + ": " + e.Message }

var (
	// ErrUnauthenticated is the 401 for a request without a valid session.
	ErrUnauthenticated = &APIError{Status: 401, Code: "unauthenticated", Message: "Faça login para continuar"}
	// ErrForbidden is the 403 for a user whose role is too low.
	ErrForbidden = &APIError{Status: 403, Code: "forbidden", Message: "Sem permissão para esta ação"}
	// ErrNotFound is the 404 for something that does not exist or is not the caller's to see.
	ErrNotFound = &APIError{Status: 404, Code: "not_found", Message: "Não encontrado"}
	// ErrRateLimited is the 429 for too many attempts.
	ErrRateLimited = &APIError{Status: 429, Code: "rate_limited", Message: "Muitas tentativas, aguarde alguns minutos"}
)

// ValidationError returns a 400 that lists the invalid fields.
func ValidationError(fields map[string]string) *APIError {
	return &APIError{Status: 400, Code: "validation", Message: "Dados inválidos", Fields: fields}
}

// ErrBadRequest returns a 400 with msg.
func ErrBadRequest(msg string) *APIError {
	return &APIError{Status: 400, Code: "validation", Message: msg}
}

// ErrConflict returns a 409 with msg.
func ErrConflict(msg string) *APIError {
	return &APIError{Status: 409, Code: "conflict", Message: msg}
}

// ErrNotFoundMsg returns a 404 with msg.
func ErrNotFoundMsg(msg string) *APIError {
	return &APIError{Status: 404, Code: "not_found", Message: msg}
}

// FieldErrors collects validation errors per field (the first error of each field wins).
type FieldErrors map[string]string

// Add records msg for field, keeping the first message when a field fails twice.
func (f FieldErrors) Add(field, msg string) {
	if _, ok := f[field]; !ok {
		f[field] = msg
	}
}

// Err returns a validation error when any field failed, and nil otherwise.
func (f FieldErrors) Err() error {
	if len(f) == 0 {
		return nil
	}
	return ValidationError(f)
}

// AsAPIError reports whether err is an *APIError and stores it in target.
func AsAPIError(err error, target **APIError) bool { return errors.As(err, target) }
