# Importação bancária (OFX e CSV) com caixa de entrada — plano

**Data:** 2026-10-11
**Status:** a implementar
**Spec de referência:** [open-finance-design](../specs/2026-10-09-open-finance-design.md) (etapa 4a). A 4b (Pluggy) continua dependendo do spike.

## Objetivo

Parar de digitar o que o banco já sabe. A pessoa importa o extrato (OFX ou CSV); as transações entram numa **caixa de entrada** com categoria sugerida; ela confirma em lote. Nada vira lançamento sem confirmação.

## Recorte desta etapa (menor versão que resolve o problema)

Dentro:
- Importar **OFX** (padrão dos bancos brasileiros) e **CSV genérico** com mapeamento de colunas escolhido na hora (data, descrição, valor).
- Contas bancárias "de arquivo" ligadas a uma carteira, uma forma de pagamento e quem paga.
- Caixa de entrada: aceitar, ignorar, vincular a um lançamento já existente, aceitar tudo que tem sugestão.
- Regras de categorização aprendidas quando a pessoa troca a categoria ("Sempre categorizar 'UBER' como Transporte").
- Deduplicação: reimportar o mesmo arquivo não duplica; lançamento manual parecido vira sugestão de vínculo.

Fora (ficam para depois, nesta ordem): perfis prontos por banco (Nubank, Inter), pagamento de fatura e transferência entre contas próprias, parcelas e estornos, Pluggy.

## Dados (`user_version` 2)

Migração nova em `internal/db` (a primeira usando o mecanismo de `migrations`):

- `bank_accounts(id, wallet_id, name, payment_method_id, paid_by_user_id, sync_from, enabled)`; só do tipo "arquivo" por enquanto, então as colunas de provedor da spec ficam para a 4b.
- `bank_transactions(id, bank_account_id, external_id, date, amount_cents, description, status, transaction_id, suggested_category_id, raw, imported_at)`, com `UNIQUE (bank_account_id, external_id)` e `status IN ('pending','accepted','matched','ignored')`.
- `categorization_rules(id, wallet_id, pattern, category_id, rename_to, priority)`.
- `transactions.source` (`'manual'|'recurring'|'bank'`), usado para saber de onde veio.

`external_id`: o `FITID` do OFX; no CSV, um hash de `data|valor|descrição|n-ésima ocorrência igual no arquivo`.

## Backend

Pacote `internal/bankimport`:
- `ParseOFX(r io.Reader) ([]Entry, error)`: parser próprio e tolerante (OFX 1.x é SGML sem fechamento de tags), com fixtures anonimizadas.
- `ParseCSV(r, mapping) ([]Entry, error)`: delimitador detectado (`;` ou `,`), números no formato brasileiro, datas `dd/mm/aaaa` e `aaaa-mm-dd`, cabeçalho opcional.
- `Classify(entry, rules, existing) Suggestion`: regra por padrão (sem acento e sem diferenciar maiúsculas, maior prioridade vence) e candidatos a vínculo (mesmo valor absoluto, data ±2 dias).

API (todas sob `/api/wallets/{walletID}`, papel mínimo `editor`):
- `GET/POST /bank-accounts`, `PUT/DELETE /bank-accounts/{id}`
- `POST /bank-accounts/{id}/import` (multipart, 5 MB no máximo) → devolve `{imported, duplicates}`
- `GET /inbox?status=pending` e `POST /inbox/{id}/accept` (categoria, descrição, divisão opcional), `/ignore`, `/match` (id do lançamento), `POST /inbox/accept-suggested`
- `GET/POST/DELETE /rules`

Valores com sinal vêm do arquivo: negativo vira `expense`, positivo `income`.

## Frontend

- Rota `/caixa` com badge de pendências na navegação.
- Lista de pendentes por dia; categoria sugerida editável na linha; ao trocar, oferece criar a regra.
- Em Ajustes, seção "Contas bancárias": cadastrar a conta e importar o arquivo (com a escolha de colunas para CSV).

## Tarefas (TDD, cada uma com seu commit)

- [ ] **1. Migração v2** e teste de que um banco novo e um banco na v1 chegam ao mesmo schema.
- [ ] **2. `ParseOFX`** com fixtures: valores com vírgula e ponto, datas com fuso, tags sem fechamento, arquivo vazio e arquivo que não é OFX.
- [ ] **3. `ParseCSV`** com `;` e `,`, número brasileiro, cabeçalho opcional, linhas em branco, colunas faltando.
- [ ] **4. `Classify`** (puro): regras, prioridade, acentos, sugestão de vínculo e limites da janela de ±2 dias.
- [ ] **5. Import e caixa de entrada** (integração): importação idempotente, aceitar cria o lançamento com `source='bank'` e valor/data corretos, ignorar, vincular, aceitar tudo sugerido, permissões por papel, limite de tamanho.
- [ ] **6. Regras**: criar a partir da troca de categoria reaplica às pendentes.
- [ ] **7. Frontend**: caixa de entrada e importação (testes de componente) e um e2e: importar um OFX pequeno, aceitar e ver o saldo mudar.
- [ ] **8. Docs**: como exportar o extrato do banco, formatos aceitos e limitações.

## Riscos

- Arquivos OFX variam muito entre bancos; por isso o parser é tolerante e eu preciso de extratos reais anonimizados para fixtures. Sem eles, o que fica garantido é o formato padrão.
- Reimportar um mês inteiro depois de lançar à mão gera sugestões de vínculo; quem aceita sem olhar pode duplicar. A tela mostra o candidato ao lado da linha.
- Dado financeiro em `raw`: fica só no SQLite local e entra no backup cifrado.
