[English](README.md) · **Português**

# gastos.app

Controle financeiro pessoal e compartilhado. Um único binário Go serve a API e o app web, e os dados ficam num arquivo SQLite.

## O que faz

- Carteiras com membros e papéis (owner, editor, reader)
- Gastos e entradas, com categorias e formas de pagamento editáveis
- Divisão de gastos entre membros (em basis points) e acertos
- Recorrências mensais de gastos, entradas e acertos, divididas do mesmo jeito
- Cartões de crédito: a compra cai na fatura certa, pelo dia de fechamento e de vencimento do cartão
- Orçamento mensal por categoria e resumo do mês
- Importação de extrato do banco (OFX e CSV) para uma caixa de entrada, com regras de categoria. Veja [Importing a bank statement](docs/bank-import.md)
- Um assistente que responde perguntas sobre os seus lançamentos em linguagem natural, com a sua chave da Anthropic. Veja [Query assistant](docs/assistant.md)
- Atualização em tempo real entre abas, aparelhos e membros, sem recarregar (SSE em `/api/events`)

### Divisão padrão por pessoa

Em Ajustes > Pessoas, quem é dono da carteira pode definir a "Parte padrão (%)" de cada membro que entra na divisão por padrão (por exemplo 60% e 40%). Quando todos que dividem por padrão têm uma porcentagem, um gasto novo é dividido por elas, ajustadas para somar 100%. Se alguém ficar em branco, a divisão é igual. No lançamento (e em cada recorrência), o botão "Ajustar" escolhe outras porcentagens só para aquele gasto, como 50% para uma pessoa e 25% para cada uma das outras duas.

## Stack

- Go (1.26 ou mais novo), `net/http` com o roteador do Go 1.22+, `log/slog`
- SQLite via `modernc.org/sqlite` (Go puro, sem CGO)
- Sessões por cookie httpOnly, senhas com bcrypt
- App web: React, Vite e TypeScript, em `web/`

## Estrutura

```text
cmd/gastos/           main: config, banco, servidor e jobs
internal/api/         handlers HTTP
internal/anthropic/   cliente da API de mensagens da Anthropic, para o assistente
internal/bankimport/  leitura de extratos OFX e CSV e regras de categoria
internal/billing/     datas da fatura do cartão
internal/config/      variáveis de ambiente
internal/db/          schema.sql, migrações, abertura e backup do banco
internal/events/      bus de eventos por trás do SSE
internal/money/       rateio em centavos
internal/recurrence/  ocorrências mensais e materializador
internal/secret/      cifra das chaves de API guardadas
web/                  app web (web/dist é embutido no binário)
docs/                 design docs, planos, operação e decisões
```

## Rodando

```bash
GASTOS_COOKIE_SECURE=false go run ./cmd/gastos
```

No PowerShell:

```powershell
$env:GASTOS_COOKIE_SECURE = 'false'; go run ./cmd/gastos
```

O servidor sobe em `:8000`. Teste com `curl localhost:8000/healthz`.

## Configuração

| Variável | Padrão | |
|---|---|---|
| `GASTOS_ADDR` | `:8000` | endereço de escuta |
| `GASTOS_DB_PATH` | `./data/gastos.db` | arquivo SQLite |
| `GASTOS_BACKUP_DIR` | `./data/backups` | backups diários |
| `GASTOS_BACKUP_KEEP` | `14` | quantos backups manter |
| `GASTOS_TZ` | `America/Sao_Paulo` | fuso que define "hoje" para as recorrências |
| `GASTOS_COOKIE_SECURE` | `true` | `false` só em desenvolvimento local (http) |
| `GASTOS_DEV` | `false` | libera CORS para `localhost:5173` |
| `GASTOS_INVITE_CODE` | vazio | código fixo aceito no cadastro depois da primeira conta (mínimo de 8 caracteres). Opcional: o normal é gerar códigos de uso único com `gastos invite [duração]` (no servidor, `sudo gastos-invite`, depois de instalar o script como root: veja `docs/ops/vdi-setup.md`). Sem nenhum dos dois, o cadastro fecha depois do primeiro usuário |
| `GASTOS_SECRET_KEY` | vazio | chave mestra (32 bytes em base64) que cifra as chaves da Anthropic guardadas. Sem ela o assistente fica indisponível. Veja [Query assistant](docs/assistant.md) |
| `GASTOS_ANTHROPIC_URL` | `https://api.anthropic.com` | só para testes (e2e): aponta o assistente para um servidor falso |
| `GASTOS_FX_URL` | `https://economia.awesomeapi.com.br/last/USD-BRL` | cotação do dólar (AwesomeAPI) usada para estimar em reais o gasto do assistente; só muda em testes |

## Testes

```bash
# backend
go vet ./...
go test -race ./...

# app web (dentro de web/)
npm run lint        # Biome
npm run typecheck
npm test            # unidade e componentes
npm run build && npm run e2e   # navegador contra o binário Go real, em desktop e celular
```

O CI roda tudo isso, além de `gofmt`, `golangci-lint`, `govulncheck`, `npm audit`, zizmor e CodeQL.

## Banco de dados

O `internal/db/schema.sql` é a base congelada, na versão 4 (`PRAGMA user_version`). Toda mudança depois dela é uma migração em `internal/db/migrations/` (a `0005_...` leva o banco da versão 4 para a 5, e assim por diante), listada em `internal/db/db.go`. Ao subir, o servidor cria um banco novo (versão 0) com o `schema.sql` e todas as migrações. Um banco entre a versão 4 e a mais recente recebe só as migrações que faltam, no lugar e sem perder dados, cada uma na sua transação. Antes da primeira migração pendente de um banco com dados, o servidor guarda uma cópia ao lado dele (`gastos.db.pre-v<N>.bak`, em que N é a versão de antes). Uma cópia anterior com o mesmo nome é mantida com um carimbo de data no nome, e se a cópia falhar o app não sobe. Versões 1 a 3, ou mais novas do que o binário conhece, fazem o app recusar subir, sem alterar o banco. Para mudar o schema, crie a próxima migração. Nunca edite o `schema.sql` nem uma migração já publicada.

## API

Todas as rotas ficam sob `/api`. O usuário e a carteira ativa vêm do cookie de sessão e do path (`/api/wallets/{walletID}/...`). A referência completa está no [design doc do backend](docs/superpowers/specs/2026-10-09-backend-foundation-design.md).

## Contribuindo

Toda mudança começa numa issue e termina numa pull request que aponta para ela (`Closes #N`). Ao abrir uma issue aparecem formulários para bug, feature e tarefa. Veja o [AGENTS.md](AGENTS.md) para as convenções, o [docs/decisions.md](docs/decisions.md) para o porquê das coisas e o [docs/security.md](docs/security.md) para o modelo de ameaças. A documentação interna é em inglês.

## Licença

Apache-2.0.
