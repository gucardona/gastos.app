# Backend Foundation: implementation plan

> **For agents:** implement task by task, in order. Each task is TDD: write the test, watch it fail, implement, watch it pass, commit. Steps use checkboxes (`- [ ]`).

**Spec:** [2026-10-09-backend-foundation-design.md](../specs/2026-10-09-backend-foundation-design.md)

**Goal:** replace `src/` with a new backend:
- in `cmd/` + `internal/`;
- a single schema;
- money in cents;
- cookie sessions;
- `/api/wallets/{walletID}/...` API with the Go 1.22+ router;
- an idempotent recurrence materializer.

**Architecture:** small, pure packages (`money`, `recurrence`), a `db` package that owns the schema, and an `api` package with a `Server` that holds the dependencies. Handlers have the signature `func(w, r) error`, and a single adapter converts errors into JSON.

**Stack:** Go 1.27 (`go.mod` asks for ≥ 1.26), `modernc.org/sqlite`, `golang.org/x/crypto/bcrypt` and only the stdlib (`net/http`, `log/slog`, `embed`).

**Branch:** `rewrite/backend`. All new code lives alongside `src/` until task 17, which removes the old code.

**Commands:** the examples run `go test` from the repo root. They work the same in PowerShell and bash.

---

## File map

| File | Action | Responsibility |
|---|---|---|
| `cmd/gastos/main.go` | Create | config → db → server → jobs → `ListenAndServe` + graceful shutdown |
| `internal/config/config.go` (+ `_test`) | Create | read and validate env |
| `internal/db/schema.sql` | Create | schema version 1 |
| `internal/db/db.go` (+ `_test`) | Create | `Open` with pragmas, `migrate` via `user_version` |
| `internal/db/backup.go` (+ `_test`) | Create | `VACUUM INTO` + rotation |
| `internal/money/money.go` (+ `_test`) | Create | `Allocate`, `EqualShares` |
| `internal/recurrence/occurrences.go` (+ `_test`) | Create | pure `Occurrences` |
| `internal/recurrence/materializer.go` (+ `_test`) | Create | generates idempotent transactions and settlements |
| `internal/events/bus.go` (+ `_test`) | Move/adapt | per-wallet bus (from `src/events`) |
| `internal/api/server.go` | Create | `Server`, `Routes`, log, recover and dev CORS middlewares |
| `internal/api/httpx.go` | Create | `apiError`, `handle`, `decode`, `writeJSON`, `querier` |
| `internal/api/auth.go` | Create | register, login, logout, me, `requireUser`, sessions, rate limit |
| `internal/api/wallets.go` | Create | wallets, members, `requireWallet`, seed |
| `internal/api/catalog.go` | Create | categories, payment-methods |
| `internal/api/transactions.go` | Create | CRUD + splits |
| `internal/api/recurrences.go` | Create | CRUD + synchronous materialization |
| `internal/api/settlements.go` | Create | CRUD |
| `internal/api/budgets.go` | Create | list, upsert, delete |
| `internal/api/summary.go` (+ `settle.go`) | Create | `/summary` + suggested settlements (pure) |
| `internal/api/events.go` | Create | SSE with heartbeat |
| `internal/api/spa.go` | Create | serves the embedded `fs.FS` with fallback to `index.html` |
| `internal/api/*_test.go` | Create | integration with `httptest` |
| `web/embed.go`, `web/dist/index.html` | Create | placeholder until phase 3 |
| `src/`, `vendor/`, `gastos-app`, `deploy.sh` | Remove | task 17 |

---

## Task 0: Branch and skeleton

- [ ] **Step 1:** create the branch and the directories

```bash
git switch -c rewrite/backend
mkdir -p cmd/gastos internal/config internal/db internal/money internal/recurrence internal/events internal/api web/dist
```

- [ ] **Step 2:** add to `.gitignore`

```
/data/
/gastos
/gastos.exe
web/node_modules/
```

`web/dist/` does **not** go in the ignore file yet, because the task 16 placeholder must be versioned for `go:embed` to compile. Phase 3 replaces this with the CI build.

- [ ] **Step 3:** commit `chore: new backend skeleton`

---

## Task 1: `config`

**Files:** `internal/config/config.go`, `internal/config/config_test.go`

- [ ] **Step 1: test**

```go
package config

import "testing"

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":8000" || c.DBPath != "./data/gastos.db" || c.BackupKeep != 14 {
		t.Fatalf("defaults inesperados: %+v", c)
	}
	if c.Location.String() != "America/Sao_Paulo" || !c.CookieSecure || c.Dev {
		t.Fatalf("defaults inesperados: %+v", c)
	}
}

func TestLoadInvalid(t *testing.T) {
	for _, m := range []map[string]string{
		{"GASTOS_BACKUP_KEEP": "0"},
		{"GASTOS_BACKUP_KEEP": "x"},
		{"GASTOS_TZ": "Marte/Base"},
		{"GASTOS_COOKIE_SECURE": "talvez"},
		{"GASTOS_DEV": "sim"},
	} {
		if _, err := Load(env(m)); err == nil {
			t.Errorf("Load(%v) deveria falhar", m)
		}
	}
}
```

- [ ] **Step 2:** `go test ./internal/config/` must fail (does not compile).

- [ ] **Step 3: implementation**

```go
package config

import (
	"fmt"
	"strconv"
	"time"
)

type Config struct {
	Addr         string
	DBPath       string
	BackupDir    string
	BackupKeep   int
	Location     *time.Location
	CookieSecure bool
	Dev          bool
}

// Load reads the configuration from getenv (os.Getenv in production) and fails early on invalid values.
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		Addr:      or(getenv("GASTOS_ADDR"), ":8000"),
		DBPath:    or(getenv("GASTOS_DB_PATH"), "./data/gastos.db"),
		BackupDir: or(getenv("GASTOS_BACKUP_DIR"), "./data/backups"),
	}

	keep, err := strconv.Atoi(or(getenv("GASTOS_BACKUP_KEEP"), "14"))
	if err != nil || keep < 1 {
		return Config{}, fmt.Errorf("GASTOS_BACKUP_KEEP inválido: %q", getenv("GASTOS_BACKUP_KEEP"))
	}
	c.BackupKeep = keep

	c.Location, err = time.LoadLocation(or(getenv("GASTOS_TZ"), "America/Sao_Paulo"))
	if err != nil {
		return Config{}, fmt.Errorf("GASTOS_TZ inválido: %w", err)
	}
	if c.CookieSecure, err = parseBool("GASTOS_COOKIE_SECURE", getenv, true); err != nil {
		return Config{}, err
	}
	if c.Dev, err = parseBool("GASTOS_DEV", getenv, false); err != nil {
		return Config{}, err
	}
	return c, nil
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func parseBool(key string, getenv func(string) string, def bool) (bool, error) {
	v := getenv(key)
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s inválido: %q", key, v)
	}
	return b, nil
}
```

`cmd/gastos/main.go` imports `_ "time/tzdata"`, so `LoadLocation` works on Windows and in minimal images. The test needs it on Windows too: add `import _ "time/tzdata"` to `config_test.go`.

- [ ] **Step 4:** `go test ./internal/config/` passes.
- [ ] **Step 5:** commit `feat(config): load and validate env`

---

## Task 2: `db`: schema, `Open`, migrate

**Files:** `internal/db/schema.sql`, `internal/db/db.go`, `internal/db/db_test.go`

- [ ] **Step 1:** create `schema.sql` with the **exact** SQL from the "Schema (version 1)" section of the spec.

- [ ] **Step 2: tests**

```go
package db

import (
	"context"
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) *sql.DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestOpenSetsUserVersion(t *testing.T) {
	d := openTemp(t)
	var v int
	if err := d.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != len(migrations)+1 {
		t.Fatalf("user_version = %d, want %d", v, len(migrations)+1)
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	for i := 0; i < 2; i++ {
		d, err := Open(path)
		if err != nil {
			t.Fatalf("abertura %d: %v", i, err)
		}
		d.Close()
	}
}

func TestPragmasOnEveryConnection(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()
	var conns []*sql.Conn
	for i := 0; i < 3; i++ { // holds 3 distinct connections from the pool
		c, err := d.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		conns = append(conns, c)
	}
	for i, c := range conns {
		var fk, timeout int
		if err := c.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fk); err != nil {
			t.Fatal(err)
		}
		if err := c.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&timeout); err != nil {
			t.Fatal(err)
		}
		if fk != 1 || timeout != 5000 {
			t.Errorf("conn %d: foreign_keys=%d busy_timeout=%d", i, fk, timeout)
		}
		c.Close()
	}
}

func TestDeleteWalletCascades(t *testing.T) {
	d := openTemp(t)
	mustExec(t, d, `INSERT INTO users (id, name, email, password_hash) VALUES (1, 'A', 'a@x', 'h')`)
	mustExec(t, d, `INSERT INTO wallets (id, name) VALUES (1, 'W')`)
	mustExec(t, d, `INSERT INTO wallet_members (wallet_id, user_id, role) VALUES (1, 1, 'owner')`)
	mustExec(t, d, `INSERT INTO categories (id, wallet_id, kind, name, icon, color, essentiality) VALUES (1, 1, 'expense', 'C', 'x', '#000000', 'essential')`)
	mustExec(t, d, `INSERT INTO transactions (wallet_id, kind, amount_cents, description, category_id, date, paid_by_user_id, created_by) VALUES (1, 'expense', 100, 'd', 1, '2026-10-01', 1, 1)`)
	mustExec(t, d, `DELETE FROM wallets WHERE id = 1`)
	for _, table := range []string{"wallet_members", "categories", "transactions"} {
		var n int
		d.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n)
		if n != 0 {
			t.Errorf("%s: %d linhas após apagar carteira", table, n)
		}
	}
}

func TestOneOwnerPerWallet(t *testing.T) {
	d := openTemp(t)
	mustExec(t, d, `INSERT INTO users (id, name, email, password_hash) VALUES (1,'A','a@x','h'), (2,'B','b@x','h')`)
	mustExec(t, d, `INSERT INTO wallets (id, name) VALUES (1, 'W')`)
	mustExec(t, d, `INSERT INTO wallet_members (wallet_id, user_id, role) VALUES (1, 1, 'owner')`)
	if _, err := d.Exec(`INSERT INTO wallet_members (wallet_id, user_id, role) VALUES (1, 2, 'owner')`); err == nil {
		t.Fatal("segundo owner deveria violar o índice único")
	}
}

func mustExec(t *testing.T, d *sql.DB, q string) {
	t.Helper()
	if _, err := d.Exec(q); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}
```

