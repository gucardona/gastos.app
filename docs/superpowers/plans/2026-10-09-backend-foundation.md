# Backend Foundation — Plano de implementação

> **Para agentes:** implemente tarefa por tarefa, em ordem. Cada tarefa é TDD: escreva o teste, veja falhar, implemente, veja passar, commit. Passos usam checkbox (`- [ ]`).

**Spec:** [2026-10-09-backend-foundation-design.md](../specs/2026-10-09-backend-foundation-design.md)

**Objetivo:** substituir `src/` por um backend novo:
- em `cmd/` + `internal/`;
- schema único;
- dinheiro em centavos;
- sessões por cookie;
- API `/api/wallets/{walletID}/...` com o roteador do Go 1.22+;
- materializador de recorrências idempotente.

**Arquitetura:** pacotes pequenos e puros (`money`, `recurrence`), um pacote `db` dono do schema e um pacote `api` com um `Server` que concentra dependências. Os handlers têm a assinatura `func(w, r) error`, e um adaptador único converte erros em JSON.

**Stack:** Go 1.27 (o `go.mod` pede ≥ 1.26), `modernc.org/sqlite`, `golang.org/x/crypto/bcrypt` e só a stdlib (`net/http`, `log/slog`, `embed`).

**Branch:** `rewrite/backend`. Todo o código novo convive com `src/` até a tarefa 17, que remove o antigo.

**Comandos:** os exemplos usam `go test` a partir da raiz do repo. Funcionam igual no PowerShell e no bash.

---

## Mapa de arquivos

| Arquivo | Ação | Responsabilidade |
|---|---|---|
| `cmd/gastos/main.go` | Criar | config → db → server → jobs → `ListenAndServe` + shutdown gracioso |
| `internal/config/config.go` (+ `_test`) | Criar | ler e validar env |
| `internal/db/schema.sql` | Criar | schema versão 1 |
| `internal/db/db.go` (+ `_test`) | Criar | `Open` com pragmas, `migrate` via `user_version` |
| `internal/db/backup.go` (+ `_test`) | Criar | `VACUUM INTO` + rotação |
| `internal/money/money.go` (+ `_test`) | Criar | `Allocate`, `EqualShares` |
| `internal/recurrence/occurrences.go` (+ `_test`) | Criar | `Occurrences` puro |
| `internal/recurrence/materializer.go` (+ `_test`) | Criar | gera transações e acertos idempotentes |
| `internal/events/bus.go` (+ `_test`) | Mover/ajustar | bus por carteira (de `src/events`) |
| `internal/api/server.go` | Criar | `Server`, `Routes`, middlewares de log, recover e CORS de dev |
| `internal/api/httpx.go` | Criar | `apiError`, `handle`, `decode`, `writeJSON`, `querier` |
| `internal/api/auth.go` | Criar | register, login, logout, me, `requireUser`, sessões, rate limit |
| `internal/api/wallets.go` | Criar | wallets, members, `requireWallet`, seed |
| `internal/api/catalog.go` | Criar | categories, payment-methods |
| `internal/api/transactions.go` | Criar | CRUD + splits |
| `internal/api/recurrences.go` | Criar | CRUD + materialização síncrona |
| `internal/api/settlements.go` | Criar | CRUD |
| `internal/api/budgets.go` | Criar | list, upsert, delete |
| `internal/api/summary.go` (+ `settle.go`) | Criar | `/summary` + acertos sugeridos (puro) |
| `internal/api/events.go` | Criar | SSE com heartbeat |
| `internal/api/spa.go` | Criar | serve o `fs.FS` embutido com fallback para `index.html` |
| `internal/api/*_test.go` | Criar | integração com `httptest` |
| `web/embed.go`, `web/dist/index.html` | Criar | placeholder até a fase 3 |
| `src/`, `vendor/`, `gastos-app`, `deploy.sh` | Remover | tarefa 17 |

---

## Tarefa 0: Branch e esqueleto

- [ ] **Passo 1:** criar a branch e os diretórios

```bash
git switch -c rewrite/backend
mkdir -p cmd/gastos internal/config internal/db internal/money internal/recurrence internal/events internal/api web/dist
```

- [ ] **Passo 2:** adicionar ao `.gitignore`

```
/data/
/gastos
/gastos.exe
web/node_modules/
```

`web/dist/` **não** entra no ignore ainda, porque o placeholder da tarefa 16 precisa estar versionado para o `go:embed` compilar. A fase 3 troca isso pelo build do CI.

- [ ] **Passo 3:** commit `chore: esqueleto do backend novo`

---

## Tarefa 1: `config`

**Arquivos:** `internal/config/config.go`, `internal/config/config_test.go`

