# Financial assistant, part 1: foundation and querying

Date: 2026-10-15. Status: awaiting review.

## Goal

A 24/7 financial advisor inside the app. The user puts their own Anthropic API key in Ajustes (Settings) and turns the assistant on; then a **Chat** tab shows up, where they ask about their expenses in text ("quanto gastei nos últimos 15 dias?" (how much did I spend in the last 15 days?), "para onde vai a maior parte do dinheiro?" (where does most of the money go?)) and get short, direct answers, with simple charts when they help. The cost of use is billed directly to the user's key.

This is **part 1 of 3**:

1. **Foundation and querying (this document):** key, settings, Chat tab, read-only tools.
2. **Writes with confirmation:** the model proposes creating, editing or removing, and the user confirms on a card. Includes deciding which wallet a new transaction is written to when several are selected.
3. **Voice:** microphone button that dictates text into the chat (browser speech recognition, pt-BR).

Out of this part: any data write by the model, voice, Google login, Open Finance.

## Decisions already made

| Topic | Decision |
| --- | --- |
| Changes (part 2) | Every change goes through a confirmation card, including new transactions. |
| History | Saved per user and per wallet. Retention configurable by the user: 1 to 30 days, default 7. |
| Model | The user chooses in Ajustes between Haiku 5.5, Sonnet 5.5 (default) and Opus 5.5. |
| Data scope | Wallet selector in the chat. Default: only the current wallet. Dropdown with "Selecionar todas" (Select all) or individual checking. Only wallets the user is a member of. |
| Key | One per user, valid in all of their wallets. |
| Approach | The server runs the model loop with fixed read tools. No free-form SQL, no dumping the database into the prompt. |
| Answers | Concise and direct: number first, no detours. Goes in the system prompt and is reinforced by `max_tokens`. |
| Suggestions in an empty conversation | None. The empty conversation shows only the text field. |
| Database | Everything goes into `schema.sql` and the migrations mechanism is removed. The production database is recreated on deploy (see Rollout). |

## Data

Changes to `internal/db/schema.sql` (which becomes the only definition, including what is today in `migrations/0002_bank_import.sql`):

- `ai_settings` (one row per user): `user_id` (PK), `api_key_enc` (BLOB, AES-GCM with nonce), `key_hint` (last 4 characters), `model`, `enabled`, `retention_days` (CHECK 1 to 30, default 7), `updated_at`.
- `chat_messages`: `id`, `user_id`, `wallet_id` (active wallet when the message was sent), `role` (`user`|`assistant`), `content`, `wallet_ids` (JSON of the wallets queried), `input_tokens`, `output_tokens`, `created_at`. Index on `(user_id, wallet_id, id)` and on `created_at`.
- Charts: the assistant content stores the chart cards as JSON alongside the text, so they reappear when the conversation is reopened.

Removal: `internal/db/migrations/`, the `//go:embed` directives for the migrations and the `migrate` loop are replaced by creating the schema when the database is opened (`CREATE TABLE IF NOT EXISTS`). The migration tests (`migration_test.go`) are replaced by a test that the schema creates a valid database and is idempotent.

## API key

- Encryption: AES-256-GCM. The master key comes from `GASTOS_SECRET_KEY` (32 bytes in base64). In `internal/config`, the variable is optional: absent or invalid disables the assistant and never prevents the server from starting; a value that is present but malformed is a configuration error.
- The Anthropic key is never returned by any route, never goes to a log or an error message. The API shows only `key_hint`.
- On save, the server validates the key with a minimal call to the Anthropic API and responds with a clear error if it is rejected. If validation fails because of the network, it saves the key as not validated and warns.

## API

All require a session (`user(...)`) and JSON.

- `GET /api/me/ai`: `{available, configured, keyHint, model, enabled, retentionDays}`. `available` is false without `GASTOS_SECRET_KEY`.
- `PUT /api/me/ai`: `{apiKey?, model?, enabled?, retentionDays?}`. A missing `apiKey` keeps the current one. `enabled: true` requires a configured key. Retention outside 1 to 30 and a model outside the fixed list are per-field validation errors.
- `DELETE /api/me/ai/key`: removes the key and turns the assistant off.
- `GET /api/chat?wallet=ID`: the user's conversation messages in that wallet (already without the expired ones).
- `DELETE /api/chat?wallet=ID`: "Nova conversa" (New conversation), deletes its messages.
- `POST /api/chat`: `{wallet, wallets: [ids], text}`. SSE response with the events `status` (for example, querying), `delta` (text), `chart` (card), `done` (token usage) and `error`. Closing the connection cancels the call to Anthropic.

`/api/me` now reports `assistantEnabled` so the frontend can decide whether to show the tab, without a second call.

