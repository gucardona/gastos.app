package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestAuthFlow(t *testing.T) {
	app := newTestApp(t)
	c := app.register("Ana", "ana@example.com")
	if c.user.ID == 0 || c.walletID == 0 {
		t.Fatalf("signup without user/wallet: %+v", c)
	}

	c.do("POST", "/api/auth/logout", nil, 204, nil)
	c.do("GET", "/api/me", nil, 401, nil)

	c.do("POST", "/api/auth/login", map[string]string{"email": "ANA@example.com", "password": "senha-segura"}, 200, nil)
	c.do("GET", "/api/me", nil, 200, nil)
}

func TestRegisterValidation(t *testing.T) {
	app := newTestApp(t)
	c := app.anon()
	var res struct {
		Error apiError `json:"error"`
	}
	c.do("POST", "/api/auth/register", map[string]string{"name": "", "email": "x", "password": "123"}, 400, &res)
	for _, f := range []string{"name", "email", "password"} {
		if res.Error.Fields[f] == "" {
			t.Errorf("missing error on %s: %+v", f, res.Error.Fields)
		}
	}
	app.register("Ana", "ana@example.com")
	c.do("POST", "/api/auth/register", map[string]string{"name": "Outra", "email": "ana@example.com", "password": "senha-segura"}, 409, nil)
}

func TestLoginWrongPassword(t *testing.T) {
	app := newTestApp(t)
	app.register("Ana", "ana@example.com")
	app.anon().do("POST", "/api/auth/login", map[string]string{"email": "ana@example.com", "password": "errada!!"}, 401, nil)
	app.anon().do("POST", "/api/auth/login", map[string]string{"email": "nao@existe.com", "password": "errada!!"}, 401, nil)
}

func TestLoginRateLimit(t *testing.T) {
	app := newTestApp(t)
	app.register("Ana", "ana@example.com")
	c := app.anon()
	for i := 0; i < 10; i++ {
		c.do("POST", "/api/auth/login", map[string]string{"email": "ana@example.com", "password": "errada!!"}, 401, nil)
	}
	c.do("POST", "/api/auth/login", map[string]string{"email": "ana@example.com", "password": "senha-segura"}, 429, nil)
}

func TestRequiresJSONContentType(t *testing.T) {
	app := newTestApp(t)
	res, err := http.Post(app.srv.URL+"/api/auth/login", "text/plain", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 415 {
		t.Fatalf("status %d, want 415", res.StatusCode)
	}
}

func TestUnknownAPIRouteIs404JSON(t *testing.T) {
	app := newTestApp(t)
	app.anon().do("GET", "/api/nao-existe", nil, 404, nil)
}

func TestSPAFallback(t *testing.T) {
	app := newTestApp(t)
	for _, p := range []string{"/", "/lancamentos", "/qualquer/rota"} {
		res, err := http.Get(app.srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		body := new(strings.Builder)
		buf := make([]byte, 512)
		n, _ := res.Body.Read(buf)
		body.Write(buf[:n])
		res.Body.Close()
		if res.StatusCode != 200 || !strings.Contains(body.String(), "<title>spa</title>") {
			t.Errorf("%s: status %d, body %q", p, res.StatusCode, body)
		}
	}
}