- [ ] **Passo 1: teste**

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

- [ ] **Passo 2:** `go test ./internal/config/` deve falhar (não compila).

- [ ] **Passo 3: implementação**

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

// Load lê a configuração de getenv (os.Getenv em produção) e falha cedo em valores inválidos.
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

`cmd/gastos/main.go` importa `_ "time/tzdata"`, para o `LoadLocation` funcionar em Windows e em imagens mínimas. O teste também precisa disso no Windows: adicione `import _ "time/tzdata"` no `config_test.go`.

- [ ] **Passo 4:** `go test ./internal/config/` passa.
- [ ] **Passo 5:** commit `feat(config): carregar e validar env`

---

## Tarefa 2: `db` — schema, `Open`, migrate

**Arquivos:** `internal/db/schema.sql`, `internal/db/db.go`, `internal/db/db_test.go`

- [ ] **Passo 1:** criar `schema.sql` com o SQL **exato** da seção "Schema (versão 1)" da spec.

- [ ] **Passo 2: testes**

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
	for i := 0; i < 3; i++ { // segura 3 conexões distintas do pool
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

Adicione `"database/sql"` aos imports.

- [ ] **Passo 3:** `go test ./internal/db/` deve falhar.

- [ ] **Passo 4: implementação**

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

// migrations[i] leva o banco da versão i+1 para i+2. O schema.sql é sempre a versão 1.
// Para mudar o schema: acrescente um item aqui; nunca edite itens antigos.
var migrations = []string{}

const pragmas = "?_pragma=foreign_keys(1)" +
	"&_pragma=journal_mode(WAL)" +
	"&_pragma=busy_timeout(5000)" +
	"&_pragma=synchronous(NORMAL)" +
	"&_txlock=immediate"

// Open abre o SQLite com pragmas aplicados em toda conexão do pool e aplica o schema.
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

O `journal_mode(WAL)` dentro de transação não tem efeito, mas aqui ele roda no DSN, na abertura da conexão, então tudo bem. Se o `tx.Exec` de várias instruções falhar no driver, divida o `schema.sql` com `strings.Split(steps[i], ";\n")` e ignore os trechos vazios.

- [ ] **Passo 5:** `go test ./internal/db/` passa.
- [ ] **Passo 6:** commit `feat(db): schema v1, pragmas por conexão e versionamento por user_version`

---

## Tarefa 3: `money`

**Arquivos:** `internal/money/money.go`, `internal/money/money_test.go`

- [ ] **Passo 1: teste**

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

- [ ] **Passo 2:** ver falhar.
- [ ] **Passo 3: implementação**

```go
package money

import "sort"

// FullShare é 100% em basis points.
const FullShare = 10000

// Allocate divide total centavos proporcionalmente a shares (maior resto).
// A soma do resultado é sempre total; empates de resto favorecem o menor índice.
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

// EqualShares divide 100% em n partes; o resto vai para a primeira.
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

- [ ] **Passo 4:** passa. **Passo 5:** commit `feat(money): rateio em centavos e partes iguais em bp`

---

## Tarefa 4: `recurrence.Occurrences`

**Arquivos:** `internal/recurrence/occurrences.go`, `internal/recurrence/occurrences_test.go`

- [ ] **Passo 1: teste**

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

- [ ] **Passo 2:** ver falhar.
- [ ] **Passo 3: implementação**

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

// Occurrences lista as datas mensais de uma recorrência entre start e until (inclusivos),
// respeitando end (inclusivo, vazio = sem fim). O dia é limitado ao último dia de cada mês.
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

- [ ] **Passo 4:** passa. **Passo 5:** commit `feat(recurrence): ocorrências mensais puras`

---

## Tarefa 5: `events`

**Arquivos:** `internal/events/bus.go`, `internal/events/bus_test.go`

- [ ] **Passo 1:** copiar `src/events/bus.go` e `bus_test.go` para `internal/events/`. Trocar a variável global `Bus` por um construtor `New() *Bus`, renomeando o tipo `EventBus` para `Bus`, e ajustar o teste para `New()`.
- [ ] **Passo 2:** adicionar um teste: `Notify` com `actorUserID = 0` (job do sistema) entrega para todos os inscritos.
- [ ] **Passo 3:** `go test ./internal/events/` passa. Se o código atual já cobre esse caso (pula só se `s.userID == actorUserID`, e userID nunca é 0), o teste passa sem mudança.
- [ ] **Passo 4:** commit `refactor(events): bus sem global`

---

## Tarefa 6: `api` — infraestrutura HTTP e harness de teste

**Arquivos:** `internal/api/httpx.go`, `internal/api/server.go`, `internal/api/testutil_test.go`

- [ ] **Passo 1: `httpx.go`**

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

// fieldErrors acumula erros de validação por campo.
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

// decode exige JSON (barreira de CSRF junto com SameSite=Lax), limita 1 MB e recusa campos desconhecidos.
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

// querier é satisfeito por *sql.DB e *sql.Tx.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// inTx executa fn numa transação (BEGIN IMMEDIATE pelo DSN).
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

- [ ] **Passo 2: `server.go`**

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
	mux.Handle("DELETE /api/wallets/{walletID}/members/{userID}", wallet(roleReader, s.removeMember)) // owner ou o próprio: checado no handler

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
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter } // mantém Flusher via ResponseController

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

// devCORS libera o Vite em localhost:5173 apenas com GASTOS_DEV=true.
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

O `r.Pattern` é preenchido pelo mux (Go 1.23+), mas o `logRequests` envolve o mux: depois do `ServeHTTP` interno, o `r` externo **não** tem o pattern. Para o log, use `r.URL.Path` ou guarde o pattern num ponteiro no context dentro do `requireUser`. A solução simples é logar `r.URL.Path` e aceitar IDs no log.

- [ ] **Passo 3: harness de teste (`testutil_test.go`)**

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

func (c *client) wpath(format string, args ...any) string {
	return fmt.Sprintf("/api/wallets/%d", c.walletID) + fmt.Sprintf(format, args...)
}
```

Adicione `"fmt"` aos imports.

As funções referenciadas (`requireUser`, `role`, etc.) só existem a partir da tarefa 7. Este passo só compila junto com a tarefa 7. Commit único das duas.

---

## Tarefa 7: auth (sessões, register, login, logout, me)

**Arquivo:** `internal/api/auth.go`, `internal/api/auth_test.go`

- [ ] **Passo 1: testes**

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

- [ ] **Passo 2:** ver falhar.
- [ ] **Passo 3: implementação**

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
	// Atrás do Caddy/nginx em localhost: confiar no X-Forwarded-For só se o peer é loopback.
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

// allow registra uma tentativa e diz se ainda está dentro do limite.
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

O teste `TestLoginRateLimit` roda do mesmo IP (`127.0.0.1`, loopback sem XFF). A chave de IP e a de e-mail estouram juntas na 11ª tentativa.

- [ ] **Passo 4:** `go test ./internal/api/ -run 'Auth|Register|Login|ContentType|Unknown'` passa. Isso só compila depois da tarefa 8, que define `createWallet`, `walletsFor`, `role` e `requireWallet`. Faça as tarefas 7 e 8 juntas e rode os testes no fim da 8.
- [ ] **Passo 5:** commit `feat(api): sessões por cookie, register/login/logout/me`

---

## Tarefa 8: wallets, members, `requireWallet`, seed

**Arquivos:** `internal/api/wallets.go`, `internal/api/wallets_test.go`

**Comportamento:**

- `type role int` com `roleReader < roleEditor < roleOwner`, mais `parseRole(string) (role, bool)` e `String()`.
- `requireWallet(min role, next)`:
  - lê `r.PathValue("walletID")`;
  - busca o `role` em `wallet_members`;
  - não é membro: **404**, para não revelar que a carteira existe;
  - papel abaixo do mínimo: **403**;
  - põe `ctxWalletID` e `ctxRole` no context.
- `createWallet(ctx, q querier, name, ownerID, splitting) (int64, error)` insere a carteira, o owner e chama `seedWallet`.
- `seedWallet` insere:
  - as 15 categorias de gasto de `src/db/db.go:117-131`, com os mesmos nomes, ícones, cores e essencialidade, mas sem `key`;
  - as 5 categorias de entrada: Salário 💼 `#0f6579`, Freelance 🧑‍💻 `#1f7d8f`, Investimentos 📈 `#f39c12`, Reembolso ↩️ `#2b92a1`, Outros ◈ `#7f9ea5`;
  - as 8 formas de pagamento de `src/db/db.go:146-153`.
- DTO de carteira:

  ```go
  type walletDTO struct {
  	ID               int64  `json:"id"`
  	Name             string `json:"name"`
  	Role             string `json:"role"`
  	SplittingEnabled bool   `json:"splittingEnabled"`
  	OwnerName        string `json:"ownerName"`
  }
  ```
- `walletsFor(ctx, userID)` faz uma query com JOIN no owner, ordenada por `w.created_at, w.id`.
- `PATCH /wallets/{id}` aceita `{name?: string, splittingEnabled?: bool}` com ponteiros. Ativar a divisão não muda transações antigas.
- `DELETE /wallets/{id}`: owner. O CASCADE limpa tudo. Notifica antes de apagar, para os outros membros saírem da tela.
- Membros:
  - `GET` devolve `[{userId, name, email, role, splitsByDefault}]` ordenado por owner, editor, reader e nome.
  - `POST {email, role}`: `role` precisa ser `editor` ou `reader`. Usuário inexistente dá 404 com a mensagem "Usuário não encontrado". Quem já é membro dá 409.
  - `PATCH .../members/{userID}` aceita `{role?, splitsByDefault?}`. Não pode alterar o owner (400).
  - `DELETE .../members/{userID}`: permitido se `role == owner` ou `userID == eu`. O owner não pode sair nem ser removido (400).
- Toda mutação de sucesso chama `s.mutated(r)`:

  ```go
  func (s *Server) mutated(r *http.Request) {
  	s.bus.Notify(walletIDFrom(r.Context()), userIDFrom(r.Context()))
  }
  ```

- [ ] **Passo 1: testes** (`wallets_test.go`)
  - `TestRegisterSeedsWallet`: o GET de categories devolve 20 itens (15 de gasto e 5 de entrada), e o de payment-methods devolve 8.
  - `TestWalletIsolation`: Bia lendo `/api/wallets/{carteira da Ana}/transactions?...` recebe **404**.
  - `TestRoles`:
    - Ana adiciona Bia como `reader`. Bia faz GET de categories (200) e POST de category (403).
    - Ana promove Bia a `editor`. O POST de category da Bia passa (201), mas o PATCH da wallet continua 403.
  - `TestMemberLeave`: Bia sai sozinha (`DELETE .../members/{bia}`, 204). Ana não consegue sair (400).
  - `TestAddMemberErrors`: e-mail inexistente dá 404; membro duplicado dá 409; `role: "owner"` dá 400.
  - `TestDeleteWallet`: Ana cria a carteira "Casa", lança uma transação e apaga a carteira (204). O GET de wallets não mostra mais "Casa".
- [ ] **Passo 2:** ver falhar. **Passo 3:** implementar. **Passo 4:** `go test ./internal/api/` passa, incluindo os testes da tarefa 7.
- [ ] **Passo 5:** commit `feat(api): carteiras, membros e papéis`

---

## Tarefa 9: categorias e formas de pagamento

**Arquivos:** `internal/api/catalog.go`, `internal/api/catalog_test.go`

**Comportamento:**

- Categoria:

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
- O GET devolve todas, incluindo as arquivadas (o front filtra), ordenadas por `kind, sort_order, id`.
- POST e PUT validam:
  - `kind` (não pode mudar no PUT);
  - `name` não vazio, com no máximo 40 caracteres;
  - `icon` não vazio;
  - `color` no formato `^#[0-9a-fA-F]{6}$`;
  - `essentiality` obrigatória se `kind == expense` e proibida se `income`;
  - nome duplicado dá 409.

  O POST usa `sort_order = max + 1`. O PUT aceita `sortOrder`, para reordenar.
- DELETE:
  - se `transactions`, `recurrences` ou `budgets` referenciam a categoria, faz `UPDATE ... SET archived_at = now` e responde 200 com o DTO (`archived: true`);
  - se não há referências, faz `DELETE` e responde 204;
  - não permite ficar sem nenhuma categoria ativa do mesmo `kind` (400).
- O PUT de uma categoria arquivada com `"archived": false` desarquiva. Por isso o campo `archived` também é aceito na entrada do PUT.
- Formas de pagamento seguem as mesmas regras, sem `kind`, `color` e `essentiality`.
- Toda mutação chama `s.mutated(r)`.

- [ ] **Passo 1: testes**
  - CRUD feliz de categoria;
  - cor inválida dá 400 com `fields.color`;
  - `income` com `essentiality` dá 400;
  - nome duplicado dá 409;
  - categoria em uso: DELETE responde 200 e `archived:true`, e a transação continua com aquele `categoryId`;
  - categoria sem uso: DELETE responde 204;
  - apagar a última categoria de entrada ativa dá 400;
  - os mesmos casos, adaptados, para formas de pagamento.
- [ ] **Passo 2–4:** ver falhar, implementar, passar.
- [ ] **Passo 5:** commit `feat(api): categorias e formas de pagamento com arquivamento`

---

## Tarefa 10: transações com splits

Esta é a tarefa de referência: os handlers das tarefas 11–13 seguem o mesmo formato.

**Arquivos:** `internal/api/transactions.go`, `internal/api/transactions_test.go`

- [ ] **Passo 1: testes**

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
	ana.do("GET", ana.wpath("/transactions"), nil, 400, nil) // from/to obrigatórios
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

	// divisão desligada: splits proibidos
	ana.do("POST", ana.wpath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": ana.user.ID, "shareBp": 10000}},
	}), 400, nil)

	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, nil)

	// sem splits: igual entre membros com splitsByDefault
	var tx transaction
	ana.do("POST", ana.wpath("/transactions"), body(nil), 201, &tx)
	if len(tx.Splits) != 2 || tx.Splits[0].AmountCents+tx.Splits[1].AmountCents != 101 {
		t.Fatalf("split padrão: %+v", tx.Splits)
	}

	// soma diferente de 10000
	ana.do("POST", ana.wpath("/transactions"), body(map[string]any{
		"splits": []map[string]any{{"userId": ana.user.ID, "shareBp": 6000}, {"userId": tx.Splits[1].UserID, "shareBp": 3000}},
	}), 400, nil)

	// não membro
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

