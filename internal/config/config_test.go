package config

import (
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
