package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gastos/internal/api/apitest"
	"gastos/internal/api/httpx"
	"gastos/internal/config"
)

// ---- /healthz: the deploy decides on rollback based on it ----

func TestHealthzOK(t *testing.T) {
	app := apitest.New(t)
	res, err := http.Get(app.URL() + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status %d, want 200", res.StatusCode)
	}
}

func TestHealthzFailsWhenDatabaseIsGone(t *testing.T) {
	app := apitest.New(t)
	//nolint:errcheck,gosec // moved unchanged, see docs/security.md
	app.Server().DB.Close()
	res, err := http.Get(app.URL() + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", res.StatusCode)
	}
}

// ---- Housekeeping: session cleanup and daily backup ----

func TestHousekeepingRemovesExpiredSessionsOnly(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	if _, err := app.Server().DB.Exec(`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (x'01', ?, '2000-01-01T00:00:00Z')`, ana.User.ID); err != nil {
		t.Fatal(err)
	}
	if n := apitest.CountRows(t, app, `SELECT COUNT(*) FROM sessions`); n != 2 {
		t.Fatalf("%d sessions before, want 2", n)
	}
	apitest.RunHousekeeping(t, app, func() bool { return apitest.CountRows(t, app, `SELECT COUNT(*) FROM sessions`) == 1 })
	if n := apitest.CountRows(t, app, `SELECT COUNT(*) FROM sessions`); n != 1 {
		t.Fatalf("%d sessions after, want 1 (the valid one stays)", n)
	}
	ana.Do("GET", "/api/me", nil, 200, nil) // the active session keeps working
}

func addChat(t *testing.T, app *apitest.App, c *apitest.Client, daysOld int) {
	t.Helper()
	at := app.Server().Now().UTC().AddDate(0, 0, -daysOld).Format(httpx.TSLayout)
	if _, err := app.Server().DB.Exec(`INSERT INTO chat_messages (user_id, wallet_id, role, content, created_at) VALUES (?, ?, 'user', 'hi', ?)`, c.User.ID, c.WalletID, at); err != nil {
		t.Fatal(err)
	}
}

func setRetention(t *testing.T, app *apitest.App, c *apitest.Client, days int) {
	t.Helper()
	if _, err := app.Server().DB.Exec(`INSERT INTO ai_settings (user_id, api_key_enc, key_hint, retention_days) VALUES (?, x'', '1234', ?)`, c.User.ID, days); err != nil {
		t.Fatal(err)
	}
}

func chatCount(t *testing.T, app *apitest.App, c *apitest.Client) int {
	t.Helper()
	var n int
	if err := app.Server().DB.QueryRow(`SELECT COUNT(*) FROM chat_messages WHERE user_id = ?`, c.User.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestHousekeepingDeletesExpiredChat(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	setRetention(t, app, ana, 3)
	addChat(t, app, ana, 4)
	addChat(t, app, ana, 2)
	apitest.RunHousekeeping(t, app, func() bool { return chatCount(t, app, ana) == 1 })
	if n := chatCount(t, app, ana); n != 1 {
		t.Fatalf("%d messages, want 1 (the 2-day-old one stays)", n)
	}
	var age int
	if err := app.Server().DB.QueryRow(`SELECT CAST(julianday(?) - julianday(created_at) AS INTEGER) FROM chat_messages`, app.Server().Now().UTC().Format(httpx.TSLayout)).Scan(&age); err != nil || age != 2 {
		t.Fatalf("remaining message age %d (err %v), want 2", age, err)
	}
}

func TestHousekeepingRetentionIsPerUser(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	setRetention(t, app, ana, 3)
	setRetention(t, app, bia, 30)
	addChat(t, app, ana, 10)
	addChat(t, app, bia, 10)
	apitest.RunHousekeeping(t, app, func() bool { return chatCount(t, app, ana) == 0 })
	if n := chatCount(t, app, ana); n != 0 {
		t.Fatalf("ana has %d messages, want 0", n)
	}
	if n := chatCount(t, app, bia); n != 1 {
		t.Fatalf("bia has %d messages, want 1", n)
	}
}

func TestHousekeepingDefaultsToSevenDays(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	addChat(t, app, ana, 8)
	addChat(t, app, ana, 6)
	apitest.RunHousekeeping(t, app, func() bool { return chatCount(t, app, ana) == 1 })
	if n := chatCount(t, app, ana); n != 1 {
		t.Fatalf("%d messages, want 1 (the 6-day-old one stays)", n)
	}
}

func TestHousekeepingMakesBackupOnlyWhenTheLastIsOld(t *testing.T) {
	app := apitest.New(t)
	if len(apitest.Backups(app)) != 0 {
		t.Fatal("there should be no backup before")
	}
	apitest.RunHousekeeping(t, app, func() bool { return len(apitest.Backups(app)) == 1 })
	if n := len(apitest.Backups(app)); n != 1 {
		t.Fatalf("%d backups, want 1", n)
	}

	// with a recent backup, running again does not create another
	settle := time.Now().Add(300 * time.Millisecond)
	apitest.RunHousekeeping(t, app, func() bool { return time.Now().After(settle) })
	if n := len(apitest.Backups(app)); n != 1 {
		t.Fatalf("%d backups after the second run, want 1", n)
	}
}

func TestHousekeepingRotatesOldBackups(t *testing.T) {
	app := apitest.NewWith(t, func(c *config.Config) { c.BackupKeep = 2 })
	dir := app.Server().Cfg.BackupDir
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	for _, name := range []string{"gastos-20200101-000000.db", "gastos-20200102-000000.db"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o640); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(p, old, old)
	}
	apitest.RunHousekeeping(t, app, func() bool {
		return len(apitest.Backups(app)) == 2 && !exists(filepath.Join(dir, "gastos-20200101-000000.db"))
	})
	if len(apitest.Backups(app)) != 2 || exists(filepath.Join(dir, "gastos-20200101-000000.db")) {
		t.Fatalf("expected to keep the 2 newest, have %v", apitest.Backups(app))
	}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// ---- CORS do modo dev ----

func TestDevCORS(t *testing.T) {
	dev := apitest.NewWith(t, func(c *config.Config) { c.Dev = true })
	prod := apitest.New(t)

	do := func(app *apitest.App, method, origin string) *http.Response {
		req, _ := http.NewRequest(method, app.URL()+"/api/me", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}

	res := do(dev, "GET", "http://localhost:5173")
	if res.Header.Get("Access-Control-Allow-Origin") != "http://localhost:5173" || res.Header.Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("dev + Vite origin: headers %v", res.Header)
	}
	if res := do(dev, "OPTIONS", "http://localhost:5173"); res.StatusCode != http.StatusNoContent {
		t.Errorf("preflight: status %d, want 204", res.StatusCode)
	}
	if res := do(dev, "GET", "https://evil.example"); res.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("an unknown origin must not get CORS, not even in dev")
	}
	if res := do(prod, "GET", "http://localhost:5173"); res.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("nobody gets CORS in production")
	}
}