- [ ] **Passo 2:** ver falhar.
- [ ] **Passo 3: implementação**

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

// validate normaliza a entrada e confere referências dentro da carteira.
// currentCategoryID permite manter uma categoria arquivada que já era da transação (0 na criação).
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

// resolveSplits aplica as regras de divisão da spec e devolve os splits finais (sem amountCents).
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

// attachSplits carrega os splits de todas as transações da lista com uma query.
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

Um `IN (...)` com muitos ids pode passar do limite de parâmetros do SQLite (32766 no modernc), mas o intervalo máximo de 366 dias mantém o volume bem abaixo disso.

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

O `validate` roda dentro do `inTx`. Com `_txlock=immediate`, a transação inteira segura o lock de escrita, o que é aceitável para um app pessoal.

- [ ] **Passo 4:** `go test ./internal/api/ -run Transaction|Splits|Editor` passa.
- [ ] **Passo 5: checagem de float**

```bash
grep -rn "float" internal/api/ internal/money/ internal/recurrence/
```

A saída esperada é vazia.

- [ ] **Passo 6:** commit `feat(api): transações com splits em basis points`

---

## Tarefa 11: recorrências e materializador

**Arquivos:** `internal/recurrence/materializer.go`, `internal/recurrence/materializer_test.go`, `internal/api/recurrences.go`, `internal/api/recurrences_test.go`

