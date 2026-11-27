# Financial assistant, part 1 (foundation and querying) — implementation plan

> **For agents:** REQUIRED SUB-SKILL: use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to run this plan task by task. Steps use `- [ ]` checkboxes.

**Goal:** the user puts their own Anthropic key in Ajustes (Settings), turns the assistant on and gets a Chat tab that answers questions about their expenses (read-only), with simple charts.

**Architecture:** the Go server stores the encrypted key (AES-GCM), runs the model loop (streaming, fixed read tools, at most 6 turns) and relays the text to the browser over SSE. The tools only see the wallets selected by the user and reuse the existing membership rules. The schema becomes a single `schema.sql` (migrations removed; the production database is recreated on deploy).

**Technologies:** Go (`net/http`, `crypto/aes`, `database/sql` + modernc sqlite), React 19 + TanStack Query, simple SVG for charts. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-10-15-query-assistant-design.md`

## Global constraints

- Code, comments and tests in English; interface texts and documentation in pt-BR.
- Exact model IDs: `claude-haiku-5-5`, `claude-sonnet-5-5` (default), `claude-opus-5-5`.
- Retention: integer from 1 to 30, default 7. Question text: 1 to 4000 characters. Loop turns: at most 6. `max_tokens` per answer: 1024. Usage limit: 20 messages per minute per user. One answer in progress per user.
- Master key: `GASTOS_SECRET_KEY` = 32 bytes in base64. Absent → assistant unavailable, server starts normally. Present and malformed → configuration error.
- The Anthropic key never shows up in an API response, log or error message; the API only shows `keyHint` (last 4 characters).
- Amounts always in cents in the tools; dates `YYYY-MM-DD` in the `cfg.Location` time zone.
- Visual: tokens, contrast, focus and the rules in `alignment.test.ts` and `focus.test.ts` still apply. No centered text outside the exceptions.
- An empty conversation shows only the text field (no suggestions).
- Before each commit: `go vet ./...`, `go test -race ./...`, `npx tsc --noEmit`, `npm run lint`, `npx vitest run` (in `web/`). Commit messages end with the `Co-Authored-By` line indicated by the environment.

## Review Focus

- Two sends at the same time from the same user (two tabs, double click): the second gets 409 "já existe uma resposta em andamento" (an answer is already in progress) and nothing is written twice. → Task 6.
- The browser closes the connection in the middle of the answer: the call to Anthropic is canceled, the user message stays saved and no incomplete answer is saved. → Task 6.
- Someone who left a wallet (or was never a member) picks that wallet in the chat: 403, no data. → Task 6.
- `GASTOS_SECRET_KEY` changed after saving the key: the chat returns the message "informe a chave de novo" (enter the key again) (not 500) and `GET /api/me/ai` shows `configured: false`. → Tasks 4 and 6.
- Bad parameters coming from the model (inverted period, longer than 5 years, limit out of range, unknown field such as `wallet_id`): the tool returns an error to the model (`is_error`), never queries and never brings the chat down. A period without transactions returns an empty list, not an error. → Task 5.

---

### Task 1: single schema, no migrations, with the assistant tables

**Files:**
- Modify: `internal/db/schema.sql`, `internal/db/db.go`
- Remove: `internal/db/migrations/0002_bank_import.sql`, `internal/db/migration_test.go`
- Test: `internal/db/db_test.go`

**Interfaces:**
- Produces: `db.Open(path string) (*sql.DB, error)` (same signature) that creates the full schema on an empty database and is idempotent (opening twice neither fails nor loses data).
- Produces: tables `ai_settings(user_id PK→users ON DELETE CASCADE, api_key_enc BLOB NOT NULL, key_hint TEXT NOT NULL, model TEXT NOT NULL DEFAULT 'claude-sonnet-5-5' CHECK (model IN (...the three IDs...)), enabled INTEGER NOT NULL DEFAULT 0, retention_days INTEGER NOT NULL DEFAULT 7 CHECK (retention_days BETWEEN 1 AND 30), updated_at TEXT)` and `chat_messages(id PK, user_id→users CASCADE, wallet_id→wallets CASCADE, role CHECK IN ('user','assistant'), content TEXT, charts TEXT NOT NULL DEFAULT '[]', wallet_ids TEXT NOT NULL DEFAULT '[]', input_tokens INTEGER NOT NULL DEFAULT 0, output_tokens INTEGER NOT NULL DEFAULT 0, created_at TEXT default now UTC)` with indexes `(user_id, wallet_id, id)` and `(created_at)`.

- [ ] **Step 1: failing tests** in `internal/db/db_test.go`: `TestOpenCreatesFullSchema` (opens a new database and checks that `bank_accounts`, `bank_transactions`, `categorization_rules`, `ai_settings`, `chat_messages` and the column `transactions.source` with the CHECK for `'manual','recurring','bank'` exist); `TestOpenIsIdempotent` (opens, inserts a user, closes, reopens, the user is still there); `TestAISettingsConstraints` (retention 0 and 31 and a model outside the list are refused; 1 and 30 pass).
- [ ] **Step 2: run** `go test ./internal/db` — expected: FAILS (tables do not exist).
- [ ] **Step 3: merge the content of `0002_bank_import.sql` into `schema.sql`** (the `source` column goes straight into the `transactions` definition; the backfill `UPDATE` goes away; the three bank tables and their indexes come in) and add the two new tables. In `db.go`, remove `migrations`, `migrationBankImport`, the migration embed directives and the `migrate` function; `Open` runs `schemaSQL` only when `PRAGMA user_version = 0` and then writes `PRAGMA user_version = 1`, inside a transaction. Delete the `migrations` folder and `migration_test.go`.
- [ ] **Step 4: run** `go test ./...` — expected: PASSES (fix any test that mentioned migrations).
- [ ] **Step 5: commit** `feat(db): single schema, assistant tables, drop migrations`.

---

### Task 2: configuration and key encryption

**Files:**
- Create: `internal/secret/secret.go`, `internal/secret/secret_test.go`
- Modify: `internal/config/config.go`, `internal/config/config_test.go`

**Interfaces:**
- Produces: `secret.ParseKey(b64 string) ([]byte, error)` (error if it does not decode to exactly 32 bytes); `secret.Seal(key, plaintext []byte) ([]byte, error)` (random nonce in front of the ciphertext); `secret.Open(key, sealed []byte) ([]byte, error)` (error if the key is different or the data was altered).
- Produces: `config.Config.SecretKey []byte` (nil if `GASTOS_SECRET_KEY` is empty) and `config.Config.AnthropicURL string` (from `GASTOS_ANTHROPIC_URL`, default `https://api.anthropic.com`; it exists for the e2e tests).

