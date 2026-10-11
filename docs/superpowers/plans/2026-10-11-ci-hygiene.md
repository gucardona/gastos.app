# Higiene do CI/CD — plano

**Data:** 2026-10-11
**Status:** a implementar

## Objetivo

Pegar cedo o que os testes não pegam (formatação, bugs estáticos, dependências vulneráveis) e tornar o deploy mais seguro. Sem serviços pagos nem contas novas: tudo roda no GitHub Actions.

## O que entra

| Item | Como | Por quê |
|---|---|---|
| Formatação Go | `test -z "$(gofmt -l .)"` no job `backend` | já houve arquivo fora do padrão que só o `gofmt` pegou |
| Lint Go | `golangci-lint` com o conjunto padrão (`govet`, `errcheck`, `staticcheck`, `ineffassign`, `unused`) | erros ignorados e código morto |
| Vulnerabilidades Go | `govulncheck ./...` | avisa de CVE nas dependências **que o código realmente chama** |
| Lint do frontend | ESLint (`typescript-eslint`, `react-hooks`) em `npm run lint` | hoje há comentários `eslint-disable` sem ESLint configurado |
| Vulnerabilidades npm | `npm audit --omit=dev --audit-level=high` | dependências de produção |
| Atualizações | Dependabot semanal para `gomod`, `npm` (em `web/`) e `github-actions`, agrupando minor e patch | PRs pequenos em vez de dívida acumulada |
| Deploy sem corrida | `concurrency: { group: deploy, cancel-in-progress: false }` | dois merges seguidos não rodam o script ao mesmo tempo |
| Teste de fumaça | depois do deploy, `curl -fsS https://gastos.gupa.dev/healthz` com 10 tentativas | o `healthz` local do script não prova que o Caddy e o DNS funcionam |
| e2e no celular | segundo projeto do Playwright com viewport 390×844 | só testamos em 1280 px |
| Teste do backup | `bash scripts/offsite-backup.test.sh` no job `backend` | o script de backup precisa continuar funcionando |

## Tarefas

- [ ] **1. gofmt e golangci-lint** no job `backend`; corrigir o que aparecer (o repositório precisa estar limpo para o job passar).
- [ ] **2. govulncheck** como job separado (`vuln`), para uma falha de vulnerabilidade não esconder uma falha de teste.
- [ ] **3. ESLint** no `web/`: `eslint.config.js`, script `lint`, passo no job `web`; corrigir o que aparecer.
- [ ] **4. `npm audit`** no job `vuln`.
- [ ] **5. Dependabot** (`.github/dependabot.yml`).
- [ ] **6. Deploy:** `concurrency` e teste de fumaça pela URL pública. O teste de fumaça só roda quando `DEPLOY_HOST` existe; a URL vem da variável de repositório `PUBLIC_URL` (sem ela, o passo é pulado).
- [ ] **7. e2e mobile:** projeto `mobile` no `playwright.config.ts`; ajustar seletores que dependem do layout de desktop (o botão "Novo lançamento" vira o `+` flutuante).
- [ ] **8. Teste do script de backup** no job `backend`.

## Fora de escopo

CodeQL (o `govulncheck` e o `npm audit` já cobrem dependências; análise estática profunda pode vir depois), SBOM, assinatura de binários, imagem Docker.

## Risco

Lint novo costuma achar coisas no código existente. A regra é corrigir o achado em vez de relaxar a regra; só se for ruído de verdade eu desligo a regra específica, com um comentário dizendo por quê.