**Materializador:**

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

// Run materializa ocorrências devidas até hoje (no fuso configurado).
// walletID == 0 processa todas as carteiras. Idempotente e seguro sob concorrência
// graças aos índices únicos (recurrence_id, occurrence).
func (m *Materializer) Run(ctx context.Context, walletID int64) error

// Loop roda Run a cada intervalo até ctx ser cancelado.
func (m *Materializer) Loop(ctx context.Context, every time.Duration)
```

**Algoritmo do `Run`:**

1. Lê as recorrências com `enabled = 1` (e `wallet_id = ?` se o argumento não for zero): `id, wallet_id, kind, amount_cents, description, category_id, payment_method_id, paid_by_user_id, to_user_id, day_of_month, start_date, end_date, created_by`. Carrega também os `recurrence_splits` de cada uma.
2. `today := m.now().In(m.loc).Format("2006-01-02")`.
3. Para cada recorrência, calcula `Occurrences(day, start, end, today)` e, para cada ocorrência, abre uma transação:
   - `kind` `expense` ou `income`: `INSERT INTO transactions (... recurrence_id, occurrence) VALUES (...) ON CONFLICT (recurrence_id, occurrence) WHERE recurrence_id IS NOT NULL DO NOTHING RETURNING id`. Se retornou id, insere os splits copiados da recorrência.
   - `kind` `settlement`: `INSERT INTO settlements (...) ON CONFLICT ... DO NOTHING`.
   - A descrição de `transactions` não pode ser vazia. Se a recorrência tiver descrição vazia, usa o nome da categoria.
4. Guarda as carteiras que receberam inserções e chama `m.bus.Notify(wid, 0)` para cada uma.

A sintaxe de upsert num índice parcial precisa repetir o `WHERE` do índice no `ON CONFLICT`. O SQLite suporta isso desde a 3.24, e o modernc traz uma versão bem mais nova.

**Testes do materializador** (`materializer_test.go`, usando `db.Open` direto e inserts SQL):

- `TestRunCreatesDueOccurrences`: recorrência no dia 5, com início em 2026-08-01 e `now` fixo em 2026-10-09 (`m.now = func() time.Time {...}`). Gera 3 transações (agosto, setembro e outubro).
- `TestRunIsIdempotent`: `Run` duas vezes, ainda 3 transações.
- `TestRunConcurrent`: 8 goroutines chamando `Run` ao mesmo tempo (com `sync.WaitGroup`), ainda 3 transações e nenhum erro além de nil.
- `TestRunRespectsTimezone`: `now` = 2026-10-05T02:00Z, que ainda é dia 4 em São Paulo. Recorrência no dia 5: outubro **não** é gerado.
- `TestRunSettlement`: uma recorrência `settlement` gera linhas em `settlements`.
- `TestRunCopiesSplits`: recorrência com splits 7000/3000 gera `transaction_splits` iguais.
- `TestDisabledSkipped`: `enabled = 0` não gera nada.

**API de recorrências:**

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

  `nextDate` é a primeira ocorrência depois de hoje, calculada com `Occurrences(day, max(start, amanhã), end, start+400 dias)[0]`. Vem `null` se acabou.
- Validação por `kind`:
  - `expense` e `income` reutilizam as regras de `transactionInput.validate`, com categoria e splits pelo `resolveSplits`;
  - `settlement` exige `toUserId` membro, diferente de `paidByUserId`, e não aceita `categoryId` nem `splits`;
  - `dayOfMonth` entre 1 e 31; `startDate` válida; `endDate` nula ou maior ou igual a `startDate`.
- POST e PUT: depois do commit, rodam `s.mat.Run(ctx, walletID)` e depois `s.mutated(r)`.
- O PUT não mexe em ocorrências já geradas.
- DELETE remove a recorrência. As ocorrências ficam, com `recurrence_id` NULL pelo `SET NULL`.
- **Testes de API:**
  - criar uma recorrência com início no passado gera as transações na hora (GET de transações no intervalo);
  - PUT muda o valor e as transações antigas mantêm o valor antigo;
  - DELETE mantém as transações com `recurrenceId: null`;
  - `settlement` sem `toUserId` dá 400;
  - reader recebe 403 no POST.

- [ ] **Passos:** testes do materializador, implementação, passar; testes de API, implementação, passar.
- [ ] **Commit:** `feat(recurrence): materializador idempotente e API de recorrências`

---

## Tarefa 12: acertos (settlements)

**Arquivos:** `internal/api/settlements.go`, `internal/api/settlements_test.go`

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
- O GET exige `from`/`to`, como em transações.
- Validação:
  - os dois usuários precisam ser membros;
  - `from` diferente de `to`;
  - `amountCents` maior que 0;
  - data válida;
  - nota com até 200 caracteres;
  - só permitido se `splitting_enabled`; senão 400.
- PUT substitui. DELETE responde 204. Mutações chamam `s.mutated(r)`.
- **Testes:** CRUD; carteira sem divisão dá 400; `from == to` dá 400; não membro dá 400.
- [ ] **Commit:** `feat(api): acertos entre membros`

---

## Tarefa 13: orçamentos (budgets)

**Arquivos:** `internal/api/budgets.go`, `internal/api/budgets_test.go`

- `GET` devolve `[{categoryId, limitCents}]`.
- `PUT /budgets/{categoryID}` com body `{limitCents}`: a categoria precisa ser `expense` da carteira e `limitCents > 0`. Faz `INSERT ... ON CONFLICT (wallet_id, category_id) DO UPDATE` e responde 200.
- `DELETE` responde 204, ou 404 se não existia.
- **Testes:** upsert duas vezes mantém 1 linha com o último valor; categoria de entrada dá 400; limite 0 dá 400.
- [ ] **Commit:** `feat(api): orçamentos mensais por categoria`

---

## Tarefa 14: `/summary` e acertos sugeridos

**Arquivos:** `internal/api/settle.go`, `internal/api/settle_test.go`, `internal/api/summary.go`, `internal/api/summary_test.go`

- [ ] **Passo 1: função pura de acertos**

```go
// settle.go
package api

