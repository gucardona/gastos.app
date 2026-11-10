package api

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// events keeps an SSE connection open and tells the user's tab whenever something they can see changed:
// a wallet's data, their list of wallets, a member's name. Authenticates with the session cookie, no token in the URL.
func (s *Server) events(w http.ResponseWriter, r *http.Request) error {
	userID := userIDFrom(r.Context())
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // nginx
	w.WriteHeader(http.StatusOK)
	if err := rc.Flush(); err != nil {
		return nil // streaming not supported; nothing to do
	}

	ch := s.bus.Subscribe(userID)
	defer s.bus.Unsubscribe(userID, ch)
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

// notifyUsers tells every open connection of these users that something changed.
func (s *Server) notifyUsers(ids ...int64) { s.bus.Notify(ids...) }

// walletMembers lists who belongs to the wallet; read it before a delete to still reach the people being removed.
func (s *Server) walletMembers(ctx context.Context, walletID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id FROM wallet_members WHERE wallet_id = ?`, walletID)
	if err != nil {
		return nil, err
	}
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

// notifyWallet tells every member of the wallet. The change is already saved, so a failure is only logged.
func (s *Server) notifyWallet(ctx context.Context, walletID int64) {
	ids, err := s.walletMembers(ctx, walletID)
	if err != nil {
		s.log.ErrorContext(ctx, "notify wallet members", "wallet", walletID, "err", err)
		return
	}
	s.notifyUsers(ids...)
}
