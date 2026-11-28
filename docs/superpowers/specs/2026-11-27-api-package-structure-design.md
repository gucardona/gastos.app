# Split internal/api into packages by area

Date: 2026-11-27. Issue: #78. Status: awaiting Gustavo's approval. Reviewed by a builder check and an architect review; their findings are folded in.

## Why

`internal/api` is one Go package with 57 files: 27 source files, 30 test files, about 12,000 lines. Opening the folder shows every area mixed together (auth, wallets, transactions, cards, bank import, the assistant), and Gustavo gets lost looking for things. The goal is a layout where each folder is one area of the app and file names say what they hold, without a folder per file.

## Goals

- Each folder under `internal/api/` is one area, with roughly 4 to 12 source files and their tests next to them.
- File names follow `<resource>.go` and `<resource>_<aspect>.go`, so related files sort together.
- No import cycles. Dependencies point one way: root → areas → `httpx`.

## Non-goals

- No change in behavior: same routes, methods, status codes, response bodies, accepted request bodies, error messages, log lines, cookies and headers.
- No new features and no fixes. Anything found along the way gets its own issue. This includes lint findings on moved code (see "Lint").
- No change to `internal/` packages outside `internal/api`, except the call to `CreateInvite` in `cmd/gastos`.
- No change to the web app.

## Anti-goals

- Layer folders (`handlers/`, `services/`, `repositories/`). They spread one area across the tree and cause cycles in Go.
- A folder per file, or a separate folder for tests.
- Interfaces or abstractions added only to make the split compile, when exporting a function does the job.

## How real Go apps do it (research)

- **Gitea** (`routers/api/v1/`): one subpackage per API area (`admin/`, `org/`, `repo/`, `user/`, `shared/`), `api.go` at the root wires every route, and a shared request-context package (`services/context`). Inside an area: one file per resource, with prefixes grouping relatives (`issue.go`, `issue_comment.go`, `issue_label.go`), tests beside the code.
- **Memos** (`server/api/`): one package, strict names (`<resource>_service.go`, `<resource>_service_<aspect>.go`), auth in its own `server/auth/`.
- **PocketBase** (`apis/`) and **Grafana** (`pkg/api/`): one package with prefixed names (`record_auth_*.go`, `middlewares_*.go`). Only generic pieces become subpackages (`apierrors/`).
- **Breaking cycles**: the wiring package imports the areas, the areas import a shared leaf package, never the reverse (Mat Ryer, "How I write HTTP services in Go after 13 years").

## Approaches considered

1. **Subpackages by area, Gitea style (chosen by Gustavo).** Solves "I get lost": each folder is one area. It costs the most: every handler moves, shared helpers become exported, and the test harness becomes its own package.
2. **Move only `assistant/` and `bank/` out.** Half the work, but `api/` keeps about 35 files.
3. **One package with prefixed names, Memos/PocketBase style.** No risk, but still one folder of 57 files. Rejected: it does not fix the problem Gustavo described.

## Layout

