# Assistente financeiro, parte 1 (base e consulta) — plano de implementação

> **Para agentes:** SUB-SKILL OBRIGATÓRIA: use superpowers:subagent-driven-development (recomendado) ou superpowers:executing-plans para executar este plano tarefa por tarefa. Os passos usam caixas `- [ ]`.

**Objetivo:** o usuário coloca a própria chave da Anthropic em Ajustes, liga o assistente e passa a ter uma aba Chat que responde perguntas sobre os gastos (só leitura), com gráficos simples.

**Arquitetura:** o servidor Go guarda a chave criptografada (AES-GCM), roda o laço do modelo (streaming, ferramentas de leitura fixas, no máximo 6 voltas) e repassa o texto ao navegador por SSE. As ferramentas só enxergam as carteiras marcadas pelo usuário e reaproveitam as regras de associação existentes. O schema vira um único `schema.sql` (migrations removidas; o banco de produção é recriado no deploy).

**Tecnologias:** Go (`net/http`, `crypto/aes`, `database/sql` + modernc sqlite), React 19 + TanStack Query, SVG simples para gráficos. Nenhuma dependência nova.

**Spec:** `docs/superpowers/specs/2026-10-15-assistente-consulta-design.md`

## Restrições globais

- Código, comentários e testes em inglês; textos da interface e documentação em pt-BR.
- IDs de modelo exatos: `claude-haiku-5-5`, `claude-sonnet-5-5` (padrão), `claude-opus-5-5`.
- Retenção: inteiro de 1 a 30, padrão 7. Texto da pergunta: 1 a 4000 caracteres. Voltas do laço: no máximo 6. `max_tokens` por resposta: 1024. Limite de uso: 20 mensagens por minuto por usuário. Uma resposta em andamento por usuário.
- Chave mestra: `GASTOS_SECRET_KEY` = 32 bytes em base64. Ausente → assistente indisponível, servidor sobe normalmente. Presente e malformada → erro de configuração.
- A chave da Anthropic nunca aparece em resposta de API, log ou mensagem de erro; a API só mostra `keyHint` (4 últimos caracteres).
- Valores sempre em centavos nas ferramentas; datas `YYYY-MM-DD` no fuso `cfg.Location`.
- Visual: tokens, contraste, foco e regras de `alignment.test.ts` e `focus.test.ts` continuam valendo. Sem texto centralizado fora das exceções.
- Conversa vazia mostra só o campo de texto (sem sugestões).
- Antes de cada commit: `go vet ./...`, `go test -race ./...`, `npx tsc --noEmit`, `npm run lint`, `npx vitest run` (em `web/`). Mensagens de commit terminam com a linha `Co-Authored-By` indicada pelo ambiente.

## Review Focus

- Dois envios ao mesmo tempo do mesmo usuário (duas abas, duplo clique): o segundo recebe 409 "já existe uma resposta em andamento" e nada é gravado duas vezes. → Tarefa 6.
- O navegador fecha a conexão no meio da resposta: a chamada à Anthropic é cancelada, a mensagem do usuário fica gravada e nenhuma resposta incompleta é gravada. → Tarefa 6.
- Quem saiu de uma carteira (ou nunca foi membro) escolhe essa carteira no chat: 403, sem dados. → Tarefa 6.
- `GASTOS_SECRET_KEY` trocada depois de salvar a chave: o chat devolve a mensagem "informe a chave de novo" (não 500) e `GET /api/me/ai` mostra `configured: false`. → Tarefas 4 e 6.
- Parâmetros ruins vindos do modelo (período invertido, maior que 5 anos, limite fora da faixa, campo desconhecido como `wallet_id`): a ferramenta devolve erro ao modelo (`is_error`), nunca consulta e nunca derruba o chat. Período sem lançamentos devolve lista vazia, não erro. → Tarefa 5.

---

### Tarefa 1: schema único, sem migrations, com as tabelas do assistente

**Arquivos:**
- Modificar: `internal/db/schema.sql`, `internal/db/db.go`
- Remover: `internal/db/migrations/0002_bank_import.sql`, `internal/db/migration_test.go`
- Testar: `internal/db/db_test.go`