Add `"database/sql"` to the imports.

- [ ] **Step 3:** `go test ./internal/db/` must fail.

- [ ] **Step 4: implementation**

```go
package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// migrations[i] takes the database from version i+1 to i+2. schema.sql is always version 1.
// To change the schema: add an item here; never edit old items.
var migrations = []string{}

const pragmas = "?_pragma=foreign_keys(1)" +
	"&_pragma=journal_mode(WAL)" +
	"&_pragma=busy_timeout(5000)" +
	"&_pragma=synchronous(NORMAL)" +
	"&_txlock=immediate"

// Open opens SQLite with pragmas applied on every pool connection and applies the schema.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	d, err := sql.Open("sqlite", path+pragmas)
	if err != nil {
		return nil, err
	}
	if err := d.Ping(); err != nil {
		d.Close()
		return nil, err
	}
	if err := migrate(d); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

func migrate(d *sql.DB) error {
	var version int
	if err := d.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	steps := append([]string{schemaSQL}, migrations...)
	if version > len(steps) {
		return fmt.Errorf("banco na versão %d, este binário conhece até %d", version, len(steps))
	}
	for i := version; i < len(steps); i++ {
		tx, err := d.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(steps[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("schema v%d: %w", i+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
```

`journal_mode(WAL)` inside a transaction has no effect, but here it runs in the DSN, when the connection is opened, so it is fine. If the multi-statement `tx.Exec` fails in the driver, split `schema.sql` with `strings.Split(steps[i], ";\n")` and ignore the empty pieces.

- [ ] **Step 5:** `go test ./internal/db/` passes.
- [ ] **Step 6:** commit `feat(db): schema v1, per-connection pragmas and versioning via user_version`

---

## Task 3: `money`

**Files:** `internal/money/money.go`, `internal/money/money_test.go`

- [ ] **Step 1: test**

```go
package money

import (
	"slices"
	"testing"
)

func TestAllocate(t *testing.T) {
	tests := []struct {
		total  int64
		shares []int64
		want   []int64
	}{
		{100, []int64{5000, 5000}, []int64{50, 50}},
		{101, []int64{5000, 5000}, []int64{51, 50}},
		{100, []int64{3334, 3333, 3333}, []int64{34, 33, 33}},
		{1, []int64{3334, 3333, 3333}, []int64{1, 0, 0}},
		{12990, []int64{7000, 3000}, []int64{9093, 3897}},
		{5, []int64{10000}, []int64{5}},
		{0, []int64{5000, 5000}, []int64{0, 0}},
	}
	for _, tc := range tests {
		got := Allocate(tc.total, tc.shares)
		if !slices.Equal(got, tc.want) {
			t.Errorf("Allocate(%d, %v) = %v, want %v", tc.total, tc.shares, got, tc.want)
		}
		var sum int64
		for _, v := range got {
			sum += v
		}
		if sum != tc.total {
			t.Errorf("Allocate(%d, %v) soma %d", tc.total, tc.shares, sum)
		}
	}
}

func TestEqualShares(t *testing.T) {
	for n, want := range map[int][]int64{
		1: {10000},
		2: {5000, 5000},
		3: {3334, 3333, 3333},
		7: {1432, 1428, 1428, 1428, 1428, 1428, 1428},
	} {
		if got := EqualShares(n); !slices.Equal(got, want) {
			t.Errorf("EqualShares(%d) = %v, want %v", n, got, want)
		}
	}
}
```

- [ ] **Step 2:** watch it fail.
- [ ] **Step 3: implementation**

```go
package money

import "sort"

// FullShare is 100% in basis points.
const FullShare = 10000

// Allocate splits total cents proportionally to shares (largest remainder).
// The sum of the result is always total; remainder ties favor the lowest index.
func Allocate(total int64, shares []int64) []int64 {
	out := make([]int64, len(shares))
	var sum int64
	for _, s := range shares {
		sum += s
	}
	if sum == 0 {
		return out
	}
	type rem struct {
		i int
		r int64
	}
	rems := make([]rem, len(shares))
	var allocated int64
	for i, s := range shares {
		out[i] = total * s / sum
		rems[i] = rem{i, total * s % sum}
		allocated += out[i]
	}
	sort.SliceStable(rems, func(a, b int) bool { return rems[a].r > rems[b].r })
	for k := 0; allocated < total; k++ {
		out[rems[k].i]++
		allocated++
	}
	return out
}

// EqualShares splits 100% into n parts; the remainder goes to the first one.
func EqualShares(n int) []int64 {
	if n <= 0 {
		return nil
	}
	out := make([]int64, n)
	base := int64(FullShare / n)
	for i := range out {
		out[i] = base
	}
	out[0] += FullShare - base*int64(n)
	return out
}
```

- [ ] **Step 4:** passes. **Step 5:** commit `feat(money): cent allocation and equal shares in bp`

---

## Task 4: `recurrence.Occurrences`

**Files:** `internal/recurrence/occurrences.go`, `internal/recurrence/occurrences_test.go`

- [ ] **Step 1: test**

```go
package recurrence

import (
	"slices"
	"testing"
)

func dates(occ []Occurrence) []string {
	out := make([]string, len(occ))
	for i, o := range occ {
		out[i] = o.Date
	}
	return out
}

func TestOccurrences(t *testing.T) {
	tests := []struct {
		name                string
		day                 int
		start, end, until   string
		want                []string
	}{
		{"mensal simples", 10, "2026-01-01", "", "2026-03-15",
			[]string{"2026-01-10", "2026-02-10", "2026-03-10"}},
		{"até antes do dia no mês corrente", 20, "2026-01-01", "", "2026-03-15",
			[]string{"2026-01-20", "2026-02-20"}},
		{"início depois do dia pula o 1º mês", 5, "2026-01-10", "", "2026-03-31",
			[]string{"2026-02-05", "2026-03-05"}},
		{"dia 31 limitado ao fim do mês", 31, "2026-01-01", "", "2026-04-30",
			[]string{"2026-01-31", "2026-02-28", "2026-03-31", "2026-04-30"}},
		{"ano bissexto", 30, "2028-02-01", "", "2028-02-29",
			[]string{"2028-02-29"}},
		{"fim inclusivo", 10, "2026-01-01", "2026-02-10", "2026-12-31",
			[]string{"2026-01-10", "2026-02-10"}},
		{"until antes do início", 10, "2026-05-01", "", "2026-04-30", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Occurrences(tc.day, tc.start, tc.end, tc.until)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(dates(got), tc.want) {
				t.Errorf("got %v, want %v", dates(got), tc.want)
			}
			for _, o := range got {
				if o.Month != o.Date[:7] {
					t.Errorf("Month %q não bate com Date %q", o.Month, o.Date)
				}
			}
		})
	}
}

func TestOccurrencesInvalid(t *testing.T) {
	if _, err := Occurrences(10, "2026-13-01", "", "2026-12-31"); err == nil {
		t.Error("start inválido deveria falhar")
	}
	if _, err := Occurrences(0, "2026-01-01", "", "2026-12-31"); err == nil {
		t.Error("dia 0 deveria falhar")
	}
}
```

- [ ] **Step 2:** watch it fail.
- [ ] **Step 3: implementation**

```go
package recurrence

import (
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

type Occurrence struct {
	Month string // YYYY-MM
	Date  string // YYYY-MM-DD
}

// Occurrences lists the monthly dates of a recurrence between start and until (inclusive),
// honoring end (inclusive, empty = no end). The day is clamped to the last day of each month.
func Occurrences(day int, start, end, until string) ([]Occurrence, error) {
	if day < 1 || day > 31 {
		return nil, fmt.Errorf("dia do mês inválido: %d", day)
	}
	s, err := time.Parse(dateLayout, start)
	if err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}
	u, err := time.Parse(dateLayout, until)
	if err != nil {
		return nil, fmt.Errorf("until: %w", err)
	}
	if end != "" {
		e, err := time.Parse(dateLayout, end)
		if err != nil {
			return nil, fmt.Errorf("end: %w", err)
		}
		if e.Before(u) {
			u = e
		}
	}

	var out []Occurrence
	for m := time.Date(s.Year(), s.Month(), 1, 0, 0, 0, 0, time.UTC); !m.After(u); m = m.AddDate(0, 1, 0) {
		last := time.Date(m.Year(), m.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
		date := time.Date(m.Year(), m.Month(), min(day, last), 0, 0, 0, 0, time.UTC)
		if date.Before(s) || date.After(u) {
			continue
		}
		out = append(out, Occurrence{Month: date.Format("2006-01"), Date: date.Format(dateLayout)})
	}
	return out, nil
}
```

- [ ] **Step 4:** passes. **Step 5:** commit `feat(recurrence): pure monthly occurrences`

---

## Task 5: `events`

**Files:** `internal/events/bus.go`, `internal/events/bus_test.go`

- [ ] **Step 1:** copy `src/events/bus.go` and `bus_test.go` to `internal/events/`. Replace the global `Bus` variable with a `New() *Bus` constructor, renaming the `EventBus` type to `Bus`, and adjust the test to use `New()`.
- [ ] **Step 2:** add a test: `Notify` with `actorUserID = 0` (system job) delivers to all subscribers.
- [ ] **Step 3:** `go test ./internal/events/` passes. If the current code already covers this case (it skips only if `s.userID == actorUserID`, and userID is never 0), the test passes without changes.
- [ ] **Step 4:** commit `refactor(events): bus without a global`

---

## Task 6: `api`: HTTP infrastructure and test harness

**Files:** `internal/api/httpx.go`, `internal/api/server.go`, `internal/api/testutil_test.go`

- [ ] **Step 1: `httpx.go`**

