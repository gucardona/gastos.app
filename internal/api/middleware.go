package api

import (
	"net/http"
	"runtime/debug"
	"strings"

	"gastos/internal/api/httpx"
)

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