import "sort"

type transfer struct {
	FromUserID  int64 `json:"fromUserId"`
	ToUserID    int64 `json:"toUserId"`
	AmountCents int64 `json:"amountCents"`
}

// suggestSettlements casa o maior devedor com o maior credor até zerar.
// net: positivo = tem a receber; negativo = deve. A soma de net deve ser 0.
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

**Testes de tabela:**

| net | Acertos esperados |
|---|---|
| `{1: 1500, 2: -1500}` | `[{2→1 1500}]` |
| `{1: 3000, 2: -1000, 3: -2000}` | `[{3→1 2000}, {2→1 1000}]` |
| `{1: 0, 2: 0}` | `nil` |
| `{1: 1000, 2: 500, 3: -1500}` | `[{3→1 1000}, {3→2 500}]` |

- [ ] **Passo 2: summary**

Parâmetro: `month=YYYY-MM`, obrigatório e válido.

Queries:
1. Totais do mês: `SELECT kind, SUM(amount_cents) FROM transactions WHERE wallet_id = ? AND date BETWEEN ? AND ? GROUP BY kind`.
2. Os mesmos totais para o mês anterior, em `previous`.
3. `byCategory`: `SELECT t.category_id, c.kind, SUM(t.amount_cents), b.limit_cents FROM transactions t JOIN categories c ... LEFT JOIN budgets b ... GROUP BY t.category_id`, mais as categorias que têm budget e nenhum gasto no mês (cents 0), via `UNION`. Ordenado por cents desc.
4. `byEssentiality`: soma por `c.essentiality` para `kind = 'expense'`.
5. `dailyAverageCents = expenseCents / dias`. Os dias são os decorridos no mês, se for o mês corrente em `cfg.Location`, ou o total de dias do mês, se for um mês passado. Para mês futuro, 0.
6. Se `splitting_enabled`, o saldo dos membros vem do histórico inteiro:
   - **pagador:** `SELECT paid_by_user_id, SUM(amount_cents) FROM transactions WHERE wallet_id = ? AND kind = 'expense' AND EXISTS(SELECT 1 FROM transaction_splits ...)` dá o crédito.
   - **participantes:** o débito é calculado em Go com `money.Allocate` por transação, para casar centavo a centavo com o que a lista mostra. Carregue `transaction_id, amount_cents, user_id, share_bp` das transações com splits, ordenados por `transaction_id, user_id`, e agrupe.
   - **settlements:** `from` ganha crédito de `amount`, `to` recebe débito de `amount`.
   - Membros sem movimento aparecem com `netCents: 0`.
   - `suggestedSettlements = suggestSettlements(net)`.