```go
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
)

type apiError struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func (e *apiError) Error() string { return e.Code + ": " + e.Message }

var (
	errUnauthenticated = &apiError{Status: 401, Code: "unauthenticated", Message: "Faça login para continuar"}
	errForbidden       = &apiError{Status: 403, Code: "forbidden", Message: "Sem permissão para esta ação"}
	errNotFound        = &apiError{Status: 404, Code: "not_found", Message: "Não encontrado"}
	errRateLimited     = &apiError{Status: 429, Code: "rate_limited", Message: "Muitas tentativas, aguarde alguns minutos"}
)

func errValidation(fields map[string]string) *apiError {
	return &apiError{Status: 400, Code: "validation", Message: "Dados inválidos", Fields: fields}
}

func errBadRequest(msg string) *apiError {
	return &apiError{Status: 400, Code: "validation", Message: msg}
}

func errConflict(msg string) *apiError {
	return &apiError{Status: 409, Code: "conflict", Message: msg}
}

// fieldErrors accumulates validation errors per field.
type fieldErrors map[string]string

func (f fieldErrors) add(field, msg string) {
	if _, ok := f[field]; !ok {
		f[field] = msg
	}
}

func (f fieldErrors) err() error {
	if len(f) == 0 {
		return nil
	}
	return errValidation(f)
}

type handlerFunc func(w http.ResponseWriter, r *http.Request) error

func (s *Server) handle(h handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			s.writeError(w, r, err)
		}
	})
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var ae *apiError
	if errors.As(err, &ae) {
		writeJSON(w, ae.Status, map[string]any{"error": ae})
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 404, map[string]any{"error": errNotFound})
		return
	}
	s.log.ErrorContext(r.Context(), "erro interno", "err", err, "path", r.URL.Path)
	writeJSON(w, 500, map[string]any{"error": &apiError{Code: "internal", Message: "Erro interno"}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// decode requires JSON (a CSRF barrier together with SameSite=Lax), limits the body to 1 MB and rejects unknown fields.
func decode(w http.ResponseWriter, r *http.Request, dst any) error {
	mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mt != "application/json" {
		return &apiError{Status: 415, Code: "validation", Message: "Content-Type deve ser application/json"}
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return errBadRequest("JSON inválido")
	}
	return nil
}

// querier is satisfied by *sql.DB and *sql.Tx.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// inTx runs fn in a transaction (BEGIN IMMEDIATE via the DSN).
func (s *Server) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
```

- [ ] **Step 2: `server.go`**

```go
package api

import (
	"database/sql"
	"io/fs"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"gastos/internal/config"
	"gastos/internal/events"
	"gastos/internal/recurrence"
)

type Server struct {
	db      *sql.DB
	cfg     config.Config
	bus     *events.Bus
	mat     *recurrence.Materializer
	log     *slog.Logger
	limiter *rateLimiter
	now     func() time.Time
}

func New(db *sql.DB, cfg config.Config, bus *events.Bus, mat *recurrence.Materializer, log *slog.Logger) *Server {
	return &Server{db: db, cfg: cfg, bus: bus, mat: mat, log: log, limiter: newRateLimiter(10, 15*time.Minute), now: time.Now}
}

func (s *Server) Routes(spa fs.FS) http.Handler {
	mux := http.NewServeMux()
	public := s.handle
	user := func(h handlerFunc) http.Handler { return s.requireUser(s.handle(h)) }
	wallet := func(min role, h handlerFunc) http.Handler { return s.requireUser(s.requireWallet(min, s.handle(h))) }

	mux.Handle("POST /api/auth/register", public(s.register))
	mux.Handle("POST /api/auth/login", public(s.login))
	mux.Handle("POST /api/auth/logout", user(s.logout))
	mux.Handle("GET /api/me", user(s.me))

	mux.Handle("GET /api/wallets", user(s.listWallets))
	mux.Handle("POST /api/wallets", user(s.createWallet))
	mux.Handle("PATCH /api/wallets/{walletID}", wallet(roleOwner, s.updateWallet))
	mux.Handle("DELETE /api/wallets/{walletID}", wallet(roleOwner, s.deleteWallet))
	mux.Handle("GET /api/wallets/{walletID}/members", wallet(roleReader, s.listMembers))
	mux.Handle("POST /api/wallets/{walletID}/members", wallet(roleOwner, s.addMember))
	mux.Handle("PATCH /api/wallets/{walletID}/members/{userID}", wallet(roleOwner, s.updateMember))
	mux.Handle("DELETE /api/wallets/{walletID}/members/{userID}", wallet(roleReader, s.removeMember)) // owner or the user themselves: checked in the handler

	mux.Handle("GET /api/wallets/{walletID}/categories", wallet(roleReader, s.listCategories))
	mux.Handle("POST /api/wallets/{walletID}/categories", wallet(roleEditor, s.createCategory))
	mux.Handle("PUT /api/wallets/{walletID}/categories/{id}", wallet(roleEditor, s.updateCategory))
	mux.Handle("DELETE /api/wallets/{walletID}/categories/{id}", wallet(roleEditor, s.deleteCategory))
	mux.Handle("GET /api/wallets/{walletID}/payment-methods", wallet(roleReader, s.listPaymentMethods))
	mux.Handle("POST /api/wallets/{walletID}/payment-methods", wallet(roleEditor, s.createPaymentMethod))
	mux.Handle("PUT /api/wallets/{walletID}/payment-methods/{id}", wallet(roleEditor, s.updatePaymentMethod))
	mux.Handle("DELETE /api/wallets/{walletID}/payment-methods/{id}", wallet(roleEditor, s.deletePaymentMethod))

	mux.Handle("GET /api/wallets/{walletID}/transactions", wallet(roleReader, s.listTransactions))
	mux.Handle("POST /api/wallets/{walletID}/transactions", wallet(roleEditor, s.createTransaction))
	mux.Handle("PUT /api/wallets/{walletID}/transactions/{id}", wallet(roleEditor, s.updateTransaction))
	mux.Handle("DELETE /api/wallets/{walletID}/transactions/{id}", wallet(roleEditor, s.deleteTransaction))

	mux.Handle("GET /api/wallets/{walletID}/recurrences", wallet(roleReader, s.listRecurrences))
	mux.Handle("POST /api/wallets/{walletID}/recurrences", wallet(roleEditor, s.createRecurrence))
	mux.Handle("PUT /api/wallets/{walletID}/recurrences/{id}", wallet(roleEditor, s.updateRecurrence))
	mux.Handle("DELETE /api/wallets/{walletID}/recurrences/{id}", wallet(roleEditor, s.deleteRecurrence))

	mux.Handle("GET /api/wallets/{walletID}/settlements", wallet(roleReader, s.listSettlements))
	mux.Handle("POST /api/wallets/{walletID}/settlements", wallet(roleEditor, s.createSettlement))
	mux.Handle("PUT /api/wallets/{walletID}/settlements/{id}", wallet(roleEditor, s.updateSettlement))
	mux.Handle("DELETE /api/wallets/{walletID}/settlements/{id}", wallet(roleEditor, s.deleteSettlement))

	mux.Handle("GET /api/wallets/{walletID}/budgets", wallet(roleReader, s.listBudgets))
	mux.Handle("PUT /api/wallets/{walletID}/budgets/{categoryID}", wallet(roleEditor, s.putBudget))
	mux.Handle("DELETE /api/wallets/{walletID}/budgets/{categoryID}", wallet(roleEditor, s.deleteBudget))

	mux.Handle("GET /api/wallets/{walletID}/summary", wallet(roleReader, s.summary))
	mux.Handle("GET /api/wallets/{walletID}/events", wallet(roleReader, s.events))

	mux.HandleFunc("GET /healthz", s.healthz)
	mux.Handle("/api/", public(func(http.ResponseWriter, *http.Request) error { return errNotFound }))
	mux.Handle("/", spaHandler(spa))

	return s.logRequests(s.recoverPanic(s.devCORS(mux)))
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if err := s.db.PingContext(r.Context()); err != nil {
		http.Error(w, "db indisponível", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ok"))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) { r.status = code; r.ResponseWriter.WriteHeader(code) }
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter } // keeps Flusher via ResponseController

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := s.now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		s.log.InfoContext(r.Context(), "http", "method", r.Method, "route", r.Pattern, "status", rec.status,
			"ms", s.now().Sub(start).Milliseconds(), "user", userIDFrom(r.Context()))
	})
}

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.log.ErrorContext(r.Context(), "panic", "value", v, "stack", string(debug.Stack()))
				writeJSON(w, 500, map[string]any{"error": &apiError{Code: "internal", Message: "Erro interno"}})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// devCORS allows Vite on localhost:5173 only with GASTOS_DEV=true.
func (s *Server) devCORS(next http.Handler) http.Handler {
	if !s.cfg.Dev {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") == "http://localhost:5173" {
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
```

`r.Pattern` is filled in by the mux (Go 1.23+), but `logRequests` wraps the mux: after the inner `ServeHTTP`, the outer `r` does **not** have the pattern. For the log, use `r.URL.Path` or keep the pattern in a pointer in the context inside `requireUser`. The simple solution is to log `r.URL.Path` and accept IDs in the log.

- [ ] **Step 3: test harness (`testutil_test.go`)**

```go
package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"gastos/internal/config"
	"gastos/internal/db"
	"gastos/internal/events"
	"gastos/internal/recurrence"
)

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
	walletID int64 // "Pessoal" (Personal) wallet created at registration
}

func (a *testApp) anon() *client {
	jar, _ := cookiejar.New(nil)
	return &client{app: a, http: &http.Client{Jar: jar}}
}

// register creates a user and stores the cookie and the personal wallet.
func (a *testApp) register(name, email string) *client {
	a.t.Helper()
	c := a.anon()
	c.do("POST", "/api/auth/register", map[string]string{"name": name, "email": email, "password": "senha-segura"}, 201, nil)
	var me struct {
		User    json.RawMessage `json:"user"`
		Wallets []struct{ ID int64 `json:"id"` } `json:"wallets"`
	}
	c.do("GET", "/api/me", nil, 200, &me)
	json.Unmarshal(me.User, &c.user)
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

func (c *client) wpath(format string, args ...any) string {
	return fmt.Sprintf("/api/wallets/%d", c.walletID) + fmt.Sprintf(format, args...)
}
```

