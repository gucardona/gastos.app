# Backend foundation — schema novo e API v2

**Data:** 2026-10-09
**Status:** Draft — aguardando revisão
**Roadmap:** [2026-10-09-roadmap.md](2026-10-09-roadmap.md)

## Objetivo

Reescrever a base do backend Go aproveitando que não há dados a preservar:

1. Um schema único e correto, sem histórico de migrations.
2. Dinheiro em centavos e splits em basis points.
3. API organizada por carteira (`/api/wallets/{walletID}/...`) com o roteador do Go 1.22+.
4. Corrigir os bugs levantados na revisão (FK por conexão, corrida de recorrência, PATCH reatribuindo dono, backup, SSE).
5. Mover cálculos do dashboard para o servidor (`/summary`), deixando o frontend novo fino.

Fora de escopo: frontend (fase 3), CI/deploy (fase 2), Open Finance (fase 4). As tabelas da fase 4 entram depois, via `user_version` 2.

## Estrutura de pastas

```
cmd/gastos/main.go          # lê config, abre DB, sobe servidor, jobs de fundo
internal/config/            # env vars → struct Config (falha cedo se inválido)
internal/db/                # Open (pragmas), schema.sql embutido, migrate (user_version), backup
internal/money/             # Allocate (rateio em centavos), helpers de basis points
internal/recurrence/        # Occurrences (puro) + Materializer (job)
internal/auth/              # hash de senha, sessões, middleware RequireUser
internal/api/               # handlers por recurso, middleware RequireWallet, helpers JSON/erros
internal/events/            # bus SSE (o atual, com heartbeat)
web/                        # app React (fase 3); web/embed.go expõe dist/ via go:embed
```

Removidos: `src/` inteiro, `vendor/` (o CI baixa módulos), o binário `gastos-app` commitado, `deploy.sh` (substituído na fase 2).

## Configuração (env)

| Var | Default | Nota |
|---|---|---|
| `GASTOS_ADDR` | `:8000` | |
| `GASTOS_DB_PATH` | `./data/gastos.db` | diretório criado se não existir |
| `GASTOS_BACKUP_DIR` | `./data/backups` | |
| `GASTOS_BACKUP_KEEP` | `14` | quantos backups diários manter |
| `GASTOS_TZ` | `America/Sao_Paulo` | define "hoje" para recorrências e para o mês corrente |
| `GASTOS_COOKIE_SECURE` | `true` | `false` só em dev local (http) |
| `GASTOS_DEV` | `false` | habilita CORS para `localhost:5173` e o seed de demo |

Sem `JWT_SECRET`: a sessão passa a ser um token opaco guardado com hash no banco.

## Banco

### Conexão

`db.Open(path)` usa o DSN com pragmas aplicados **em toda conexão**:

```
file:<path>?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)
```

`db.SetMaxOpenConns(1)` para escrita não é necessário com WAL + `busy_timeout`, mas as transações de escrita usam `BEGIN IMMEDIATE` (via `_txlock=immediate` no DSN) para evitar `SQLITE_BUSY` em upgrade de lock.

### Versionamento

`schema.sql` cria tudo (versão 1). `migrate.go` lê `PRAGMA user_version`:

- `0` → executa `schema.sql` numa transação, seta `user_version = 1`.
- `N` → aplica `migrations[N:]`, um slice de strings SQL, cada uma numa transação, incrementando `user_version`.

Hoje o slice é vazio. São cerca de 30 linhas, sem tabela de controle e sem "ignorar coluna duplicada".

### Schema (versão 1)

Convenções: `id INTEGER PRIMARY KEY`, datas civis como `TEXT 'YYYY-MM-DD'`, timestamps como `TEXT` ISO-8601 UTC, booleanos `INTEGER CHECK (x IN (0,1))`, dinheiro `*_cents INTEGER`.

