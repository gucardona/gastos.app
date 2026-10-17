package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"gastos/internal/anthropic"
	"gastos/internal/secret"
)

var aiModels = []string{"claude-haiku-5-5", "claude-sonnet-5-5", "claude-opus-5-5"}

var (
	errAIKeyUnreadable = errors.New("assistant API key cannot be decrypted")
	errAIDisabled      = errors.New("assistant is disabled")
)

type aiRow struct {
	enc       []byte // empty when the user has no key (the row only holds model/retention)
	hint      string
	model     string
	enabled   bool
	retention int
	daily     int64 // spending limits in centavos; 0 = none
	monthly   int64
}

func (s *Server) loadAI(ctx context.Context, userID int64) (aiRow, error) {
	r := aiRow{model: "claude-sonnet-5-5", retention: 7}
	err := s.db.QueryRowContext(ctx,
		`SELECT api_key_enc, key_hint, model, enabled, retention_days, COALESCE(daily_limit_cents, 0), COALESCE(monthly_limit_cents, 0)
		 FROM ai_settings WHERE user_id = ?`, userID).
		Scan(&r.enc, &r.hint, &r.model, &r.enabled, &r.retention, &r.daily, &r.monthly)
	if errors.Is(err, sql.ErrNoRows) {
		return r, nil
	}
	return r, err
}

func (s *Server) openKey(enc []byte) (string, bool) {
	if len(enc) == 0 || s.cfg.SecretKey == nil {
		return "", false
	}
	k, err := secret.Open(s.cfg.SecretKey, enc)
	return string(k), err == nil
}

// userAIKey returns the user's decrypted API key and model, or errAIDisabled / errAIKeyUnreadable.
func (s *Server) userAIKey(ctx context.Context, userID int64) (key, model string, err error) {
	r, err := s.loadAI(ctx, userID)
	if err != nil {
		return "", "", err
	}
	if len(r.enc) == 0 || !r.enabled {
		return "", "", errAIDisabled
	}
	key, ok := s.openKey(r.enc)
	if !ok {
		return "", "", errAIKeyUnreadable
	}
	return key, r.model, nil
}

type aiDTO struct {
	Available     bool    `json:"available"`
	Configured    bool    `json:"configured"`
	KeyHint       string  `json:"keyHint"`
	Model         string  `json:"model"`
	Enabled       bool    `json:"enabled"`
	RetentionDays int     `json:"retentionDays"`
	Warning       *string `json:"warning,omitempty"`

	DailyLimitCents   int64    `json:"dailyLimitCents"`
	MonthlyLimitCents int64    `json:"monthlyLimitCents"`
	UsedTodayCents    int64    `json:"usedTodayCents"`
	UsedMonthCents    int64    `json:"usedMonthCents"`
	UsdBrl            *float64 `json:"usdBrl"`
	UsdBrlAt          *string  `json:"usdBrlAt"`
}

func (s *Server) aiDTO(ctx context.Context, userID int64, r aiRow) (aiDTO, error) {
	_, readable := s.openKey(r.enc)
	d := aiDTO{Available: s.cfg.SecretKey != nil, Configured: readable, Model: r.model, Enabled: r.enabled && readable, RetentionDays: r.retention,
		DailyLimitCents: r.daily, MonthlyLimitCents: r.monthly}
	if readable {
		d.KeyHint = r.hint
	}
	today, month, err := s.aiSpend(ctx, userID)
	if err != nil {
		return d, err
	}
	d.UsedTodayCents, d.UsedMonthCents = microToCents(today), microToCents(month)
	if micro, at, ok := s.fxRate(ctx); ok {
		rate := float64(micro) / 1e6
		d.UsdBrl, d.UsdBrlAt = &rate, &at
	}
	return d, nil
}

// aiSpend returns the user's estimated assistant spend today and this calendar month (in cfg.Location), in micro-reais.
func (s *Server) aiSpend(ctx context.Context, userID int64) (today, month int64, err error) {
	day := s.now().In(s.cfg.Location).Format(dateLayout)
	err = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE WHEN day = ? THEN cost_micro_brl END), 0), COALESCE(SUM(cost_micro_brl), 0)
		FROM ai_usage WHERE user_id = ? AND day >= ? AND day <= ?`, day, userID, day[:8]+"01", day).Scan(&today, &month)
	return today, month, err
}

// checkAILimit refuses a question once today's or this month's spend reached the user's limit. Spend is only
// known after an answer, so one answer may overshoot the limit. Without a USD-BRL rate the spend is recorded
// as zero, so limits are not enforced until the first rate arrives.
func (s *Server) checkAILimit(ctx context.Context, userID int64) error {
	r, err := s.loadAI(ctx, userID)
	if err != nil || (r.daily == 0 && r.monthly == 0) {
		return err
	}
	if _, _, ok := s.fxRate(ctx); !ok {
		return nil
	}
	today, month, err := s.aiSpend(ctx, userID)
	switch {
	case err != nil:
		return err
	case r.daily > 0 && today >= r.daily*10_000:
		return &apiError{Status: http.StatusTooManyRequests, Code: "limit",
			Message: "Limite diário do assistente atingido (" + formatBRL(r.daily) + "). Volta amanhã ou aumente o limite em Ajustes."}
	case r.monthly > 0 && month >= r.monthly*10_000:
		return &apiError{Status: http.StatusTooManyRequests, Code: "limit",
			Message: "Limite mensal do assistente atingido (" + formatBRL(r.monthly) + "). Volta no mês que vem ou aumente o limite em Ajustes."}
	}
	return nil
}

// recordUsage adds an exchange's tokens and estimated cost to today's ai_usage row. It runs on a detached
// context so a question cancelled by the client still counts.
func (s *Server) recordUsage(ctx context.Context, userID int64, model string, inTok, outTok int) {
	if inTok == 0 && outTok == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	var cost int64
	if rate, _, ok := s.fxRate(ctx); ok {
		cost = costMicroBRL(model, inTok, outTok, rate)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO ai_usage (user_id, day, cost_micro_brl, input_tokens, output_tokens) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, day) DO UPDATE SET cost_micro_brl = cost_micro_brl + excluded.cost_micro_brl,
		  input_tokens = input_tokens + excluded.input_tokens, output_tokens = output_tokens + excluded.output_tokens`,
		userID, s.now().In(s.cfg.Location).Format(dateLayout), cost, inTok, outTok); err != nil {
		s.log.ErrorContext(ctx, "chat: record usage", "err", err)
	}
}

