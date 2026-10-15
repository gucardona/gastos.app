package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"gastos/internal/config"
)

const aiTestKey = "sk-ant-test-key-1234"

var aiSecret = bytes.Repeat([]byte{7}, 32)

type aiSettings struct {
	Available     bool    `json:"available"`
	Configured    bool    `json:"configured"`
	KeyHint       string  `json:"keyHint"`
	Model         string  `json:"model"`
	Enabled       bool    `json:"enabled"`
	RetentionDays int     `json:"retentionDays"`
	Warning       *string `json:"warning"`
}

// newAIApp starts a server wired to a fake Anthropic API whose reply status can be changed through the returned counter.
func newAIApp(t *testing.T, withSecret bool) (*testApp, *atomic.Int32) {
	t.Helper()
	status := &atomic.Int32{}
	status.Store(200)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if st := int(status.Load()); st != 200 {
			w.WriteHeader(st)
			fmt.Fprint(w, `{"error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"message_stop\"}\n\n")
	}))
	t.Cleanup(fake.Close)
	return newTestAppWith(t, func(c *config.Config) {
		c.AnthropicURL = fake.URL
		if withSecret {
			c.SecretKey = aiSecret
		}
	}), status
}

func TestAISettingsDefaults(t *testing.T) {
	app, _ := newAIApp(t, true)
	ana := app.register("Ana", "ana@example.com")
	var got aiSettings
	ana.do("GET", "/api/me/ai", nil, 200, &got)
	want := aiSettings{Available: true, Model: "claude-sonnet-5-5", RetentionDays: 7}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSaveKeyNeverReturnsIt(t *testing.T) {
	app, _ := newAIApp(t, true)
	ana := app.register("Ana", "ana@example.com")
	var raw json.RawMessage
	ana.do("PUT", "/api/me/ai", map[string]any{"apiKey": aiTestKey, "enabled": true}, 200, &raw)
	var got aiSettings
	json.Unmarshal(raw, &got)
	if !got.Configured || got.KeyHint != "1234" || !got.Enabled || got.Warning != nil {
		t.Fatalf("put: %s", raw)
	}
	if strings.Contains(string(raw), aiTestKey) {
		t.Fatalf("put leaked the key: %s", raw)
	}
	ana.do("GET", "/api/me/ai", nil, 200, &raw)
	if strings.Contains(string(raw), aiTestKey) || !strings.Contains(string(raw), `"keyHint":"1234"`) {
		t.Fatalf("get: %s", raw)
	}
	ana.do("GET", "/api/me", nil, 200, &raw)
	if strings.Contains(string(raw), aiTestKey) {
		t.Fatalf("me leaked the key: %s", raw)
	}
}

func TestSaveKeyStoredEncrypted(t *testing.T) {
	app, _ := newAIApp(t, true)
	ana := app.register("Ana", "ana@example.com")
	ana.do("PUT", "/api/me/ai", map[string]any{"apiKey": aiTestKey}, 200, nil)
	var enc []byte
	if err := app.s.db.QueryRow(`SELECT api_key_enc FROM ai_settings WHERE user_id = ?`, ana.user.ID).Scan(&enc); err != nil {
		t.Fatal(err)
	}
	if len(enc) == 0 || bytes.Contains(enc, []byte(aiTestKey)) {
		t.Fatalf("api_key_enc is not encrypted: %q", enc)
	}
}

func TestRejectedKeyIsNotSaved(t *testing.T) {
	app, status := newAIApp(t, true)
	ana := app.register("Ana", "ana@example.com")
	status.Store(401)
	var e struct {
		Error apiError `json:"error"`
	}
	if st := ana.doRaw("PUT", "/api/me/ai", map[string]any{"apiKey": aiTestKey}, &e); st != 400 {
		t.Fatalf("status %d", st)
	}
	if e.Error.Fields["apiKey"] != "chave recusada pela Anthropic" {
		t.Fatalf("error: %+v", e.Error)
	}
	var got aiSettings
	ana.do("GET", "/api/me/ai", nil, 200, &got)
	if got.Configured {
		t.Fatal("rejected key was saved")
	}
}

func TestSaveKeyOnNetworkFailureWarns(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	url := dead.URL
	dead.Close()
	app := newTestAppWith(t, func(c *config.Config) { c.AnthropicURL = url; c.SecretKey = aiSecret })
	ana := app.register("Ana", "ana@example.com")
	var got aiSettings
	ana.do("PUT", "/api/me/ai", map[string]any{"apiKey": aiTestKey}, 200, &got)
	if !got.Configured || got.Warning == nil {
		t.Fatalf("got %+v", got)
	}
}

func TestRetentionAndModelValidation(t *testing.T) {
	app, _ := newAIApp(t, true)
	ana := app.register("Ana", "ana@example.com")
	for _, body := range []map[string]any{
		{"retentionDays": 0}, {"retentionDays": 31}, {"model": "gpt-4"},
	} {
		var e struct {
			Error apiError `json:"error"`
		}
		if st := ana.doRaw("PUT", "/api/me/ai", body, &e); st != 400 || len(e.Error.Fields) != 1 {
			t.Fatalf("%v: status %d, %+v", body, st, e.Error)
		}
	}
	var got aiSettings
	ana.do("PUT", "/api/me/ai", map[string]any{"retentionDays": 30, "model": "claude-opus-5-5"}, 200, &got)
	if got.RetentionDays != 30 || got.Model != "claude-opus-5-5" || got.Configured {
		t.Fatalf("got %+v", got)
	}
	ana.do("GET", "/api/me/ai", nil, 200, &got)
	if got.RetentionDays != 30 || got.Model != "claude-opus-5-5" {
		t.Fatalf("not persisted: %+v", got)
	}
}

func TestEnableNeedsKey(t *testing.T) {
	app, _ := newAIApp(t, true)
	ana := app.register("Ana", "ana@example.com")
	var e struct {
		Error apiError `json:"error"`
	}
	if st := ana.doRaw("PUT", "/api/me/ai", map[string]any{"enabled": true}, &e); st != 400 || e.Error.Fields["enabled"] == "" {
		t.Fatalf("status %d, %+v", st, e.Error)
	}
}

func TestUnavailableWithoutSecretKey(t *testing.T) {
	app, _ := newAIApp(t, false)
	ana := app.register("Ana", "ana@example.com")
	var got aiSettings
	ana.do("GET", "/api/me/ai", nil, 200, &got)
	if got.Available {
		t.Fatal("available without SecretKey")
	}
	var e struct {
		Error apiError `json:"error"`
	}
	if st := ana.doRaw("PUT", "/api/me/ai", map[string]any{"apiKey": aiTestKey}, &e); st != 400 || e.Error.Message != "assistente indisponível neste servidor" {
		t.Fatalf("status %d, %+v", st, e.Error)
	}
}

func TestRemoveKeyDisables(t *testing.T) {
	app, _ := newAIApp(t, true)
	ana := app.register("Ana", "ana@example.com")
	ana.do("PUT", "/api/me/ai", map[string]any{"apiKey": aiTestKey, "enabled": true}, 200, nil)
	ana.do("DELETE", "/api/me/ai/key", nil, 204, nil)
	var got aiSettings
	ana.do("GET", "/api/me/ai", nil, 200, &got)
	if got.Configured || got.Enabled || got.KeyHint != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestKeyMissingAfterSecretRotation(t *testing.T) {
	app, _ := newAIApp(t, true)
	ana := app.register("Ana", "ana@example.com")
	ana.do("PUT", "/api/me/ai", map[string]any{"apiKey": aiTestKey, "enabled": true}, 200, nil)
	app.s.cfg.SecretKey = bytes.Repeat([]byte{9}, 32)
	var got aiSettings
	ana.do("GET", "/api/me/ai", nil, 200, &got)
	if got.Configured {
		t.Fatalf("got %+v", got)
	}
	var me struct {
		AssistantEnabled bool `json:"assistantEnabled"`
	}
	ana.do("GET", "/api/me", nil, 200, &me)
	if me.AssistantEnabled {
		t.Fatal("assistantEnabled with an unreadable key")
	}
	if _, _, err := app.s.userAIKey(t.Context(), ana.user.ID); err != errAIKeyUnreadable {
		t.Fatalf("err = %v", err)
	}
}

func TestMeReportsAssistantEnabled(t *testing.T) {
	app, _ := newAIApp(t, true)
	ana := app.register("Ana", "ana@example.com")
	var me struct {
		AssistantEnabled bool `json:"assistantEnabled"`
	}
	ana.do("GET", "/api/me", nil, 200, &me)
	if me.AssistantEnabled {
		t.Fatal("enabled by default")
	}
	ana.do("PUT", "/api/me/ai", map[string]any{"apiKey": aiTestKey, "enabled": true}, 200, nil)
	ana.do("GET", "/api/me", nil, 200, &me)
	if !me.AssistantEnabled {
		t.Fatal("not enabled")
	}
	key, model, err := app.s.userAIKey(t.Context(), ana.user.ID)
	if err != nil || key != aiTestKey || model != "claude-sonnet-5-5" {
		t.Fatalf("userAIKey = %q %q %v", key, model, err)
	}
	ana.do("PUT", "/api/me/ai", map[string]any{"enabled": false}, 200, nil)
	if _, _, err := app.s.userAIKey(t.Context(), ana.user.ID); err != errAIDisabled {
		t.Fatalf("err = %v", err)
	}
}

func TestSettingsArePerUser(t *testing.T) {
	app, _ := newAIApp(t, true)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	ana.do("PUT", "/api/me/ai", map[string]any{"apiKey": aiTestKey, "enabled": true}, 200, nil)
	var got aiSettings
	bia.do("GET", "/api/me/ai", nil, 200, &got)
	if got.Configured || got.Enabled || got.KeyHint != "" {
		t.Fatalf("bia sees %+v", got)
	}
}
