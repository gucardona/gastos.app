package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"gastos/internal/config"
)

// fetchSite sends a request with the given Sec-Fetch-Site ("" = header absent) using c's cookies.
func fetchSite(c *client, method, path, site string) *http.Response {
	c.app.t.Helper()
	req, _ := http.NewRequest(method, c.app.srv.URL+path, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	if site != "" {
		req.Header.Set("Sec-Fetch-Site", site)
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.app.t.Fatal(err)
	}
	res.Body.Close()
	return res
}

func TestCrossSiteWritesAreRefused(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	for _, site := range []string{"cross-site", "same-site", "none"} {
		res := fetchSite(ana, "POST", ana.wpath("/inbox/accept-suggested"), site)
		if res.StatusCode != 403 || !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") {
			t.Errorf("%s: status %d, content-type %q", site, res.StatusCode, res.Header.Get("Content-Type"))
		}
	}
	if res := fetchSite(ana, "GET", ana.wpath("/inbox"), "cross-site"); res.StatusCode != 200 {
		t.Errorf("cross-site GET: status %d, want 200", res.StatusCode)
	}
	for _, site := range []string{"same-origin", ""} { // absent: old browsers, curl, tests
		if res := fetchSite(ana, "POST", ana.wpath("/inbox/accept-suggested"), site); res.StatusCode != 200 {
			t.Errorf("%q: status %d, want 200", site, res.StatusCode)
		}
	}
	if res := fetchSite(ana, "POST", "/api/auth/logout", "cross-site"); res.StatusCode != 403 {
		t.Errorf("cross-site logout: status %d, want 403", res.StatusCode)
	}
	ana.do("GET", "/api/me", nil, 200, nil) // still signed in
}

func TestDevModeAllowsSameSiteWrites(t *testing.T) {
	app := newTestAppWith(t, func(c *config.Config) { c.Dev = true })
	ana := app.register("Ana", "ana@example.com")
	if res := fetchSite(ana, "POST", ana.wpath("/inbox/accept-suggested"), "same-site"); res.StatusCode != 200 {
		t.Errorf("same-site in dev: status %d, want 200", res.StatusCode)
	}
	if res := fetchSite(ana, "POST", ana.wpath("/inbox/accept-suggested"), "cross-site"); res.StatusCode != 403 {
		t.Errorf("cross-site in dev: status %d, want 403", res.StatusCode)
	}
}

func checkCommonHeaders(t *testing.T, where string, h http.Header) {
	t.Helper()
	for k, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "same-origin",
		"X-Frame-Options":        "DENY",
	} {
		if got := h.Get(k); got != want {
			t.Errorf("%s: %s = %q, want %q", where, k, got, want)
		}
	}
	csp := h.Get("Content-Security-Policy")
	for _, part := range []string{"default-src 'self'", "script-src 'self'", "frame-ancestors 'none'", "connect-src 'self'"} {
		if !strings.Contains(csp, part) {
			t.Errorf("%s: CSP %q lacks %q", where, csp, part)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")

	res := fetchSite(ana, "GET", "/api/me", "")
	checkCommonHeaders(t, "api", res.Header)
	if got := res.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("api: Cache-Control %q, want no-store", got)
	}
	if got := res.Header.Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS without secure cookies: %q", got)
	}

	res = fetchSite(ana, "GET", "/", "")
	checkCommonHeaders(t, "spa", res.Header)
	if got := res.Header.Get("Cache-Control"); got == "no-store" {
		t.Errorf("spa: Cache-Control %q", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", app.srv.URL+"/api/events", nil)
	sse, err := ana.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer sse.Body.Close()
	checkCommonHeaders(t, "sse", sse.Header)
	if got := sse.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("sse: Cache-Control %q, want no-cache", got)
	}
}

func TestHSTSOnlyWithSecureCookies(t *testing.T) {
	app := newTestAppWith(t, func(c *config.Config) { c.CookieSecure = true })
	res := fetchSite(app.anon(), "GET", "/api/auth/registration", "")
	if got := res.Header.Get("Strict-Transport-Security"); got != "max-age=31536000" {
		t.Errorf("HSTS = %q", got)
	}
}