**Interfaces:**
- Produz: `db.Open(path string) (*sql.DB, error)` (mesma assinatura) que cria o schema completo num banco vazio e é idempotente (abrir duas vezes não falha nem perde dados).
- Produz: tabelas `ai_settings(user_id PK→users ON DELETE CASCADE, api_key_enc BLOB NOT NULL, key_hint TEXT NOT NULL, model TEXT NOT NULL DEFAULT 'claude-sonnet-5-5' CHECK (model IN (...os três IDs...)), enabled INTEGER NOT NULL DEFAULT 0, retention_days INTEGER NOT NULL DEFAULT 7 CHECK (retention_days BETWEEN 1 AND 30), updated_at TEXT)` e `chat_messages(id PK, user_id→users CASCADE, wallet_id→wallets CASCADE, role CHECK IN ('user','assistant'), content TEXT, charts TEXT NOT NULL DEFAULT '[]', wallet_ids TEXT NOT NULL DEFAULT '[]', input_tokens INTEGER NOT NULL DEFAULT 0, output_tokens INTEGER NOT NULL DEFAULT 0, created_at TEXT default agora UTC)` com índices `(user_id, wallet_id, id)` e `(created_at)`.

- [ ] **Passo 1: testes que falham** em `internal/db/db_test.go`: `TestOpenCreatesFullSchema` (abre banco novo e confere que existem `bank_accounts`, `bank_transactions`, `categorization_rules`, `ai_settings`, `chat_messages` e a coluna `transactions.source` com o CHECK de `'manual','recurring','bank'`); `TestOpenIsIdempotent` (abre, insere um usuário, fecha, reabre, o usuário continua); `TestAISettingsConstraints` (retenção 0 e 31 e um modelo fora da lista são recusados; 1 e 30 passam).
- [ ] **Passo 2: rodar** `go test ./internal/db` — esperado: FALHA (tabelas inexistentes).
- [ ] **Passo 3: juntar o conteúdo de `0002_bank_import.sql` no `schema.sql`** (a coluna `source` entra direto na definição de `transactions`; o `UPDATE` de backfill some; as três tabelas bancárias e seus índices entram) e acrescentar as duas tabelas novas. Em `db.go`, remover `migrations`, `migrationBankImport`, as diretivas de embed das migrations e a função `migrate`; `Open` executa o `schemaSQL` apenas quando `PRAGMA user_version = 0` e então grava `PRAGMA user_version = 1`, dentro de uma transação. Apagar a pasta `migrations` e `migration_test.go`.
- [ ] **Passo 4: rodar** `go test ./...` — esperado: PASSA (corrigir qualquer teste que citava migrations).
- [ ] **Passo 5: commit** `feat(db): single schema, assistant tables, drop migrations`.

---

### Tarefa 2: configuração e cifra da chave

**Arquivos:**
- Criar: `internal/secret/secret.go`, `internal/secret/secret_test.go`
- Modificar: `internal/config/config.go`, `internal/config/config_test.go`

**Interfaces:**
- Produz: `secret.ParseKey(b64 string) ([]byte, error)` (erro se não decodificar em exatamente 32 bytes); `secret.Seal(key, plaintext []byte) ([]byte, error)` (nonce aleatório na frente do texto cifrado); `secret.Open(key, sealed []byte) ([]byte, error)` (erro se a chave for outra ou o dado foi alterado).
- Produz: `config.Config.SecretKey []byte` (nil se `GASTOS_SECRET_KEY` vazia) e `config.Config.AnthropicURL string` (de `GASTOS_ANTHROPIC_URL`, padrão `https://api.anthropic.com`; existe para os testes e2e).