- [ ] **Step 1: failing tests**: `TestSealOpenRoundTrip`; `TestOpenWithAnotherKeyFails`; `TestOpenTamperedFails`; `TestSealUsesFreshNonce` (two encryptions of the same text differ); `TestParseKeyRejectsWrongLength`. In `config_test.go`: key absent → `SecretKey == nil`; valid key → 32 bytes; malformed key → `Load` error; `AnthropicURL` default and overridden.
- [ ] **Step 2: run** `go test ./internal/secret ./internal/config` — expected: FAILS.
- [ ] **Step 3: implement** with AES-256-GCM from the standard library and `Load` according to the interfaces.
- [ ] **Step 4: run** the same tests — expected: PASSES.
- [ ] **Step 5: commit** `feat(config): GASTOS_SECRET_KEY and AES-GCM sealing for stored API keys`.

---

### Task 3: Anthropic API client (streaming and tools)

**Files:**
- Create: `internal/anthropic/client.go`, `internal/anthropic/client_test.go`

**Interfaces:**
- Produces:
  - `type Client struct { BaseURL, APIKey string; HTTP *http.Client }`
  - `type Block struct { Type string; Text string; ID, Name string; Input json.RawMessage; ToolUseID string; Content string; IsError bool }` with the API's JSON tags (`text`, `tool_use`, `tool_result`).
  - `type Message struct { Role string; Content []Block }`
  - `type Tool struct { Name, Description string; InputSchema json.RawMessage }`
  - `type Request struct { Model, System string; Messages []Message; Tools []Tool; MaxTokens int }`
  - `type Result struct { Blocks []Block; StopReason string; InputTokens, OutputTokens int }`
  - `type APIError struct { Status int; Kind, Message string }` (implements `error`; never contains the key)
  - `func (c *Client) Stream(ctx context.Context, req Request, onText func(string)) (Result, error)`
  - `func (c *Client) Check(ctx context.Context, model string) error` (minimal call with `max_tokens: 1`)