- [ ] **Passo 3: teste de cenário fixo** (`summary_test.go`)
  - Carteira com divisão, Ana e Bia.
  - Lançamentos em outubro de 2026:
    - gasto de 10000 pago pela Ana, dividido 50/50;
    - gasto de 3001 pago pela Bia, dividido 50/50;
    - entrada de 500000 da Ana.
  - Acerto: Bia paga 1000 para a Ana.
  - Gasto de 2000 em setembro, pago pela Ana, sem divisão.

    Isso não é possível com a divisão ligada, porque o `resolveSplits` aplica o padrão. Lance o gasto de setembro **antes** de ligar a divisão.
  - Esperado para `?month=2026-10`:
    - `incomeCents = 500000`;
    - `expenseCents = 13001`;
    - `previous.expenseCents = 2000`.
  - Saldos:
    - a Ana pagou 10000 e deve 5000 da própria parte e 1500 (`Allocate(3001, [5000, 5000])`, que dá `[1501, 1500]`, com a Ana no índice 0 por `user_id` menor);
    - Ana: `+10000 - 5000 - 1501 + 1000 = 4499`;
    - Bia: `+3001 - 5000 - 1500 - 1000 = -4499`.

  Confira os índices e a ordenação: os splits padrão saem em ordem de `user_id`, e a Ana tem o menor id. Esperado: `suggestedSettlements = [{Bia→Ana 4499}]`.
  - Use `s.now` fixo em 2026-10-15 (no fuso de São Paulo) para `dailyAverageCents = 13001 / 15 = 866`.