- [ ] **Passo 1: testes que falham**: `TestSealOpenRoundTrip`; `TestOpenWithAnotherKeyFails`; `TestOpenTamperedFails`; `TestSealUsesFreshNonce` (duas cifras do mesmo texto diferem); `TestParseKeyRejectsWrongLength`. Em `config_test.go`: chave ausente → `SecretKey == nil`; chave válida → 32 bytes; chave malformada → erro de `Load`; `AnthropicURL` padrão e sobrescrito.
- [ ] **Passo 2: rodar** `go test ./internal/secret ./internal/config` — esperado: FALHA.
- [ ] **Passo 3: implementar** com AES-256-GCM da biblioteca padrão e `Load` conforme as interfaces.
- [ ] **Passo 4: rodar** os mesmos testes — esperado: PASSA.
- [ ] **Passo 5: commit** `feat(config): GASTOS_SECRET_KEY and AES-GCM sealing for stored API keys`.

---

### Tarefa 3: cliente da API da Anthropic (streaming e ferramentas)

**Arquivos:**
- Criar: `internal/anthropic/client.go`, `internal/anthropic/client_test.go`

**Interfaces:**
- Produz:
  - `type Client struct { BaseURL, APIKey string; HTTP *http.Client }`
  - `type Block struct { Type string; Text string; ID, Name string; Input json.RawMessage; ToolUseID string; Content string; IsError bool }` com tags JSON da API (`text`, `tool_use`, `tool_result`).
  - `type Message struct { Role string; Content []Block }`
  - `type Tool struct { Name, Description string; InputSchema json.RawMessage }`
  - `type Request struct { Model, System string; Messages []Message; Tools []Tool; MaxTokens int }`
  - `type Result struct { Blocks []Block; StopReason string; InputTokens, OutputTokens int }`
  - `type APIError struct { Status int; Kind, Message string }` (implementa `error`; nunca contém a chave)
  - `func (c *Client) Stream(ctx context.Context, req Request, onText func(string)) (Result, error)`
  - `func (c *Client) Check(ctx context.Context, model string) error` (chamada mínima com `max_tokens: 1`)
- Contrato: `POST {BaseURL}/v1/messages` com cabeçalhos `x-api-key`, `anthropic-version: 2023-06-01`, `content-type: application/json` e `"stream": true`. Lê os eventos SSE `content_block_start`, `content_block_delta` (`text_delta`, `input_json_delta`), `content_block_stop`, `message_delta` (stop_reason, usage) e `error`. Texto chega a `onText` conforme os deltas.

- [ ] **Passo 1: testes que falham** com `httptest.Server`: `TestStreamTextDeltas` (vários `text_delta` chamam `onText` na ordem e `Result.Blocks` traz o texto inteiro); `TestStreamToolUse` (um bloco `tool_use` com JSON de entrada montado de vários `input_json_delta`; `StopReason == "tool_use"`); `TestStreamSendsHeadersAndBody` (cabeçalhos, `model`, `tools`, `stream:true`); `TestStreamHTTPErrors` (401, 429 e 529 viram `*APIError` com `Status` e sem a chave na mensagem); `TestStreamCancelStopsRequest` (cancelar o contexto interrompe e devolve `context.Canceled`); `TestCheck` (200 → nil; 401 → `*APIError`).
- [ ] **Passo 2: rodar** `go test ./internal/anthropic` — esperado: FALHA.
- [ ] **Passo 3: implementar** com `net/http` e um leitor de linhas `bufio.Scanner` de buffer grande; sem dependências.
- [ ] **Passo 4: rodar** — esperado: PASSA.
- [ ] **Passo 5: commit** `feat(anthropic): minimal streaming client with tool use`.

---

### Tarefa 4: configurações do assistente na API

**Arquivos:**
- Criar: `internal/api/ai_settings.go`, `internal/api/ai_settings_test.go`
- Modificar: `internal/api/server.go` (rotas, campo do cliente Anthropic), `internal/api/auth.go` (`me` devolve `assistantEnabled`)

