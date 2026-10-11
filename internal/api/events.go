package api

import (
	"fmt"
	"net/http"
	"time"
)

func (s *Server) eventsRoutes(mux *http.ServeMux, wallet routeWrap) {
	mux.Handle("GET /api/wallets/{walletID}/events", wallet(roleReader, s.events))
}

// events mantém uma conexão SSE aberta e avisa quando outro membro altera a carteira.
// Autentica pelo cookie de sessão, sem token na URL.
func (s *Server) events(w http.ResponseWriter, r *http.Request) error {
	walletID, userID := walletIDFrom(r.Context()), userIDFrom(r.Context())
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // nginx
	w.WriteHeader(http.StatusOK)
	if err := rc.Flush(); err != nil {
		return nil // sem suporte a streaming; nada a fazer
	}

	ch := s.bus.Subscribe(walletID, userID)
	defer s.bus.Unsubscribe(walletID, ch)
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-ch:
			fmt.Fprint(w, "data: {\"type\":\"changed\"}\n\n")
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
		}
		if err := rc.Flush(); err != nil {
			return nil
		}
	}
}
