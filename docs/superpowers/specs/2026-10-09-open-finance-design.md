# Importação bancária e Open Finance

**Data:** 2026-10-09
**Status:** Draft. A etapa 4b depende de um spike e das perguntas em aberto 1 e 3 do roadmap.
**Roadmap:** [2026-10-09-roadmap.md](2026-10-09-roadmap.md)

## Objetivo

Parar de digitar lançamentos que o banco já conhece. As transações bancárias chegam numa **caixa de entrada**. Lá ganham categoria sugerida, são deduplicadas contra o que já foi lançado à mão e viram `transactions` com um toque. Nada entra no histórico sem confirmação, a não ser por regra explícita de auto-aceite.

## Restrições do Open Finance Brasil

- Só instituições autorizadas pelo Banco Central acessam o Open Finance diretamente. Um app pessoal precisa de um agregador.
- Os agregadores comerciais cobram valores inviáveis para uso pessoal. Os relatos públicos falam em cerca de R$ 2,5 mil/mês (Pluggy) e R$ 6 mil/mês (Belvo).
- Existe o **Meu Pluggy**, gratuito para uso pessoal. Você conecta seus bancos no Meu Pluggy e autoriza sua aplicação, que lê os dados pela API da Pluggy. Isso vem de relatos de usuários, não de documentação oficial, e precisa de spike (ver 4b).

Por isso a feature é dividida em duas etapas. A **4a** entrega valor sem depender de ninguém. A **4b** pluga uma fonte automática no mesmo funil.

## Modelo de dados (`user_version` 2)

```sql
CREATE TABLE bank_connections (
  id             INTEGER PRIMARY KEY,
  user_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider       TEXT NOT NULL CHECK (provider IN ('file','pluggy')),
  external_id    TEXT,                       -- itemId no Pluggy; NULL para 'file'
  institution    TEXT NOT NULL,              -- "Nubank", "Itaú"...
  status         TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','needs_reauth','error','disabled')),
  last_synced_at TEXT,
  last_error     TEXT,
  created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  UNIQUE (provider, external_id)
);

CREATE TABLE bank_accounts (
  id                INTEGER PRIMARY KEY,
  connection_id     INTEGER NOT NULL REFERENCES bank_connections(id) ON DELETE CASCADE,
  external_id       TEXT NOT NULL,           -- id da conta no provedor ou "agência/conta" do OFX
  name              TEXT NOT NULL,           -- "Conta corrente", "Cartão Ultravioleta"
  type              TEXT NOT NULL CHECK (type IN ('checking','savings','credit_card')),
  wallet_id         INTEGER REFERENCES wallets(id) ON DELETE SET NULL,    -- carteira de destino
  payment_method_id INTEGER REFERENCES payment_methods(id),               -- ex.: cartão → "Crédito"
  paid_by_user_id   INTEGER NOT NULL REFERENCES users(id),
  sync_from         TEXT NOT NULL,           -- não importa nada antes desta data
  enabled           INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1)),
  UNIQUE (connection_id, external_id)
);

CREATE TABLE bank_transactions (
  id                INTEGER PRIMARY KEY,
  bank_account_id   INTEGER NOT NULL REFERENCES bank_accounts(id) ON DELETE CASCADE,
  external_id       TEXT NOT NULL,           -- id do provedor, FITID do OFX ou hash para CSV
  date              TEXT NOT NULL,
  amount_cents      INTEGER NOT NULL,        -- com sinal: negativo = saída
  description       TEXT NOT NULL,
  provider_category TEXT,
  installment       TEXT,                    -- "3/10" quando o provedor informa
  raw               TEXT NOT NULL,           -- JSON original, para reprocessar regras
  status            TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','accepted','matched','ignored')),
  transaction_id    INTEGER REFERENCES transactions(id) ON DELETE SET NULL,
  ignore_reason     TEXT CHECK (ignore_reason IN ('user','card_bill_payment','internal_transfer','duplicate')),
  suggested_category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
  imported_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  UNIQUE (bank_account_id, external_id)
);
CREATE INDEX bank_transactions_inbox ON bank_transactions(status, date DESC);

CREATE TABLE categorization_rules (
  id                INTEGER PRIMARY KEY,
  wallet_id         INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  pattern           TEXT NOT NULL,           -- substring case/acento-insensitive na descrição
  category_id       INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  rename_to         TEXT,                    -- "IFD*IFOOD 123" → "iFood"
  auto_accept       INTEGER NOT NULL DEFAULT 0 CHECK (auto_accept IN (0,1)),
  priority          INTEGER NOT NULL DEFAULT 0,
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
```

`transactions` ganha `source TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual','recurring','bank'))`.

Uma conexão pertence a um **usuário** (é o login bancário dele). Cada conta bancária aponta para uma **carteira**: o cartão pessoal vai para "Pessoal" e a conta conjunta para "Casa". Na caixa de entrada, cada membro vê as transações das contas mapeadas para a carteira ativa.

## Funil de ingestão (comum a 4a e 4b)

```
fonte (OFX/CSV | Pluggy) → normalizar → UPSERT bank_transactions (external_id único)
   → classificar: 1) transferência interna? 2) pagamento de fatura? 3) duplicata de lançamento manual?
                  4) regra de categorização → sugestão (+ auto-aceite)
   → caixa de entrada
```

Regras de classificação, executadas na ordem:

