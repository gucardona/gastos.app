package api

import (
	"context"
	"time"

	"gastos/internal/db"
)

// Housekeeping removes expired sessions and chat, ensures a daily backup and refreshes the USD-BRL rate daily, until ctx is cancelled.
func (s *Server) Housekeeping(ctx context.Context, every time.Duration) {
	run := func() {
		now := s.now().UTC()
		if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.Format(tsLayout)); err != nil && ctx.Err() == nil {
			s.log.Error("clean up sessions", "err", err)
		}
		// retention is per user (7 days without settings); both sides use the tsLayout string format
		if _, err := s.db.ExecContext(ctx, `DELETE FROM chat_messages WHERE created_at <= strftime('%Y-%m-%dT%H:%M:%SZ', ?, '-' ||
			COALESCE((SELECT retention_days FROM ai_settings WHERE user_id = chat_messages.user_id), 7) || ' days')`, now.Format(tsLayout)); err != nil && ctx.Err() == nil {
			s.log.Error("clean up chat messages", "err", err)
		}
		if db.LastBackupAge(s.cfg.BackupDir, now) > 24*time.Hour {
			if p, err := db.Backup(ctx, s.db, s.cfg.BackupDir, s.cfg.BackupKeep, now); err != nil {
				if ctx.Err() == nil {
					s.log.Error("backup", "err", err)
				}
			} else {
				s.log.Info("backup", "path", p)
			}
		}
		// Last, so a slow quote service (10s timeout) never delays the steps above. An empty URL (tests) skips it.
		if s.cfg.FXURL != "" && s.fxStale(ctx) {
			if err := s.refreshFX(ctx); err != nil && ctx.Err() == nil {
				s.log.Warn("refresh usd-brl rate", "err", err)
			}
		}
	}
	run()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}