- Contract: `POST {BaseURL}/v1/messages` with headers `x-api-key`, `anthropic-version: 2023-06-01`, `content-type: application/json` and `"stream": true`. Reads the SSE events `content_block_start`, `content_block_delta` (`text_delta`, `input_json_delta`), `content_block_stop`, `message_delta` (stop_reason, usage) and `error`. Text reaches `onText` as the deltas arrive.

- [ ] **Step 1: failing tests** with `httptest.Server`: `TestStreamTextDeltas` (several `text_delta` call `onText` in order and `Result.Blocks` carries the whole text); `TestStreamToolUse` (a `tool_use` block with input JSON assembled from several `input_json_delta`; `StopReason == "tool_use"`); `TestStreamSendsHeadersAndBody` (headers, `model`, `tools`, `stream:true`); `TestStreamHTTPErrors` (401, 429 and 529 become `*APIError` with `Status` and without the key in the message); `TestStreamCancelStopsRequest` (canceling the context interrupts and returns `context.Canceled`); `TestCheck` (200 → nil; 401 → `*APIError`).
- [ ] **Step 2: run** `go test ./internal/anthropic` — expected: FAILS.
- [ ] **Step 3: implement** with `net/http` and a `bufio.Scanner` line reader with a large buffer; no dependencies.
- [ ] **Step 4: run** — expected: PASSES.
- [ ] **Step 5: commit** `feat(anthropic): minimal streaming client with tool use`.

---

### Task 4: assistant settings in the API

**Files:**
- Create: `internal/api/ai_settings.go`, `internal/api/ai_settings_test.go`
- Modify: `internal/api/server.go` (routes, Anthropic client field), `internal/api/auth.go` (`me` returns `assistantEnabled`)

**Interfaces:**
- Consumes: `secret.Seal/Open`, `anthropic.Client.Check`, `config.Config.SecretKey/AnthropicURL`.
- Produces routes (all with `user(...)`): `GET /api/me/ai` → `{available, configured, keyHint, model, enabled, retentionDays}`; `PUT /api/me/ai` with `{apiKey?, model?, enabled?, retentionDays?}`; `DELETE /api/me/ai/key` (204; removes the key and turns it off).
- Produces: `func (s *Server) userAIKey(ctx context.Context, userID int64) (key string, model string, err error)` that returns `errAIKeyUnreadable` when decryption fails and `errAIDisabled` when there is no key or `enabled` is false; `GET /api/me` now includes `assistantEnabled` (true only with a readable key and `enabled`).
- Rules: a missing `apiKey` keeps the current one; `enabled:true` without a key → field error; retention outside 1–30 and a model outside the list → per-field errors; without `SecretKey` on the server, `available:false` and the `PUT` with `apiKey` responds 400 "assistente indisponível neste servidor" (assistant unavailable on this server); when saving a new key, `Check` validates it (401 → `apiKey` field error "chave recusada pela Anthropic" (key rejected by Anthropic); network failure → saves and responds with `warning`).

- [ ] **Step 1: failing tests** in `ai_settings_test.go` (fake Anthropic server via `httptest` and `AnthropicURL` in `newTestAppWith`): `TestAISettingsDefaults`; `TestSaveKeyNeverReturnsIt` (the response and the `GET` show only `keyHint` and the key does not appear in any body); `TestSaveKeyStoredEncrypted` (the `api_key_enc` column does not contain the key text); `TestRejectedKeyIsNotSaved`; `TestRetentionAndModelValidation`; `TestEnableNeedsKey`; `TestUnavailableWithoutSecretKey`; `TestRemoveKeyDisables`; `TestKeyMissingAfterSecretRotation` (changes the test server's `SecretKey`; `configured:false`, `assistantEnabled:false`); `TestMeReportsAssistantEnabled`; `TestSettingsArePerUser` (Ana's key does not show up for Bia).
- [ ] **Step 2: run** `go test ./internal/api -run 'AISettings|SaveKey|Retention|Enable|Unavailable|RemoveKey|SecretRotation|MeReports|PerUser'` — expected: FAILS.
- [ ] **Step 3: implement** handlers and routes according to the interfaces, using `decode`, `fieldErrors` and `errBadRequest`; `New` creates the client with `cfg.AnthropicURL` (or the default).
- [ ] **Step 4: run** this step's tests and `go test ./internal/api` — expected: PASSES.
- [ ] **Step 5: commit** `feat(api): assistant settings with an encrypted per-user API key`.