Add `"fmt"` to the imports.

The referenced functions (`requireUser`, `role`, etc.) only exist from task 7 on. This step only compiles together with task 7. A single commit for both.

---

## Task 7: auth (sessions, register, login, logout, me)

**Files:** `internal/api/auth.go`, `internal/api/auth_test.go`

- [ ] **Step 1: tests**

```go
func TestAuthFlow(t *testing.T) {
	app := newTestApp(t)
	c := app.register("Ana", "ana@example.com")
	if c.user.ID == 0 || c.walletID == 0 {
		t.Fatalf("registro sem user/carteira: %+v", c)
	}

	c.do("POST", "/api/auth/logout", nil, 204, nil)
	c.do("GET", "/api/me", nil, 401, nil)

	c.do("POST", "/api/auth/login", map[string]string{"email": "ANA@example.com", "password": "senha-segura"}, 200, nil)
	c.do("GET", "/api/me", nil, 200, nil)
}

func TestRegisterValidation(t *testing.T) {
	app := newTestApp(t)
	c := app.anon()
	c.do("POST", "/api/auth/register", map[string]string{"name": "", "email": "x", "password": "123"}, 400, nil)
	app.register("Ana", "ana@example.com")
	c.do("POST", "/api/auth/register", map[string]string{"name": "Outra", "email": "ana@example.com", "password": "senha-segura"}, 409, nil)
}

func TestLoginWrongPassword(t *testing.T) {
	app := newTestApp(t)
	app.register("Ana", "ana@example.com")
	app.anon().do("POST", "/api/auth/login", map[string]string{"email": "ana@example.com", "password": "errada!!"}, 401, nil)
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
	if res.StatusCode != 415 {
		t.Fatalf("status %d, want 415", res.StatusCode)
	}
}

func TestUnknownAPIRouteIs404JSON(t *testing.T) {
	app := newTestApp(t)
	app.anon().do("GET", "/api/nao-existe", nil, 404, nil)
}
```

- [ ] **Step 2:** watch it fail.
- [ ] **Step 3: implementation**

```go
package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookie = "gastos_session"
	sessionTTL    = 30 * 24 * time.Hour
	sessionRenew  = 15 * 24 * time.Hour
	tsLayout      = "2006-01-02T15:04:05Z"
)

type ctxKey int

const (
	ctxUserID ctxKey = iota
	ctxWalletID
	ctxRole
)

func userIDFrom(ctx context.Context) int64 { v, _ := ctx.Value(ctxUserID).(int64); return v }

type userDTO struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	fe := fieldErrors{}
	if in.Name == "" {
		fe.add("name", "obrigatório")
	}
	if _, err := mail.ParseAddress(in.Email); err != nil {
		fe.add("email", "e-mail inválido")
	}
	if len(in.Password) < 8 {
		fe.add("password", "mínimo de 8 caracteres")
	}
	if err := fe.err(); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), 12)
	if err != nil {
		return err
	}

	var userID int64
	err = s.inTx(r.Context(), func(tx *sql.Tx) error {
		err := tx.QueryRowContext(r.Context(),
			`INSERT INTO users (name, email, password_hash) VALUES (?, ?, ?) RETURNING id`,
			in.Name, in.Email, string(hash)).Scan(&userID)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				return errConflict("E-mail já cadastrado")
			}
			return err
		}
		_, err = createWallet(r.Context(), tx, "Pessoal", userID, false)
		return err
	})
	if err != nil {
		return err
	}
	if err := s.startSession(r.Context(), w, userID); err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, userDTO{userID, in.Name, in.Email})
	return nil
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !s.limiter.allow("ip:"+clientIP(r)) || !s.limiter.allow("email:"+in.Email) {
		return errRateLimited
	}
	var u userDTO
	var hash string
	err := s.db.QueryRowContext(r.Context(),
		`SELECT id, name, email, password_hash FROM users WHERE email = ?`, in.Email).
		Scan(&u.ID, &u.Name, &u.Email, &hash)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil) {
		return &apiError{Status: 401, Code: "unauthenticated", Message: "E-mail ou senha incorretos"}
	}
	if err != nil {
		return err
	}
	if err := s.startSession(r.Context(), w, u.ID); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, u)
	return nil
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	if c, err := r.Cookie(sessionCookie); err == nil {
		h := sha256.Sum256([]byte(c.Value))
		if _, err := s.db.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash = ?`, h[:]); err != nil {
			return err
		}
	}
	s.setSessionCookie(w, "", time.Unix(0, 0))
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) error {
	var u userDTO
	if err := s.db.QueryRowContext(r.Context(), `SELECT id, name, email FROM users WHERE id = ?`,
		userIDFrom(r.Context())).Scan(&u.ID, &u.Name, &u.Email); err != nil {
		return err
	}
	wallets, err := s.walletsFor(r.Context(), u.ID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u, "wallets": wallets})
	return nil
}

func (s *Server) startSession(ctx context.Context, w http.ResponseWriter, userID int64) error {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(token))
	exp := s.now().UTC().Add(sessionTTL)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		h[:], userID, exp.Format(tsLayout)); err != nil {
		return err
	}
	s.setSessionCookie(w, token, exp)
	return nil
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string, exp time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", Expires: exp,
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			s.writeError(w, r, errUnauthenticated)
			return
		}
		h := sha256.Sum256([]byte(c.Value))
		now := s.now().UTC()
		var userID int64
		var expStr string
		err = s.db.QueryRowContext(r.Context(),
			`SELECT user_id, expires_at FROM sessions WHERE token_hash = ? AND expires_at > ?`,
			h[:], now.Format(tsLayout)).Scan(&userID, &expStr)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				err = errUnauthenticated
			}
			s.writeError(w, r, err)
			return
		}
		if exp, _ := time.Parse(tsLayout, expStr); exp.Sub(now) < sessionRenew {
			newExp := now.Add(sessionTTL)
			if _, err := s.db.ExecContext(r.Context(), `UPDATE sessions SET expires_at = ? WHERE token_hash = ?`,
				newExp.Format(tsLayout), h[:]); err == nil {
				s.setSessionCookie(w, c.Value, newExp)
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxUserID, userID)))
	})
}

func clientIP(r *http.Request) string {
	// Behind Caddy/nginx on localhost: trust X-Forwarded-For only if the peer is loopback.
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
	}
	return host
}

type rateLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string][]time.Time
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{max: max, window: window, hits: map[string][]time.Time{}}
}

// allow records an attempt and says whether it is still within the limit.
func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}
```

The `TestLoginRateLimit` test runs from the same IP (`127.0.0.1`, loopback without XFF). The IP key and the email key overflow together on the 11th attempt.

- [ ] **Step 4:** `go test ./internal/api/ -run 'Auth|Register|Login|ContentType|Unknown'` passes. This only compiles after task 8, which defines `createWallet`, `walletsFor`, `role` and `requireWallet`. Do tasks 7 and 8 together and run the tests at the end of 8.
- [ ] **Step 5:** commit `feat(api): cookie sessions, register/login/logout/me`

---

## Task 8: wallets, members, `requireWallet`, seed

**Files:** `internal/api/wallets.go`, `internal/api/wallets_test.go`

**Behavior:**

- `type role int` with `roleReader < roleEditor < roleOwner`, plus `parseRole(string) (role, bool)` and `String()`.
- `requireWallet(min role, next)`:
  - reads `r.PathValue("walletID")`;
  - looks up the `role` in `wallet_members`;
  - not a member: **404**, so it does not reveal that the wallet exists;
  - role below the minimum: **403**;
  - puts `ctxWalletID` and `ctxRole` in the context.
- `createWallet(ctx, q querier, name, ownerID, splitting) (int64, error)` inserts the wallet and the owner, and calls `seedWallet`.
- `seedWallet` inserts:
  - the 15 expense categories from `src/db/db.go:117-131`, with the same names, icons, colors and essentiality, but without `key`;
  - the 5 income categories: Salário (Salary) 💼 `#0f6579`, Freelance 🧑‍💻 `#1f7d8f`, Investimentos (Investments) 📈 `#f39c12`, Reembolso (Reimbursement) ↩️ `#2b92a1`, Outros (Other) ◈ `#7f9ea5`;
  - the 8 payment methods from `src/db/db.go:146-153`.
- Wallet DTO:

  ```go
  type walletDTO struct {
  	ID               int64  `json:"id"`
  	Name             string `json:"name"`
  	Role             string `json:"role"`
  	SplittingEnabled bool   `json:"splittingEnabled"`
  	OwnerName        string `json:"ownerName"`
  }
  ```
- `walletsFor(ctx, userID)` runs one query with a JOIN on the owner, ordered by `w.created_at, w.id`.
- `PATCH /wallets/{id}` accepts `{name?: string, splittingEnabled?: bool}` with pointers. Enabling splitting does not change old transactions.
- `DELETE /wallets/{id}`: owner. The CASCADE cleans everything. It notifies before deleting, so the other members leave the screen.
- Members:
  - `GET` returns `[{userId, name, email, role, splitsByDefault}]` ordered by owner, editor, reader and name.
  - `POST {email, role}`: `role` must be `editor` or `reader`. A nonexistent user gives 404 with the message "Usuário não encontrado" (User not found). Someone who is already a member gives 409.
  - `PATCH .../members/{userID}` accepts `{role?, splitsByDefault?}`. It cannot change the owner (400).
  - `DELETE .../members/{userID}`: allowed if `role == owner` or `userID == me`. The owner cannot leave or be removed (400).
- Every successful mutation calls `s.mutated(r)`:

  ```go
  func (s *Server) mutated(r *http.Request) {
  	s.bus.Notify(walletIDFrom(r.Context()), userIDFrom(r.Context()))
  }
  ```

- [ ] **Step 1: tests** (`wallets_test.go`)
  - `TestRegisterSeedsWallet`: the categories GET returns 20 items (15 expense and 5 income), and the payment-methods one returns 8.
  - `TestWalletIsolation`: Bia reading `/api/wallets/{Ana's wallet}/transactions?...` gets **404**.
  - `TestRoles`:
    - Ana adds Bia as `reader`. Bia does a categories GET (200) and a category POST (403).
    - Ana promotes Bia to `editor`. Bia's category POST goes through (201), but the wallet PATCH is still 403.
  - `TestMemberLeave`: Bia leaves on their own (`DELETE .../members/{bia}`, 204). Ana cannot leave (400).
  - `TestAddMemberErrors`: nonexistent email gives 404; duplicate member gives 409; `role: "owner"` gives 400.
  - `TestDeleteWallet`: Ana creates the wallet "Casa" (Home), posts a transaction and deletes the wallet (204). The wallets GET no longer shows "Casa".