```sql
CREATE TABLE users (
  id            INTEGER PRIMARY KEY,
  name          TEXT NOT NULL,
  email         TEXT NOT NULL UNIQUE COLLATE NOCASE,
  password_hash TEXT NOT NULL,
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE sessions (
  token_hash BLOB PRIMARY KEY,                       -- sha256 do token enviado no cookie
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
) WITHOUT ROWID;
CREATE INDEX sessions_user ON sessions(user_id);

CREATE TABLE wallets (
  id                INTEGER PRIMARY KEY,
  name              TEXT NOT NULL,
  splitting_enabled INTEGER NOT NULL DEFAULT 0 CHECK (splitting_enabled IN (0,1)),
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE wallet_members (
  wallet_id         INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  user_id           INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role              TEXT NOT NULL CHECK (role IN ('owner','editor','reader')),
  splits_by_default INTEGER NOT NULL DEFAULT 1 CHECK (splits_by_default IN (0,1)),
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  PRIMARY KEY (wallet_id, user_id)
) WITHOUT ROWID;
CREATE INDEX wallet_members_user ON wallet_members(user_id);
CREATE UNIQUE INDEX wallet_members_one_owner ON wallet_members(wallet_id) WHERE role = 'owner';

CREATE TABLE categories (
  id           INTEGER PRIMARY KEY,
  wallet_id    INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  kind         TEXT NOT NULL CHECK (kind IN ('expense','income')),
  name         TEXT NOT NULL,
  icon         TEXT NOT NULL,
  color        TEXT NOT NULL,
  essentiality TEXT CHECK (essentiality IN ('essential','nonessential','investment')),
  sort_order   INTEGER NOT NULL DEFAULT 0,
  archived_at  TEXT,
  UNIQUE (wallet_id, kind, name),
  CHECK ((kind = 'expense') = (essentiality IS NOT NULL))
);

CREATE TABLE payment_methods (
  id          INTEGER PRIMARY KEY,
  wallet_id   INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  icon        TEXT NOT NULL,
  sort_order  INTEGER NOT NULL DEFAULT 0,
  archived_at TEXT,
  UNIQUE (wallet_id, name)
);

CREATE TABLE recurrences (
  id                INTEGER PRIMARY KEY,
  wallet_id         INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  kind              TEXT NOT NULL CHECK (kind IN ('expense','income','settlement')),
  amount_cents      INTEGER NOT NULL CHECK (amount_cents > 0),
  description       TEXT NOT NULL DEFAULT '',
  category_id       INTEGER REFERENCES categories(id),
  payment_method_id INTEGER REFERENCES payment_methods(id),
  paid_by_user_id   INTEGER NOT NULL REFERENCES users(id),   -- expense: quem paga; income: quem recebe; settlement: quem envia
  to_user_id        INTEGER REFERENCES users(id),            -- só settlement
  day_of_month      INTEGER NOT NULL CHECK (day_of_month BETWEEN 1 AND 31),
  start_date        TEXT NOT NULL,
  end_date          TEXT,
  enabled           INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1)),
  created_by        INTEGER NOT NULL REFERENCES users(id),
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  CHECK ((kind = 'settlement') = (to_user_id IS NOT NULL)),
  CHECK ((kind = 'settlement') = (category_id IS NULL)),
  CHECK (end_date IS NULL OR end_date >= start_date)
);
CREATE INDEX recurrences_wallet ON recurrences(wallet_id, enabled);

CREATE TABLE recurrence_splits (
  recurrence_id INTEGER NOT NULL REFERENCES recurrences(id) ON DELETE CASCADE,
  user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  share_bp      INTEGER NOT NULL CHECK (share_bp BETWEEN 1 AND 10000),
  PRIMARY KEY (recurrence_id, user_id)
) WITHOUT ROWID;

CREATE TABLE transactions (
  id                INTEGER PRIMARY KEY,
  wallet_id         INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  kind              TEXT NOT NULL CHECK (kind IN ('expense','income')),
  amount_cents      INTEGER NOT NULL CHECK (amount_cents > 0),
  description       TEXT NOT NULL,
  category_id       INTEGER NOT NULL REFERENCES categories(id),
  payment_method_id INTEGER REFERENCES payment_methods(id),
  date              TEXT NOT NULL,
  paid_by_user_id   INTEGER NOT NULL REFERENCES users(id),
  created_by        INTEGER NOT NULL REFERENCES users(id),
  recurrence_id     INTEGER REFERENCES recurrences(id) ON DELETE SET NULL,
  occurrence        TEXT,                                    -- 'YYYY-MM' quando gerada por recorrência
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE INDEX transactions_wallet_date ON transactions(wallet_id, date DESC, id DESC);
CREATE UNIQUE INDEX transactions_recurrence_occurrence
  ON transactions(recurrence_id, occurrence) WHERE recurrence_id IS NOT NULL;

CREATE TABLE transaction_splits (
  transaction_id INTEGER NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
  user_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  share_bp       INTEGER NOT NULL CHECK (share_bp BETWEEN 1 AND 10000),
  PRIMARY KEY (transaction_id, user_id)
) WITHOUT ROWID;
CREATE INDEX transaction_splits_user ON transaction_splits(user_id);

CREATE TABLE settlements (
  id            INTEGER PRIMARY KEY,
  wallet_id     INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  from_user_id  INTEGER NOT NULL REFERENCES users(id),
  to_user_id    INTEGER NOT NULL REFERENCES users(id),
  amount_cents  INTEGER NOT NULL CHECK (amount_cents > 0),
  date          TEXT NOT NULL,
  note          TEXT NOT NULL DEFAULT '',
  recurrence_id INTEGER REFERENCES recurrences(id) ON DELETE SET NULL,
  occurrence    TEXT,
  created_by    INTEGER NOT NULL REFERENCES users(id),
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  CHECK (from_user_id <> to_user_id)
);
CREATE INDEX settlements_wallet_date ON settlements(wallet_id, date DESC);
CREATE UNIQUE INDEX settlements_recurrence_occurrence
  ON settlements(recurrence_id, occurrence) WHERE recurrence_id IS NOT NULL;

CREATE TABLE budgets (
  wallet_id   INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  limit_cents INTEGER NOT NULL CHECK (limit_cents > 0),
  PRIMARY KEY (wallet_id, category_id)
) WITHOUT ROWID;
```

