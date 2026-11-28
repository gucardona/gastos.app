// Package apitest is the HTTP test harness shared by the tests of every area: a real server on httptest,
// a client with a cookie jar and the helpers that more than one area uses.
package apitest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
	_ "time/tzdata" //nolint:revive // moved unchanged, see docs/security.md

	"golang.org/x/crypto/bcrypt"

	"gastos/internal/api"
	"gastos/internal/api/httpx"
	"gastos/internal/api/ledger"
	"gastos/internal/config"
	"gastos/internal/db"
	"gastos/internal/events"
	"gastos/internal/recurrence"
	"gastos/internal/testutil"
)

// Invite is the invite code every test app is configured with.
const Invite = "test-invite-code"

// init lowers the bcrypt cost so sign-ups in tests are fast.
func init() { api.BcryptCost = bcrypt.MinCost }

// App is a running server on httptest with its own database.
type App struct {
	t   *testing.T
	srv *httptest.Server
	s   *api.Server
}

// New starts an App with the default configuration.
func New(t *testing.T) *App { return NewWith(t, nil) }

// NewWith lets a test tweak the configuration (dev mode, backup directory...) before the server starts.
func NewWith(t *testing.T, tweak func(*config.Config)) *App {
	t.Helper()
	d, err := db.Open(filepath.Join(testutil.TempDir(t), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	cfg := config.Config{Location: loc, CookieSecure: false, BackupDir: filepath.Join(testutil.TempDir(t), "backups"), BackupKeep: 3, InviteCode: Invite}
	if tweak != nil {
		tweak(&cfg)
	}
	bus := events.New()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := api.New(d, cfg, bus, recurrence.NewMaterializer(d, bus, loc, log), log)
	spa := fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>spa</title>")}}
	srv := httptest.NewServer(s.Routes(spa))
	t.Cleanup(func() { srv.Close(); d.Close() }) //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	return &App{t: t, srv: srv, s: s}
}

// Client is an HTTP client with its own cookie jar, signed in as one user.
type Client struct {
	app  *App
	http *http.Client
	User struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	WalletID int64 // the "Pessoal" wallet created on signup
}

// Anon returns a client with no session.
func (a *App) Anon() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{app: a, http: &http.Client{Jar: serverClockJar{jar}}}
}

// serverClockJar keeps cookies without their expiry date. Tests freeze the server clock (Env.Now) in the past,
// so the session cookie's Expires is computed from that frozen date, while the standard jar compares it with
// the real clock and silently drops it once it is more than 30 days old. The server still checks the session
// against its own clock. Deletions (logout: an empty value or an expiry in the past) still go through.
type serverClockJar struct{ http.CookieJar }

func (j serverClockJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	for _, c := range cookies {
		if c.Value != "" && c.MaxAge >= 0 {
			c.Expires = time.Time{}
		}
	}
	j.CookieJar.SetCookies(u, cookies)
}

// Register creates a user, keeps the cookie and the personal wallet.
func (a *App) Register(name, email string) *Client {
	a.t.Helper()
	c := a.Anon()
	c.Do("POST", "/api/auth/register", map[string]string{"name": name, "email": email, "password": "senha-segura", "inviteCode": Invite}, 201, &c.User)
	var me struct {
		Wallets []struct {
			ID int64 `json:"id"`
		} `json:"wallets"`
	}
	c.Do("GET", "/api/me", nil, 200, &me)
	c.WalletID = me.Wallets[0].ID
	return c
}

// DoRaw makes the request and returns the status without asserting it; out (if not nil) receives the JSON body.
func (c *Client) DoRaw(method, path string, body any, out any) int {
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
	defer res.Body.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	raw, _ := io.ReadAll(res.Body)
	if out != nil && len(raw) > 0 {
		_ = json.Unmarshal(raw, out)
	}
	return res.StatusCode
}

// Do makes the request, checks the status and decodes the response into out (if not nil).
func (c *Client) Do(method, path string, body any, wantStatus int, out any) {
	c.app.t.Helper()
	var raw json.RawMessage
	status := c.DoRaw(method, path, body, &raw)
	if status != wantStatus {
		c.app.t.Fatalf("%s %s: status %d, want %d; body: %s", method, path, status, wantStatus, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			c.app.t.Fatalf("%s %s: decode: %v; body: %s", method, path, err, raw)
		}
	}
}

// WPath builds a path inside the client's active wallet.
func (c *Client) WPath(format string, args ...any) string {
	return fmt.Sprintf("/api/wallets/%d", c.WalletID) + fmt.Sprintf(format, args...)
}

// Itoa formats an id for a path.
func Itoa(n int64) string { return fmt.Sprint(n) }

// Upload sends a multipart form with a single file field named "file" and returns the status and raw body.
func (c *Client) Upload(path string, fields map[string]string, filename string, content []byte) (int, []byte) {
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
	fw.Write(content) //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	mw.Close()        //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	req, _ := http.NewRequest("POST", c.app.srv.URL+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	res, err := c.http.Do(req)
	if err != nil {
		c.app.t.Fatal(err)
	}
	defer res.Body.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, raw
}

// Status makes a JSON request and returns only the status code.
func (c *Client) Status(method, path string, body any) int {
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
	res.Body.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	return res.StatusCode
}

// Server returns the server under test; its Env field is the environment the handlers share.
func (a *App) Server() *api.Server { return a.s }

// SetNow replaces the server's clock.
func (a *App) SetNow(now func() time.Time) { a.s.Now = now }

// ShareWallet adds other to owner's wallet with the given role and points other.WalletID at it.
func ShareWallet(owner, other *Client, role string) {
	owner.app.t.Helper()
	owner.Do("POST", owner.WPath("/members"), map[string]string{"email": other.User.Email, "role": role}, 201, nil)
	other.WalletID = owner.WalletID
}

// FirstCategory returns the id of the first active category of the given kind in the client's wallet.
func FirstCategory(t *testing.T, c *Client, kind string) int64 {
	t.Helper()
	var cats []struct {
		ID       int64  `json:"id"`
		Kind     string `json:"kind"`
		Archived bool   `json:"archived"`
	}
	c.Do("GET", c.WPath("/categories"), nil, 200, &cats)
	for _, cat := range cats {
		if cat.Kind == kind && !cat.Archived {
			return cat.ID
		}
	}
	t.Fatalf("no %s category", kind)
	return 0
}

// MonthsAgoFirst returns the 1st of the month n months ago, in the app's time zone.
func MonthsAgoFirst(t *testing.T, app *App, n int) string {
	t.Helper()
	now := time.Now().In(app.s.Cfg.Location)
	return time.Date(now.Year(), now.Month()-time.Month(n), 1, 0, 0, 0, 0, time.UTC).Format(httpx.DateLayout)
}

// T returns the test the app belongs to.
func (a *App) T() *testing.T { return a.t }

// App returns the app the client talks to.
func (c *Client) App() *App { return c.app }

// CardInfo is the card part of a payment method, as the API returns it.
type CardInfo struct {
	ClosingDay      *int   `json:"closingDay"`
	DueDay          *int   `json:"dueDay"`
	Issuer          string `json:"issuer"`
	CardForm        string `json:"cardForm"`
	Last4           string `json:"last4"`
	BestPurchaseDay int    `json:"bestPurchaseDay"`
}

// PaymentMethod is a payment method as the API returns it.
type PaymentMethod struct {
	ID       int64     `json:"id"`
	Name     string    `json:"name"`
	Archived bool      `json:"archived"`
	Card     *CardInfo `json:"card"`
}

// Card2027 creates a card that closes on the 20th and is due on the 27th.
func Card2027(c *Client) PaymentMethod {
	c.app.t.Helper()
	var m PaymentMethod
	c.Do("POST", c.WPath("/payment-methods"), map[string]any{"name": "Itaú Black", "icon": "💳",
		"card": map[string]any{"closingDay": 20, "dueDay": 27, "issuer": " Itaú Mastercard ", "cardForm": "virtual", "last4": " 1234 "}}, 201, &m)
	return m
}

// Purchase posts a transaction on the given method and returns it.
func Purchase(c *Client, kind, date string, method int64) ledger.Transaction {
	c.app.t.Helper()
	var tx ledger.Transaction
	c.Do("POST", c.WPath("/transactions"), map[string]any{"kind": kind, "amountCents": 8700, "description": "Jantar",
		"categoryId": FirstCategory(c.app.t, c, kind), "paymentMethodId": method, "date": date, "paidByUserId": c.User.ID}, 201, &tx)
	return tx
}

// ListRange lists the transactions from the given date to a year later.
func ListRange(c *Client, from string) []ledger.Transaction {
	c.app.t.Helper()
	var list []ledger.Transaction
	start, _ := time.Parse(httpx.DateLayout, from)
	to := start.AddDate(0, 0, 365).Format(httpx.DateLayout)
	c.Do("GET", c.WPath("/transactions?from=%s&to=%s", from, to), nil, 200, &list)
	return list
}

// OfxRows builds an OFX 1.x (SGML) statement from "YYYYMMDD|amount|fitid|memo" rows.
func OfxRows(rows ...string) []byte {
	var b strings.Builder
	b.WriteString("OFXHEADER:100\nCHARSET:1252\n\n<OFX><BANKTRANLIST>\n")
	for _, r := range rows {
		f := strings.SplitN(r, "|", 4)
		fmt.Fprintf(&b, "<STMTTRN><TRNTYPE>X<DTPOSTED>%s<TRNAMT>%s<FITID>%s<MEMO>%s</STMTTRN>\n", f[0], f[1], f[2], f[3])
	}
	b.WriteString("</BANKTRANLIST></OFX>\n")
	return []byte(b.String())
}

// BankAccount decodes the id of a bank account.
type BankAccount struct {
	ID int64 `json:"id"`
}

// ImportSummary is the result of a statement import.
type ImportSummary struct {
	Imported   int `json:"imported"`
	Duplicates int `json:"duplicates"`
}

// InboxItem is a bank line waiting in the inbox.
type InboxItem struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
}

