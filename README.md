# gastos.app

Controle financeiro pessoal e compartilhado. Um único binário Go serve a API e o frontend, e os dados ficam num arquivo SQLite.

> **Em reescrita.** O backend novo (`cmd/` + `internal/`) está pronto; o frontend React (`web/`) ainda é um placeholder. Veja o [roadmap](docs/superpowers/specs/2026-10-09-roadmap.md).

## O que faz

- Carteiras com membros e papéis (owner, editor, reader)
- Gastos e entradas com categorias e formas de pagamento editáveis
- Divisão de gastos entre membros (em basis points) e acertos
- Recorrências mensais de gastos, entradas e acertos
- Orçamento mensal por categoria e resumo do mês
- Atualização em tempo real entre membros (SSE)

## Stack

- Go (≥ 1.26), `net/http` com o roteador do Go 1.22+, `log/slog`
- SQLite via `modernc.org/sqlite` (Go puro, sem CGO)
- Sessões por cookie httpOnly, senhas com bcrypt
- Frontend: React + Vite + TypeScript (fase 3, em `web/`)

## Estrutura

```text
cmd/gastos/        main: config, banco, servidor e jobs
internal/config/   variáveis de ambiente
internal/db/       schema.sql, abertura do banco, versionamento e backup
internal/money/    rateio em centavos
internal/recurrence/  ocorrências mensais e materializador
internal/events/   bus de eventos para o SSE
internal/api/      handlers HTTP
web/               frontend (dist/ é embutido no binário)
docs/superpowers/  specs e planos
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
| `GASTOS_TZ` | `America/Sao_Paulo` | fuso de "hoje" para recorrências |
| `GASTOS_COOKIE_SECURE` | `true` | `false` só em desenvolvimento local (http) |
| `GASTOS_DEV` | `false` | libera CORS para `localhost:5173` |
| `GASTOS_INVITE_CODE` | vazio | código exigido no cadastro depois da primeira conta (mínimo de 8 caracteres). Sem ele, o cadastro fecha depois do primeiro usuário |
| `GASTOS_SECRET_KEY` | vazio | chave mestra (32 bytes em base64) que cifra as chaves da Anthropic guardadas. Sem ela o assistente fica indisponível. Veja [Assistente de consulta](docs/assistente.md) |
| `GASTOS_ANTHROPIC_URL` | `https://api.anthropic.com` | só para testes (e2e): aponta o assistente para um servidor falso |

## Testes

```bash
# backend
go vet ./...
go test -race ./...

# frontend (dentro de web/)
npm run lint        # Biome
npm run typecheck
npm test            # unidade e componentes
npm run build && npm run e2e   # navegador contra o binário Go real, em desktop e celular
```

O CI roda tudo isso, além de `gofmt`, `govulncheck` e `npm audit`.

## Banco de dados

O `internal/db/schema.sql` é a base congelada, na versão 3 (`PRAGMA user_version`). Toda mudança depois dela é uma migração em `internal/db/migrations/` (a `0004_...` leva o banco da versão 3 para a 4, e assim por diante), registrada na lista `migrations` de `internal/db/db.go`. Ao subir, o servidor cria um banco novo (versão 0) com o `schema.sql` e todas as migrações; um banco entre a versão 3 e a mais recente recebe só as migrações que faltam, no lugar e sem perder dados, cada uma na sua transação. Versões 1 e 2, ou mais novas do que o binário conhece, fazem o app recusar subir. Para mudar o schema, crie a próxima migração; nunca edite o `schema.sql` nem uma migração já publicada.

## API

Todas as rotas ficam sob `/api`. Autenticação e carteira ativa vêm do cookie de sessão e do path (`/api/wallets/{walletID}/...`). A referência completa está na [spec do backend](docs/superpowers/specs/2026-10-09-backend-foundation-design.md).

Para importar extratos do banco (OFX/CSV), veja [Importar o extrato do banco](docs/bank-import.md).

Para perguntar aos seus próprios lançamentos em linguagem natural, veja [Assistente de consulta](docs/assistente.md).

## Licença

Apache-2.0.