func (s *Server) getAI(w http.ResponseWriter, r *http.Request) error {
	uid := userIDFrom(r.Context())
	row, err := s.loadAI(r.Context(), uid)
	if err != nil {
		return err
	}
	d, err := s.aiDTO(r.Context(), uid, row)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, d)
	return nil
}

func (s *Server) putAI(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		APIKey        *string `json:"apiKey"`
		Model         *string `json:"model"`
		Enabled       *bool   `json:"enabled"`
		RetentionDays *int    `json:"retentionDays"`

		DailyLimitCents   *int64 `json:"dailyLimitCents"`
		MonthlyLimitCents *int64 `json:"monthlyLimitCents"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	uid := userIDFrom(r.Context())
	cur, err := s.loadAI(r.Context(), uid)
	if err != nil {
		return err
	}
	newKey := ""
	if in.APIKey != nil {
		if s.cfg.SecretKey == nil {
			return errBadRequest("assistente indisponível neste servidor")
		}
		if newKey = strings.TrimSpace(*in.APIKey); newKey == "" {
			return errValidation(map[string]string{"apiKey": "obrigatório"})
		}
	}
	fe := fieldErrors{}
	if in.Model != nil {
		if !slices.Contains(aiModels, *in.Model) {
			fe.add("model", "modelo inválido")
		} else {
			cur.model = *in.Model
		}
	}
	if in.RetentionDays != nil {
		if *in.RetentionDays < 1 || *in.RetentionDays > 30 {
			fe.add("retentionDays", "use de 1 a 30 dias")
		} else {
			cur.retention = *in.RetentionDays
		}
	}
	for field, lim := range map[string]struct{ in, cur *int64 }{
		"dailyLimitCents": {in.DailyLimitCents, &cur.daily}, "monthlyLimitCents": {in.MonthlyLimitCents, &cur.monthly},
	} {
		switch {
		case lim.in == nil:
		case *lim.in < 0 || *lim.in > 100_000_00: // 0 removes the limit; at most R$ 100.000,00
			fe.add(field, "use um valor entre R$ 0,01 e R$ 100.000,00")
		default:
			*lim.cur = *lim.in
		}
	}
	_, hadKey := s.openKey(cur.enc)
	if in.Enabled != nil {
		if *in.Enabled && newKey == "" && !hadKey {
			fe.add("enabled", "configure uma chave antes de ativar")
		}
		cur.enabled = *in.Enabled
	}
	if err := fe.err(); err != nil {
		return err
	}

	var warning *string
	if newKey != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		err := s.anthropicClient(newKey).Check(ctx, cur.model)
		var ae *anthropic.APIError
		switch {
		case errors.As(err, &ae) && ae.Status == http.StatusUnauthorized:
			return errValidation(map[string]string{"apiKey": "chave recusada pela Anthropic"})
		case err != nil:
			w := "Não foi possível validar a chave agora; ela foi salva mesmo assim."
			warning = &w
		}
		if cur.enc, err = secret.Seal(s.cfg.SecretKey, []byte(newKey)); err != nil {
			return err
		}
		cur.hint = newKey[max(0, len(newKey)-4):]
	}

	if cur.enc == nil {
		cur.enc = []byte{} // a nil slice binds as NULL, which the NOT NULL column refuses
	}
	if _, err := s.db.ExecContext(r.Context(),
		`INSERT INTO ai_settings (user_id, api_key_enc, key_hint, model, enabled, retention_days, daily_limit_cents, monthly_limit_cents)
		 VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, 0), NULLIF(?, 0))
		 ON CONFLICT(user_id) DO UPDATE SET api_key_enc = excluded.api_key_enc, key_hint = excluded.key_hint, model = excluded.model,
		   enabled = excluded.enabled, retention_days = excluded.retention_days, daily_limit_cents = excluded.daily_limit_cents,
		   monthly_limit_cents = excluded.monthly_limit_cents, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')`,
		uid, cur.enc, cur.hint, cur.model, cur.enabled, cur.retention, cur.daily, cur.monthly); err != nil {
		return err
	}
	d, err := s.aiDTO(r.Context(), uid, cur)
	if err != nil {
		return err
	}
	d.Warning = warning
	writeJSON(w, http.StatusOK, d)
	return nil
}

func (s *Server) deleteAIKey(w http.ResponseWriter, r *http.Request) error {
	if _, err := s.db.ExecContext(r.Context(),
		`UPDATE ai_settings SET api_key_enc = x'', key_hint = '', enabled = 0 WHERE user_id = ?`, userIDFrom(r.Context())); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) anthropicClient(key string) *anthropic.Client {
	return &anthropic.Client{BaseURL: s.anthropicURL, APIKey: key}
}