**Interfaces:**
- Consome: `secret.Seal/Open`, `anthropic.Client.Check`, `config.Config.SecretKey/AnthropicURL`.
- Produz rotas (todas com `user(...)`): `GET /api/me/ai` → `{available, configured, keyHint, model, enabled, retentionDays}`; `PUT /api/me/ai` com `{apiKey?, model?, enabled?, retentionDays?}`; `DELETE /api/me/ai/key` (204; remove a chave e desliga).
- Produz: `func (s *Server) userAIKey(ctx context.Context, userID int64) (key string, model string, err error)` que devolve `errAIKeyUnreadable` quando a decifra falha e `errAIDisabled` quando não há chave ou `enabled` é falso; `GET /api/me` passa a incluir `assistantEnabled` (true só com chave legível e `enabled`).
- Regras: `apiKey` ausente mantém a atual; `enabled:true` sem chave → erro de campo; retenção fora de 1–30 e modelo fora da lista → erros por campo; sem `SecretKey` no servidor, `available:false` e o `PUT` com `apiKey` responde 400 "assistente indisponível neste servidor"; ao salvar uma chave nova, `Check` valida (401 → erro de campo `apiKey` "chave recusada pela Anthropic"; falha de rede → salva e responde com `warning`).

- [ ] **Passo 1: testes que falham** em `ai_settings_test.go` (servidor Anthropic falso via `httptest` e `AnthropicURL` no `newTestAppWith`): `TestAISettingsDefaults`; `TestSaveKeyNeverReturnsIt` (a resposta e o `GET` mostram só `keyHint` e a chave não aparece em nenhum corpo); `TestSaveKeyStoredEncrypted` (a coluna `api_key_enc` não contém o texto da chave); `TestRejectedKeyIsNotSaved`; `TestRetentionAndModelValidation`; `TestEnableNeedsKey`; `TestUnavailableWithoutSecretKey`; `TestRemoveKeyDisables`; `TestKeyMissingAfterSecretRotation` (troca `SecretKey` do servidor de teste; `configured:false`, `assistantEnabled:false`); `TestMeReportsAssistantEnabled`; `TestSettingsArePerUser` (a chave da Ana não aparece para a Bia).
- [ ] **Passo 2: rodar** `go test ./internal/api -run 'AISettings|SaveKey|Retention|Enable|Unavailable|RemoveKey|SecretRotation|MeReports|PerUser'` — esperado: FALHA.
- [ ] **Passo 3: implementar** handlers e rotas conforme as interfaces, usando `decode`, `fieldErrors` e `errBadRequest`; `New` cria o cliente com `cfg.AnthropicURL` (ou o padrão).
- [ ] **Passo 4: rodar** os testes da etapa e `go test ./internal/api` — esperado: PASSA.
- [ ] **Passo 5: commit** `feat(api): assistant settings with an encrypted per-user API key`.

---

### Tarefa 5: ferramentas de leitura do consultor

**Arquivos:**
- Criar: `internal/api/assistant_tools.go`, `internal/api/assistant_tools_test.go`

**Interfaces:**
- Produz:
  - `type chartDTO struct { Kind string; Title string; Points []chartPoint }` (`Kind`: `bars|line|pie`; `chartPoint{Label string; Value float64}`; até 24 pontos; JSON `kind`, `title`, `points`, `label`, `value`).
  - `func assistantTools() []anthropic.Tool` com `resumo_por_categoria`, `totais_por_periodo`, `listar_lancamentos`, `maiores_gastos`, `mostrar_grafico`, cada uma com `input_schema` JSON (`additionalProperties: false`).
  - `func (s *Server) runTool(ctx context.Context, walletIDs []int64, name string, input json.RawMessage) (text string, chart *chartDTO, err error)` — `walletIDs` vem sempre do servidor; `err` é erro de uso da ferramenta (volta ao modelo como `is_error`).
- Regras: `de`/`ate` obrigatórios (`YYYY-MM-DD`), `de <= ate`, intervalo máximo de 5 anos; `agrupar` ∈ `dia|semana|mes`; `limite` 1–50; `n` 1–20; texto de busca até 80 caracteres. Resultados em JSON compacto com valores em centavos e, quando há mais de uma carteira, o nome da carteira em cada item. Considera só `kind = 'expense'` nos resumos de gasto, e informa o total de receitas separadamente em `totais_por_periodo`. Campos desconhecidos na entrada → erro. Reaproveita `s.totals` para totais; consultas novas ficam neste arquivo.