- [ ] **Commit:** `feat(api): resumo mensal e acertos sugeridos`

---

## Tarefa 15: SSE

**Arquivo:** `internal/api/events.go`, `internal/api/events_test.go`

```go
func (s *Server) events(w http.ResponseWriter, r *http.Request) error {
	walletID, userID := walletIDFrom(r.Context()), userIDFrom(r.Context())
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // nginx
	w.WriteHeader(http.StatusOK)
	if err := rc.Flush(); err != nil {
		return nil // streaming não suportado; nada a fazer
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

O `http.Server` em `main.go` usa `WriteTimeout: 0` por causa do SSE e define `ReadHeaderTimeout: 10s`. O `rc.Flush()` funciona através do `statusRecorder` graças ao `Unwrap()`.

**Teste:**
- A Ana abre o stream com um `http.Client` com o cookie, e a request roda numa goroutine com `bufio.Reader`.
- A Bia (editora) cria uma transação.
- Em até 2 segundos, a Ana lê `data: {"type":"changed"}`. Cancele o contexto no fim.
- Um segundo caso: a Ana criando uma transação **não** gera evento para o próprio stream. Espere 300ms e confirme que nada chegou.

- [ ] **Commit:** `feat(api): SSE por carteira com heartbeat`

---

## Tarefa 16: backup, SPA embutida e `main.go`

**Arquivos:** `internal/db/backup.go` (+ `_test`), `internal/api/spa.go` (+ `_test`), `web/embed.go`, `web/dist/index.html`, `cmd/gastos/main.go`

- [ ] **Passo 1: backup**

```go
// Backup grava uma cópia consistente em dir com timestamp e mantém só as keep mais recentes.
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
	sort.Strings(matches) // timestamp no nome ⇒ ordem lexicográfica = cronológica
	for len(matches) > keep {
		os.Remove(matches[0])
		matches = matches[1:]
	}
	return path, nil
}

