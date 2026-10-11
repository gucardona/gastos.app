package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gastos/internal/config"
)

// ---- /healthz: o deploy decide o rollback por ele ----

func TestHealthzOK(t *testing.T) {
	app := newTestApp(t)
	res, err := http.Get(app.srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status %d, want 200", res.StatusCode)
	}
}

func TestHealthzFailsWhenDatabaseIsGone(t *testing.T) {
	app := newTestApp(t)
	app.s.db.Close()
	res, err := http.Get(app.srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", res.StatusCode)
	}
}

// ---- Housekeeping: limpeza de sessões e backup diário ----

func countRows(t *testing.T, app *testApp, query string) int {
	t.Helper()
	var n int
	if err := app.s.db.QueryRow(query).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// runHousekeeping executa o job até a condição ser verdadeira e então o cancela.
func runHousekeeping(t *testing.T, app *testApp, until func() bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { app.s.Housekeeping(ctx, time.Hour); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	for !until() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Housekeeping não parou ao cancelar o contexto")
	}
}

func backups(app *testApp) []string {
	files, _ := filepath.Glob(filepath.Join(app.s.cfg.BackupDir, "gastos-*.db"))
	return files
}

func TestHousekeepingRemovesExpiredSessionsOnly(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	if _, err := app.s.db.Exec(`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (x'01', ?, '2000-01-01T00:00:00Z')`, ana.user.ID); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, app, `SELECT COUNT(*) FROM sessions`); n != 2 {
		t.Fatalf("%d sessões antes, want 2", n)
	}
	runHousekeeping(t, app, func() bool { return countRows(t, app, `SELECT COUNT(*) FROM sessions`) == 1 })
	if n := countRows(t, app, `SELECT COUNT(*) FROM sessions`); n != 1 {
		t.Fatalf("%d sessões depois, want 1 (a válida fica)", n)
	}
	ana.do("GET", "/api/me", nil, 200, nil) // a sessão ativa continua funcionando
}

func TestHousekeepingMakesBackupOnlyWhenTheLastIsOld(t *testing.T) {
	app := newTestApp(t)
	if len(backups(app)) != 0 {
		t.Fatal("não deveria haver backup antes")
	}
	runHousekeeping(t, app, func() bool { return len(backups(app)) == 1 })
	if n := len(backups(app)); n != 1 {
		t.Fatalf("%d backups, want 1", n)
	}

	// com um backup recente, rodar de novo não cria outro
	settle := time.Now().Add(300 * time.Millisecond)
	runHousekeeping(t, app, func() bool { return time.Now().After(settle) })
	if n := len(backups(app)); n != 1 {
		t.Fatalf("%d backups depois da segunda execução, want 1", n)
	}
}

func TestHousekeepingRotatesOldBackups(t *testing.T) {
	app := newTestAppWith(t, func(c *config.Config) { c.BackupKeep = 2 })
	dir := app.s.cfg.BackupDir
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
	runHousekeeping(t, app, func() bool { return len(backups(app)) == 2 && !exists(filepath.Join(dir, "gastos-20200101-000000.db")) })
	if len(backups(app)) != 2 || exists(filepath.Join(dir, "gastos-20200101-000000.db")) {
		t.Fatalf("esperava manter os 2 mais novos, tenho %v", backups(app))
	}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// ---- CORS do modo dev ----

func TestDevCORS(t *testing.T) {
	dev := newTestAppWith(t, func(c *config.Config) { c.Dev = true })
	prod := newTestApp(t)

	do := func(app *testApp, method, origin string) *http.Response {
		req, _ := http.NewRequest(method, app.srv.URL+"/api/me", nil)
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
		t.Errorf("dev + origem do Vite: cabeçalhos %v", res.Header)
	}
	if res := do(dev, "OPTIONS", "http://localhost:5173"); res.StatusCode != http.StatusNoContent {
		t.Errorf("preflight: status %d, want 204", res.StatusCode)
	}
	if res := do(dev, "GET", "https://evil.example"); res.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("origem desconhecida não deve receber CORS, nem em dev")
	}
	if res := do(prod, "GET", "http://localhost:5173"); res.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("em produção ninguém recebe CORS")
	}
}
