package api_test

import (
	"net/http"
	"strings"
	"testing"

	"gastos/internal/api/apitest"
)

func TestRequiresJSONContentType(t *testing.T) {
	app := apitest.New(t)
	res, err := http.Post(app.URL()+"/api/auth/login", "text/plain", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	if res.StatusCode != 415 {
		t.Fatalf("status %d, want 415", res.StatusCode)
	}
}

func TestUnknownAPIRouteIs404JSON(t *testing.T) {
	app := apitest.New(t)
	app.Anon().Do("GET", "/api/nao-existe", nil, 404, nil)
}

func TestSPAFallback(t *testing.T) {
	app := apitest.New(t)
	for _, p := range []string{"/", "/lancamentos", "/qualquer/rota"} {
		res, err := http.Get(app.URL() + p)
		if err != nil {
			t.Fatal(err)
		}
		body := new(strings.Builder)
		buf := make([]byte, 512)
		n, _ := res.Body.Read(buf)
		body.Write(buf[:n])
		res.Body.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
		if res.StatusCode != 200 || !strings.Contains(body.String(), "<title>spa</title>") {
			t.Errorf("%s: status %d, body %q", p, res.StatusCode, body)
		}
	}
}
