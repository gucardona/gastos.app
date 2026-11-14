package main

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"gastos/internal/db"
	"gastos/internal/testutil"
)

func TestParseInviteTTL(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"":     24 * time.Hour,
		"2h":   2 * time.Hour,
		"30m":  30 * time.Minute,
		"5m":   5 * time.Minute,
		"3d":   72 * time.Hour,
		"7d":   7 * 24 * time.Hour,
		"168h": 7 * 24 * time.Hour,
	} {
		args := []string{in}
		if in == "" {
			args = nil
		}
		got, err := parseInviteTTL(args)
		if err != nil || got != want {
			t.Errorf("parseInviteTTL(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range [][]string{{"4m"}, {"8d"}, {"169h"}, {"0"}, {"-2h"}, {"abc"}, {"d"}, {"1.5d"}, {"2h", "3h"}} {
		if _, err := parseInviteTTL(bad); err == nil {
			t.Errorf("parseInviteTTL(%q) should fail", bad)
		}
	}
}

func inviteEnv(path string) func(string) string {
	return func(k string) string {
		return map[string]string{"GASTOS_DB_PATH": path, "GASTOS_TZ": "America/Sao_Paulo"}[k]
	}
}

func TestInviteCommandPrintsOneCodeAndStoresItsHash(t *testing.T) {
	path := filepath.Join(testutil.TempDir(t), "g.db")
	now := time.Date(2026, 11, 14, 17, 30, 0, 0, time.UTC) // 14:30 in São Paulo
	var out bytes.Buffer
	if err := runInvite([]string{"2h"}, inviteEnv(path), &out, now); err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`^Código: ([2-9A-Z]{4}-[2-9A-Z]{4}-[2-9A-Z]{4}) \(vale até 14/11/2026 às 16:30, uso único\)\n$`).FindStringSubmatch(out.String())
	if m == nil {
		t.Fatalf("output %q", out.String())
	}
	d, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var hash []byte
	var expires string
	if err := d.QueryRow(`SELECT code_hash, expires_at FROM invite_codes`).Scan(&hash, &expires); err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256([]byte(strings.ReplaceAll(m[1], "-", "")))
	if !bytes.Equal(hash, want[:]) || expires != "2026-11-14T19:30:00Z" {
		t.Fatalf("stored hash matches %v, expires %q", bytes.Equal(hash, want[:]), expires)
	}
	assertNoPlainCode(t, d, m[1])
}

func assertNoPlainCode(t *testing.T, d *sql.DB, code string) {
	t.Helper()
	var n int
	plain := strings.ReplaceAll(code, "-", "")
	d.QueryRow(`SELECT COUNT(*) FROM invite_codes WHERE instr(CAST(code_hash AS TEXT), ?) > 0 OR instr(CAST(code_hash AS TEXT), ?) > 0`, code, plain).Scan(&n)
	if n != 0 {
		t.Fatal("the code is stored in plain text")
	}
}

func TestInviteCommandRefusesBadInput(t *testing.T) {
	path := filepath.Join(testutil.TempDir(t), "g.db")
	var out bytes.Buffer
	err := runInvite([]string{"10d"}, inviteEnv(path), &out, time.Now())
	if err == nil || !strings.Contains(err.Error(), "duração inválida") {
		t.Fatalf("want a pt-BR duration error, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("printed %q on error", out.String())
	}
}