**O que mudou em relação ao schema atual:**

| Antes | Agora | Por quê |
|---|---|---|
| `expenses` + `incomes` | `transactions.kind` | uma lista, um formulário, um modelo para import bancário |
| `incomes.type` (texto livre) | `categories` com `kind='income'` | entradas ganham categorias editáveis como os gastos |
| `category`/`payment` como chave texto | `category_id`/`payment_method_id` FK | integridade; renomear não quebra nada |
| apagar categoria com "substituição" | `archived_at` (soft delete) quando em uso | some do formulário, histórico intacto |
| `recurring_expenses`/`_incomes`/`_payments` | `recurrences.kind` | uma tabela, um materializador |
| `percentage REAL` | `share_bp INTEGER` | soma exata 10000 |
| `user_id` ambíguo (dono? pagador?) | `paid_by_user_id` editável + `created_by` imutável | corrige o bug de PATCH reatribuindo dono |
| `account_split_participants` | `wallet_members.splits_by_default` | 1 coluna em vez de 1 tabela |
| `goals` | `budgets` | nome reflete o uso (limite mensal por categoria) |
| `schema_migrations` | `PRAGMA user_version` | |

### Seed por carteira

Ao criar carteira, `seedWallet(tx, walletID)` insere as 15 categorias de gasto atuais, 5 de entrada (Salário, Freelance, Investimentos, Reembolso, Outros) e as 8 formas de pagamento atuais. Os dados ficam numa tabela Go, como hoje.

### Backup

Job diário (e no boot, se o último backup tiver mais de 24h): `VACUUM INTO '<dir>/gastos-YYYYMMDD-HHMMSS.db'`, depois remove os mais antigos além de `GASTOS_BACKUP_KEEP`. Nunca sobrescreve um backup existente. `VACUUM INTO` gera uma cópia consistente mesmo com WAL.

## Dinheiro e splits

`money.Allocate(totalCents int64, sharesBP []int64) []int64`: distribui centavos pelo método do maior resto. A soma do resultado é sempre igual a `totalCents`, e o desempate vai para o menor índice, de forma determinística.

Regras de split (só quando `wallets.splitting_enabled = 1`):
- Sem `splits` no request: divide igualmente entre membros com `splits_by_default = 1`, em bp, com o resto indo para o primeiro.
- Com `splits`: cada `userId` precisa ser membro, sem repetição, cada `shareBp` entre 1 e 10000, soma exatamente 10000.
- Com `splitting_enabled = 0`: `splits` deve vir vazio; nenhuma linha em `transaction_splits`.

Saldos entre membros, calculados no `/summary`, considerando o histórico inteiro da carteira:
- Para cada gasto com splits: `paid_by` recebe crédito de `amount`; cada participante recebe débito de `Allocate(amount, shares)[i]`.
- `settlement` de A para B: A recebe crédito, B recebe débito.
- Acertos sugeridos: algoritmo guloso que casa o maior devedor com o maior credor (o mesmo resultado que a UI atual mostra).

## Recorrências

