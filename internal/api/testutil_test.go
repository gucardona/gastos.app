package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"
	_ "time/tzdata"

	"golang.org/x/crypto/bcrypt"

	"gastos/internal/config"
	"gastos/internal/db"
	"gastos/internal/events"
	"gastos/internal/recurrence"
	"gastos/internal/testutil"
)

func init() { bcryptCost = bcrypt.MinCost }

type testApp struct {
	t   *testing.T
	srv *httptest.Server
	s   *Server
}

func newTestApp(t *testing.T) *testApp { return newTestAppWith(t, nil) }

// newTestAppWith lets a test tweak the configuration (dev mode, backup directory...) before the server starts.
func newTestAppWith(t *testing.T, tweak func(*config.Config)) *testApp {
	t.Helper()
	d, err := db.Open(filepath.Join(testutil.TempDir(t), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	cfg := config.Config{Location: loc, CookieSecure: false, BackupDir: filepath.Join(testutil.TempDir(t), "backups"), BackupKeep: 3}
	if tweak != nil {
		tweak(&cfg)
	}
	bus := events.New()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(d, cfg, bus, recurrence.NewMaterializer(d, bus, loc, log), log)
	spa := fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>spa</title>")}}
	srv := httptest.NewServer(s.Routes(spa))
	t.Cleanup(func() { srv.Close(); d.Close() })
	return &testApp{t: t, srv: srv, s: s}
}

type client struct {
	app  *testApp
	http *http.Client
	user struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	walletID int64 // the "Pessoal" wallet created on signup
}

func (a *testApp) anon() *client {
	jar, _ := cookiejar.New(nil)
	return &client{app: a, http: &http.Client{Jar: jar}}
}

// register creates a user, keeps the cookie and the personal wallet.
func (a *testApp) register(name, email string) *client {
	a.t.Helper()
	c := a.anon()
	c.do("POST", "/api/auth/register", map[string]string{"name": name, "email": email, "password": "senha-segura"}, 201, &c.user)
	var me struct {
		Wallets []struct {
			ID int64 `json:"id"`
		} `json:"wallets"`
	}
	c.do("GET", "/api/me", nil, 200, &me)
	c.walletID = me.Wallets[0].ID
	return c
}

// do makes the request, checks the status and decodes the response into out (if not nil).
func (c *client) do(method, path string, body any, wantStatus int, out any) {
	c.app.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.app.srv.URL+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.app.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != wantStatus {
		c.app.t.Fatalf("%s %s: status %d, want %d; body: %s", method, path, res.StatusCode, wantStatus, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			c.app.t.Fatalf("%s %s: decode: %v; body: %s", method, path, err, raw)
		}
	}
}

// wpath builds a path inside the client's active wallet.
func (c *client) wpath(format string, args ...any) string {
	return fmt.Sprintf("/api/wallets/%d", c.walletID) + fmt.Sprintf(format, args...)
}

func itoa(n int64) string { return fmt.Sprint(n) }

// upload sends a multipart form with a single file field named "file" and returns the status and raw body.
func (c *client) upload(path string, fields map[string]string, filename string, content []byte) (int, []byte) {
	c.app.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			c.app.t.Fatal(err)
		}
	}
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		c.app.t.Fatal(err)
	}
	fw.Write(content)
	mw.Close()
	req, _ := http.NewRequest("POST", c.app.srv.URL+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	res, err := c.http.Do(req)
	if err != nil {
		c.app.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, raw
}

// status makes a JSON request and returns only the status code.
func (c *client) status(method, path string, body any) int {
	c.app.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.app.srv.URL+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.app.t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}