## Model loop

1. Validates: non-empty text of up to 4000 characters, non-empty `wallets` and all of them with the user as a member (403 otherwise), assistant enabled and key present.
2. Applies the usage limit (per user, messages per minute, in memory like the login limiter) and writes the user message.
3. Builds the context: advisor instructions in pt-BR (conciseness, numbers first, amounts in R$, no making things up), today's date in the configured time zone, names of the selected wallets and their categories, and the latest messages of the conversation up to a size cap.
4. Calls the Anthropic messages API with the tools, fixed `max_tokens`, streaming. If the model asks for a tool, the server runs it, returns the result and repeats, at most 6 turns per question.
5. Writes the assistant's final answer with the token usage.

## Tools (read-only)

All receive the selected wallets from the server, never from the model, and validate parameters like any route. Amounts in cents; the model formats them. They reuse the queries from `summary.go` instead of new SQL.

- `resumo_por_categoria(de, ate)`: total and share per category.
- `totais_por_periodo(de, ate, agrupar)`: `dia`, `semana` or `mes`.
- `listar_lancamentos(de, ate, categoria?, texto?, limite)`: up to 50.
- `maiores_gastos(de, ate, n)`: up to 20.
- `mostrar_grafico(tipo, titulo, pontos)`: `barras`, `linha` or `pizza`, up to 24 points `{rotulo, valor}`. It queries nothing; it becomes the `chart` event.

With more than one wallet selected, each result states the source wallet. Transaction descriptions are data, not instructions: in this part there is no write tool, so malicious text in a description cannot change anything.

## Errors

Invalid key (401), no balance, Anthropic usage limit (429), overload and network failure become the `error` event with a pt-BR message saying what to do. No error mentions the key. History is not lost.

## Retention

`Housekeeping` deletes `chat_messages` older than each user's `retention_days` (a query with a join on `ai_settings`). A user with no row in `ai_settings` has no messages.

## Interface

- **Chat tab** in the navigation (bottom bar on mobile, sidebar on desktop, after Caixa (Inbox)), only when `assistantEnabled`.
  - Top: wallet selector (dropdown with checkboxes, "Selecionar todas", minimum one). Closed, it shows the wallet name or "N carteiras" (N wallets).
  - User messages on the right, advisor messages on the left, in plain text. A discreet status line while querying.
  - Footer: text field and send button, which turns into "parar" (stop) during the answer. Enter sends, Shift+Enter breaks a line. Menu with "Nova conversa".
  - Empty conversation: only the text field.
  - Charts in simple SVG (bars, line, pie), with no new library, using the token colors and alternative text ("Alimentação: R$ 820, 31%").
- **Ajustes > Assistente** (assistant) (group "Dados" (Data)): key (password field; once saved it shows `•••• 4f2a` with "Trocar" (Replace) and "Remover" (Remove)), model (Segmented), "Ativar chat" (Enable chat), "Guardar conversas por N dias" (Keep conversations for N days) and the notice that the queried data is sent to Anthropic. Without `GASTOS_SECRET_KEY` on the server, the screen explains it in one line instead of accepting the key.
- Follows the current visual system: tokens, contrast, focus and the alignment rules of the style tests.

## Tests (written before the code)

- **Go:** encrypts and decrypts the key and never exposes it in responses or logs; retention and model validation; save refused without `GASTOS_SECRET_KEY`; chat against a test server that mimics Anthropic (tool loop, 6-turn limit, 401, 429, cancellation); 403 on another user's wallet; tools sum only the selected wallets; retention deletes only the expired items of each user; messages-per-minute limit; schema created from scratch and idempotent.
- **Vitest:** tab only with the assistant enabled, wallet selector (current default, select all, minimum one), send and stop, chart cards, Ajustes > Assistente screen, style test rules.
- **e2e:** configure, turn on, see the tab, ask and get an answer with a chart, against a fake Anthropic server. The real key never goes into a test.

## Rollout

1. A single PR for part 1, with this spec, the plan and `docs/assistente.md` (pt-BR: configuring the key, privacy, costs).
2. Before the deploy, generate `GASTOS_SECRET_KEY` on the VPS and put it in the service. The value stays only on the server, never in the chat or the repository.
3. Back up the database and **recreate the production database** (decided: the current data can be discarded). The "yes" is asked again at that moment, because deleting cannot be undone.
4. Each person creates their account and puts their own key in Ajustes. Rafa uses their own key.

## Known risks

- A lost or changed master key invalidates the saved keys; the user has to enter them again. Documented.
- The data of the queried transactions goes to Anthropic; the notice stays visible in Ajustes.
- The cost of a long conversation grows with the context; the cap on the history sent and `max_tokens` limit that.