// LastBackupAge devolve há quanto tempo foi o último backup (ou um valor enorme se nenhum).
func LastBackupAge(dir string, now time.Time) time.Duration
```

**Testes:**
- 3 backups com `now` diferentes e `keep = 2`: sobram os 2 mais novos.
- O arquivo gerado abre com `db.Open` e tem `user_version = 1`.
- Um backup com o mesmo `now` duas vezes dá erro, e o arquivo existente fica intacto. O `VACUUM INTO` falha se o destino existe, que é o comportamento desejado: nunca sobrescreve.

- [ ] **Passo 2: SPA**

```go
// spaHandler serve arquivos de fsys; caminhos sem arquivo correspondente recebem index.html.
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
		if strings.HasPrefix(p, "assets/") { // arquivos com hash do Vite
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}
```

**Testes** com `fstest.MapFS`:
- `/` devolve o index;
- `/lancamentos` devolve o index;
- `/assets/app.js` devolve o arquivo com cache imutável;
- `/api/x` dá 404 JSON (vem do mux, não da SPA).

- [ ] **Passo 3: placeholder do frontend**

`web/embed.go`:
```go
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist devolve o build do frontend com dist/ como raiz.
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

- [ ] **Passo 4: `cmd/gastos/main.go`**

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

	go mat.Loop(ctx, time.Hour)       // roda uma vez já no início
	go srv.Housekeeping(ctx, time.Hour) // limpa sessões expiradas + backup diário

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

`Server.Housekeeping(ctx, every)` fica em `internal/api/housekeeping.go`. A cada tick, faz `DELETE FROM sessions WHERE expires_at <= now` e, se `db.LastBackupAge > 24h`, chama `db.Backup`. Os SSE abertos seguram o `Shutdown` até o timeout de 10 segundos; é aceitável.

- [ ] **Passo 5: smoke manual**

```bash
GASTOS_COOKIE_SECURE=false go run ./cmd/gastos
curl -s localhost:8000/healthz
curl -s -c /tmp/c -H 'Content-Type: application/json' -d '{"name":"Ana","email":"ana@example.com","password":"senha-segura"}' localhost:8000/api/auth/register
curl -s -b /tmp/c localhost:8000/api/me
```

No PowerShell: `$env:GASTOS_COOKIE_SECURE='false'; go run ./cmd/gastos`. Para os `curl` no Windows, use `curl.exe` e um arquivo de cookie em `$env:TEMP`.

O esperado é `ok`, depois o JSON do usuário e depois `{user, wallets:[{... "name":"Pessoal" ...}]}`.

- [ ] **Passo 6:** commit `feat: main, backup rotativo e SPA embutida`

---

## Tarefa 17: remover o backend antigo

- [ ] **Passo 1:**

```bash
git rm -r src vendor deploy.sh gastos-app
go mod tidy
```

- [ ] **Passo 2:** reescrever o README:
  - stack nova;
  - como rodar em dev (`go run ./cmd/gastos` e, na fase 3, `npm run dev` em `web/`);
  - variáveis de ambiente (tabela da spec);
  - link para o roadmap.

  Remover as seções de API antiga e de segurança pendente que foram resolvidas.
- [ ] **Passo 3: verificação final**

```bash
go vet ./...
go test ./... -race    # -race exige CGO; no Windows sem gcc, rode sem -race
grep -rn "float" internal/
go build -o gastos ./cmd/gastos
```

O esperado:
- `vet` limpo;
- todos os testes passando;
- `grep` sem saída;
- o binário gerado.

Apague o binário depois (ele está no `.gitignore`).

- [ ] **Passo 4:** commit `chore: remove backend antigo, vendor e binário commitado`
- [ ] **Passo 5:** abrir o PR `rewrite/backend → main` com o checklist dos critérios de aceite da spec.

---

## Ordem e paralelismo

```
0 → 1 → 2 → 3 ─┐
          4 ───┼→ 5 → 6+7+8 → 9 → 10 → 11 → 12 → 13 → 14 → 15 → 16 → 17
               │
 (3 e 4 são puras e independentes; podem ir em paralelo com 2)
```