- [ ] **Passo 1: testes que falham** (dados criados pela API de teste com duas carteiras e dois usuários): `TestResumoPorCategoria` (soma e participação corretas, só as carteiras passadas); `TestTotaisPorPeriodoGroups` (dia, semana, mês); `TestListarLancamentosFilters` (categoria, texto, limite); `TestMaioresGastosOrder`; `TestMostrarGraficoReturnsChart` (validação: tipo inválido, mais de 24 pontos, título vazio); `TestToolsOnlySeeGivenWallets` (uma carteira fora de `walletIDs` nunca entra, mesmo com lançamentos); `TestToolRejectsBadInput` (período invertido, maior que 5 anos, `limite` 0 e 51, `wallet_id` como campo desconhecido, JSON inválido); `TestEmptyPeriodIsNotAnError`; `TestToolNamesWalletWhenMany`.
- [ ] **Passo 2: rodar** `go test ./internal/api -run 'Resumo|Totais|Listar|Maiores|Grafico|ToolsOnly|ToolRejects|EmptyPeriod|ToolNames'` — esperado: FALHA.
- [ ] **Passo 3: implementar** as cinco ferramentas e o despachante `runTool`; descrições das ferramentas em pt-BR dizendo ao modelo quando usar cada uma.
- [ ] **Passo 4: rodar** — esperado: PASSA.
- [ ] **Passo 5: commit** `feat(api): read-only tools for the finance assistant`.

---

### Tarefa 6: endpoint do chat (laço do modelo, SSE, histórico)

**Arquivos:**
- Criar: `internal/api/chat.go`, `internal/api/chat_test.go`
- Modificar: `internal/api/server.go` (rotas, `chatLimiter`, mapa de respostas em andamento)

**Interfaces:**
- Consome: `s.userAIKey`, `anthropic.Client.Stream`, `assistantTools`, `s.runTool`.
- Produz rotas (todas `user(...)`): `POST /api/chat` `{wallet int64, wallets []int64, text string}` com resposta `text/event-stream` e eventos `status` (`{text}`), `delta` (`{text}`), `chart` (`chartDTO`), `done` (`{inputTokens, outputTokens}`) e `error` (`{message}`); `GET /api/chat?wallet=ID` → `[{id, role, content, charts, walletIds, createdAt}]` em ordem crescente; `DELETE /api/chat?wallet=ID` → 204.
- Produz: `func systemPrompt(now time.Time, loc *time.Location, wallets []walletInfo) string` em pt-BR: consultor financeiro, respostas curtas e diretas (número primeiro, sem rodeios), valores em R$, nunca inventar, usar as ferramentas, gráfico só quando ajudar, a data de hoje e os nomes das carteiras e categorias.
- Regras: valida texto (1–4000 caracteres), `wallets` não vazio sem repetidos e com o usuário como membro de todos (senão 403), assistente ativo (senão 409 com mensagem clara); `chatLimiter` de 20/min por usuário (429); um envio por usuário por vez (409); grava a mensagem do usuário antes de chamar a Anthropic; envia no máximo as últimas 20 mensagens (corte também por 24.000 caracteres); laço com no máximo 6 voltas; erros da Anthropic (401, 429, 529, rede) e `errAIKeyUnreadable` viram evento `error` em pt-BR sem citar a chave; resposta final e gráficos gravados só quando completos; contexto cancelado não grava resposta.

- [ ] **Passo 1: testes que falham** com Anthropic falsa que responde roteiros SSE: `TestChatStreamsAnswer` (eventos `delta` e `done`, mensagens gravadas e `GET` devolve as duas); `TestChatToolLoop` (o falso pede `resumo_por_categoria`, recebe o `tool_result` certo e então responde); `TestChatChartEvent`; `TestChatStopsAfterSixTurns`; `TestChatRejectsOtherUsersWallet` (403 e nenhuma chamada à Anthropic); `TestChatNeedsEnabledAssistant`; `TestChatValidatesText`; `TestChatRateLimit`; `TestChatOneAtATime` (segundo envio durante o primeiro → 409; depois de terminar, libera); `TestChatClientCancelSavesNoAnswer`; `TestChatAnthropicErrorsBecomeEvents` (401, 429, 529: evento `error` em pt-BR, sem a chave no corpo); `TestChatKeyUnreadableAfterRotation`; `TestChatHistoryIsPerUserAndWallet`; `TestChatDeleteClearsOnlyThatConversation`; `TestChatSendsOnlyRecentHistory`; `TestSystemPromptMentionsTodayAndWallets`.
- [ ] **Passo 2: rodar** `go test ./internal/api -run 'Chat|SystemPrompt'` — esperado: FALHA.
- [ ] **Passo 3: implementar** `chat.go` conforme as regras; escrever SSE com `http.ResponseController` e `Flush`, no mesmo estilo de `events.go`.
- [ ] **Passo 4: rodar** `go test -race ./internal/api` — esperado: PASSA.
- [ ] **Passo 5: commit** `feat(api): finance assistant chat with streaming and read-only tools`.

