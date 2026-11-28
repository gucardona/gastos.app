package api

import (
	"fmt"
	"net/http"
	"time"

	"gastos/internal/api/httpx"
)

// events keeps an SSE connection open and tells the user's tab whenever something they can see changed:
// a wallet's data, their list of wallets, a member's name. Authenticates with the session cookie, no token in the URL.
func (s *Server) events(w http.ResponseWriter, r *http.Request) error {
	userID := httpx.UserIDFrom(r.Context())
	// subscribed before the headers go out: once the client sees the stream open, no notice can slip by
	ch := s.Bus.Subscribe(userID)
	defer s.Bus.Unsubscribe(userID, ch)

	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // nginx
	w.WriteHeader(http.StatusOK)
	if err := rc.Flush(); err != nil {
		return nil // streaming not supported; nothing to do
	}

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
