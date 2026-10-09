package api

import (
	"context"
	"time"

	"gastos/internal/db"
)

// Housekeeping remove sessões expiradas e garante um backup diário, até ctx ser cancelado.
func (s *Server) Housekeeping(ctx context.Context, every time.Duration) {
	run := func() {
		now := s.now().UTC()
		if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.Format(tsLayout)); err != nil && ctx.Err() == nil {
			s.log.Error("limpar sessões", "err", err)
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