---

### Tarefa 7: retenção das conversas

**Arquivos:**
- Modificar: `internal/api/housekeeping.go`, `internal/api/ops_test.go`

**Interfaces:**
- Consome: tabelas `chat_messages` e `ai_settings`; `s.now()`.
- Produz: o `Housekeeping` apaga `chat_messages` mais velhas que `retention_days` de cada usuário (`created_at <= now - retention_days` dias); usuário sem linha em `ai_settings` usa 7 dias.

- [ ] **Passo 1: testes que falham** em `ops_test.go` (usando `runHousekeeping`): `TestHousekeepingDeletesExpiredChat` (usuário com 3 dias de retenção: mensagem de 4 dias some, de 2 dias fica); `TestHousekeepingRetentionIsPerUser` (outro usuário com 30 dias mantém a mensagem de 10 dias); `TestHousekeepingDefaultsToSevenDays`.
- [ ] **Passo 2: rodar** `go test ./internal/api -run Housekeeping` — esperado: FALHA nos três novos.
- [ ] **Passo 3: implementar** um único `DELETE` com subconsulta em `ai_settings` dentro de `run`, registrando erro como os demais passos.
- [ ] **Passo 4: rodar** — esperado: PASSA.
- [ ] **Passo 5: commit** `feat(api): delete chat messages past each user's retention`.

---

### Tarefa 8: Ajustes > Assistente (frontend)

**Arquivos:**
- Criar: `web/src/pages/settings/AssistantSection.tsx`, `web/src/pages/settings/AssistantSection.test.tsx`
- Modificar: `web/src/api/types.ts` (`Me.assistantEnabled: boolean`, `AISettings`), `web/src/api/queries.ts` (`useAISettings`), `web/src/pages/Settings.tsx` (seção `assistente`, grupo 2, rótulo "Assistente"), `web/src/pages/SettingsPage.test.tsx` (lista de seções), `web/src/test/fixtures.tsx` (`assistantEnabled: false`)

**Interfaces:**
- Consome: `GET/PUT /api/me/ai`, `DELETE /api/me/ai/key`; componentes `ListGroup`, `Segmented`, `Switch`, `useAction`.
- Produz: `export function AssistantSection()`; `export const useAISettings = () => UseQueryResult<AISettings>` (chave `["ai"]`, invalidada ao salvar) e a mutação também invalida `["me"]`.
- Comportamento: se `available` é falso, mostra uma linha explicando que o servidor não tem `GASTOS_SECRET_KEY` e nada mais; sem chave salva, campo de senha "Chave da API" com botão "Salvar chave"; com chave, mostra `•••• hint`, "Trocar" e "Remover"; Segmented de modelo (Haiku, Sonnet, Opus); interruptor "Ativar chat" (desabilitado sem chave); campo numérico "Guardar conversas por (dias)" 1–30; aviso fixo de privacidade em pt-BR ("Os dados dos lançamentos consultados são enviados à Anthropic com a sua chave."); erro de campo `apiKey` aparece abaixo do campo.

