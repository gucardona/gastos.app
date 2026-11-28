package auth_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gastos/internal/api/apitest"
	"gastos/internal/api/auth"
	"gastos/internal/api/httpx"
)

func TestAuthFlow(t *testing.T) {
	app := apitest.New(t)
	c := app.Register("Ana", "ana@example.com")
	if c.User.ID == 0 || c.WalletID == 0 {
		t.Fatalf("signup without user/wallet: %+v", c)
	}

	c.Do("POST", "/api/auth/logout", nil, 204, nil)
	c.Do("GET", "/api/me", nil, 401, nil)

	c.Do("POST", "/api/auth/login", map[string]string{"email": "ANA@example.com", "password": "senha-segura"}, 200, nil)
	c.Do("GET", "/api/me", nil, 200, nil)
}

func TestRegisterValidation(t *testing.T) {
	app := apitest.New(t)
	c := app.Anon()
	var res struct {
		Error httpx.APIError `json:"error"`
	}
	c.Do("POST", "/api/auth/register", map[string]string{"name": "", "email": "x", "password": "123"}, 400, &res)
	for _, f := range []string{"name", "email", "password"} {
		if res.Error.Fields[f] == "" {
			t.Errorf("missing error on %s: %+v", f, res.Error.Fields)
		}
	}
	app.Register("Ana", "ana@example.com")
	c.Do("POST", "/api/auth/register", map[string]string{"name": "Outra", "email": "ana@example.com", "password": "senha-segura", "inviteCode": apitest.Invite}, 409, nil)
}

func TestLoginWrongPassword(t *testing.T) {
	app := apitest.New(t)
	app.Register("Ana", "ana@example.com")
	app.Anon().Do("POST", "/api/auth/login", map[string]string{"email": "ana@example.com", "password": "errada!!"}, 401, nil)
	app.Anon().Do("POST", "/api/auth/login", map[string]string{"email": "nao@existe.com", "password": "errada!!"}, 401, nil)
}

func TestLoginRateLimit(t *testing.T) {
	app := apitest.New(t)
	app.Register("Ana", "ana@example.com")
	c := app.Anon()
	for i := 0; i < 10; i++ {
		c.Do("POST", "/api/auth/login", map[string]string{"email": "ana@example.com", "password": "errada!!"}, 401, nil)
	}
	c.Do("POST", "/api/auth/login", map[string]string{"email": "ana@example.com", "password": "senha-segura"}, 429, nil)
}

// loginFrom tries to sign in as if from ip (the test server is on loopback, so X-Forwarded-For is trusted).
func loginFrom(t *testing.T, app *apitest.App, ip, email, password string) int {
	t.Helper()
	body := `{"email":"` + email + `","password":"` + password + `"}`
	req, _ := http.NewRequest("POST", app.URL()+"/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", ip)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	return res.StatusCode
}

func TestFailedLoginsCannotLockOutTheRightPassword(t *testing.T) {
	app := apitest.New(t)
	app.Register("Ana", "ana@example.com")
	for i := range 12 {
		loginFrom(t, app, "203.0.113."+apitest.Itoa(int64(i)), "ana@example.com", "errada!!")
	}
	if got := loginFrom(t, app, "198.51.100.7", "ana@example.com", "senha-segura"); got != 200 {
		t.Fatalf("right password after others' failures: status %d, want 200", got)
	}
	// guessing is still limited: once an address is blocked, even the right password is refused from it
	for range 10 {
		loginFrom(t, app, "192.0.2.1", "ana@example.com", "errada!!")
	}
	if got := loginFrom(t, app, "192.0.2.1", "ana@example.com", "outra-errada"); got != 429 {
		t.Fatalf("11th failure from one address: status %d, want 429", got)
	}
}

func TestRegisterRejectsPasswordsOver72Bytes(t *testing.T) {
	app := apitest.New(t)
	var res struct {
		Error httpx.APIError `json:"error"`
	}
	app.Anon().Do("POST", "/api/auth/register", map[string]string{"name": "Ana", "email": "ana@example.com",
		"password": strings.Repeat("a", 73), "inviteCode": apitest.Invite}, 400, &res)
	if res.Error.Fields["password"] == "" {
		t.Fatalf("no password error: %+v", res.Error)
	}
}

func TestClientIP(t *testing.T) {
	req := func(remote, xff string) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = remote
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return r
	}
	tests := []struct{ name, remote, xff, want string }{
		{"direct, no proxy", "203.0.113.9:5555", "", "203.0.113.9"},
		{"a trusted local proxy uses XFF", "127.0.0.1:4000", "198.51.100.7, 10.0.0.1", "198.51.100.7"},
		{"XFF forged by an external client is ignored", "203.0.113.9:5555", "1.2.3.4", "203.0.113.9"},
		{"IPv6 loopback is trusted too", "[::1]:4000", "198.51.100.7", "198.51.100.7"},
		{"loopback without XFF", "127.0.0.1:4000", "", "127.0.0.1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := auth.ClientIP(req(tc.remote, tc.xff)); got != tc.want {
				t.Errorf("clientIP = %q, want %q", got, tc.want)
			}
		})
	}
}
