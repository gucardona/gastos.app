package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
)

func init() { bcryptCost = bcrypt.MinCost }

type testApp struct {
	t   *testing.T
	srv *httptest.Server
	s   *Server
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	cfg := config.Config{Location: loc, CookieSecure: false}
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
	walletID int64 // carteira "Pessoal" criada no registro
}

func (a *testApp) anon() *client {
	jar, _ := cookiejar.New(nil)
	return &client{app: a, http: &http.Client{Jar: jar}}
}

// register cria usuário, guarda o cookie e a carteira pessoal.
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

// do faz a request, confere o status e decodifica a resposta em out (se não nil).
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

// wpath monta um path dentro da carteira ativa do client.
func (c *client) wpath(format string, args ...any) string {
	return fmt.Sprintf("/api/wallets/%d", c.walletID) + fmt.Sprintf(format, args...)
}

func itoa(n int64) string { return fmt.Sprint(n) }
