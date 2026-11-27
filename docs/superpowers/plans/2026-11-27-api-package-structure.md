# Split internal/api by area Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the flat `internal/api` package (57 files) into Gitea-style area packages (`httpx`, `auth`, `wallet`, `ledger`, `bank`, `assistant`, `apitest`) without changing any behavior.

**Architecture:** First pin today's behavior with a golden route probe. Then do every rename and export in place, still inside the flat package. Then move code area by area in commits that only move code. Root `api` wires the areas; areas import `httpx`; `bank` and `assistant` import `ledger`; `auth` imports `wallet` and `assistant`.

**Tech Stack:** Go 1.27, `net/http` ServeMux patterns, SQLite, `gopls rename`, golangci-lint.

**Spec:** `docs/superpowers/specs/2026-11-27-api-package-structure-design.md`

## Global Constraints

- Zero behavior change: same routes, methods, status codes, response bodies, accepted request bodies, error messages, log lines, cookies and headers.
- No fixes in this PR. Lint findings on moved code get `//nolint:<linter> // moved unchanged, see docs/security.md`.
- No change outside `internal/api`, except `cmd/gastos` calling `auth.CreateInvite`.
- Production gains exactly two accessors: `(*api.Server).Env() *httpx.Env` and `(*api.Server).Assistant() *assistant.Handler`.
- Every exported field on a request-decoded type has an explicit JSON tag; `TransactionInput.InvoiceDue` is `json:"-"`.
- `auth.BcryptCost = 12` in production; `apitest` lowers it to `bcrypt.MinCost` in `init`.
- Every package and every top-level declaration has a Go doc comment starting with its name. English in code; pt-BR strings stay byte-identical.
- After every task: `go build ./... && go vet ./... && go test -race ./...` green.
- Commits: Conventional Commits, `refactor:` (or `test:` for task 1), body says why, `Refs #78`.

## Review Focus

1. A route's minimum role silently changes when `roleEditor` is retyped as `httpx.RoleEditor`: the golden probe (Task 1) pins the reader/editor/owner status per route.
2. A request body that is a 400 today becomes accepted after a field is exported: Task 3 adds `TestInvoiceDueNotAccepted` (POST a transaction with `"invoiceDue"` → 400).
3. Package-level state (`_dummyHash`, limiters, regexes) initialized differently after the move: Task 6 keeps `_dummyHash` as a package var in `auth` and the probe covers login.
4. A middleware lost or reordered when `Routes` is rewired: the probe pins security headers, CSP and the cross-site POST rejection.
5. A test silently dropped during the move: Task 10 diffs the sorted list of test names against the Task 1 baseline.

---

### Task 1: Pin today's behavior

**Files:**
- Create: `internal/api/routes_test.go` (package `api_test`)
- Create (not committed): `.superpowers/sdd/<plan>/baseline-tests.txt`

**Interfaces:**
- Produces: `routes_test.go`, which must pass unchanged at the end of Task 10. It uses only `api.New`, `(*api.Server).Routes`, `httptest`, `internal/db`, `internal/config`, `internal/events`, `internal/recurrence` (as `testutil_test.go` does today), never `apitest`.

- [ ] **Step 1: Write `TestRouteTable`**

A table with one row per `mux.Handle` call (63 today; `grep -h 'mux.Handle' internal/api/*.go`) plus `GET /healthz` and `GET /`:

```go
type probe struct {
	method, path string // path with {walletID} and other wildcards already filled, see below
	anon, outsider, reader, editor, owner int // status codes pinned from today's run
}
```

For each row: build a fresh server (fresh temp DB). Register `owner`, then `editor`, `reader` and `outsider`. The owner adds `editor` and `reader` to the owner's default wallet with those roles; `outsider` stays outside. Probe in the order anon, outsider, reader, editor, owner (destructive owner calls go last), with body `{}` for POST/PUT/PATCH. `{walletID}` is the owner's wallet. Every other wildcard is `999999`. For `GET /api/events`, read only the status and close the body. Each probe records `status` plus the JSON `code` field when the body is JSON. Pin both: the table carries `codes [5]string` too.

