# Trava de cadastro — plano

**Data:** 2026-10-11
**Status:** a implementar

## Problema

`gastos.gupa.dev` é público e `POST /api/auth/register` aceita qualquer pessoa. Cada conta nova cria uma carteira e grava no banco. Para um app pessoal isso é só superfície de ataque (abuso, lixo, custo de bcrypt na CPU da VDI).

## Decisão

Cadastro **fechado por padrão**, com duas exceções controladas:

| Situação | Cadastro |
|---|---|
| Banco sem nenhum usuário | aberto, para criar a primeira conta |
| `GASTOS_INVITE_CODE` definido | exige o código no cadastro (para convidar a Bia, por exemplo) |
| Banco com usuários e sem código | **fechado** (`403`) |

Por que um código e não convite por e-mail: o app não tem como enviar e-mail, e o código é um segredo que você passa à pessoa por onde preferir. Para encerrar convites, basta remover a variável e reiniciar.

## Backend

- `config.Config` ganha `InviteCode string` (env `GASTOS_INVITE_CODE`, vazio por padrão). Mínimo de 8 caracteres quando definido; `Load` falha cedo se for mais curto.
- `GET /api/auth/registration` (público) devolve `{"mode":"open"|"code"|"closed"}`: `open` se não há usuários, `code` se há código configurado, `closed` caso contrário. O frontend usa para montar a tela.
- `POST /api/auth/register`:
  - `closed` → `403` com `code: "forbidden"` e mensagem "O cadastro está fechado. Peça um convite.".
  - `code` → campo `inviteCode` obrigatório; comparação em tempo constante (`subtle.ConstantTimeCompare`); código errado → `403` com a mesma mensagem genérica (não revela se o cadastro existe).
  - `open` → como hoje. A primeira conta continua sendo criada em transação, e o modo é recalculado dentro da transação para duas requisições simultâneas não criarem duas "primeiras" contas.
  - Passa pelo mesmo rate limit do login (10 por 15 minutos, por IP).
- `register` ignora `inviteCode` quando o modo é `open`.

## Frontend

- `Login` consulta `/api/auth/registration` ao montar.
  - `closed`: esconde "Criar uma conta".
  - `code`: mostra o campo "Código de convite" no cadastro.
  - `open`: como hoje (só acontece na primeira conta).
- Erro `403` do cadastro aparece como mensagem geral.

## Tarefas

- [ ] **1. Config** (teste primeiro): `GASTOS_INVITE_CODE` vazio e válido passam; menos de 8 caracteres falha.
- [ ] **2. Modo de cadastro** (teste primeiro): função pura `registrationMode(userCount, code)`; testes de tabela para as três situações.
- [ ] **3. Endpoint `/api/auth/registration`** e a trava em `register`, com testes de integração:
  - primeira conta passa sem código;
  - segunda conta sem código → `403`;
  - com código certo → `201`; com código errado ou ausente → `403`;
  - duas contas simultâneas sem usuários: só uma é criada (`t.Parallel` + goroutines);
  - o rate limit também vale para o cadastro.
- [ ] **4. Frontend**: `Login` por modo (testes de componente para os três modos); e2e: cadastro com código e fechado sem.
- [ ] **5. Docs**: `docs/ops/vdi-setup.md` (variável no `production.env`, como convidar e como fechar) e README.

## Fora de escopo

Convite por e-mail, links de convite de uso único, aprovação manual de contas.

## Risco

Quem já tem conta não é afetado. Quem esquecer de definir o código depois de criar a primeira conta fica com o cadastro fechado, que é o comportamento seguro.
