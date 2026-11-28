package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
	_ "time/tzdata"

	"gastos/internal/api"
	"gastos/internal/config"
	"gastos/internal/db"
	"gastos/internal/events"
	"gastos/internal/recurrence"
	"gastos/internal/testutil"
)

// probe pins what one route answers to each kind of caller: the status and the JSON error code.
// guard is the route's wrapper: "public", "user", or the minimum wallet role ("reader", "editor", "owner").
type probe struct {
	method, path                          string // {walletID} is filled with the owner's wallet, any other wildcard with 999999
	guard                                 string
	anon, outsider, reader, editor, owner int
	codes                                 [5]string // error code per caller, same order; "" when the body is not an error
}

// routeTable has one row per route registered by Routes, plus /healthz and the SPA fallback.
// The values were recorded by running the code before the package split.
var routeTable = []probe{
	{"GET", "/api/auth/registration", "public", 200, 200, 200, 200, 200, [5]string{"", "", "", "", ""}},
	{"POST", "/api/auth/register", "public", 403, 403, 403, 403, 403, [5]string{"forbidden", "forbidden", "forbidden", "forbidden", "forbidden"}},
	{"POST", "/api/auth/login", "public", 401, 401, 401, 401, 401, [5]string{"unauthenticated", "unauthenticated", "unauthenticated", "unauthenticated", "unauthenticated"}},
	{"POST", "/api/auth/logout", "user", 401, 204, 204, 204, 204, [5]string{"unauthenticated", "", "", "", ""}},
	{"GET", "/api/me", "user", 401, 200, 200, 200, 200, [5]string{"unauthenticated", "", "", "", ""}},
	{"GET", "/api/events", "user", 401, 200, 200, 200, 200, [5]string{"", "", "", "", ""}},
	{"PATCH", "/api/me", "user", 401, 400, 400, 400, 400, [5]string{"unauthenticated", "validation", "validation", "validation", "validation"}},
	{"PUT", "/api/me/password", "user", 401, 400, 400, 400, 400, [5]string{"unauthenticated", "validation", "validation", "validation", "validation"}},
	{"GET", "/api/me/ai", "user", 401, 200, 200, 200, 200, [5]string{"unauthenticated", "", "", "", ""}},
	{"PUT", "/api/me/ai", "user", 401, 200, 200, 200, 200, [5]string{"unauthenticated", "", "", "", ""}},
	{"DELETE", "/api/me/ai/key", "user", 401, 204, 204, 204, 204, [5]string{"unauthenticated", "", "", "", ""}},
	{"POST", "/api/chat", "user", 401, 400, 400, 400, 400, [5]string{"unauthenticated", "validation", "validation", "validation", "validation"}},
	{"GET", "/api/chat", "user", 401, 400, 400, 400, 400, [5]string{"unauthenticated", "validation", "validation", "validation", "validation"}},
	{"DELETE", "/api/chat", "user", 401, 400, 400, 400, 400, [5]string{"unauthenticated", "validation", "validation", "validation", "validation"}},
	{"GET", "/api/wallets", "user", 401, 200, 200, 200, 200, [5]string{"unauthenticated", "", "", "", ""}},
	{"POST", "/api/wallets", "user", 401, 400, 400, 400, 400, [5]string{"unauthenticated", "validation", "validation", "validation", "validation"}},
	{"PATCH", "/api/wallets/{walletID}", "owner", 401, 404, 403, 403, 400, [5]string{"unauthenticated", "not_found", "forbidden", "forbidden", "validation"}},
	{"DELETE", "/api/wallets/{walletID}", "owner", 401, 404, 403, 403, 204, [5]string{"unauthenticated", "not_found", "forbidden", "forbidden", ""}},
	{"GET", "/api/wallets/{walletID}/members", "reader", 401, 404, 200, 200, 200, [5]string{"unauthenticated", "not_found", "", "", ""}},
	{"POST", "/api/wallets/{walletID}/members", "owner", 401, 404, 403, 403, 400, [5]string{"unauthenticated", "not_found", "forbidden", "forbidden", "validation"}},
	{"PATCH", "/api/wallets/{walletID}/members/999999", "owner", 401, 404, 403, 403, 404, [5]string{"unauthenticated", "not_found", "forbidden", "forbidden", "not_found"}},
	{"PUT", "/api/wallets/{walletID}/default-payment-method", "editor", 401, 404, 403, 200, 200, [5]string{"unauthenticated", "not_found", "forbidden", "", ""}},
	{"DELETE", "/api/wallets/{walletID}/members/999999", "reader", 401, 404, 403, 403, 404, [5]string{"unauthenticated", "not_found", "forbidden", "forbidden", "not_found"}},
	{"GET", "/api/wallets/{walletID}/bank-accounts", "reader", 401, 404, 200, 200, 200, [5]string{"unauthenticated", "not_found", "", "", ""}},
	{"POST", "/api/wallets/{walletID}/bank-accounts", "editor", 401, 404, 403, 400, 400, [5]string{"unauthenticated", "not_found", "forbidden", "validation", "validation"}},
	{"PUT", "/api/wallets/{walletID}/bank-accounts/999999", "editor", 401, 404, 403, 404, 404, [5]string{"unauthenticated", "not_found", "forbidden", "not_found", "not_found"}},
	{"DELETE", "/api/wallets/{walletID}/bank-accounts/999999", "editor", 401, 404, 403, 404, 404, [5]string{"unauthenticated", "not_found", "forbidden", "not_found", "not_found"}},
	{"POST", "/api/wallets/{walletID}/bank-accounts/999999/import", "editor", 401, 404, 403, 404, 404, [5]string{"unauthenticated", "not_found", "forbidden", "not_found", "not_found"}},
	{"GET", "/api/wallets/{walletID}/inbox", "reader", 401, 404, 200, 200, 200, [5]string{"unauthenticated", "not_found", "", "", ""}},
	{"POST", "/api/wallets/{walletID}/inbox/accept-suggested", "editor", 401, 404, 403, 200, 200, [5]string{"unauthenticated", "not_found", "forbidden", "", ""}},
	{"POST", "/api/wallets/{walletID}/inbox/999999/accept", "editor", 401, 404, 403, 404, 404, [5]string{"unauthenticated", "not_found", "forbidden", "not_found", "not_found"}},
	{"POST", "/api/wallets/{walletID}/inbox/999999/ignore", "editor", 401, 404, 403, 404, 404, [5]string{"unauthenticated", "not_found", "forbidden", "not_found", "not_found"}},
	{"POST", "/api/wallets/{walletID}/inbox/999999/match", "editor", 401, 404, 403, 404, 404, [5]string{"unauthenticated", "not_found", "forbidden", "not_found", "not_found"}},
	{"GET", "/api/wallets/{walletID}/rules", "reader", 401, 404, 200, 200, 200, [5]string{"unauthenticated", "not_found", "", "", ""}},
	{"POST", "/api/wallets/{walletID}/rules", "editor", 401, 404, 403, 400, 400, [5]string{"unauthenticated", "not_found", "forbidden", "validation", "validation"}},
	{"DELETE", "/api/wallets/{walletID}/rules/999999", "editor", 401, 404, 403, 404, 404, [5]string{"unauthenticated", "not_found", "forbidden", "not_found", "not_found"}},
	{"GET", "/api/wallets/{walletID}/budgets", "reader", 401, 404, 200, 200, 200, [5]string{"unauthenticated", "not_found", "", "", ""}},
	{"PUT", "/api/wallets/{walletID}/budgets/999999", "editor", 401, 404, 403, 400, 400, [5]string{"unauthenticated", "not_found", "forbidden", "validation", "validation"}},
	{"DELETE", "/api/wallets/{walletID}/budgets/999999", "editor", 401, 404, 403, 404, 404, [5]string{"unauthenticated", "not_found", "forbidden", "not_found", "not_found"}},
	{"GET", "/api/wallets/{walletID}/categories", "reader", 401, 404, 200, 200, 200, [5]string{"unauthenticated", "not_found", "", "", ""}},
	{"POST", "/api/wallets/{walletID}/categories", "editor", 401, 404, 403, 400, 400, [5]string{"unauthenticated", "not_found", "forbidden", "validation", "validation"}},
	{"PUT", "/api/wallets/{walletID}/categories/999999", "editor", 401, 404, 403, 404, 404, [5]string{"unauthenticated", "not_found", "forbidden", "not_found", "not_found"}},
	{"DELETE", "/api/wallets/{walletID}/categories/999999", "editor", 401, 404, 403, 404, 404, [5]string{"unauthenticated", "not_found", "forbidden", "not_found", "not_found"}},
	{"GET", "/api/wallets/{walletID}/payment-methods", "reader", 401, 404, 200, 200, 200, [5]string{"unauthenticated", "not_found", "", "", ""}},
	{"POST", "/api/wallets/{walletID}/payment-methods", "editor", 401, 404, 403, 400, 400, [5]string{"unauthenticated", "not_found", "forbidden", "validation", "validation"}},
	{"PUT", "/api/wallets/{walletID}/payment-methods/999999", "editor", 401, 404, 403, 400, 400, [5]string{"unauthenticated", "not_found", "forbidden", "validation", "validation"}},
	{"DELETE", "/api/wallets/{walletID}/payment-methods/999999", "editor", 401, 404, 403, 404, 404, [5]string{"unauthenticated", "not_found", "forbidden", "not_found", "not_found"}},
	{"GET", "/api/wallets/{walletID}/recurrences", "reader", 401, 404, 200, 200, 200, [5]string{"unauthenticated", "not_found", "", "", ""}},
	{"POST", "/api/wallets/{walletID}/recurrences", "editor", 401, 404, 403, 400, 400, [5]string{"unauthenticated", "not_found", "forbidden", "validation", "validation"}},
	{"PUT", "/api/wallets/{walletID}/recurrences/999999", "editor", 401, 404, 403, 404, 404, [5]string{"unauthenticated", "not_found", "forbidden", "not_found", "not_found"}},
	{"DELETE", "/api/wallets/{walletID}/recurrences/999999", "editor", 401, 404, 403, 404, 404, [5]string{"unauthenticated", "not_found", "forbidden", "not_found", "not_found"}},
	{"GET", "/api/wallets/{walletID}/settlements", "reader", 401, 404, 400, 400, 400, [5]string{"unauthenticated", "not_found", "validation", "validation", "validation"}},
	{"POST", "/api/wallets/{walletID}/settlements", "editor", 401, 404, 403, 400, 400, [5]string{"unauthenticated", "not_found", "forbidden", "validation", "validation"}},
	{"PUT", "/api/wallets/{walletID}/settlements/999999", "editor", 401, 404, 403, 404, 404, [5]string{"unauthenticated", "not_found", "forbidden", "not_found", "not_found"}},
	{"DELETE", "/api/wallets/{walletID}/settlements/999999", "editor", 401, 404, 403, 404, 404, [5]string{"unauthenticated", "not_found", "forbidden", "not_found", "not_found"}},
	{"GET", "/api/wallets/{walletID}/summary", "reader", 401, 404, 400, 400, 400, [5]string{"unauthenticated", "not_found", "validation", "validation", "validation"}},
	{"GET", "/api/wallets/{walletID}/transactions", "reader", 401, 404, 400, 400, 400, [5]string{"unauthenticated", "not_found", "validation", "validation", "validation"}},
	{"POST", "/api/wallets/{walletID}/transactions", "editor", 401, 404, 403, 400, 400, [5]string{"unauthenticated", "not_found", "forbidden", "validation", "validation"}},
	{"PUT", "/api/wallets/{walletID}/transactions/999999", "editor", 401, 404, 403, 404, 404, [5]string{"unauthenticated", "not_found", "forbidden", "not_found", "not_found"}},
	{"DELETE", "/api/wallets/{walletID}/transactions/999999", "editor", 401, 404, 403, 404, 404, [5]string{"unauthenticated", "not_found", "forbidden", "not_found", "not_found"}},
	{"GET", "/healthz", "public", 200, 200, 200, 200, 200, [5]string{"", "", "", "", ""}},
	{"GET", "/api/nope", "public", 404, 404, 404, 404, 404, [5]string{"not_found", "not_found", "not_found", "not_found", "not_found"}},
	{"GET", "/", "public", 200, 200, 200, 200, 200, [5]string{"", "", "", "", ""}},
}

