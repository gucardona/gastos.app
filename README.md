# gastos.app — Backend Go + SQLite

Controle financeiro pessoal com backend real em Go e banco SQLite.

## Estrutura

```
gastos/
├── go.mod
├── Makefile
└── src/
    ├── main.go                 ← servidor HTTP + roteamento
    ├── db/db.go                ← SQLite init + migrations automáticas
    ├── models/models.go        ← structs User, Expense, Income, Goal
    ├── middleware/auth.go      ← validação JWT em rotas protegidas
    ├── handlers/
    │   ├── auth.go             ← POST /api/auth/register|login|me
    │   ├── expenses.go         ← GET/POST/DELETE /api/expenses[/:id]
    │   ├── incomes.go          ← GET/POST/DELETE /api/incomes[/:id]
    │   └── goals.go            ← GET/POST/DELETE /api/goals[/:category]
    └── web/
        └── index.html          ← frontend Alpine.js (consome a API)
```

## Pré-requisitos

- Go 1.22+ com CGO habilitado
- `gcc` instalado (necessário para `mattn/go-sqlite3`)
- SQLite3 dev headers: `apt install libsqlite3-dev` (Ubuntu/Debian)

## Instalação

```bash
# 1. Baixar dependências
go mod tidy

# 2. Rodar em desenvolvimento
make run
# ou diretamente:
CGO_ENABLED=1 go run ./src/main.go

# 3. Acessar
open http://localhost:8000
```

## Build para produção

```bash
make build
./gastos
```

## Variáveis de ambiente

| Variável  | Padrão        | Descrição                   |
|-----------|---------------|-----------------------------|
| `PORT`    | `8000`        | Porta HTTP do servidor      |
| `DB_PATH` | `./gastos.db` | Caminho do arquivo SQLite   |

```bash
PORT=3000 DB_PATH=/data/gastos.db ./gastos
```

## API REST

Todas as rotas protegidas exigem header `Authorization: Bearer <token>`.

### Auth

| Método | Rota                  | Body                                      | Resposta               |
|--------|-----------------------|-------------------------------------------|------------------------|
| POST   | `/api/auth/register`  | `{name, email, password}`                 | `{token, user}`        |
| POST   | `/api/auth/login`     | `{email, password}`                       | `{token, user}`        |
| GET    | `/api/auth/me`        | —                                         | `{id, name, email}`    |

### Gastos

| Método | Rota                   | Descrição                    |
|--------|------------------------|------------------------------|
| GET    | `/api/expenses`        | Lista todos os gastos        |
| GET    | `/api/expenses?month=4&year=2026` | Filtra por mês/ano |
| POST   | `/api/expenses`        | Cria gasto                   |
| DELETE | `/api/expenses/:id`    | Remove gasto por ID          |

### Entradas

| Método | Rota                   | Descrição                    |
|--------|------------------------|------------------------------|
| GET    | `/api/incomes`         | Lista todas as entradas      |
| POST   | `/api/incomes`         | Cria entrada                 |
| DELETE | `/api/incomes/:id`     | Remove entrada por ID        |

### Metas

| Método | Rota                        | Descrição                         |
|--------|-----------------------------|-----------------------------------|
| GET    | `/api/goals`                | Lista metas                       |
| POST   | `/api/goals`                | Cria/atualiza meta por categoria  |
| DELETE | `/api/goals/:category`      | Remove meta por categoria         |

## Segurança

- Senhas armazenadas com hash SHA-256 + salt (troque por bcrypt em produção)
- Tokens JWT com expiração de 30 dias
- Cada usuário só acessa seus próprios dados (user_id em todas as queries)
- Foreign keys e WAL mode habilitados no SQLite

## Conta Demo

Ao clicar em "Entrar com conta demo", o app cria automaticamente o usuário
`demo@gastos.app` com dados de exemplo para explorar todas as funcionalidades.