- [ ] **Step 2:** watch it fail. **Step 3:** implement. **Step 4:** `go test ./internal/api/` passes, including the task 7 tests.
- [ ] **Step 5:** commit `feat(api): wallets, members and roles`

---

## Task 9: categories and payment methods

**Files:** `internal/api/catalog.go`, `internal/api/catalog_test.go`

**Behavior:**

- Category:

  ```go
  type categoryDTO struct {
  	ID           int64   `json:"id"`
  	Kind         string  `json:"kind"`
  	Name         string  `json:"name"`
  	Icon         string  `json:"icon"`
  	Color        string  `json:"color"`
  	Essentiality *string `json:"essentiality"`
  	SortOrder    int     `json:"sortOrder"`
  	Archived     bool    `json:"archived"`
  }
  ```
- The GET returns all of them, including archived ones (the front end filters), ordered by `kind, sort_order, id`.
- POST and PUT validate:
  - `kind` (cannot change in PUT);
  - `name` not empty, at most 40 characters;
  - `icon` not empty;
  - `color` in the format `^#[0-9a-fA-F]{6}$`;
  - `essentiality` required if `kind == expense` and forbidden if `income`;
  - duplicate name gives 409.

  POST uses `sort_order = max + 1`. PUT accepts `sortOrder`, for reordering.
- DELETE:
  - if `transactions`, `recurrences` or `budgets` reference the category, runs `UPDATE ... SET archived_at = now` and responds 200 with the DTO (`archived: true`);
  - if there are no references, runs `DELETE` and responds 204;
  - it does not allow leaving the wallet without any active category of the same `kind` (400).
- PUT on an archived category with `"archived": false` unarchives it. That is why the `archived` field is also accepted in the PUT input.
- Payment methods follow the same rules, without `kind`, `color` and `essentiality`.
- Every mutation calls `s.mutated(r)`.

- [ ] **Step 1: tests**
  - happy-path category CRUD;
  - invalid color gives 400 with `fields.color`;
  - `income` with `essentiality` gives 400;
  - duplicate name gives 409;
  - category in use: DELETE responds 200 and `archived:true`, and the transaction keeps that `categoryId`;
  - unused category: DELETE responds 204;
  - deleting the last active income category gives 400;
  - the same cases, adapted, for payment methods.
- [ ] **Step 2–4:** watch it fail, implement, watch it pass.
- [ ] **Step 5:** commit `feat(api): categories and payment methods with archiving`

---

## Task 10: transactions with splits

This is the reference task: the handlers in tasks 11–13 follow the same shape.

**Files:** `internal/api/transactions.go`, `internal/api/transactions_test.go`

- [ ] **Step 1: tests**

```go
func TestTransactionCRUD(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	cat := firstCategory(t, ana, "expense")

	var created transaction
	ana.do("POST", ana.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 12990, "description": "Mercado",
		"categoryId": cat, "date": "2026-10-09",
	}, 201, &created)
	if created.PaidByUserID != ana.user.ID || created.CreatedBy != ana.user.ID || len(created.Splits) != 0 {
		t.Fatalf("criado: %+v", created)
	}

	var list []transaction
	ana.do("GET", ana.wpath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 1 || list[0].AmountCents != 12990 {
		t.Fatalf("lista: %+v", list)
	}

	ana.do("PUT", ana.wpath("/transactions/%d", created.ID), map[string]any{
		"kind": "expense", "amountCents": 5000, "description": "Feira",
		"categoryId": cat, "date": "2026-10-08",
	}, 200, &created)
	if created.AmountCents != 5000 || created.Description != "Feira" {
		t.Fatalf("atualizado: %+v", created)
	}

	ana.do("DELETE", ana.wpath("/transactions/%d", created.ID), nil, 204, nil)
	ana.do("GET", ana.wpath("/transactions?from=2026-10-01&to=2026-10-31"), nil, 200, &list)
	if len(list) != 0 {
		t.Fatalf("deveria estar vazia: %+v", list)
	}
}

func TestTransactionValidation(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	incomeCat := firstCategory(t, ana, "income")
	var res struct{ Error apiError `json:"error"` }
	ana.do("POST", ana.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 0, "description": "", "categoryId": incomeCat, "date": "2026-02-30",
	}, 400, &res)
	for _, f := range []string{"amountCents", "description", "categoryId", "date"} {
		if res.Error.Fields[f] == "" {
			t.Errorf("faltou erro em %s: %+v", f, res.Error.Fields)
		}
	}
	ana.do("GET", ana.wpath("/transactions"), nil, 400, nil) // from/to required
}

func TestEditorKeepsCreatorAndPayer(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	ana.do("POST", ana.wpath("/members"), map[string]string{"email": "bia@example.com", "role": "editor"}, 201, nil)
	bia.walletID = ana.walletID
	cat := firstCategory(t, ana, "expense")

	var tx transaction
	ana.do("POST", ana.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "Luz", "categoryId": cat, "date": "2026-10-01",
	}, 201, &tx)
	bia.do("PUT", bia.wpath("/transactions/%d", tx.ID), map[string]any{
		"kind": "expense", "amountCents": 1100, "description": "Luz", "categoryId": cat, "date": "2026-10-01",
		"paidByUserId": ana.user.ID,
	}, 200, &tx)
	if tx.CreatedBy != ana.user.ID || tx.PaidByUserID != ana.user.ID {
		t.Fatalf("edição da Bia mudou autoria/pagador: %+v", tx)
	}
}

func TestSplits(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	app.register("Bia", "bia@example.com")
	ana.do("POST", ana.wpath("/members"), map[string]string{"email": "bia@example.com", "role": "editor"}, 201, nil)
	cat := firstCategory(t, ana, "expense")
	body := func(extra map[string]any) map[string]any {
		b := map[string]any{"kind": "expense", "amountCents": 101, "description": "Pizza", "categoryId": cat, "date": "2026-10-01"}
		for k, v := range extra {
			b[k] = v
		}
		return b
	}

	// splitting off: splits forbidden
	ana.do("POST", ana.wpath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": ana.user.ID, "shareBp": 10000}},
	}), 400, nil)

	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, nil)

	// no splits: equal among members with splitsByDefault
	var tx transaction
	ana.do("POST", ana.wpath("/transactions"), body(nil), 201, &tx)
	if len(tx.Splits) != 2 || tx.Splits[0].AmountCents+tx.Splits[1].AmountCents != 101 {
		t.Fatalf("split padrão: %+v", tx.Splits)
	}

	// sum different from 10000
	ana.do("POST", ana.wpath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": ana.user.ID, "shareBp": 6000}, {"userId": tx.Splits[1].UserID, "shareBp": 3000}},
	}), 400, nil)

	// not a member
	ana.do("POST", ana.wpath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": 9999, "shareBp": 10000}},
	}), 400, nil)
}

func firstCategory(t *testing.T, c *client, kind string) int64 {
	t.Helper()
	var cats []struct {
		ID   int64  `json:"id"`
		Kind string `json:"kind"`
	}
	c.do("GET", c.wpath("/categories"), nil, 200, &cats)
	for _, cat := range cats {
		if cat.Kind == kind {
			return cat.ID
		}
	}
	t.Fatalf("sem categoria %s", kind)
	return 0
}
```

- [ ] **Step 2:** watch it fail.
- [ ] **Step 3: implementation**