Assertions, also checked in the test itself:
- `anon` is 401 on every `user`/`wallet` route.
- `outsider` is 404 with code `not_found` on every `wallet` route.
- `reader` is 403 with code `forbidden` on every editor/owner route.

Fill the expected values by running the test against today's code and copying the actual values into the table. Every value must then be read and checked against the rules above before commit.

- [ ] **Step 2: Write `TestMiddleware`**

On `GET /api/me` as a user: the security headers and CSP match what `security_test.go`'s `checkCommonHeaders` asserts today. Assert them literally, without calling that helper. A cross-site `POST /api/wallets` (`Sec-Fetch-Site: cross-site`) is rejected with today's status and code.

- [ ] **Step 3: Run it**

Run: `go test ./internal/api -run 'TestRouteTable|TestMiddleware' -v`
Expected: PASS against today's code.

- [ ] **Step 4: Record the baseline**

Run: `go test -race -v ./internal/api/... 2>&1 | grep -o '^=== RUN.*' | sed 's/=== RUN *//' | sort > <workspace>/baseline-tests.txt`
Expected: about 237 top-level tests plus subtests.

- [ ] **Step 5: Commit** `test: pin the API route table before splitting the package`

### Task 2: `httpx` package and `Env`

**Files:**
- Create: `internal/api/httpx/httpx.go`, `errors.go`, `context.go`, `ratelimit.go`
- Modify: every `internal/api/*.go` that uses the moved names; `server.go` (`Server` embeds `*httpx.Env`)

