package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"
)

var fxClient = &http.Client{Timeout: 10 * time.Second}

// refreshFX fetches the USD-BRL quote from cfg.FXURL and stores its ask price (what a buyer pays, the
// conservative side). On any failure the stored rate is left as it was.
func (s *Server) refreshFX(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.FXURL, nil)
	if err != nil {
		return err
	}
	res, err := fxClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("fx: status %d", res.StatusCode)
	}
	var body struct {
		USDBRL struct {
			Ask string `json:"ask"`
		} `json:"USDBRL"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&body); err != nil {
		return fmt.Errorf("fx: decode: %w", err)
	}
	ask, err := strconv.ParseFloat(body.USDBRL.Ask, 64)
	if err != nil || !(ask > 0 && ask <= 100) { // also rejects NaN
		return fmt.Errorf("fx: implausible ask %q", body.USDBRL.Ask)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO fx_rate (id, usd_brl_micro, fetched_at) VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET usd_brl_micro = excluded.usd_brl_micro, fetched_at = excluded.fetched_at`,
		int64(math.Round(ask*1e6)), s.now().UTC().Format(tsLayout))
	return err
}

// fxRate returns the stored rate in micro-reais per dollar and when it was fetched (tsLayout); ok is false when none.
func (s *Server) fxRate(ctx context.Context) (micro int64, fetchedAt string, ok bool) {
	err := s.db.QueryRowContext(ctx, `SELECT usd_brl_micro, fetched_at FROM fx_rate WHERE id = 1`).Scan(&micro, &fetchedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.log.Error("read fx rate", "err", err)
	}
	return micro, fetchedAt, err == nil
}

// fxStale reports whether the rate is missing or older than 20 hours (a daily fetch, with slack for the hourly tick).
func (s *Server) fxStale(ctx context.Context) bool {
	_, at, ok := s.fxRate(ctx)
	if !ok {
		return true
	}
	t, err := time.Parse(tsLayout, at)
	return err != nil || s.now().Sub(t) > 20*time.Hour
}