// NewAccount creates a bank account named name.
func NewAccount(t *testing.T, c *Client, name string) BankAccount {
	t.Helper()
	var a BankAccount
	c.Do("POST", c.WPath("/bank-accounts"), map[string]any{"name": name}, 201, &a)
	return a
}

// ImportFile uploads an OFX statement to the account and checks the status.
func ImportFile(t *testing.T, c *Client, accountID int64, content []byte, wantStatus int) ImportSummary {
	t.Helper()
	status, raw := c.Upload(c.WPath("/bank-accounts/%d/import", accountID), map[string]string{"format": "ofx"}, "extrato.ofx", content)
	if status != wantStatus {
		t.Fatalf("import: status %d, want %d; body: %s", status, wantStatus, raw)
	}
	var out ImportSummary
	_ = json.Unmarshal(raw, &out)
	return out
}

// Pending lists the inbox.
func Pending(t *testing.T, c *Client) []InboxItem {
	t.Helper()
	var list []InboxItem
	c.Do("GET", c.WPath("/inbox"), nil, 200, &list)
	return list
}

// FindByDescription returns the first inbox item whose description contains the text.
func FindByDescription(t *testing.T, list []InboxItem, contains string) InboxItem {
	t.Helper()
	for _, it := range list {
		if strings.Contains(it.Description, contains) {
			return it
		}
	}
	t.Fatalf("no inbox item containing %q in %+v", contains, list)
	return InboxItem{}
}

// OpenStream opens the user's SSE stream and delivers every line read on the channel.
// The server subscribes before sending the headers, so the stream is live once it returns.
func OpenStream(t *testing.T, c *Client) <-chan string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, "GET", c.app.srv.URL+"/api/events", nil)
	res, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("status %d, content-type %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	lines := make(chan string, 16)
	go func() {
		defer res.Body.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	return lines
}

// URL returns the base URL of the test server.
func (a *App) URL() string { return a.srv.URL }

// HTTP returns the client's HTTP client, with its cookie jar, for requests the helpers do not cover (streams).
func (c *Client) HTTP() *http.Client { return c.http }

// RunHousekeeping runs the housekeeping job until the condition is true and then cancels it.
func RunHousekeeping(t *testing.T, app *App, until func() bool) {
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
		t.Fatal("Housekeeping did not stop when the context was cancelled")
	}
}

// Backups lists the backup files the app has written.
func Backups(app *App) []string {
	files, _ := filepath.Glob(filepath.Join(app.s.Cfg.BackupDir, "gastos-*.db"))
	return files
}