**Interfaces:**
- Produces (package `httpx`):
  - `type Env struct { DB *sql.DB; Cfg config.Config; Bus *events.Bus; Mat *recurrence.Materializer; Log *slog.Logger; Now func() time.Time }`
  - Methods on `*Env`: `Handle(HandlerFunc) http.Handler`, `WriteError(w, r, error)`, `InTx(ctx, func(*sql.Tx) error) error`, `Mutated(*http.Request)`, `NotifyUsers(...)`, `NotifyWallet(ctx, walletID int64)`, `WalletMembers(...)`.
  - Functions: `NotifyCtx`, `Decode`, `WriteJSON`, `PathID`, `ParseRole`, `ValidDate`, `AsAPIError`, `IsUnique`, `ValidationError` (today's validation constructor), `ErrBadRequest(msg)`, `ErrConflict(msg)`, `ErrNotFoundMsg(msg)`.
  - Vars: `ErrUnauthenticated`, `ErrForbidden`, `ErrNotFound`, `ErrRateLimited`.
  - Types: `HandlerFunc`, `RouteWrap func(Role, HandlerFunc) http.Handler`, `Querier`, `APIError`, `FieldErrors`, `Role` with `RoleReader`, `RoleEditor`, `RoleOwner`, `RateLimiter` with `NewRateLimiter(n int, per time.Duration)` and `Allow`.
  - Consts: `DateLayout`.
  - Context: `UserIDFrom`, `WalletIDFrom`, `RoleFrom`, `WithUser(ctx, id)`, `WithWallet(ctx, id, Role)`.
  - Exact parameter lists are today's, unchanged.

- [ ] **Step 1:** Move the declarations above from `httpx.go`, `events.go` (notify helpers only; the SSE handler stays), `auth.go` (context keys, `rateLimiter`), `wallets.go` (`role`) and `transactions.go` (`dateLayout`, `validDate`). Use `gopls rename` for each identifier first, then cut and paste. Bodies stay identical.
- [ ] **Step 2:** `Server` embeds `*httpx.Env`; `New` builds it. `s.db` → `s.DB`, `s.now` → `s.Now`, and so on, via `gopls rename` on the fields before embedding.
- [ ] **Step 3:** Run `go build ./... && go vet ./... && go test -race ./internal/api/...`. Expected: PASS, `TestRouteTable` unchanged.
- [ ] **Step 4: Commit** `refactor: move shared HTTP plumbing to internal/api/httpx`

### Task 3: Rename and export in place

**Files:**
- Modify: `internal/api/*.go` (no file moves)

**Interfaces:**
- Produces, all still in package `api`:
  - `CreateWallet`, `WalletsFor`, `WalletDTO`, `RequireWallet`, `RequireUser`.
  - `SaveTransaction`, `TransactionInput` (with `InvoiceDue *string `json:"-"``), `Transaction`, `Split`, `WithAmounts`, `MonthTotals`, `InvoiceDue` (the card helper bank uses).
  - `UserAIKey`, `ErrAIDisabled`, `ErrAIKeyUnreadable`, `BcryptCost`.
  - The `Routes` local `wallet` becomes `inWallet`.
  - The compiler decides any other name that later crosses a package. Rename it here, not during a move.

- [ ] **Step 1: Write `TestInvoiceDueNotAccepted`** in `transactions_test.go`: POST a valid expense with an extra `"invoiceDue": "2026-12-10"` → 400, code `validation`.
- [ ] **Step 2:** Run it. Expected: PASS on today's code. It pins current behavior.
- [ ] **Step 3:** Every rename with `gopls rename`. Every exported field on a request-decoded struct gets an explicit `json` tag equal to its current wire name.
- [ ] **Step 4:** Run the full suite. Expected: PASS, including `TestInvoiceDueNotAccepted` and `TestRouteTable`.
- [ ] **Step 5: Commit** `refactor: export the names that will cross packages`

### Task 4: `wallet/` and `apitest/`

**Files:**
- Create: `internal/api/wallet/{routes,wallet,wallet_seed,member,access}.go`, `wallet/wallet_test.go`, `wallet/member_test.go`
- Create: `internal/api/apitest/apitest.go`
- Modify: `server.go` (wire `wallet.Routes`), `auth.go`, the other callers

**Interfaces:**
- Produces:
  - `wallet.Handler{ *httpx.Env }` and `wallet.Routes(mux *http.ServeMux, user func(httpx.HandlerFunc) http.Handler, inWallet httpx.RouteWrap)`.
  - Exported functions: `wallet.CreateWallet`, `wallet.WalletsFor`, `wallet.RequireWallet`.
  - `apitest` gets the API listed in the spec's "Tests" section, plus the cross-area helpers that wallet tests need (`ShareWallet`, `FirstCategory`, …). Each helper is added when the first area test needs it.
  - `api.Server` gets `Env() *httpx.Env` here. `Assistant()` arrives in Task 8.
- Note: root tests keep using the internal `testutil_test.go` until Task 9. Both harnesses coexist until then.

- [ ] **Step 1:** Create `apitest` by porting `testutil_test.go` (cookie jar, fake SPA, `init` lowering `api.BcryptCost`).
- [ ] **Step 2:** Move the wallet declarations (spec Layout) and their tests (`package wallet_test`, plus `export_test.go` if needed). Bodies are byte-identical to Task 3 except package qualifiers.
- [ ] **Step 3:** Run the full suite. Expected: PASS, `TestRouteTable` unchanged.
- [ ] **Step 4: Commit** `refactor: move wallets and members to internal/api/wallet`

### Task 5: `ledger/`

**Files:**
- Create: `internal/api/ledger/` files per spec Layout and their tests per the spec's test table (`cards_test.go`'s `runTool` cases stay in the root until Task 8).
- Modify: `server.go`, `bank.go`, `chat.go`, `assistant_tools.go`, `summary.go` callers

**Interfaces:**
- Produces: `ledger.Handler{ *httpx.Env }`, `ledger.Routes(mux, user, inWallet)`, and the exported names from Task 3 under `ledger.`.

- [ ] **Step 1:** Move the code and tests. Bodies are byte-identical except qualifiers.
- [ ] **Step 2:** Run the full suite. Expected: PASS.
- [ ] **Step 3: Commit** `refactor: move transactions and the money to internal/api/ledger`

### Task 6: `auth/`

**Files:**
- Create: `internal/api/auth/{routes,signup,session,profile,invite}.go` and the tests per the spec table
- Modify: `server.go`, `cmd/gastos/invite.go:66` (`auth.CreateInvite`)

**Interfaces:**
- Produces: `auth.Handler{ *httpx.Env; limiter *httpx.RateLimiter; AI <assistant handle> }`, `auth.Routes`, `auth.RequireUser`, `auth.CreateInvite`, `auth.BcryptCost`.
- Until Task 8, `AI` points at whatever value provides `UserAIKey` today. Use the narrowest form that compiles. In Task 8 it becomes `*assistant.Handler`, the spec's design.
- `_dummyHash` stays a package-level var in `auth`.

