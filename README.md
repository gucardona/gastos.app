# gastos.app

Aplicativo de controle financeiro pessoal com backend em Go, persistencia em SQLite e frontend server-rendered em um unico `index.html`.

O projeto foi desenhado para ser simples de subir, facil de entender e rapido de iterar: um binario Go serve a interface web, expoe a API HTTP e grava os dados localmente em `gastos.db`.

## O que o app faz

- Cadastro e login com JWT
- Controle de gastos por categoria, forma de pagamento e data
- Registro de entradas
- Metas mensais por categoria
- Dashboard com saldo, media diaria, distribuicao por perfil de gasto e graficos
- Conta demo preenchida automaticamente pela interface para exploracao rapida

## Stack

- Go `1.26.2`
- SQLite via `modernc.org/sqlite`
- JWT via `github.com/golang-jwt/jwt/v5`
- Hash de senha com `bcrypt`
- Frontend em HTML + CSS + Alpine.js + Chart.js

## Como o projeto esta organizado

```text
gastos.app/
├── deploy.sh
├── go.mod
├── gastos.db
├── src/
│   ├── main.go
│   ├── db/
│   │   └── db.go
│   ├── handlers/
│   │   ├── auth.go
│   │   ├── expenses.go
│   │   ├── incomes.go
│   │   ├── goals.go
│   │   └── validation.go
│   ├── middleware/
│   │   └── auth.go
│   ├── models/
│   │   └── models.go
│   └── web/
│       ├── index.html
│       ├── icon.png
│       └── apple-touch-icon.png
└── vendor/
```

## Arquitetura em uma frase

O servidor inicia o SQLite, aplica criacao de tabelas e migracoes simples, protege as rotas da API com JWT e serve o frontend estatico em `src/web`.

## Fluxo principal

1. O usuario abre a interface web servida pelo proprio backend.
2. O frontend faz `register` ou `login` em `/api/auth/...`.
3. O token JWT retornado passa a ser enviado no header `Authorization: Bearer <token>`.
4. A API responde apenas com dados do usuario autenticado.
5. Gastos, entradas e metas sao persistidos no SQLite local.

## Banco de dados

O banco e criado automaticamente em `./gastos.db` quando o servidor sobe.

Tabelas principais:

- `users`
- `expenses`
- `incomes`
- `goals`

Detalhes relevantes:

- `foreign_keys` e habilitado no startup
- metas usam unicidade por `user_id + category`
- ha migracoes incrementais simples executadas no boot
- indices basicos existem para consultas por usuario e data

## Requisitos para rodar localmente

- Go instalado
- ambiente capaz de compilar dependencias Go normais

Nao ha dependencia de SQLite CLI nem de headers nativos extras no codigo atual, porque o projeto usa `modernc.org/sqlite`.

## Rodando o projeto

Na raiz de `gastos.app`:

```bash
go run ./src/main.go
```

Por padrao o servidor sobe em:

```text
http://localhost:8000
```

Se quiser trocar a porta:

```bash
PORT=3000 go run ./src/main.go
```

## Build

Gerar binario:

```bash
go build -o gastos-app ./src/main.go
```

Executar binario:

```bash
./gastos-app
```

## Deploy

O repositório inclui um script simples:

```bash
./deploy.sh
```

Hoje ele faz:

- build do binario `gastos-app`
- restart de um servico `systemd` chamado `gastos`
- exibicao do status do servico

Isso pressupoe que o host de deploy ja tenha uma unit `gastos.service` configurada.

## Variaveis de ambiente

| Variavel | Padrao | Uso |
| --- | --- | --- |
| `PORT` | `8000` | Porta HTTP do servidor |
| `JWT_SECRET` | `troque-por-segredo-forte` | Segredo usado para assinar e validar JWT |

Observacao importante: o caminho do banco esta fixo hoje em `./gastos.db` dentro de `src/main.go`.

## API

Todas as rotas abaixo usam JSON.

Rotas protegidas exigem:

```http
Authorization: Bearer <token>
```

### Auth

#### `POST /api/auth/register`

Body:

```json
{
  "name": "Gustavo",
  "email": "gustavo@exemplo.com",
  "password": "123456"
}
```

Resposta:

```json
{
  "token": "jwt...",
  "user": {
    "id": 1,
    "name": "Gustavo",
    "email": "gustavo@exemplo.com"
  }
}
```

Regras:

- `name` obrigatorio
- `email` obrigatorio
- `password` com minimo de 6 caracteres

#### `POST /api/auth/login`

Body:

```json
{
  "email": "gustavo@exemplo.com",
  "password": "123456"
}
```

Resposta: mesmo formato do `register`.

### Gastos

#### `GET /api/expenses`

Lista todos os gastos do usuario autenticado, ordenados por `date DESC, id DESC`.

#### `POST /api/expenses`

Body:

```json
{
  "amount": 89.9,
  "description": "Supermercado",
  "category": "groceries",
  "payment": "credit",
  "date": "2026-04-14"
}
```

Campos obrigatorios:

- `amount > 0`
- `description`
- `category`
- `payment`
- `date` no formato `YYYY-MM-DD`

#### `DELETE /api/expenses/:id`

Remove apenas um gasto pertencente ao usuario autenticado.

### Entradas

#### `GET /api/incomes`

Lista todas as entradas do usuario autenticado.

#### `POST /api/incomes`

Body:

```json
{
  "amount": 3500,
  "description": "Salario",
  "type": "salary",
  "date": "2026-04-05"
}
```

Campos obrigatorios:

- `amount > 0`
- `description`
- `type`
- `date` no formato `YYYY-MM-DD`

#### `DELETE /api/incomes/:id`

Remove apenas uma entrada pertencente ao usuario autenticado.

### Metas

#### `GET /api/goals`

Lista metas do usuario autenticado.

#### `POST /api/goals`

Body:

```json
{
  "category": "groceries",
  "limit": 1200
}
```

Comportamento:

- cria a meta se ela nao existir
- atualiza a meta se ja houver uma combinacao igual de `user_id + category`

#### `DELETE /api/goals/:category`

Remove a meta da categoria informada para o usuario autenticado.

## Respostas de erro

Erros retornam JSON no formato:

```json
{
  "error": "mensagem"
}
```

Status comuns:

- `400` para payload invalido ou campos faltando
- `401` para token ausente ou invalido
- `404` para recurso nao encontrado
- `405` para metodo nao permitido
- `409` para e-mail ja cadastrado
- `500` para falhas internas

## Seguranca

Pontos ja implementados:

- senhas sao armazenadas com `bcrypt`
- JWT assinado com `HS256`
- token com expiracao de 30 dias
- isolamento de dados por `user_id`
- rotas protegidas por middleware de autenticacao

Pontos que merecem endurecimento antes de producao real:

- o fallback de `JWT_SECRET` e inseguro e deve ser substituido por segredo forte
- CORS esta aberto com `Access-Control-Allow-Origin: *`
- ainda nao existe rate limiting
- ainda nao existe refresh token
- nao ha camada formal de configuracao para ambiente

## Frontend

O frontend fica concentrado em [`src/web/index.html`](./src/web/index.html).

Ele inclui:

- tela de login e registro
- dashboard
- navegacao mobile e desktop
- formularios de gastos, entradas e metas
- graficos com Chart.js
- painel de conta

Essa abordagem deixa o projeto facil de portar e debugar, mas o arquivo cresce rapido. Se o app continuar evoluindo, vale separar HTML, estilos e scripts em arquivos dedicados.

## Conta demo

Ao entrar pela interface, o frontend tenta autenticar a conta:

```text
demo@gastos.app / demo123
```

Se ela nao existir, o proprio frontend faz o cadastro e popula dados de exemplo para demonstracao.

## Limites atuais do projeto

- o banco esta fixo em `./gastos.db`
- nao ha testes automatizados no repositório
- o frontend esta centralizado em um unico arquivo
- nao ha Dockerfile nem pipeline de CI configurados aqui
- a API nao implementa update parcial para gastos e entradas

## Direcoes naturais de evolucao

- extrair configuracao para variaveis de ambiente estruturadas
- adicionar testes para handlers e validacoes
- separar frontend em arquivos menores
- criar endpoint de healthcheck
- adicionar filtros por periodo na API
- preparar backup e estrategia de migracao do banco

## Licenca

Este README nao declara uma licenca para o projeto. Se o repositório for publico, vale adicionar uma explicitamente.