`recurrence.Occurrences(dayOfMonth int, start, end, until string) []Occurrence` é uma função pura, com datas `YYYY-MM-DD` (`end` vazio = sem fim). Ela devolve `{Month "YYYY-MM", Date "YYYY-MM-DD"}` para cada mês entre `start` e `until`. O dia é limitado ao último do mês, e meses com data fora de `[start, end]` ficam de fora.

`Materializer.Run(ctx, walletID *int64)`:
- Lê recorrências `enabled = 1` (de todas as carteiras ou de uma).
- Para cada ocorrência até **hoje em `GASTOS_TZ`**, faz `INSERT ... ON CONFLICT DO NOTHING` em `transactions` ou `settlements`, junto com os splits, numa transação por ocorrência.
- O índice único `(recurrence_id, occurrence)` garante idempotência, mesmo com execuções concorrentes.
- Se inseriu algo, chama `events.Notify(walletID, 0)`.

Quando roda:
- no boot;
- a cada hora (`time.Ticker`);
- de forma síncrona depois de criar ou editar uma recorrência, só para aquela carteira, para a UI mostrar o lançamento na hora.

**GETs nunca escrevem.**

Editar uma recorrência não altera lançamentos já gerados. Apagar uma recorrência mantém os lançamentos e zera o `recurrence_id`, pelo `SET NULL`.

## Autenticação

- `POST /api/auth/register`, `POST /api/auth/login`: geram um token aleatório de 32 bytes, gravam `sha256` em `sessions` com validade de 30 dias e enviam o cookie `gastos_session` (`HttpOnly`, `SameSite=Lax`, `Secure` conforme config, `Path=/`).
- `POST /api/auth/logout`: apaga a sessão e expira o cookie.
- `GET /api/me`: devolve `{ user, wallets: [...] }`, que é o bootstrap do frontend.
- Renovação deslizante: se faltam menos de 15 dias, estende `expires_at` e reenvia o cookie.
- Sessões expiradas são removidas pelo job horário.
- CSRF: mutações exigem `Content-Type: application/json`. Formulários HTML cross-site não conseguem enviar esse header sem preflight, e o servidor não responde preflight cross-origin fora de `GASTOS_DEV`. Isso, mais `SameSite=Lax`, cobre o caso.
- Senha com bcrypt custo 12, mínimo de 8 caracteres.
- Rate limit em memória no login: 10 tentativas por 15 minutos, por IP e por e-mail. Responde `429`.

## API v2

Base JSON, erros no formato:

```json
{ "error": { "code": "validation", "message": "Dados inválidos", "fields": { "amountCents": "deve ser maior que zero" } } }
```

Códigos: `validation` (400), `unauthenticated` (401), `forbidden` (403), `not_found` (404), `conflict` (409), `rate_limited` (429), `internal` (500).

Middleware em cadeia: `logRequest → recoverPanic → RequireUser → RequireWallet(minRole)`.
`RequireWallet` lê `r.PathValue("walletID")` e põe `{walletID, role}` no context. Os papéis são `reader < editor < owner`.

| Método e rota | Papel mínimo | Nota |
|---|---|---|
| `POST /api/auth/register` | — | cria usuário e carteira "Pessoal" |
| `POST /api/auth/login` · `POST /api/auth/logout` | — / user | |
| `GET /api/me` | user | |
| `GET /api/wallets` · `POST /api/wallets` | user | |
| `PATCH /api/wallets/{walletID}` · `DELETE` | owner | |
| `GET /api/wallets/{walletID}/members` | reader | |
| `POST .../members` · `PATCH .../members/{userID}` | owner | convite por e-mail de usuário existente |
| `DELETE .../members/{userID}` | owner, ou o próprio usuário | sair da carteira |
| `GET/POST .../categories` · `PUT/DELETE .../categories/{id}` | reader / editor | DELETE: arquiva se em uso, apaga se não |
| `GET/POST .../payment-methods` · `PUT/DELETE .../payment-methods/{id}` | reader / editor | idem |
| `GET .../transactions?from=&to=&kind=&categoryId=&q=` | reader | `from`/`to` obrigatórios (máx. 366 dias) |
| `POST .../transactions` · `PUT/DELETE .../transactions/{id}` | editor | |
| `GET/POST .../recurrences` · `PUT/DELETE .../recurrences/{id}` | reader / editor | POST/PUT materializam na hora |
| `GET .../settlements?from=&to=` · `POST` · `PUT/DELETE .../settlements/{id}` | reader / editor | |
| `GET .../budgets` · `PUT .../budgets/{categoryId}` · `DELETE` | reader / editor | PUT faz upsert |
| `GET .../summary?month=YYYY-MM` | reader | ver abaixo |
| `GET .../events` | reader | SSE |
| `GET /healthz` | — | `200 ok` se `db.Ping()` passar |
| `GET /*` | — | SPA embutida; rotas desconhecidas devolvem `index.html` |

