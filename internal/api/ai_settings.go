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
}

func (s *Server) loadAI(ctx context.Context, userID int64) (aiRow, error) {
	r := aiRow{model: "claude-sonnet-5-5", retention: 7}
	err := s.db.QueryRowContext(ctx,
		`SELECT api_key_enc, key_hint, model, enabled, retention_days FROM ai_settings WHERE user_id = ?`, userID).
		Scan(&r.enc, &r.hint, &r.model, &r.enabled, &r.retention)
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
}

func (s *Server) aiDTO(r aiRow) aiDTO {
	_, readable := s.openKey(r.enc)
	d := aiDTO{Available: s.cfg.SecretKey != nil, Configured: readable, Model: r.model, Enabled: r.enabled && readable, RetentionDays: r.retention}
	if readable {
		d.KeyHint = r.hint
	}
	return d
}

func (s *Server) getAI(w http.ResponseWriter, r *http.Request) error {
	row, err := s.loadAI(r.Context(), userIDFrom(r.Context()))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, s.aiDTO(row))
	return nil
}

func (s *Server) putAI(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		APIKey        *string `json:"apiKey"`
		Model         *string `json:"model"`
		Enabled       *bool   `json:"enabled"`
		RetentionDays *int    `json:"retentionDays"`
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
		`INSERT INTO ai_settings (user_id, api_key_enc, key_hint, model, enabled, retention_days) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET api_key_enc = excluded.api_key_enc, key_hint = excluded.key_hint, model = excluded.model,
		   enabled = excluded.enabled, retention_days = excluded.retention_days, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')`,
		uid, cur.enc, cur.hint, cur.model, cur.enabled, cur.retention); err != nil {
		return err
	}
	d := s.aiDTO(cur)
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