```text
internal/api/
  server.go          New, Server, Routes (wires every area), healthz
  middleware.go      logRequests, recoverPanic, devCORS, securityHeaders, sameOriginWrites, statusRecorder
  spa.go             the embedded web app
  events.go          GET /api/events (SSE)
  housekeeping.go    Housekeeping
  routes_test.go     the route table, pinned (see Verification)

  httpx/             the kit every area uses; imports nothing from internal/api
    httpx.go         Env (DB, Cfg, Bus, Log, Now, Mat), InTx, Mutated, NotifyUsers, NotifyWallet, NotifyCtx, WalletMembers,
                     HandlerFunc, Handle, WriteError, RouteWrap, Decode, WriteJSON, PathID, Querier, Role + ParseRole, DateLayout, ValidDate
    errors.go        APIError, the Err* values, FieldErrors, AsAPIError, IsUnique
    context.go       UserIDFrom, WalletIDFrom, RoleFrom, and the setters
    ratelimit.go     RateLimiter

  auth/              sign-up, sessions, profile; imports httpx, wallet, assistant
    routes.go        Handler, Routes
    signup.go        registration mode, registrationInfo, register
    session.go       login, logout, me, startSession, setSessionCookie, RequireUser, clientIP
    profile.go       rename, changePassword
    invite.go        invite codes, CreateInvite (called by cmd/gastos)

  wallet/            wallets and members; imports httpx
    routes.go        Handler, Routes
    wallet.go        WalletDTO, WalletsFor, walletByID, CreateWallet, list/create/update/delete
    wallet_seed.go   seedWallet (default categories and payment methods)
    member.go        memberDTO, list/add/update/remove, setDefaultPaymentMethod
    access.go        RequireWallet

  ledger/            the money: everything that reads or writes transactions; imports httpx
    routes.go        Handler, Routes (transactions, categories, payment methods, budgets, recurrences, settlements, summary)
    transaction.go   Transaction, TransactionInput, validation, list/create/update/delete, SaveTransaction
    transaction_split.go      Split, resolveSplits, WithAmounts, replaceTransactionSplits
    transaction_duplicate.go  checkDuplicate
    category.go      categories
    payment_method.go         payment methods
    card.go          card fields of a payment method, invoice due date
    budget.go        budgets
    recurrence.go    recurrences
    settlement.go    settlements
    settlement_suggest.go     suggestSettlements
    summary.go       the monthly summary, MonthTotals

  bank/              bank accounts, statement import, inbox, rules; imports httpx, ledger
    routes.go        Handler, Routes
    account.go       bank accounts
    statement_import.go       importStatement, csvMapping, storeEntries, loadCandidates
    inbox.go         list, accept, ignore, match, acceptSuggested
    rule.go          categorization rules, reclassify

  assistant/         the query assistant; imports httpx, ledger
    routes.go        Handler, Routes
    chat.go          get/delete/post chat, systemPrompt
    chat_tools.go    the tools the model calls
    settings.go      AI settings, UserAIKey, ErrAIDisabled, ErrAIKeyUnreadable
    cost.go          token prices and spend
    fx.go            USD to BRL rate

  apitest/           the HTTP test harness, used by every area's tests (see "Tests")
```

The package is `auth/`, not `account/`, so it does not clash with bank accounts (`bank/account.go`) and matches the `/api/auth` routes and today's `auth.go`.

`httpx` is a shared package, which AGENTS.md normally avoids ("no util or common"). It is allowed here because it has one job, the HTTP plumbing every area handler uses, and the alternative is an import cycle. `docs/decisions.md` records this.

### Test files

Every `*_test.go` moves with the file it tests and is renamed the same way.

| Today | After |
|---|---|
| auth_test.go | auth/session_test.go |
| registration_test.go | auth/signup_test.go |
| profile_test.go, password_test.go | auth/profile_test.go, auth/password_test.go |
| invites_test.go | auth/invite_test.go |
| wallets_test.go | wallet/wallet_test.go |
| default_method_test.go | wallet/member_test.go |
| transactions_test.go | ledger/transaction_test.go |
| duplicate_test.go | ledger/transaction_duplicate_test.go |
| default_share_test.go | ledger/transaction_split_test.go |
| catalog_test.go | ledger/category_test.go (payment-method cases may go to payment_method_test.go) |
| cards_test.go | ledger/card_test.go; the cases that call the assistant's `runTool` go to assistant/chat_tools_test.go |
| budgets_test.go | ledger/budget_test.go |
| recurrences_test.go, recurrence_skip_test.go | ledger/recurrence_test.go, ledger/recurrence_skip_test.go |
| settlements_test.go | ledger/settlement_test.go |
| settle_test.go | ledger/settlement_suggest_test.go |
| summary_test.go, summary_card_test.go, summary_future_test.go, summary_upcoming_test.go | ledger/ with the same names |
| bank_test.go | bank/, split by resource where the cases separate cleanly |
| chat_test.go | assistant/chat_test.go |
| assistant_tools_test.go | assistant/chat_tools_test.go |
| ai_settings_test.go | assistant/settings_test.go |
| ai_cost_test.go | assistant/cost_test.go |
| fx_test.go | assistant/fx_test.go |
| events_test.go | events_test.go (root) |
| security_test.go | middleware_test.go (root) |
| ops_test.go | root, next to housekeeping and healthz |
| gaps_test.go, limits_test.go, units_test.go | the area each case covers (they mix areas today) |
| testutil_test.go | apitest/apitest.go |