```go
package api

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gastos/internal/money"
)

const dateLayout = "2006-01-02"

type split struct {
	UserID      int64 `json:"userId"`
	ShareBP     int64 `json:"shareBp"`
	AmountCents int64 `json:"amountCents"`
}

type transaction struct {
	ID              int64   `json:"id"`
	Kind            string  `json:"kind"`
	AmountCents     int64   `json:"amountCents"`
	Description     string  `json:"description"`
	CategoryID      int64   `json:"categoryId"`
	PaymentMethodID *int64  `json:"paymentMethodId"`
	Date            string  `json:"date"`
	PaidByUserID    int64   `json:"paidByUserId"`
	CreatedBy       int64   `json:"createdBy"`
	RecurrenceID    *int64  `json:"recurrenceId"`
	Splits          []split `json:"splits"`
}

type transactionInput struct {
	Kind            string  `json:"kind"`
	AmountCents     int64   `json:"amountCents"`
	Description     string  `json:"description"`
	CategoryID      int64   `json:"categoryId"`
	PaymentMethodID *int64  `json:"paymentMethodId"`
	Date            string  `json:"date"`
	PaidByUserID    int64   `json:"paidByUserId"`
	Splits          []split `json:"splits"`
}

func walletIDFrom(ctx context.Context) int64 { v, _ := ctx.Value(ctxWalletID).(int64); return v }

func pathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, errNotFound
	}
	return id, nil
}

func validDate(s string) bool { _, err := time.Parse(dateLayout, s); return err == nil }

// validate normalizes the input and checks references within the wallet.
// currentCategoryID allows keeping an archived category that was already the transaction's (0 on create).
func (in *transactionInput) validate(ctx context.Context, q querier, walletID, userID, currentCategoryID int64) error {
	in.Description = strings.TrimSpace(in.Description)
	if in.PaidByUserID == 0 {
		in.PaidByUserID = userID
	}
	fe := fieldErrors{}
	if in.Kind != "expense" && in.Kind != "income" {
		fe.add("kind", "deve ser expense ou income")
	}
	if in.AmountCents <= 0 {
		fe.add("amountCents", "deve ser maior que zero")
	}
	if in.Description == "" || len(in.Description) > 200 {
		fe.add("description", "obrigatória, até 200 caracteres")
	}
	if !validDate(in.Date) {
		fe.add("date", "data inválida")
	}

	var catKind string
	var archived bool
	err := q.QueryRowContext(ctx, `SELECT kind, archived_at IS NOT NULL FROM categories WHERE id = ? AND wallet_id = ?`,
		in.CategoryID, walletID).Scan(&catKind, &archived)
	switch {
	case err == sql.ErrNoRows:
		fe.add("categoryId", "categoria não encontrada")
	case err != nil:
		return err
	case catKind != in.Kind:
		fe.add("categoryId", "categoria não é do tipo "+in.Kind)
	case archived && in.CategoryID != currentCategoryID:
		fe.add("categoryId", "categoria arquivada")
	}

	if in.PaymentMethodID != nil {
		var ok bool
		if err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM payment_methods WHERE id = ? AND wallet_id = ?)`,
			*in.PaymentMethodID, walletID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			fe.add("paymentMethodId", "forma de pagamento não encontrada")
		}
	}

	var isMember bool
	if err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM wallet_members WHERE wallet_id = ? AND user_id = ?)`,
		walletID, in.PaidByUserID).Scan(&isMember); err != nil {
		return err
	}
	if !isMember {
		fe.add("paidByUserId", "não é membro da carteira")
	}

	if err := fe.err(); err != nil {
		return err
	}
	splits, err := resolveSplits(ctx, q, walletID, in.Kind, in.Splits)
	if err != nil {
		return err
	}
	in.Splits = splits
	return nil
}

// resolveSplits applies the split rules from the spec and returns the final splits (without amountCents).
func resolveSplits(ctx context.Context, q querier, walletID int64, kind string, requested []split) ([]split, error) {
	var splitting bool
	if err := q.QueryRowContext(ctx, `SELECT splitting_enabled FROM wallets WHERE id = ?`, walletID).Scan(&splitting); err != nil {
		return nil, err
	}
	if !splitting || kind != "expense" {
		if len(requested) > 0 {
			return nil, errValidation(map[string]string{"splits": "divisão não se aplica"})
		}
		return nil, nil
	}

	rows, err := q.QueryContext(ctx, `SELECT user_id, splits_by_default FROM wallet_members WHERE wallet_id = ? ORDER BY user_id`, walletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := map[int64]bool{}
	var defaults []int64
	for rows.Next() {
		var id int64
		var byDefault bool
		if err := rows.Scan(&id, &byDefault); err != nil {
			return nil, err
		}
		members[id] = true
		if byDefault {
			defaults = append(defaults, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(requested) == 0 {
		if len(defaults) == 0 {
			return nil, nil
		}
		shares := money.EqualShares(len(defaults))
		out := make([]split, len(defaults))
		for i, id := range defaults {
			out[i] = split{UserID: id, ShareBP: shares[i]}
		}
		return out, nil
	}

	seen := map[int64]bool{}
	var total int64
	for _, sp := range requested {
		switch {
		case !members[sp.UserID]:
			return nil, errValidation(map[string]string{"splits": "participante não é membro da carteira"})
		case seen[sp.UserID]:
			return nil, errValidation(map[string]string{"splits": "participante repetido"})
		case sp.ShareBP < 1 || sp.ShareBP > money.FullShare:
			return nil, errValidation(map[string]string{"splits": "parte deve estar entre 0,01% e 100%"})
		}
		seen[sp.UserID] = true
		total += sp.ShareBP
	}
	if total != money.FullShare {
		return nil, errValidation(map[string]string{"splits": "as partes devem somar 100%"})
	}
	out := make([]split, len(requested))
	for i, sp := range requested {
		out[i] = split{UserID: sp.UserID, ShareBP: sp.ShareBP}
	}
	return out, nil
}

func withAmounts(total int64, splits []split) []split {
	shares := make([]int64, len(splits))
	for i, sp := range splits {
		shares[i] = sp.ShareBP
	}
	amounts := money.Allocate(total, shares)
	out := make([]split, len(splits))
	for i, sp := range splits {
		out[i] = split{UserID: sp.UserID, ShareBP: sp.ShareBP, AmountCents: amounts[i]}
	}
	return out
}

func replaceTransactionSplits(ctx context.Context, q querier, txID int64, splits []split) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM transaction_splits WHERE transaction_id = ?`, txID); err != nil {
		return err
	}
	for _, sp := range splits {
		if _, err := q.ExecContext(ctx, `INSERT INTO transaction_splits (transaction_id, user_id, share_bp) VALUES (?, ?, ?)`,
			txID, sp.UserID, sp.ShareBP); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) listTransactions(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID := r.Context(), walletIDFrom(r.Context())
	qs := r.URL.Query()
	from, to := qs.Get("from"), qs.Get("to")
	fe := fieldErrors{}
	if !validDate(from) {
		fe.add("from", "obrigatório (YYYY-MM-DD)")
	}
	if !validDate(to) {
		fe.add("to", "obrigatório (YYYY-MM-DD)")
	}
	if fe.err() == nil {
		f, _ := time.Parse(dateLayout, from)
		t, _ := time.Parse(dateLayout, to)
		if t.Before(f) || t.Sub(f) > 366*24*time.Hour {
			fe.add("to", "intervalo inválido (máx. 366 dias)")
		}
	}
	if err := fe.err(); err != nil {
		return err
	}

	query := `SELECT id, kind, amount_cents, description, category_id, payment_method_id, date, paid_by_user_id, created_by, recurrence_id
	          FROM transactions WHERE wallet_id = ? AND date BETWEEN ? AND ?`
	args := []any{walletID, from, to}
	if k := qs.Get("kind"); k != "" {
		query += ` AND kind = ?`
		args = append(args, k)
	}
	if c := qs.Get("categoryId"); c != "" {
		query += ` AND category_id = ?`
		args = append(args, c)
	}
	if term := strings.TrimSpace(qs.Get("q")); term != "" {
		query += ` AND description LIKE '%' || ? || '%'`
		args = append(args, term)
	}
	query += ` ORDER BY date DESC, id DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	list := []transaction{}
	index := map[int64]int{}
	for rows.Next() {
		var t transaction
		if err := rows.Scan(&t.ID, &t.Kind, &t.AmountCents, &t.Description, &t.CategoryID, &t.PaymentMethodID,
			&t.Date, &t.PaidByUserID, &t.CreatedBy, &t.RecurrenceID); err != nil {
			return err
		}
		t.Splits = []split{}
		index[t.ID] = len(list)
		list = append(list, t)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := s.attachSplits(ctx, list, index); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, list)
	return nil
}

// attachSplits loads the splits of all transactions in the list with one query.
func (s *Server) attachSplits(ctx context.Context, list []transaction, index map[int64]int) error {
	if len(list) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(list))
	args := make([]any, len(list))
	for i, t := range list {
		args[i] = t.ID
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT transaction_id, user_id, share_bp FROM transaction_splits
		 WHERE transaction_id IN (`+placeholders[:len(placeholders)-1]+`) ORDER BY transaction_id, user_id`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var txID int64
		var sp split
		if err := rows.Scan(&txID, &sp.UserID, &sp.ShareBP); err != nil {
			return err
		}
		i := index[txID]
		list[i].Splits = append(list[i].Splits, sp)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range list {
		list[i].Splits = withAmounts(list[i].AmountCents, list[i].Splits)
	}
	return nil
}
```

An `IN (...)` with many ids can exceed SQLite's parameter limit (32766 in modernc), but the 366-day maximum range keeps the volume well below that.

```go
func (s *Server) createTransaction(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), walletIDFrom(r.Context()), userIDFrom(r.Context())
	var in transactionInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	var out transaction
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if err := in.validate(ctx, tx, walletID, userID, 0); err != nil {
			return err
		}
		err := tx.QueryRowContext(ctx, `
			INSERT INTO transactions (wallet_id, kind, amount_cents, description, category_id, payment_method_id, date, paid_by_user_id, created_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			walletID, in.Kind, in.AmountCents, in.Description, in.CategoryID, in.PaymentMethodID, in.Date, in.PaidByUserID, userID,
		).Scan(&out.ID)
		if err != nil {
			return err
		}
		return replaceTransactionSplits(ctx, tx, out.ID, in.Splits)
	})
	if err != nil {
		return err
	}
	out = transaction{ID: out.ID, Kind: in.Kind, AmountCents: in.AmountCents, Description: in.Description,
		CategoryID: in.CategoryID, PaymentMethodID: in.PaymentMethodID, Date: in.Date,
		PaidByUserID: in.PaidByUserID, CreatedBy: userID, Splits: withAmounts(in.AmountCents, in.Splits)}
	s.mutated(r)
	writeJSON(w, http.StatusCreated, out)
	return nil
}

func (s *Server) updateTransaction(w http.ResponseWriter, r *http.Request) error {
	ctx, walletID, userID := r.Context(), walletIDFrom(r.Context()), userIDFrom(r.Context())
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in transactionInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	var out transaction
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		var currentCat int64
		if err := tx.QueryRowContext(ctx,
			`SELECT category_id, created_by, recurrence_id FROM transactions WHERE id = ? AND wallet_id = ?`, id, walletID,
		).Scan(&currentCat, &out.CreatedBy, &out.RecurrenceID); err != nil {
			return err // sql.ErrNoRows → 404 via writeError
		}
		if err := in.validate(ctx, tx, walletID, userID, currentCat); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE transactions SET kind = ?, amount_cents = ?, description = ?, category_id = ?, payment_method_id = ?,
			       date = ?, paid_by_user_id = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
			WHERE id = ? AND wallet_id = ?`,
			in.Kind, in.AmountCents, in.Description, in.CategoryID, in.PaymentMethodID, in.Date, in.PaidByUserID, id, walletID,
		); err != nil {
			return err
		}
		return replaceTransactionSplits(ctx, tx, id, in.Splits)
	})
	if err != nil {
		return err
	}
	out = transaction{ID: id, Kind: in.Kind, AmountCents: in.AmountCents, Description: in.Description,
		CategoryID: in.CategoryID, PaymentMethodID: in.PaymentMethodID, Date: in.Date, PaidByUserID: in.PaidByUserID,
		CreatedBy: out.CreatedBy, RecurrenceID: out.RecurrenceID, Splits: withAmounts(in.AmountCents, in.Splits)}
	s.mutated(r)
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) deleteTransaction(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(r.Context(), `DELETE FROM transactions WHERE id = ? AND wallet_id = ?`, id, walletIDFrom(r.Context()))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errNotFound
	}
	s.mutated(r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
```

`validate` runs inside `inTx`. With `_txlock=immediate`, the whole transaction holds the write lock, which is acceptable for a personal app.

- [ ] **Step 4:** `go test ./internal/api/ -run Transaction|Splits|Editor` passes.
- [ ] **Step 5: float check**

```bash
grep -rn "float" internal/api/ internal/money/ internal/recurrence/
```

The expected output is empty.

- [ ] **Step 6:** commit `feat(api): transactions with splits in basis points`

---

## Task 11: recurrences and materializer

**Files:** `internal/recurrence/materializer.go`, `internal/recurrence/materializer_test.go`, `internal/api/recurrences.go`, `internal/api/recurrences_test.go`

**Materializer:**

```go
package recurrence

type Materializer struct {
	db  *sql.DB
	bus *events.Bus
	loc *time.Location
	log *slog.Logger
	now func() time.Time
}

func NewMaterializer(db *sql.DB, bus *events.Bus, loc *time.Location, log *slog.Logger) *Materializer {
	return &Materializer{db: db, bus: bus, loc: loc, log: log, now: time.Now}
}

// Run materializes occurrences due up to today (in the configured time zone).
// walletID == 0 processes all wallets. Idempotent and safe under concurrency
// thanks to the unique indexes (recurrence_id, occurrence).
func (m *Materializer) Run(ctx context.Context, walletID int64) error

// Loop runs Run every interval until ctx is canceled.
func (m *Materializer) Loop(ctx context.Context, every time.Duration)
```

**`Run` algorithm:**

1. Reads the recurrences with `enabled = 1` (and `wallet_id = ?` if the argument is not zero): `id, wallet_id, kind, amount_cents, description, category_id, payment_method_id, paid_by_user_id, to_user_id, day_of_month, start_date, end_date, created_by`. It also loads the `recurrence_splits` of each one.
2. `today := m.now().In(m.loc).Format("2006-01-02")`.
3. For each recurrence, computes `Occurrences(day, start, end, today)` and, for each occurrence, opens a transaction:
   - `kind` `expense` or `income`: `INSERT INTO transactions (... recurrence_id, occurrence) VALUES (...) ON CONFLICT (recurrence_id, occurrence) WHERE recurrence_id IS NOT NULL DO NOTHING RETURNING id`. If it returned an id, inserts the splits copied from the recurrence.
   - `kind` `settlement`: `INSERT INTO settlements (...) ON CONFLICT ... DO NOTHING`.
   - The `transactions` description cannot be empty. If the recurrence has an empty description, it uses the category name.
4. Keeps the wallets that received inserts and calls `m.bus.Notify(wid, 0)` for each one.

The upsert syntax on a partial index must repeat the index's `WHERE` in the `ON CONFLICT`. SQLite supports this since 3.24, and modernc ships a much newer version.

**Materializer tests** (`materializer_test.go`, using `db.Open` directly and SQL inserts):

- `TestRunCreatesDueOccurrences`: recurrence on day 5, starting 2026-08-01 and `now` fixed at 2026-10-09 (`m.now = func() time.Time {...}`). Generates 3 transactions (August, September and October).
- `TestRunIsIdempotent`: `Run` twice, still 3 transactions.
- `TestRunConcurrent`: 8 goroutines calling `Run` at the same time (with `sync.WaitGroup`), still 3 transactions and no error other than nil.
- `TestRunRespectsTimezone`: `now` = 2026-10-05T02:00Z, which is still the 4th in São Paulo. Recurrence on day 5: October is **not** generated.
- `TestRunSettlement`: a `settlement` recurrence generates rows in `settlements`.
- `TestRunCopiesSplits`: a recurrence with splits 7000/3000 generates identical `transaction_splits`.
- `TestDisabledSkipped`: `enabled = 0` generates nothing.

**Recurrences API:**

- DTO:

  ```go
  type recurrenceDTO struct {
  	ID              int64   `json:"id"`
  	Kind            string  `json:"kind"`
  	AmountCents     int64   `json:"amountCents"`
  	Description     string  `json:"description"`
  	CategoryID      *int64  `json:"categoryId"`
  	PaymentMethodID *int64  `json:"paymentMethodId"`
  	PaidByUserID    int64   `json:"paidByUserId"`
  	ToUserID        *int64  `json:"toUserId"`
  	DayOfMonth      int     `json:"dayOfMonth"`
  	StartDate       string  `json:"startDate"`
  	EndDate         *string `json:"endDate"`
  	Enabled         bool    `json:"enabled"`
  	NextDate        *string `json:"nextDate"`
  	Splits          []split `json:"splits"`
  }
  ```

  `nextDate` is the first occurrence after today, computed with `Occurrences(day, max(start, tomorrow), end, start+400 days)[0]`. It is `null` if the recurrence has ended.
- Validation per `kind`:
  - `expense` and `income` reuse the `transactionInput.validate` rules, with category and splits through `resolveSplits`;
  - `settlement` requires `toUserId` to be a member, different from `paidByUserId`, and does not accept `categoryId` or `splits`;
  - `dayOfMonth` between 1 and 31; valid `startDate`; `endDate` null or greater than or equal to `startDate`.
- POST and PUT: after the commit, run `s.mat.Run(ctx, walletID)` and then `s.mutated(r)`.
- PUT does not touch occurrences already generated.
- DELETE removes the recurrence. The occurrences stay, with `recurrence_id` NULL through `SET NULL`.
- **API tests:**
  - creating a recurrence that starts in the past generates the transactions right away (transactions GET over the range);
  - PUT changes the amount and the old transactions keep the old amount;
  - DELETE keeps the transactions with `recurrenceId: null`;
  - `settlement` without `toUserId` gives 400;
  - reader gets 403 on POST.

- [ ] **Steps:** materializer tests, implementation, watch them pass; API tests, implementation, watch them pass.
- [ ] **Commit:** `feat(recurrence): idempotent materializer and recurrences API`

---

## Task 12: settlements

**Files:** `internal/api/settlements.go`, `internal/api/settlements_test.go`

- DTO:

  ```go
  type settlementDTO struct {
  	ID           int64  `json:"id"`
  	FromUserID   int64  `json:"fromUserId"`
  	ToUserID     int64  `json:"toUserId"`
  	AmountCents  int64  `json:"amountCents"`
  	Date         string `json:"date"`
  	Note         string `json:"note"`
  	RecurrenceID *int64 `json:"recurrenceId"`
  	CreatedBy    int64  `json:"createdBy"`
  }
  ```
- The GET requires `from`/`to`, as in transactions.
- Validation:
  - both users must be members;
  - `from` different from `to`;
  - `amountCents` greater than 0;
  - valid date;
  - note up to 200 characters;
  - only allowed if `splitting_enabled`; otherwise 400.
- PUT replaces. DELETE responds 204. Mutations call `s.mutated(r)`.
- **Tests:** CRUD; wallet without splitting gives 400; `from == to` gives 400; non-member gives 400.
- [ ] **Commit:** `feat(api): settlements between members`

---

## Task 13: budgets

**Files:** `internal/api/budgets.go`, `internal/api/budgets_test.go`

- `GET` returns `[{categoryId, limitCents}]`.
- `PUT /budgets/{categoryID}` with body `{limitCents}`: the category must be an `expense` category of the wallet and `limitCents > 0`. Runs `INSERT ... ON CONFLICT (wallet_id, category_id) DO UPDATE` and responds 200.
- `DELETE` responds 204, or 404 if it did not exist.
- **Tests:** upserting twice keeps 1 row with the last value; an income category gives 400; limit 0 gives 400.
- [ ] **Commit:** `feat(api): monthly budgets per category`

---

## Task 14: `/summary` and suggested settlements

**Files:** `internal/api/settle.go`, `internal/api/settle_test.go`, `internal/api/summary.go`, `internal/api/summary_test.go`

- [ ] **Step 1: pure settlement function**

```go
// settle.go
package api

import "sort"

type transfer struct {
	FromUserID  int64 `json:"fromUserId"`
	ToUserID    int64 `json:"toUserId"`
	AmountCents int64 `json:"amountCents"`
}

// suggestSettlements matches the largest debtor with the largest creditor until everything is zero.
// net: positive = is owed money; negative = owes money. The sum of net must be 0.
func suggestSettlements(net map[int64]int64) []transfer {
	type bal struct {
		id int64
		v  int64
	}
	var debtors, creditors []bal
	for id, v := range net {
		switch {
		case v < 0:
			debtors = append(debtors, bal{id, -v})
		case v > 0:
			creditors = append(creditors, bal{id, v})
		}
	}
	order := func(b []bal) {
		sort.Slice(b, func(i, j int) bool {
			if b[i].v != b[j].v {
				return b[i].v > b[j].v
			}
			return b[i].id < b[j].id
		})
	}
	order(debtors)
	order(creditors)
	var out []transfer
	for i, j := 0, 0; i < len(debtors) && j < len(creditors); {
		amt := min(debtors[i].v, creditors[j].v)
		out = append(out, transfer{debtors[i].id, creditors[j].id, amt})
		debtors[i].v -= amt
		creditors[j].v -= amt
		if debtors[i].v == 0 {
			i++
		}
		if creditors[j].v == 0 {
			j++
		}
	}
	return out
}
```

**Table tests:**

| net | Expected settlements |
|---|---|
| `{1: 1500, 2: -1500}` | `[{2→1 1500}]` |
| `{1: 3000, 2: -1000, 3: -2000}` | `[{3→1 2000}, {2→1 1000}]` |
| `{1: 0, 2: 0}` | `nil` |
| `{1: 1000, 2: 500, 3: -1500}` | `[{3→1 1000}, {3→2 500}]` |

- [ ] **Step 2: summary**

Parameter: `month=YYYY-MM`, required and valid.

Queries:
1. Month totals: `SELECT kind, SUM(amount_cents) FROM transactions WHERE wallet_id = ? AND date BETWEEN ? AND ? GROUP BY kind`.
2. The same totals for the previous month, in `previous`.
3. `byCategory`: `SELECT t.category_id, c.kind, SUM(t.amount_cents), b.limit_cents FROM transactions t JOIN categories c ... LEFT JOIN budgets b ... GROUP BY t.category_id`, plus the categories that have a budget and no expense in the month (cents 0), via `UNION`. Ordered by cents desc.
4. `byEssentiality`: sum per `c.essentiality` for `kind = 'expense'`.
5. `dailyAverageCents = expenseCents / days`. The days are the elapsed days of the month if it is the current month in `cfg.Location`, or the total days of the month if it is a past month. For a future month, 0.
6. If `splitting_enabled`, the member balance comes from the entire history:
   - **payer:** `SELECT paid_by_user_id, SUM(amount_cents) FROM transactions WHERE wallet_id = ? AND kind = 'expense' AND EXISTS(SELECT 1 FROM transaction_splits ...)` gives the credit.
   - **participants:** the debit is computed in Go with `money.Allocate` per transaction, to match the list to the cent. Load `transaction_id, amount_cents, user_id, share_bp` of the transactions with splits, ordered by `transaction_id, user_id`, and group them.
   - **settlements:** `from` is credited `amount`, `to` is debited `amount`.
   - Members with no activity show up with `netCents: 0`.
   - `suggestedSettlements = suggestSettlements(net)`.

- [ ] **Step 3: fixed scenario test** (`summary_test.go`)
  - Wallet with splitting, Ana and Bia.
  - Transactions in October 2026:
    - expense of 10000 paid by Ana, split 50/50;
    - expense of 3001 paid by Bia, split 50/50;
    - income of 500000 for Ana.
  - Settlement: Bia pays 1000 to Ana.
  - Expense of 2000 in September, paid by Ana, without splitting.

    This is not possible with splitting on, because `resolveSplits` applies the default. Post the September expense **before** turning splitting on.
  - Expected for `?month=2026-10`:
    - `incomeCents = 500000`;
    - `expenseCents = 13001`;
    - `previous.expenseCents = 2000`.
  - Balances:
    - Ana paid 10000 and owes 5000 for their own share and 1500 (`Allocate(3001, [5000, 5000])`, which gives `[1501, 1500]`, with Ana at index 0 because of the lower `user_id`);
    - Ana: `+10000 - 5000 - 1501 + 1000 = 4499`;
    - Bia: `+3001 - 5000 - 1500 - 1000 = -4499`.

  Check the indexes and the ordering: the default splits come out in `user_id` order, and Ana has the lowest id. Expected: `suggestedSettlements = [{Bia→Ana 2499}]`. Implemented in `summary_test.go` and passing.
  - Use `s.now` fixed at 2026-10-15 (in the São Paulo time zone) for `dailyAverageCents = 13001 / 15 = 866`.

- [ ] **Commit:** `feat(api): monthly summary and suggested settlements`

---

## Task 15: SSE

**File:** `internal/api/events.go`, `internal/api/events_test.go`

```go
func (s *Server) events(w http.ResponseWriter, r *http.Request) error {
	walletID, userID := walletIDFrom(r.Context()), userIDFrom(r.Context())
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // nginx
	w.WriteHeader(http.StatusOK)
	if err := rc.Flush(); err != nil {
		return nil // streaming not supported; nothing to do
	}

	ch := s.bus.Subscribe(walletID, userID)
	defer s.bus.Unsubscribe(walletID, ch)
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-ch:
			fmt.Fprint(w, "data: {\"type\":\"changed\"}\n\n")
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
		}
		if err := rc.Flush(); err != nil {
			return nil
		}
	}
}
```

The `http.Server` in `main.go` uses `WriteTimeout: 0` because of SSE and sets `ReadHeaderTimeout: 10s`. `rc.Flush()` works through the `statusRecorder` thanks to `Unwrap()`.

**Test:**
- Ana opens the stream with an `http.Client` that has the cookie, and the request runs in a goroutine with a `bufio.Reader`.
- Bia (an editor) creates a transaction.
- Within 2 seconds, Ana reads `data: {"type":"changed"}`. Cancel the context at the end.
- A second case: Ana creating a transaction does **not** produce an event for their own stream. Wait 300ms and confirm that nothing arrived.

- [ ] **Commit:** `feat(api): per-wallet SSE with heartbeat`

---

## Task 16: backup, embedded SPA and `main.go`

**Files:** `internal/db/backup.go` (+ `_test`), `internal/api/spa.go` (+ `_test`), `web/embed.go`, `web/dist/index.html`, `cmd/gastos/main.go`

- [ ] **Step 1: backup**

```go
// Backup writes a consistent copy into dir with a timestamp and keeps only the newest keep copies.
func Backup(ctx context.Context, d *sql.DB, dir string, keep int, now time.Time) (string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "gastos-"+now.UTC().Format("20060102-150405")+".db")
	if _, err := d.ExecContext(ctx, `VACUUM INTO ?`, path); err != nil {
		return "", err
	}
	matches, err := filepath.Glob(filepath.Join(dir, "gastos-*.db"))
	if err != nil {
		return path, err
	}
	sort.Strings(matches) // timestamp in the name ⇒ lexicographic order = chronological
	for len(matches) > keep {
		os.Remove(matches[0])
		matches = matches[1:]
	}
	return path, nil
}

