package api

import (
	"context"
	"crypto/sha256"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"gastos/internal/config"
)

func newInvite(t *testing.T, app *testApp, ttl time.Duration) string {
	t.Helper()
	code, _, err := CreateInvite(context.Background(), app.s.db, app.s.now(), ttl)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

// noEnvCode is an app whose only way in is a code made by `gastos invite`.
func noEnvCode(t *testing.T) *testApp {
	app := newTestAppWith(t, func(c *config.Config) { c.InviteCode = "" })
	app.register("Ana", "ana@example.com")
	return app
}

func TestInviteCodeFormat(t *testing.T) {
	format := regexp.MustCompile(`^[` + inviteAlphabet + `]{4}-[` + inviteAlphabet + `]{4}-[` + inviteAlphabet + `]{4}$`)
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c, err := newInviteCode()
		if err != nil {
			t.Fatal(err)
		}
		if !format.MatchString(c) {
			t.Fatalf("code %q does not match XXXX-XXXX-XXXX", c)
		}
		seen[c] = true
	}
	if len(seen) != 200 {
		t.Fatalf("%d distinct codes out of 200", len(seen))
	}
	for _, ambiguous := range "01ILOU" {
		if strings.ContainsRune(inviteAlphabet, ambiguous) {
			t.Errorf("alphabet has the ambiguous %q", ambiguous)
		}
	}
}