- [ ] **Step 1:** Move code and tests.
- [ ] **Step 2:** Run the full suite and `go build ./cmd/gastos`. Expected: PASS.
- [ ] **Step 3: Commit** `refactor: move sign-up, sessions and profile to internal/api/auth`

### Task 7: `bank/`

**Files:**
- Create: `internal/api/bank/{routes,account,statement_import,inbox,rule}.go` and the tests split from `bank_test.go`

- [ ] **Step 1:** Move code and tests.
- [ ] **Step 2:** Run the full suite. Expected: PASS.
- [ ] **Step 3: Commit** `refactor: move bank import and the inbox to internal/api/bank`

### Task 8: `assistant/`

**Files:**
- Create: `internal/api/assistant/{routes,chat,chat_tools,settings,cost,fx}.go`, their tests, `export_test.go` (`runTool`, `busy`, `fxRate`, `refreshFX`, `userAIKey` as needed), and `chat_tools_test.go` (which takes the `runTool` cases from `cards_test.go`)

**Interfaces:**
- Produces: `assistant.Handler{ *httpx.Env; chatLimiter *httpx.RateLimiter; chatMu sync.Mutex; chatInFlight map[int64]bool; anthropicURL string }`, `assistant.Routes`, `(*assistant.Handler).UserAIKey`, `assistant.ErrAIDisabled`, `assistant.ErrAIKeyUnreadable`.
- `(*api.Server).Assistant() *assistant.Handler`. `Housekeeping` calls the assistant's FX refresh through it.

- [ ] **Step 1:** Move code and tests. `auth.Handler.AI` becomes `*assistant.Handler`.
- [ ] **Step 2:** Run the full suite. Expected: PASS.
- [ ] **Step 3: Commit** `refactor: move the query assistant to internal/api/assistant`

### Task 9: Root cleanup

**Files:**
- Modify/Create: `internal/api/server.go`, `middleware.go`, `events.go`, `events_test.go`, `middleware_test.go` (from `security_test.go`), `ops_test.go`, `export_test.go` if needed
- Delete: `internal/api/testutil_test.go`, after the last root test moves to `apitest`

- [ ] **Step 1:** Split `server.go` into `server.go` and `middleware.go`. Move the root tests to `package api_test`. Rehome the cases from `gaps_test.go`, `limits_test.go` and `units_test.go` per the spec table.
- [ ] **Step 2:** Run the full suite. Expected: PASS. `ls internal/api` shows only the root files in the spec Layout.
- [ ] **Step 3: Commit** `refactor: keep only wiring and middleware in internal/api`

### Task 10: Docs, lint and proof

**Files:**
- Modify: doc comments everywhere they are missing, `docs/decisions.md` (one entry: the area split and why `httpx` is allowed despite the "no util/common" rule), `docs/security.md:23,58-61` (new file paths)

- [ ] **Step 1:** Run `golangci-lint run`. Add doc comments. Moved findings get `//nolint:<linter> // moved unchanged, see docs/security.md`. Expected: 0 issues.
- [ ] **Step 2: Test names.** Rerun the Task 1 baseline command into `after-tests.txt`, then `diff baseline-tests.txt after-tests.txt`. Expected: only `TestInvoiceDueNotAccepted` added. Rows that moved package keep the same name.
- [ ] **Step 3: Moved code.** For each move commit (Tasks 4 to 9), write a throwaway script in the workspace. It extracts each top-level func body with `go/ast` from the Task 3 commit and from HEAD, strips package qualifiers (`httpx.`, `ledger.`, `wallet.`, `auth.`, `assistant.`, `api.`), and compares. Expected: no differences.
- [ ] **Step 4:** `TestRouteTable` and `TestMiddleware` pass with the file unchanged since Task 1: `git diff <task1-commit> HEAD -- internal/api/routes_test.go` is empty.
- [ ] **Step 5:** `cd web && npm run build && npm run e2e`. Expected: same pass count as master.
- [ ] **Step 6: Commit** `docs: record the internal/api split`