// callers are the roles of the probed users, in the order the table lists them.
var callers = [5]string{"anon", "outsider", "reader", "editor", "owner"}

// routeEnv starts a fresh server and returns one HTTP client per caller plus the owner's wallet ID.
func routeEnv(t *testing.T) (map[string]*http.Client, string, string) {
	t.Helper()
	d, err := db.Open(filepath.Join(testutil.TempDir(t), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	const invite = "test-invite-code"
	cfg := config.Config{Location: loc, BackupDir: filepath.Join(testutil.TempDir(t), "backups"), BackupKeep: 3, InviteCode: invite}
	bus := events.New()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := api.New(d, cfg, bus, recurrence.NewMaterializer(d, bus, loc, log), log)
	var spa fs.FS = fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>spa</title>")}}
	srv := httptest.NewServer(s.Routes(spa))
	t.Cleanup(func() { srv.Close(); _ = d.Close() })

	clients := map[string]*http.Client{"anon": {}}
	var walletID string
	for _, name := range []string{"owner", "editor", "reader", "outsider"} {
		jar, _ := cookiejar.New(nil)
		c := &http.Client{Jar: jar}
		clients[name] = c
		email := name + "@example.com"
		status, _ := send(t, c, srv.URL, "POST", "/api/auth/register",
			map[string]string{"name": name, "email": email, "password": "senha-segura", "inviteCode": invite})
		if status != 201 {
			t.Fatalf("register %s: status %d", name, status)
		}
	}
	_, body := send(t, clients["owner"], srv.URL, "GET", "/api/me", nil)
	var me struct {
		Wallets []struct {
			ID json.Number `json:"id"`
		} `json:"wallets"`
	}
	if err := json.Unmarshal(body, &me); err != nil || len(me.Wallets) == 0 {
		t.Fatalf("GET /api/me: %v; body %s", err, body)
	}
	walletID = me.Wallets[0].ID.String()
	for _, name := range []string{"editor", "reader"} {
		status, _ := send(t, clients["owner"], srv.URL, "POST", "/api/wallets/"+walletID+"/members",
			map[string]string{"email": name + "@example.com", "role": name})
		if status != 201 {
			t.Fatalf("add %s: status %d", name, status)
		}
	}
	return clients, srv.URL, walletID
}

// send makes one request, with a JSON body when body is not nil, and returns the status and the raw body.
func send(t *testing.T, c *http.Client, base, method, path string, body any) (int, []byte) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	// The SSE route never ends: stop waiting for the body once the status is known.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, method, base+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = res.Body.Close() }()
	if path == "/api/events" {
		return res.StatusCode, nil
	}
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, raw
}

// errCode returns the "error.code" field of a JSON error body, or "" when there is none.
func errCode(raw []byte) string {
	var e struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &e) != nil {
		return ""
	}
	return e.Error.Code
}