Formato de transação, igual na entrada e na saída:

```json
{
  "id": 42, "kind": "expense", "amountCents": 12990, "description": "Mercado",
  "categoryId": 3, "paymentMethodId": 1, "date": "2026-10-09",
  "paidByUserId": 1, "createdBy": 1,
  "recurrenceId": null,
  "splits": [{ "userId": 1, "shareBp": 5000, "amountCents": 6495 }, { "userId": 2, "shareBp": 5000, "amountCents": 6495 }]
}
```

`PUT` substitui todos os campos editáveis; o formulário sempre envia o objeto inteiro, o que evita a ambiguidade de "campo ausente" contra `null` num PATCH parcial. `PATCH` fica só para wallet (`name`, `splittingEnabled`) e membro (`role`, `splitsByDefault`). `createdBy`, `recurrenceId` e `id` são somente leitura. `paidByUserId` omitido ou `0` vale o usuário atual. Uma categoria arquivada é aceita no `PUT` só se já era a categoria da transação.

O GET de transações carrega os splits com **uma** query (`WHERE transaction_id IN (...)`), sem N+1.

### `/summary`

```json
{
  "month": "2026-10",
  "incomeCents": 850000, "expenseCents": 412300, "balanceCents": 437700,
  "dailyAverageCents": 45811,
  "byCategory": [{ "categoryId": 3, "kind": "expense", "cents": 98000, "budgetCents": 120000 }],
  "byEssentiality": { "essential": 300000, "nonessential": 90000, "investment": 22300 },
  "previous": { "incomeCents": 850000, "expenseCents": 398000 },
  "memberBalances": [{ "userId": 1, "netCents": 15000 }, { "userId": 2, "netCents": -15000 }],
  "suggestedSettlements": [{ "fromUserId": 2, "toUserId": 1, "amountCents": 15000 }]
}
```

`dailyAverageCents` usa os dias decorridos no mês, se for o mês corrente em `GASTOS_TZ`, ou os dias do mês, se for um mês passado.
`memberBalances` e `suggestedSettlements` só aparecem com `splitting_enabled`.

### Eventos (SSE)

- Autentica pelo cookie e pelo `RequireWallet`, sem token na URL.
- Manda `: ping\n\n` a cada 25 segundos.
- Toda mutação bem-sucedida em recurso de carteira chama `events.Notify(walletID, actorUserID)`, incluindo categorias, formas de pagamento, budgets, membros e exclusão de carteira. A notificação é feita por um helper `mutated(r)` chamado no fim dos handlers, não à mão em cada um.
- Payload: `data: {"type":"changed"}`. O frontend invalida as queries da carteira.

## Observabilidade

`log/slog` em JSON. Cada request gera um log com método, rota (o pattern, não o path cru), status, duração e `userID`. Panics são logados com stack e respondem `500`.

## Testes

- **Unitários puros:** `money.Allocate`, `recurrence.Occurrences`, sugestão de acertos, validações.
- **Integração** (`httptest` + SQLite em `t.TempDir()`), um arquivo por recurso em `internal/api`, cobrindo:
  - fluxo de auth com cookie;
  - permissões por papel em cada rota;
  - CRUD de cada recurso;
  - regras de split;
  - idempotência do materializador, incluindo duas execuções concorrentes;
  - `ON DELETE CASCADE` (apagar carteira apaga transações);
  - arquivamento de categoria em uso;
  - números do `/summary` com um cenário fixo.
- Os testes atuais em `src/` são descartados junto com o código antigo.

## Critérios de aceite

- `go test ./...` passa; `go vet ./...` limpo.
- Banco novo sobe só com `schema.sql`, com `user_version = 1`.
- Duas chamadas simultâneas do materializador não duplicam lançamentos (teste com `t.Parallel` e goroutines).
- Apagar uma carteira remove todas as linhas dependentes (teste conta as linhas).
- Nenhum `float64` em valores monetários no pacote `api` (checagem com `grep` no plano).
- `curl /healthz` responde 200; um GET de rota SPA desconhecida devolve o `index.html` embutido.