1. **Pagamento de fatura de cartão:** saída na conta corrente com descrição tipo "PAGTO FATURA", "PAGAMENTO CARTAO" ou similar, quando existe uma conta `credit_card` conectada. Vai para `ignored/card_bill_payment`. As compras já entram pelo cartão.
2. **Transferência interna:** saída numa conta e entrada de mesmo valor em outra conta conectada do mesmo usuário, com até 2 dias de diferença. Os dois lados vão para `ignored/internal_transfer`.
3. **Duplicata de lançamento manual:** existe `transaction` na carteira de destino com mesmo valor absoluto, data ±2 dias, `source='manual'` e sem vínculo bancário. A transação fica com `status='pending'` e uma sugestão de vínculo ("Já lançado: Mercado 09/10?"). Confirmar vira `matched`, sem criar lançamento novo.
4. **Regra de categorização:** a regra de maior prioridade cujo `pattern` aparece na descrição normalizada define `suggested_category_id` (e `rename_to`). Com `auto_accept`, cria a `transaction` direto como `accepted`.
5. **Sem regra:** usa o `provider_category` mapeado para uma categoria da carteira (tabela de mapeamento no código). Se não houver mapeamento, fica sem sugestão.

Valor negativo vira `kind='expense'`, positivo vira `kind='income'`. Estornos (positivos no cartão) entram como `income` com sugestão de vínculo ao gasto original, quando o valor bate. Parcelas usam a data e o valor da parcela, com "(3/10)" na descrição.

## Caixa de entrada (UI, rota `/caixa`)

- Lista de pendentes agrupada por dia. Cada linha mostra descrição, valor, conta de origem e categoria sugerida (editável inline).
- Ações:
  - **Aceitar** (`A`);
  - **Ignorar** (`I`);
  - **Vincular a lançamento existente**, quando há sugestão;
  - **Aceitar todos com sugestão**.
- Ao trocar a categoria de uma linha, a UI oferece **"Sempre categorizar 'UBER' como Transporte?"**. Isso cria uma `categorization_rule` e reaplica a regra às outras pendentes.
- Badge com o número de pendências na navegação.
- Em Ajustes, a seção **Conexões bancárias** mostra:
  - status;
  - última sincronização;
  - "sincronizar agora";
  - mapeamento conta → carteira, forma de pagamento e quem paga;
  - data de início (`sync_from`).

## 4a — Importação de arquivo (sem terceiros)

- `POST /api/bank-accounts/{id}/import` (multipart) aceita:
  - **OFX:** formato padrão dos bancos brasileiros, com FITID como `external_id`;
  - **CSV:** com perfis conhecidos (Nubank conta, Nubank cartão, Inter) e `external_id = sha256(data|valor|descrição|n-ésima ocorrência no arquivo)`.
- A conta bancária do tipo `file` é criada à mão em Ajustes.
- Reimportar o mesmo arquivo é idempotente, pelo `UNIQUE (bank_account_id, external_id)`.
- Parser OFX próprio (SGML/XML simples, cerca de 150 linhas) com testes de fixture anonimizados.

## 4b — Pluggy (Meu Pluggy)

**Spike antes de qualquer código (1 dia, com timebox):**
- Criar a aplicação no dashboard da Pluggy e conectar o Meu Pluggy.
- Confirmar que a API de transações funciona sem custo para as contas do próprio usuário, e com quais limites.
- Confirmar o fluxo de autorização (Pluggy Connect widget com o conector Meu Pluggy? `itemId` manual?).
- Confirmar o formato dos webhooks e a autenticação deles.
- Registrar tudo em `docs/superpowers/specs/<data>-pluggy-spike.md`.
- Se o spike falhar, a 4a continua sendo a solução e a 4b é arquivada.

**Integração (assumindo spike ok):**
- `internal/bank/pluggy`: client HTTP com `client_id`/`client_secret` (env `PLUGGY_CLIENT_ID`/`PLUGGY_CLIENT_SECRET`). A API key, de vida curta, fica em cache na memória.
- Endpoints usados: items, accounts, transactions (paginado, por `from`).
- Conectar: o frontend abre o Pluggy Connect com um connect token gerado pelo backend. No `onSuccess`, envia o `itemId` para `POST /api/bank-connections`. O backend busca as contas, e o usuário mapeia cada uma para uma carteira.
- Sincronização:
  - job a cada 6 horas;
  - `POST /api/bank-connections/{id}/sync` manual;
  - `POST /api/webhooks/pluggy` (rota pública, com validação do segredo do webhook) agenda o sync do item.
- Busca incremental desde `last_synced_at - 7 dias`, porque os bancos lançam com atraso e alteram transações pendentes. Faz upsert por `external_id`.
- Transação que muda de valor depois de aceita: marca `transaction` com aviso. Não altera automaticamente.
- `status='needs_reauth'` quando o provedor pede novo consentimento; a UI mostra o aviso. O consentimento do Open Finance expira em até 12 meses.

## Segurança

- `client_secret` só no `.env` da VDI, nunca no frontend nem no git.
- O campo `raw` pode conter dados sensíveis (nome de contraparte de Pix). Fica só no SQLite local. O backup segue o mesmo cuidado do banco.
- O webhook exige HTTPS público, o que depende do proxy da fase 2.

## Testes

- Fixtures OFX e CSV anonimizadas e testes do parser.
- Testes de tabela das regras de classificação: fatura, transferência interna, duplicata, regra, estorno e parcela.
- Idempotência: importar duas vezes não muda contagens.
- Client Pluggy testado contra um `httptest.Server` com respostas gravadas no spike.

## Critérios de aceite

- **4a:** importar o OFX de um mês de conta corrente e o CSV do cartão gera a caixa de entrada com:
  - pagamento de fatura ignorado;
  - transferências entre contas próprias ignoradas;
  - gastos já lançados à mão sugeridos como vínculo.
  Aceitar tudo produz um `/summary` sem dupla contagem.
- **4b:** conectar via Meu Pluggy e, em até 6 horas (ou no sync manual), ver as transações novas na caixa sem nenhuma ação.