// probeAll calls one route as every caller, destructive owner last, and returns the statuses and codes.
func probeAll(t *testing.T, method, path string) ([5]int, [5]string) {
	clients, base, walletID := routeEnv(t)
	path = strings.ReplaceAll(path, "{walletID}", walletID)
	var body any
	if method == "POST" || method == "PUT" || method == "PATCH" {
		body = map[string]string{}
	}
	var statuses [5]int
	var codes [5]string
	for i, who := range callers {
		var raw []byte
		statuses[i], raw = send(t, clients[who], base, method, path, body)
		codes[i] = errCode(raw)
	}
	return statuses, codes
}

// TestRouteTable pins the status and error code of every route for every kind of caller.
func TestRouteTable(t *testing.T) {
	t.Parallel()
	for _, tt := range routeTable {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()
			got, codes := probeAll(t, tt.method, tt.path)
			want := [5]int{tt.anon, tt.outsider, tt.reader, tt.editor, tt.owner}
			if got != want {
				t.Errorf("statuses %v, want %v", got, want)
			}
			if codes != tt.codes {
				t.Errorf("codes %q, want %q", codes, tt.codes)
			}
			if tt.guard != "public" && got[0] != 401 {
				t.Errorf("anon: status %d, want 401", got[0])
			}
			switch tt.guard {
			case "reader", "editor", "owner":
				if got[1] != 404 || codes[1] != "not_found" {
					t.Errorf("outsider: %d %q, want 404 not_found", got[1], codes[1])
				}
			}
			switch tt.guard {
			case "editor", "owner":
				if got[2] != 403 || codes[2] != "forbidden" {
					t.Errorf("reader: %d %q, want 403 forbidden", got[2], codes[2])
				}
			}
			if tt.guard == "owner" && (got[3] != 403 || codes[3] != "forbidden") {
				t.Errorf("editor: %d %q, want 403 forbidden", got[3], codes[3])
			}
		})
	}
}

// TestMiddleware pins the security headers on API responses and the cross-site write barrier.
func TestMiddleware(t *testing.T) {
	t.Parallel()
	clients, base, _ := routeEnv(t)
	c := clients["owner"]

	res, err := c.Get(base + "/api/me")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	for k, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "same-origin",
		"X-Frame-Options":        "DENY",
		"Cache-Control":          "no-store",
	} {
		if got := res.Header.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	csp := res.Header.Get("Content-Security-Policy")
	for _, part := range []string{"default-src 'self'", "script-src 'self'", "frame-ancestors 'none'", "connect-src 'self'"} {
		if !strings.Contains(csp, part) {
			t.Errorf("CSP %q lacks %q", csp, part)
		}
	}

	req, _ := http.NewRequest("POST", base+"/api/wallets", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	res, err = c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != 403 || errCode(raw) != "forbidden" {
		t.Errorf("cross-site POST: %d %q, want 403 forbidden", res.StatusCode, errCode(raw))
	}
}