## How the code changes

1. **Shared environment.** Today every handler is a method on `*api.Server`. After the split, `httpx.Env` holds what handlers share (`DB`, `Cfg`, `Bus`, `Log`, `Now`, `Mat`). Each area has `type Handler struct { *httpx.Env; ... }` with its own extra state (the login limiter in `auth`; the chat limiter, in-flight map and Anthropic URL in `assistant`). Handlers keep the receiver name `s`, so bodies change only where a name became exported (`s.db` → `s.DB`, `writeJSON` → `httpx.WriteJSON`).
2. **Export only what crosses a package.** A function moves as is and becomes exported only when another package calls it (`ledger.SaveTransaction` for the inbox, `wallet.CreateWallet` for sign-up, `UserAIKey` for `me`). Types that exported functions take or return are exported too, so the lint's `unexported-return` stays quiet: `WalletDTO`, `TransactionInput`, `Split`, `Transaction`. `auth.Handler` holds a pointer to the `assistant.Handler` and calls `s.AI.UserAIKey`. Every exported name gets a doc comment.
3. **Exported fields on decoded types keep the wire format.** `decode` matches JSON keys without regard to case and rejects unknown fields. Exporting a field that has no JSON tag would make the API accept a new key. So every field exported by this refactor on a type that is decoded from a request carries an explicit tag. `transactionInput.invoiceDue`, which `bank` reads, becomes `InvoiceDue *string `json:"-"``, so a body with `"invoiceDue"` still gets a 400.
4. **Routes.** `api.New` builds `Env` and each area's `Handler` once, because `Housekeeping` (FX refresh), the root middleware (`now`) and the test harness all need the same `Env` whether or not `Routes` has run. `Server.Routes` calls `area.Routes(mux, wrap)` the way `catalogRoutes(mux, wallet)` works today. The local `wallet := func(...)` in `Routes` is renamed `inWallet`, so it does not hide the `wallet` package. Middleware order and every pattern string stay identical.
5. **`bcryptCost`.** Today `_bcryptCost = 12` in wallets.go, used only by auth.go, and lowered by an `init` in the test helper. It moves to `auth` as the exported `auth.BcryptCost = 12`, and `apitest` lowers it in its own `init`. Production keeps cost 12. `_dummyHash` is a package var computed before any `init`, so it keeps cost 12 in tests too, as today.
6. **Production accessors for the harness.** `apitest` lives in another package and cannot reach `Server`'s unexported state. `api.Server` gains exactly two methods, `Env() *httpx.Env` and `Assistant() *assistant.Handler`, each with a doc comment saying it exists for tests. No other production API is added.
7. **`cmd/gastos`.** Calls `auth.CreateInvite` instead of `api.CreateInvite`. `api.New`, `Server.Routes` and `Server.Housekeeping` keep their signatures.

## Tests

- **Why external test packages.** `apitest` imports `api`, which imports every area, so a test in `package ledger` that uses `apitest` is an import cycle. Every test that uses the harness is therefore an external test package (`package ledger_test`, `package api_test`). Pure unit tests of unexported functions that do not use the harness stay in the area's own package.
- **`export_test.go`.** An area whose external tests call unexported functions (`runTool`, `busy`, `fxRate`, `refreshFX`, `userAIKey`, `notifyWallet`) gets an `export_test.go` in its own package that exposes them. It is the standard Go pattern and is compiled only for that package's tests, so it never reaches production.
- **`apitest` API.** Ported from `testutil_test.go`, exported:
  - `New(t) *App` and `NewWith(t, tweak func(*config.Config)) *App`.
  - `App.Register(name, email) *Client`, `App.Anon() *Client`, `App.Server() *api.Server` (and through it `Env()` and `Assistant()`), `App.SetNow(func() time.Time)`.
  - `Client.Do`, `Client.DoRaw`, `Client.Status`, `Client.Upload`, `Client.WPath`, `Client.User`, `Client.WalletID`, and `Itoa`.
  - The cookie jar that follows the server clock, and the fake SPA.