- [ ] **Passo 1: testes que falham** em `AssistantSection.test.tsx`: indisponível sem chave mestra; salva a chave e envia `{apiKey}`; depois de salva nunca mostra o valor, só o final; "Remover" chama o `DELETE`; trocar o modelo envia `{model}`; o interruptor envia `{enabled}` e está desabilitado sem chave; retenção fora de 1–30 mostra erro e não envia; mostra erro do servidor em `apiKey`; mostra o aviso de privacidade. Em `SettingsPage.test.tsx`: a lista passa a ter 10 seções, com "Assistente" no grupo "Dados".
- [ ] **Passo 2: rodar** `npx vitest run src/pages` — esperado: FALHA.
- [ ] **Passo 3: implementar** seguindo o padrão de `WalletSection.tsx` (formulários `list-row`, `useAction`).
- [ ] **Passo 4: rodar** `npx vitest run` e `npx tsc --noEmit` — esperado: PASSA.
- [ ] **Passo 5: commit** `feat(web): Ajustes > Assistente with the API key, model and retention`.

---

### Tarefa 9: gráficos em SVG

**Arquivos:**
- Criar: `web/src/components/Chart.tsx`, `web/src/components/Chart.test.tsx`
- Modificar: `web/src/styles.css` (classes `.chart`), `web/src/api/types.ts` (`ChartData`)

**Interfaces:**
- Produz: `export function Chart({ chart }: { chart: ChartData })` com `ChartData = { kind: "bars" | "line" | "pie"; title: string; points: { label: string; value: number }[] }`; renderiza `<figure>` com `<figcaption>` do título, SVG com `role="img"` e `aria-label` listando "rótulo: valor" (valores em centavos, formatados com `formatBRL` de `web/src/lib`), cores só por variáveis de `tokens.css`.

- [ ] **Passo 1: testes que falham**: renderiza cada tipo (barras: uma barra por ponto; linha: um `polyline`; pizza: uma fatia por ponto); o `aria-label` traz "Alimentação: R$ 820,00, 31%" para pizza; pontos vazios mostram "Sem dados"; valores zero não quebram a pizza; rótulos longos não estouram (classe de truncamento).
- [ ] **Passo 2: rodar** `npx vitest run src/components/Chart.test.tsx` — esperado: FALHA.
- [ ] **Passo 3: implementar** SVG puro, sem biblioteca; estilos no `styles.css` respeitando `alignment.test.ts` (sem `text-align: center` novo fora da lista).
- [ ] **Passo 4: rodar** `npx vitest run` — esperado: PASSA (inclui os testes de estilo).
- [ ] **Passo 5: commit** `feat(web): small SVG charts for the assistant`.

---

### Tarefa 10: aba Chat (frontend)

**Arquivos:**
- Criar: `web/src/pages/Chat.tsx`, `web/src/pages/Chat.test.tsx`, `web/src/components/WalletPicker.tsx`, `web/src/components/WalletPicker.test.tsx`, `web/src/lib/chatStream.ts`, `web/src/lib/chatStream.test.ts`
- Modificar: `web/src/components/nav.ts` e `nav.test.ts` (`navItems(splitting, assistant)` acrescenta `{ to: "/chat", label: "Chat", icon: "chat" }` depois de Caixa), `TabBar.tsx`, `Sidebar.tsx` (passam `assistantEnabled`), `Icon.tsx` (ícone `chat`), `App.tsx` (rota `chat`, redireciona para `/` se o assistente estiver desligado), `context.ts`/`Shell.tsx` (`assistantEnabled` no contexto), `api/queries.ts` (`useChat(walletId)`), `styles.css`

**Interfaces:**
- Produz: `streamChat(body: { wallet: number; wallets: number[]; text: string }, on: { delta(t: string): void; status(t: string): void; chart(c: ChartData): void; done(): void; error(m: string): void }, signal: AbortSignal): Promise<void>` — `fetch` POST + leitura do corpo em stream, separando eventos SSE (`event:`/`data:`), tolerando eventos cortados entre pedaços.
- Produz: `WalletPicker({ wallets, selected, onChange })` — botão que mostra o nome da carteira ou "N carteiras", abre uma lista com caixas de marcar e "Selecionar todas"; impede desmarcar a última.
- `Chat()`: padrão de seleção é só a carteira ativa e volta a ela quando a carteira ativa muda; mensagens do usuário à direita e do consultor à esquerda em texto simples; `Chart` dentro da conversa; linha de status enquanto consulta; campo de texto (Enter envia, Shift+Enter quebra linha); botão enviar vira "Parar" durante a resposta (aborta o `fetch`); menu "Nova conversa" chama o `DELETE`; mostra o erro do evento `error` como mensagem do consultor; conversa vazia mostra só o campo de texto.