---

### Task 5: advisor read tools

**Files:**
- Create: `internal/api/assistant_tools.go`, `internal/api/assistant_tools_test.go`

**Interfaces:**
- Produces:
  - `type chartDTO struct { Kind string; Title string; Points []chartPoint }` (`Kind`: `bars|line|pie`; `chartPoint{Label string; Value float64}`; up to 24 points; JSON `kind`, `title`, `points`, `label`, `value`).
  - `func assistantTools() []anthropic.Tool` with `resumo_por_categoria`, `totais_por_periodo`, `listar_lancamentos`, `maiores_gastos`, `mostrar_grafico`, each with a JSON `input_schema` (`additionalProperties: false`).
  - `func (s *Server) runTool(ctx context.Context, walletIDs []int64, name string, input json.RawMessage) (text string, chart *chartDTO, err error)` — `walletIDs` always comes from the server; `err` is a tool usage error (goes back to the model as `is_error`).
- Rules: `de`/`ate` required (`YYYY-MM-DD`), `de <= ate`, maximum range of 5 years; `agrupar` ∈ `dia|semana|mes`; `limite` 1–50; `n` 1–20; search text up to 80 characters. Results in compact JSON with amounts in cents and, when there is more than one wallet, the wallet name on each item. Considers only `kind = 'expense'` in the expense summaries, and reports the total income separately in `totais_por_periodo`. Unknown fields in the input → error. Reuses `s.totals` for totals; new queries live in this file.

- [ ] **Step 1: failing tests** (data created through the test API with two wallets and two users): `TestResumoPorCategoria` (correct sum and share, only the wallets passed); `TestTotaisPorPeriodoGroups` (day, week, month); `TestListarLancamentosFilters` (category, text, limit); `TestMaioresGastosOrder`; `TestMostrarGraficoReturnsChart` (validation: invalid type, more than 24 points, empty title); `TestToolsOnlySeeGivenWallets` (a wallet outside `walletIDs` never gets in, even with transactions); `TestToolRejectsBadInput` (inverted period, longer than 5 years, `limite` 0 and 51, `wallet_id` as an unknown field, invalid JSON); `TestEmptyPeriodIsNotAnError`; `TestToolNamesWalletWhenMany`.
- [ ] **Step 2: run** `go test ./internal/api -run 'Resumo|Totais|Listar|Maiores|Grafico|ToolsOnly|ToolRejects|EmptyPeriod|ToolNames'` — expected: FAILS.
- [ ] **Step 3: implement** the five tools and the `runTool` dispatcher; tool descriptions in pt-BR telling the model when to use each one.
- [ ] **Step 4: run** — expected: PASSES.
- [ ] **Step 5: commit** `feat(api): read-only tools for the finance assistant`.

---

### Task 6: chat endpoint (model loop, SSE, history)

**Files:**
- Create: `internal/api/chat.go`, `internal/api/chat_test.go`
- Modify: `internal/api/server.go` (routes, `chatLimiter`, map of answers in progress)

**Interfaces:**
- Consumes: `s.userAIKey`, `anthropic.Client.Stream`, `assistantTools`, `s.runTool`.
- Produces routes (all `user(...)`): `POST /api/chat` `{wallet int64, wallets []int64, text string}` with a `text/event-stream` response and events `status` (`{text}`), `delta` (`{text}`), `chart` (`chartDTO`), `done` (`{inputTokens, outputTokens}`) and `error` (`{message}`); `GET /api/chat?wallet=ID` → `[{id, role, content, charts, walletIds, createdAt}]` in ascending order; `DELETE /api/chat?wallet=ID` → 204.
- Produces: `func systemPrompt(now time.Time, loc *time.Location, wallets []walletInfo) string` in pt-BR: financial advisor, short and direct answers (number first, no detours), amounts in R$, never make things up, use the tools, chart only when it helps, today's date and the names of the wallets and categories.
- Rules: validates text (1–4000 characters), `wallets` non-empty with no duplicates and with the user as a member of all of them (otherwise 403), assistant enabled (otherwise 409 with a clear message); `chatLimiter` of 20/min per user (429); one send per user at a time (409); writes the user message before calling Anthropic; sends at most the last 20 messages (also cut at 24,000 characters); loop with at most 6 turns; Anthropic errors (401, 429, 529, network) and `errAIKeyUnreadable` become an `error` event in pt-BR without mentioning the key; final answer and charts saved only when complete; a canceled context does not save an answer.

