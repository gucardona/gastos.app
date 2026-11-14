package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"gastos/internal/api"
	"gastos/internal/config"
	"gastos/internal/db"
)

const (
	minInviteTTL = 5 * time.Minute
	maxInviteTTL = 7 * 24 * time.Hour
)

var errInviteUsage = errors.New("duração inválida: use, por exemplo, 30m, 2h ou 3d (de 5m a 7d; sem nada, vale 24h)")

// parseInviteTTL reads the optional duration of `gastos invite`: a Go duration (2h, 30m) or whole days (3d).
func parseInviteTTL(args []string) (time.Duration, error) {
	if len(args) == 0 {
		return 24 * time.Hour, nil
	}
	if len(args) > 1 {
		return 0, errInviteUsage
	}
	var ttl time.Duration
	if days, ok := strings.CutSuffix(args[0], "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil {
			return 0, errInviteUsage
		}
		ttl = time.Duration(n) * 24 * time.Hour
	} else {
		var err error
		if ttl, err = time.ParseDuration(args[0]); err != nil {
			return 0, errInviteUsage
		}
	}
	if ttl < minInviteTTL || ttl > maxInviteTTL {
		return 0, errInviteUsage
	}
	return ttl, nil
}

// runInvite is `gastos invite [duration]`: it stores a new single-use code and prints it, and only it, to out.
func runInvite(args []string, getenv func(string) string, out io.Writer, now time.Time) error {
	ttl, err := parseInviteTTL(args)
	if err != nil {
		return err
	}
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	d, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer d.Close()
	code, exp, err := api.CreateInvite(context.Background(), d, now, ttl)
	if err != nil {
		return err
	}
	exp = exp.In(cfg.Location)
	_, err = fmt.Fprintf(out, "Código: %s (vale até %s às %s, uso único)\n", code, exp.Format("02/01/2006"), exp.Format("15:04"))
	return err
}