func TestNormalizeInvite(t *testing.T) {
	for in, want := range map[string]string{
		"K3J9-XQ2M-7PRT":     "K3J9XQ2M7PRT",
		" k3j9-xq2m-7prt ":   "K3J9XQ2M7PRT",
		"k3j9 xq2m 7prt":     "K3J9XQ2M7PRT",
		"K3J9XQ2M7PRT":       "K3J9XQ2M7PRT",
		"k3j9\t-xq2m-7prt\n": "K3J9XQ2M7PRT",
	} {
		if got := normalizeInvite(in); got != want {
			t.Errorf("normalizeInvite(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCreateInviteStoresOnlyTheHash(t *testing.T) {
	app := newTestApp(t)
	code, exp, err := CreateInvite(context.Background(), app.s.db, app.s.now(), 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if d := exp.Sub(app.s.now()); d < 2*time.Hour-time.Second || d > 2*time.Hour+time.Second {
		t.Errorf("expires in %v, want 2h", d)
	}
	want := sha256.Sum256([]byte(normalizeInvite(code)))
	var hash []byte
	var expires string
	var usedAt, usedBy any
	if err := app.s.db.QueryRow(`SELECT code_hash, expires_at, used_at, used_by FROM invite_codes`).Scan(&hash, &expires, &usedAt, &usedBy); err != nil {
		t.Fatal(err)
	}
	if string(hash) != string(want[:]) || expires != exp.UTC().Format(tsLayout) || usedAt != nil || usedBy != nil {
		t.Fatalf("row: hash ok %v, expires %q (want %q), used %v/%v", string(hash) == string(want[:]), expires, exp.UTC().Format(tsLayout), usedAt, usedBy)
	}
}

func TestInviteCodeIsSingleUse(t *testing.T) {
	app := noEnvCode(t)
	code := newInvite(t, app, time.Hour)
	if status, _ := signUp(app, "bia@example.com", code); status != http.StatusCreated {
		t.Fatalf("first use: status %d, want 201", status)
	}
	var usedBy int64
	if err := app.s.db.QueryRow(`SELECT used_by FROM invite_codes WHERE used_at IS NOT NULL`).Scan(&usedBy); err != nil {
		t.Fatal(err)
	}
	var bia int64
	app.s.db.QueryRow(`SELECT id FROM users WHERE email = 'bia@example.com'`).Scan(&bia)
	if usedBy != bia {
		t.Errorf("used_by = %d, want %d", usedBy, bia)
	}
	status, used := signUp(app, "cai@example.com", code)
	_, wrong := signUp(app, "cai@example.com", "AAAA-BBBB-CCCC")
	if status != http.StatusForbidden || used.Message != wrong.Message || used.Code != wrong.Code {
		t.Fatalf("second use: status %d (%+v), want 403 looking like a wrong code (%+v)", status, used, wrong)
	}
}

func TestExpiredInviteCodeIsRefused(t *testing.T) {
	app := noEnvCode(t)
	code := newInvite(t, app, time.Hour)
	app.s.now = func() time.Time { return time.Now().Add(time.Hour + time.Minute) }
	status, expired := signUp(app, "bia@example.com", code)
	_, wrong := signUp(app, "bia@example.com", "AAAA-BBBB-CCCC")
	if status != http.StatusForbidden || expired.Message != wrong.Message || expired.Code != wrong.Code {
		t.Fatalf("expired code: status %d (%+v), want 403 looking like a wrong code (%+v)", status, expired, wrong)
	}
}

func TestInviteCodeTypedLoosely(t *testing.T) {
	app := noEnvCode(t)
	code := newInvite(t, app, time.Hour)
	loose := " " + strings.ToLower(strings.ReplaceAll(code, "-", " ")) + " "
	if status, _ := signUp(app, "bia@example.com", loose); status != http.StatusCreated {
		t.Fatalf("lowercase with spaces: status %d, want 201", status)
	}
	code = newInvite(t, app, time.Hour)
	if status, _ := signUp(app, "cai@example.com", strings.ReplaceAll(code, "-", "")); status != http.StatusCreated {
		t.Fatalf("without dashes: status %d, want 201", status)
	}
}

func TestEnvInviteCodeStillWorksWithDBCodes(t *testing.T) {
	app := newTestApp(t)
	app.register("Ana", "ana@example.com")
	newInvite(t, app, time.Hour)
	if status, _ := signUp(app, "bia@example.com", testInvite); status != http.StatusCreated {
		t.Fatalf("env code: status %d, want 201", status)
	}
	if n := countRows(t, app, `SELECT COUNT(*) FROM invite_codes WHERE used_at IS NULL`); n != 1 {
		t.Fatalf("the env code must not consume a DB code: %d unused, want 1", n)
	}
}

func TestOnlyOneSignupPerInviteCodeAtOnce(t *testing.T) {
	app := noEnvCode(t)
	code := newInvite(t, app, time.Hour)
	statuses := make([]int, 4)
	var wg sync.WaitGroup
	for i := range statuses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i], _ = signUp(app, []string{"a@example.com", "b@example.com", "c@example.com", "d@example.com"}[i], code)
		}()
	}
	wg.Wait()
	created := 0
	for _, s := range statuses {
		switch s {
		case http.StatusCreated:
			created++
		case http.StatusForbidden:
		default:
			t.Fatalf("unexpected status in %v", statuses)
		}
	}
	if created != 1 {
		t.Fatalf("statuses %v: want exactly one 201", statuses)
	}
	if n := countRows(t, app, `SELECT COUNT(*) FROM users`); n != 2 {
		t.Fatalf("%d users, want 2 (Ana and one invitee)", n)
	}
}

func TestRegistrationModeFollowsDBCodes(t *testing.T) {
	app := noEnvCode(t)
	if m := registrationModeOf(t, app); m != "closed" {
		t.Fatalf("no codes: mode %q, want closed", m)
	}
	code := newInvite(t, app, time.Hour)
	if m := registrationModeOf(t, app); m != "code" {
		t.Fatalf("a valid code: mode %q, want code", m)
	}
	signUp(app, "bia@example.com", code)
	if m := registrationModeOf(t, app); m != "closed" {
		t.Fatalf("code used: mode %q, want closed", m)
	}
	newInvite(t, app, time.Hour)
	app.s.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if m := registrationModeOf(t, app); m != "closed" {
		t.Fatalf("code expired: mode %q, want closed", m)
	}
}

func TestHousekeepingRemovesOldInviteCodes(t *testing.T) {
	app := newTestApp(t)
	now := app.s.now().UTC()
	at := func(d time.Duration) string { return now.Add(d).Format(tsLayout) }
	day := 24 * time.Hour
	for _, row := range []struct {
		hash    string
		expires string
		usedAt  any
	}{
		{"keep-valid", at(time.Hour), nil},
		{"keep-expired-recently", at(-6 * day), nil},
		{"keep-used-recently", at(-6 * day), at(-6*day - time.Hour)},
		{"drop-expired-long-ago", at(-8 * day), nil},
		{"drop-used-long-ago", at(-day), at(-8 * day)}, // a 7-day code used 8 days ago, expired only yesterday
	} {
		if _, err := app.s.db.Exec(`INSERT INTO invite_codes (code_hash, expires_at, used_at) VALUES (?, ?, ?)`, []byte(row.hash), row.expires, row.usedAt); err != nil {
			t.Fatal(err)
		}
	}
	runHousekeeping(t, app, func() bool { return countRows(t, app, `SELECT COUNT(*) FROM invite_codes`) == 3 })
	if n := countRows(t, app, `SELECT COUNT(*) FROM invite_codes WHERE CAST(code_hash AS TEXT) LIKE 'keep-%'`); n != 3 {
		t.Fatalf("%d kept, want 3", n)
	}
	if n := countRows(t, app, `SELECT COUNT(*) FROM invite_codes`); n != 3 {
		t.Fatalf("%d rows left, want 3", n)
	}
}