// LastBackupAge returns how long ago the last backup was (or a huge value if there is none).
func LastBackupAge(dir string, now time.Time) time.Duration
```

**Tests:**
- 3 backups with different `now` values and `keep = 2`: the 2 newest remain.
- The generated file opens with `db.Open` and has `user_version = 1`.
- A backup with the same `now` twice gives an error, and the existing file stays intact. `VACUUM INTO` fails if the destination exists, which is the desired behavior: it never overwrites.

- [ ] **Step 2: SPA**

```go
// spaHandler serves files from fsys; paths with no matching file get index.html.
func spaHandler(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(fsys, p); err != nil {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			w.Header().Set("Cache-Control", "no-cache")
			files.ServeHTTP(w, r2)
			return
		}
		if strings.HasPrefix(p, "assets/") { // Vite hashed files
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}
```

**Tests** with `fstest.MapFS`:
- `/` returns the index;
- `/lancamentos` (a transactions screen route) returns the index;
- `/assets/app.js` returns the file with immutable cache;
- `/api/x` gives a JSON 404 (it comes from the mux, not the SPA).

- [ ] **Step 3: frontend placeholder**

`web/embed.go`:
```go
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the frontend build with dist/ as the root.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
```

`web/dist/index.html`:
```html
<!doctype html><meta charset="utf-8"><title>gastos</title><p>Frontend em construção (fase 3).</p>
```

- [ ] **Step 4: `cmd/gastos/main.go`**

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"gastos/internal/api"
	"gastos/internal/config"
	"gastos/internal/db"
	"gastos/internal/events"
	"gastos/internal/recurrence"
	"gastos/web"
)

var version = "dev"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("encerrando", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	d, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer d.Close()

	if db.LastBackupAge(cfg.BackupDir, time.Now()) > 24*time.Hour {
		if p, err := db.Backup(ctx, d, cfg.BackupDir, cfg.BackupKeep, time.Now()); err != nil {
			log.Error("backup inicial", "err", err)
		} else {
			log.Info("backup", "path", p)
		}
	}

	bus := events.New()
	mat := recurrence.NewMaterializer(d, bus, cfg.Location, log)
	srv := api.New(d, cfg, bus, mat, log)

	go mat.Loop(ctx, time.Hour)       // runs once right at the start
	go srv.Housekeeping(ctx, time.Hour) // cleans up expired sessions + daily backup

	httpSrv := &http.Server{Addr: cfg.Addr, Handler: srv.Routes(web.Dist()), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		httpSrv.Shutdown(shutdownCtx)
	}()

	log.Info("ouvindo", "addr", cfg.Addr, "version", version)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
```

`Server.Housekeeping(ctx, every)` lives in `internal/api/housekeeping.go`. On each tick, it runs `DELETE FROM sessions WHERE expires_at <= now` and, if `db.LastBackupAge > 24h`, calls `db.Backup`. Open SSE connections hold `Shutdown` until the 10-second timeout; that is acceptable.

- [ ] **Step 5: manual smoke test**

```bash
GASTOS_COOKIE_SECURE=false go run ./cmd/gastos
curl -s localhost:8000/healthz
curl -s -c /tmp/c -H 'Content-Type: application/json' -d '{"name":"Ana","email":"ana@example.com","password":"senha-segura"}' localhost:8000/api/auth/register
curl -s -b /tmp/c localhost:8000/api/me
```

On PowerShell: `$env:GASTOS_COOKIE_SECURE='false'; go run ./cmd/gastos`. For the `curl` calls on Windows, use `curl.exe` and a cookie file in `$env:TEMP`.

Expected: `ok`, then the user JSON, then `{user, wallets:[{... "name":"Pessoal" ...}]}`.

- [ ] **Step 6:** commit `feat: main, rotating backup and embedded SPA`

---

## Task 17: remove the old backend

- [ ] **Step 1:**

```bash
git rm -r src vendor deploy.sh gastos-app
go mod tidy
```

- [ ] **Step 2:** rewrite the README:
  - new stack;
  - how to run in dev (`go run ./cmd/gastos` and, in phase 3, `npm run dev` in `web/`);
  - environment variables (table from the spec);
  - link to the roadmap.

  Remove the old API and pending-security sections that have been resolved.
- [ ] **Step 3: final verification**

```bash
go vet ./...
go test ./... -race    # -race requires CGO; on Windows without gcc, run without -race
grep -rn "float" internal/
go build -o gastos ./cmd/gastos
```

Expected:
- `vet` clean;
- all tests passing;
- `grep` with no output;
- the generated binary.

Delete the binary afterwards (it is in `.gitignore`).

- [ ] **Step 4:** commit `chore: remove old backend, vendor and committed binary`
- [ ] **Step 5:** open the PR `rewrite/backend → main` with the acceptance criteria checklist from the spec.

---

## Order and parallelism

```
0 → 1 → 2 → 3 ─┐
          4 ───┼→ 5 → 6+7+8 → 9 → 10 → 11 → 12 → 13 → 14 → 15 → 16 → 17
               │
 (3 and 4 are pure and independent; they can run in parallel with 2)
```