- [ ] **Step 1: failing tests** with a fake Anthropic that answers with SSE scripts: `TestChatStreamsAnswer` (`delta` and `done` events, messages saved and `GET` returns both); `TestChatToolLoop` (the fake asks for `resumo_por_categoria`, receives the right `tool_result` and then answers); `TestChatChartEvent`; `TestChatStopsAfterSixTurns`; `TestChatRejectsOtherUsersWallet` (403 and no call to Anthropic); `TestChatNeedsEnabledAssistant`; `TestChatValidatesText`; `TestChatRateLimit`; `TestChatOneAtATime` (second send during the first → 409; after it finishes, it frees up); `TestChatClientCancelSavesNoAnswer`; `TestChatAnthropicErrorsBecomeEvents` (401, 429, 529: `error` event in pt-BR, without the key in the body); `TestChatKeyUnreadableAfterRotation`; `TestChatHistoryIsPerUserAndWallet`; `TestChatDeleteClearsOnlyThatConversation`; `TestChatSendsOnlyRecentHistory`; `TestSystemPromptMentionsTodayAndWallets`.
- [ ] **Step 2: run** `go test ./internal/api -run 'Chat|SystemPrompt'` — expected: FAILS.
- [ ] **Step 3: implement** `chat.go` according to the rules; write SSE with `http.ResponseController` and `Flush`, in the same style as `events.go`.
- [ ] **Step 4: run** `go test -race ./internal/api` — expected: PASSES.
- [ ] **Step 5: commit** `feat(api): finance assistant chat with streaming and read-only tools`.

---

### Task 7: conversation retention

**Files:**
- Modify: `internal/api/housekeeping.go`, `internal/api/ops_test.go`

**Interfaces:**
- Consumes: tables `chat_messages` and `ai_settings`; `s.now()`.
- Produces: `Housekeeping` deletes `chat_messages` older than each user's `retention_days` (`created_at <= now - retention_days` days); a user without a row in `ai_settings` uses 7 days.

- [ ] **Step 1: failing tests** in `ops_test.go` (using `runHousekeeping`): `TestHousekeepingDeletesExpiredChat` (user with 3 days of retention: a 4-day-old message goes away, a 2-day-old one stays); `TestHousekeepingRetentionIsPerUser` (another user with 30 days keeps the 10-day-old message); `TestHousekeepingDefaultsToSevenDays`.
- [ ] **Step 2: run** `go test ./internal/api -run Housekeeping` — expected: FAILS on the three new ones.
- [ ] **Step 3: implement** a single `DELETE` with a subquery on `ai_settings` inside `run`, logging the error like the other steps.
- [ ] **Step 4: run** — expected: PASSES.
- [ ] **Step 5: commit** `feat(api): delete chat messages past each user's retention`.

---

### Task 8: Ajustes > Assistente (frontend)

**Files:**
- Create: `web/src/pages/settings/AssistantSection.tsx`, `web/src/pages/settings/AssistantSection.test.tsx`
- Modify: `web/src/api/types.ts` (`Me.assistantEnabled: boolean`, `AISettings`), `web/src/api/queries.ts` (`useAISettings`), `web/src/pages/Settings.tsx` (section `assistente`, group 2, label "Assistente"), `web/src/pages/SettingsPage.test.tsx` (list of sections), `web/src/test/fixtures.tsx` (`assistantEnabled: false`)