- [ ] **Passo 1: testes que falham**: `chatStream.test.ts` (eventos completos, eventos partidos em dois pedaços, `error` chama `on.error`, abortar resolve sem lançar); `WalletPicker.test.tsx` (rótulo com uma e com várias carteiras, selecionar todas, não desmarca a última, usa teclado); `Chat.test.tsx` (padrão só a carteira atual; envia com Enter e não com Shift+Enter; mostra a resposta chegando em pedaços e o gráfico; botão "Parar" aborta; "Nova conversa" limpa; erro do servidor aparece; sem sugestões na conversa vazia; troca de carteira ativa reinicia a seleção); `nav.test.ts` (Chat só com assistente ligado; "Chat" depois de "Caixa"); `TabBar`/`Sidebar` mostram o item.
- [ ] **Passo 2: rodar** `npx vitest run` — esperado: FALHA nos novos.
- [ ] **Passo 3: implementar** conforme as interfaces; dropdown no estilo das pickers existentes (`Sheet` no celular, popover no desktop só se já houver um padrão; senão usar `Sheet` nos dois).
- [ ] **Passo 4: rodar** `npx vitest run`, `npx tsc --noEmit`, `npm run lint` — esperado: PASSA.
- [ ] **Passo 5: commit** `feat(web): Chat tab with a wallet picker and streamed answers`.

---

### Tarefa 11: e2e, documentação e roteiro de deploy

**Arquivos:**
- Criar: `web/e2e/assistant.spec.ts`, `web/e2e/fake-anthropic.mjs`, `docs/assistente.md`
- Modificar: `web/playwright.config.ts` (sobe o Anthropic falso e define `GASTOS_SECRET_KEY` de teste e `GASTOS_ANTHROPIC_URL` para ele), `README.md` (link para `docs/assistente.md`), `docs/ops/deploy.md` ou o doc de operação existente (variável nova)

**Interfaces:**
- Consome: tudo acima. O falso responde ao `Check` e a duas perguntas de roteiro: uma que pede `resumo_por_categoria` seguida de `mostrar_grafico` e texto, e uma que responde só texto.

- [ ] **Passo 1: teste e2e que falha** (`desktop` e `mobile`): cadastro; em Ajustes > Assistente salva uma chave de teste, escolhe modelo e liga; a aba Chat aparece; envia "quanto gastei este mês?"; vê a resposta e o gráfico; o seletor mostra a carteira atual e permite "Selecionar todas"; "Nova conversa" limpa; recarregar a página mantém a conversa; sem chave a aba não aparece.
- [ ] **Passo 2: rodar** `npm run build` e `npm run e2e -- -g assistente` — esperado: FALHA.
- [ ] **Passo 3: implementar** o falso (Node, sem dependências) e a configuração; a chave real nunca entra em teste.
- [ ] **Passo 4: rodar** a suíte completa (`go test -race ./...`, `npx vitest run`, `npm run e2e`) — esperado: PASSA.
- [ ] **Passo 5: escrever `docs/assistente.md`** (pt-BR): o que é, como gerar `GASTOS_SECRET_KEY` (`openssl rand -base64 32`) e onde colocar no serviço da VPS, como cada usuário coloca a própria chave, privacidade (dados consultados vão à Anthropic), custos (cobrados na chave de cada um; modelo e retenção em Ajustes), o que acontece se a chave mestra mudar, e o roteiro de deploy: backup, parar o serviço, mover o banco antigo para fora do caminho, subir a nova versão (cria o banco novo) e cada pessoa recria a conta.
- [ ] **Passo 6: commit** `test(e2e): assistant flow against a fake Anthropic; docs and deploy steps`.

---

## Fora deste plano

Ferramentas de escrita com confirmação (parte 2), voz (parte 3) e a execução do deploy de produção, que exige o "sim" do usuário no momento (apaga o banco atual) e a chave mestra gerada na VPS.
