package api

import (
	"net/http"
	"strings"
	"testing"
)

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