**Interfaces:**
- Consumes: `GET/PUT /api/me/ai`, `DELETE /api/me/ai/key`; components `ListGroup`, `Segmented`, `Switch`, `useAction`.
- Produces: `export function AssistantSection()`; `export const useAISettings = () => UseQueryResult<AISettings>` (key `["ai"]`, invalidated on save) and the mutation also invalidates `["me"]`.
- Behavior: if `available` is false, shows a line explaining that the server does not have `GASTOS_SECRET_KEY` and nothing else; with no saved key, a password field "Chave da API" (API key) with a "Salvar chave" (Save key) button; with a key, shows `•••• hint`, "Trocar" (Replace) and "Remover" (Remove); model Segmented (Haiku, Sonnet, Opus); switch "Ativar chat" (Enable chat) (disabled without a key); numeric field "Guardar conversas por (dias)" (Keep conversations for (days)) 1–30; fixed privacy notice in pt-BR ("Os dados dos lançamentos consultados são enviados à Anthropic com a sua chave." (The data of the queried transactions is sent to Anthropic with your key.)); `apiKey` field error shows below the field.

- [ ] **Step 1: failing tests** in `AssistantSection.test.tsx`: unavailable without the master key; saves the key and sends `{apiKey}`; once saved it never shows the value, only the end; "Remover" calls `DELETE`; changing the model sends `{model}`; the switch sends `{enabled}` and is disabled without a key; retention outside 1–30 shows an error and does not send; shows the server error on `apiKey`; shows the privacy notice. In `SettingsPage.test.tsx`: the list now has 10 sections, with "Assistente" in the "Dados" (Data) group.
- [ ] **Step 2: run** `npx vitest run src/pages` — expected: FAILS.
- [ ] **Step 3: implement** following the `WalletSection.tsx` pattern (`list-row` forms, `useAction`).
- [ ] **Step 4: run** `npx vitest run` and `npx tsc --noEmit` — expected: PASSES.
- [ ] **Step 5: commit** `feat(web): Ajustes > Assistente with the API key, model and retention`.

---

### Task 9: SVG charts

**Files:**
- Create: `web/src/components/Chart.tsx`, `web/src/components/Chart.test.tsx`
- Modify: `web/src/styles.css` (`.chart` classes), `web/src/api/types.ts` (`ChartData`)

**Interfaces:**
- Produces: `export function Chart({ chart }: { chart: ChartData })` with `ChartData = { kind: "bars" | "line" | "pie"; title: string; points: { label: string; value: number }[] }`; renders a `<figure>` with a `<figcaption>` of the title, SVG with `role="img"` and an `aria-label` listing "label: value" (values in cents, formatted with `formatBRL` from `web/src/lib`), colors only through variables from `tokens.css`.

- [ ] **Step 1: failing tests**: renders each type (bars: one bar per point; line: a `polyline`; pie: one slice per point); the `aria-label` carries "Alimentação: R$ 820,00, 31%" for pie; empty points show "Sem dados" (No data); zero values do not break the pie; long labels do not overflow (truncation class).
- [ ] **Step 2: run** `npx vitest run src/components/Chart.test.tsx` — expected: FAILS.
- [ ] **Step 3: implement** plain SVG, no library; styles in `styles.css` respecting `alignment.test.ts` (no new `text-align: center` outside the list).
- [ ] **Step 4: run** `npx vitest run` — expected: PASSES (includes the style tests).
- [ ] **Step 5: commit** `feat(web): small SVG charts for the assistant`.

---

### Task 10: Chat tab (frontend)

**Files:**
- Create: `web/src/pages/Chat.tsx`, `web/src/pages/Chat.test.tsx`, `web/src/components/WalletPicker.tsx`, `web/src/components/WalletPicker.test.tsx`, `web/src/lib/chatStream.ts`, `web/src/lib/chatStream.test.ts`
- Modify: `web/src/components/nav.ts` and `nav.test.ts` (`navItems(splitting, assistant)` adds `{ to: "/chat", label: "Chat", icon: "chat" }` after Caixa (Inbox)), `TabBar.tsx`, `Sidebar.tsx` (pass `assistantEnabled`), `Icon.tsx` (`chat` icon), `App.tsx` (`chat` route, redirects to `/` if the assistant is off), `context.ts`/`Shell.tsx` (`assistantEnabled` in the context), `api/queries.ts` (`useChat(walletId)`), `styles.css`