- **Shared test helpers.** A helper used by tests in more than one area moves to `apitest`, exported. A helper used by one area stays in that area's tests. Known cross-area helpers: `firstCategory`, `shareWallet`, `monthsAgoFirst`, `listRange`, `newInvite`, `purchase`, `card`, `ofxRows`, `newAccount`, `importFile`, `openStream`, `newAIApp`, `aiTestKey`, `runHousekeeping`, `backups`, `countRows`. The compiler finds any the list misses.
- No test is deleted or weakened. `export_test.go` files, the `apitest` package and the two accessors count as test harness in the diff review.

## Lint

The split puts moved code in new files, so `golangci-lint` (`new-from-merge-base`) sees every moved line as new. The findings already listed in `docs/security.md` (gosec G202, G120, G124, G101, and errcheck) will fire on code that has not changed. They are not fixed in this PR. Each one gets `//nolint:<linter> // moved unchanged, see docs/security.md`. Fixes go to their own issues.

## Order of work (each step leaves everything green)

1. **Pin behavior.** Write `routes_test.go` against today's code (see Verification). Then record the baseline: the sorted list of test names from `go test -v ./... | grep '^=== RUN'`.
2. **`httpx/`.** Create it and move the shared plumbing into it. `Server` embeds `*httpx.Env`.
3. **Rename in place.** Still in the flat package, do every rename and export with `gopls rename` (safe by construction): `s.db` → `s.DB`, `writeJSON` → `httpx.WriteJSON`, `saveTransaction` → `SaveTransaction`, the local `wallet` → `inWallet`, and so on. One commit, no code moves.
4. **Move, area by area.** `wallet/`, then `ledger/` (the biggest), then `auth/`, `bank/`, `assistant/`, with `apitest/` created alongside the first area. Each area is one commit that only moves code: function bodies are byte-identical to step 3 once package qualifiers are stripped.
5. **Root cleanup.** `server.go`, `middleware.go`, `events.go`.
6. **Docs and lint.** Doc comments, `//nolint` notes on moved findings, `docs/decisions.md`, and the file references in `docs/security.md` (lines 23 and 58-61).

After each step: `go build ./...`, `go vet ./...`, `go test -race ./...`.

## Verification that nothing changed

- **Route table.** `http.ServeMux` cannot list its patterns, and wrapping it would change `Server.Routes`. So `routes_test.go` (package `api_test`, using `api.New` and `httptest` directly, not `apitest`) holds a hand-written table of every method + path in today's `Routes`, with the minimum role of each. For each route it asserts:
  - without a session: 401 (the session check runs before the wallet check);
  - as a non-member of the wallet: 404 with the `not_found` body, as today (`requireWallet` hides wallets from non-members);
  - as a reader: 403 on editor and owner routes, not 403 on reader routes; as an editor: 403 on owner routes;
  - as the owner: the status and error code today's code returns, pinned in the table (golden values).

  Only `{walletID}` gets a real id; other wildcards get a missing id, because a real row per route would make the probe a second test suite. A wildcard renamed without its `PathValue` is caught instead by the moved-code check (pattern strings move verbatim) and by the existing tests, which call routes with real ids.

  It also pins the middleware: the security headers and CSP on a response, and a cross-site POST rejected. It is written first, against today's code, and must pass unchanged at the end.
- **Test names.** The sorted list of test names after the branch equals the baseline from step 1. Tests that moved package show up under the same name.
- **Moved code.** For each move commit, strip package qualifiers and compare every function body with the step 3 commit. Any difference other than qualifiers is a finding.
- **E2E.** `npm run build && npm run e2e` on desktop and phone: same pass count as master.
- **Reviewers.** An architect reviewer on the finished branch and a QA reviewer on the test move.

## Risks

- **Hidden init order.** Package-level vars (`_dummyHash`, regexes) are computed at init. They move with their package and keep their order, so nothing changes at runtime. Every test binary that imports `apitest` computes the cost-12 `_dummyHash` at init: tests get slower to start, and behave as today.
- **Large diff.** About 12,000 lines move. Doing every rename in place first (step 3) is what keeps the move commits reviewable.

## Left to the builder

- Exact split of `auth.go` and `wallets.go` lines among the files listed, following the function names above.
- Whether a test that covers two areas goes with one or is split, as long as no assertion is lost.
- Names of unexported helpers.
