package config

import (
	"bytes"
	"encoding/base64"
	"testing"
	_ "time/tzdata"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":8000" || c.DBPath != "./data/gastos.db" || c.BackupKeep != 14 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if c.Location.String() != "America/Sao_Paulo" || !c.CookieSecure || c.Dev {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestLoadInvalid(t *testing.T) {
	for _, m := range []map[string]string{
		{"GASTOS_BACKUP_KEEP": "0"},
		{"GASTOS_BACKUP_KEEP": "x"},
		{"GASTOS_TZ": "Marte/Base"},
		{"GASTOS_COOKIE_SECURE": "talvez"},
		{"GASTOS_DEV": "sim"},
	} {
		if _, err := Load(env(m)); err == nil {
			t.Errorf("Load(%v) should fail", m)
		}
	}
}

func TestLoadInviteCode(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil || c.InviteCode != "" {
		t.Fatalf("no invite code by default: %+v, %v", c, err)
	}
	c, err = Load(env(map[string]string{"GASTOS_INVITE_CODE": "  open-sesame-42  "}))
	if err != nil {
		t.Fatal(err)
	}
	if c.InviteCode != "open-sesame-42" {
		t.Fatalf("the code should be trimmed, got %q", c.InviteCode)
	}
}

func TestLoadRejectsAShortInviteCode(t *testing.T) {
	// a short code is a guessable one: refuse to start rather than run with a weak secret
	for _, code := range []string{"abc", "1234567", "  short  "} {
		if _, err := Load(env(map[string]string{"GASTOS_INVITE_CODE": code})); err == nil {
			t.Errorf("GASTOS_INVITE_CODE=%q should be rejected", code)
		}
	}
}

func TestLoadSecretKey(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil || c.SecretKey != nil {
		t.Fatalf("no secret key by default: %+v, %v", c, err)
	}
	valid := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	c, err = Load(env(map[string]string{"GASTOS_SECRET_KEY": valid}))
	if err != nil || len(c.SecretKey) != 32 {
		t.Fatalf("valid secret key should load as 32 bytes: %+v, %v", c, err)
	}
	if _, err := Load(env(map[string]string{"GASTOS_SECRET_KEY": "curta"})); err == nil {
		t.Fatal("malformed GASTOS_SECRET_KEY should fail Load")
	}
}

func TestLoadFXURL(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil || c.FXURL != "https://economia.awesomeapi.com.br/last/USD-BRL" {
		t.Fatalf("default FXURL: %q, %v", c.FXURL, err)
	}
	c, err = Load(env(map[string]string{"GASTOS_FX_URL": "http://127.0.0.1:9998/fx"}))
	if err != nil || c.FXURL != "http://127.0.0.1:9998/fx" {
		t.Fatalf("overridden FXURL: %q, %v", c.FXURL, err)
	}
}

func TestLoadAnthropicURL(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil || c.AnthropicURL != "https://api.anthropic.com" {
		t.Fatalf("default AnthropicURL: %q, %v", c.AnthropicURL, err)
	}
	c, err = Load(env(map[string]string{"GASTOS_ANTHROPIC_URL": "http://127.0.0.1:9999"}))
	if err != nil || c.AnthropicURL != "http://127.0.0.1:9999" {
		t.Fatalf("overridden AnthropicURL: %q, %v", c.AnthropicURL, err)
	}
}