**Interfaces:**
- Produces: `streamChat(body: { wallet: number; wallets: number[]; text: string }, on: { delta(t: string): void; status(t: string): void; chart(c: ChartData): void; done(): void; error(m: string): void }, signal: AbortSignal): Promise<void>` — `fetch` POST + streamed body reading, splitting the SSE events (`event:`/`data:`), tolerating events cut between chunks.
- Produces: `WalletPicker({ wallets, selected, onChange })` — a button that shows the wallet name or "N carteiras" (N wallets), opens a list with checkboxes and "Selecionar todas" (Select all); prevents unchecking the last one.
- `Chat()`: default selection is only the active wallet and goes back to it when the active wallet changes; user messages on the right and advisor messages on the left in plain text; `Chart` inside the conversation; status line while querying; text field (Enter sends, Shift+Enter breaks a line); the send button turns into "Parar" (Stop) during the answer (aborts the `fetch`); "Nova conversa" (New conversation) menu calls `DELETE`; shows the `error` event's error as an advisor message; empty conversation shows only the text field.

- [ ] **Step 1: failing tests**: `chatStream.test.ts` (complete events, events split into two chunks, `error` calls `on.error`, aborting resolves without throwing); `WalletPicker.test.tsx` (label with one and with several wallets, select all, does not uncheck the last one, works with the keyboard); `Chat.test.tsx` (default only the current wallet; sends with Enter and not with Shift+Enter; shows the answer arriving in chunks and the chart; "Parar" button aborts; "Nova conversa" clears; server error shows up; no suggestions in the empty conversation; changing the active wallet resets the selection); `nav.test.ts` (Chat only with the assistant on; "Chat" after "Caixa"); `TabBar`/`Sidebar` show the item.
- [ ] **Step 2: run** `npx vitest run` — expected: FAILS on the new ones.
- [ ] **Step 3: implement** according to the interfaces; dropdown in the style of the existing pickers (`Sheet` on mobile, popover on desktop only if there is already a pattern; otherwise use `Sheet` on both).
- [ ] **Step 4: run** `npx vitest run`, `npx tsc --noEmit`, `npm run lint` — expected: PASSES.
- [ ] **Step 5: commit** `feat(web): Chat tab with a wallet picker and streamed answers`.

---

### Task 11: e2e, documentation and deploy steps

**Files:**
- Create: `web/e2e/assistant.spec.ts`, `web/e2e/fake-anthropic.mjs`, `docs/assistente.md`
- Modify: `web/playwright.config.ts` (starts the fake Anthropic and sets a test `GASTOS_SECRET_KEY` and `GASTOS_ANTHROPIC_URL` pointing to it), `README.md` (link to `docs/assistente.md`), `docs/ops/deploy.md` or the existing operations doc (new variable)

**Interfaces:**
- Consumes: everything above. The fake answers `Check` and two scripted questions: one that asks for `resumo_por_categoria` followed by `mostrar_grafico` and text, and one that answers with text only.

- [ ] **Step 1: failing e2e test** (`desktop` and `mobile`): sign-up; in Ajustes > Assistente saves a test key, chooses a model and turns it on; the Chat tab shows up; sends "quanto gastei este mês?" (how much did I spend this month?); sees the answer and the chart; the selector shows the current wallet and allows "Selecionar todas"; "Nova conversa" clears; reloading the page keeps the conversation; without a key the tab does not show up.
- [ ] **Step 2: run** `npm run build` and `npm run e2e -- -g assistente` — expected: FAILS.
- [ ] **Step 3: implement** the fake (Node, no dependencies) and the configuration; the real key never goes into a test.
- [ ] **Step 4: run** the full suite (`go test -race ./...`, `npx vitest run`, `npm run e2e`) — expected: PASSES.
- [ ] **Step 5: write `docs/assistente.md`** (pt-BR): what it is, how to generate `GASTOS_SECRET_KEY` (`openssl rand -base64 32`) and where to put it in the VPS service, how each user puts in their own key, privacy (queried data goes to Anthropic), costs (billed to each person's key; model and retention in Ajustes), what happens if the master key changes, and the deploy steps: backup, stop the service, move the old database out of the way, start the new version (creates the new database) and each person recreates their account.
- [ ] **Step 6: commit** `test(e2e): assistant flow against a fake Anthropic; docs and deploy steps`.

---

## Out of this plan

Write tools with confirmation (part 2), voice (part 3) and running the production deploy, which requires the user's "yes" at that moment (it deletes the current database) and the master key generated on the VPS.
