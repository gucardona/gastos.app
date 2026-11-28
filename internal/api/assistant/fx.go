package assistant

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

	"gastos/internal/api/httpx"
)

var _fxClient = &http.Client{Timeout: 10 * time.Second}

// RefreshFX fetches the USD-BRL quote from cfg.FXURL and stores its ask price (what a buyer pays, the
// conservative side). On any failure the stored rate is left as it was.
func (s *Handler) RefreshFX(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.Cfg.FXURL, nil)
	if err != nil {
		return err
	}
	res, err := _fxClient.Do(req)
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
	if err != nil || !(ask >= 1 && ask <= 20) { // also rejects NaN
		return fmt.Errorf("fx: implausible ask %q", body.USDBRL.Ask)
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO fx_rate (id, usd_brl_micro, fetched_at) VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET usd_brl_micro = excluded.usd_brl_micro, fetched_at = excluded.fetched_at`,
		int64(math.Round(ask*1e6)), s.Now().UTC().Format(httpx.TSLayout))
	return err
}

// fxRate returns the stored rate in micro-reais per dollar and when it was fetched (httpx.TSLayout); ok is false when none.
func (s *Handler) fxRate(ctx context.Context) (micro int64, fetchedAt string, ok bool) {
	err := s.DB.QueryRowContext(ctx, `SELECT usd_brl_micro, fetched_at FROM fx_rate WHERE id = 1`).Scan(&micro, &fetchedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.Log.Error("read fx rate", "err", err)
	}
	return micro, fetchedAt, err == nil
}

// FXStale reports whether the rate is missing or older than 20 hours (a daily fetch, with slack for the hourly tick).
func (s *Handler) FXStale(ctx context.Context) bool {
	_, at, ok := s.fxRate(ctx)
	if !ok {
		return true
	}
	t, err := time.Parse(httpx.TSLayout, at)
	return err != nil || s.Now().Sub(t) > 20*time.Hour
}
