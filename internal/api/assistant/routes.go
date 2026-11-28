// Package assistant serves the query assistant: the chat and its tools, the user's AI settings, the cost
// estimate and the USD-BRL rate behind it.
package assistant

import (
	"net/http"
	"sync"

	"gastos/internal/api/httpx"
)

// Handler serves the assistant routes. It holds the chat rate limiter, the users with an answer being
// generated and the Anthropic URL.
type Handler struct {
	*httpx.Env
	chatLimiter  *httpx.RateLimiter
	chatMu       sync.Mutex
	chatInFlight map[int64]bool // users with an answer being generated
	anthropicURL string
}

// New returns a Handler sharing env, with the chat rate limit and the Anthropic URL (aiURL) set.
func New(env *httpx.Env, aiURL string, chatLimiter *httpx.RateLimiter) *Handler {
	return &Handler{Env: env, anthropicURL: aiURL, chatLimiter: chatLimiter, chatInFlight: map[int64]bool{}}
}

// Routes registers the AI settings and chat routes on mux. They are session-only, so user wraps them.
func (s *Handler) Routes(mux *http.ServeMux, user func(httpx.HandlerFunc) http.Handler) {
	mux.Handle("GET /api/me/ai", user(s.getAI))
	mux.Handle("PUT /api/me/ai", user(s.putAI))
	mux.Handle("DELETE /api/me/ai/key", user(s.deleteAIKey))
	mux.Handle("POST /api/chat", user(s.postChat))
	mux.Handle("GET /api/chat", user(s.getChat))
	mux.Handle("DELETE /api/chat", user(s.deleteChat))
}
