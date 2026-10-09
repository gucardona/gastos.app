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
		t.Fatalf("defaults inesperados: %+v", c)
	}
	if c.Location.String() != "America/Sao_Paulo" || !c.CookieSecure || c.Dev {
		t.Fatalf("defaults inesperados: %+v", c)
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
			t.Errorf("Load(%v) deveria falhar", m)
		}
	}
}
