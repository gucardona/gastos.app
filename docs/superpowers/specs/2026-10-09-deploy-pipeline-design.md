# Pipeline de deploy — local → GitHub → VDI

**Data:** 2026-10-09
**Status:** Draft — depende das perguntas em aberto 1 e 2 do roadmap
**Roadmap:** [2026-10-09-roadmap.md](2026-10-09-roadmap.md)

## Objetivo

Desenvolver tudo localmente (Windows), fazer push no GitHub e ter staging e produção atualizados na VDI Hetzner (`46.225.89.218`) sem compilar nada lá.

## Fluxo

```
local ──push──▶ GitHub ──Actions──▶ build (web + go) ──scp+ssh──▶ VDI
                  │                                               ├─ gastos-staging (:8001)  ← qualquer push em branch ≠ main
                  │                                               └─ gastos          (:8000)  ← push em main (ou tag)
                  └─ PR: só testes, sem deploy
```

## Desenvolvimento local

- `go run ./cmd/gastos` com `GASTOS_DEV=true GASTOS_COOKIE_SECURE=false`.
- `npm run dev` em `web/`: Vite em `:5173` com proxy de `/api` para `:8000`. Cookies funcionam porque, do ponto de vista do browser, tudo vem de `localhost:5173`.
- Um `Makefile` (ou `justfile`) e um `scripts/dev.ps1` sobem os dois juntos.
- Pré-requisitos no Windows: Git, Go e Node LTS (já instalados ou instaláveis via `winget`).

## Build (GitHub Actions)

Um workflow `.github/workflows/ci.yml` com três jobs.

1. **test**, em todo push e PR:
   - `go vet ./...`, `go test ./...`;
   - em `web/`: `npm ci`, `npm run lint`, `npm run typecheck`, `npm test`.
2. **build**, só em push:
   - `npm ci && npm run build` em `web/`, gerando `web/dist`;
   - `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$GITHUB_SHA" -o gastos ./cmd/gastos`. O driver `modernc.org/sqlite` é Go puro, então o cross-compile funciona sem CGO.
   - O binário fica disponível como artifact.
3. **deploy**, só em push e com `needs: build`:
   - o ambiente vem da branch: `main` vai para `production`, o resto para `staging`;
   - usa GitHub Environments, com proteção opcional por aprovação manual em `production`.

## Layout na VDI

```
/opt/gastos/
  production/
    releases/<sha>/gastos
    current -> releases/<sha>
  staging/
    releases/<sha>/gastos
    current -> releases/<sha>
/var/lib/gastos/production/   # gastos.db, backups/
/var/lib/gastos/staging/
/etc/gastos/production.env    # GASTOS_ADDR, GASTOS_DB_PATH, ... (fora do git)
/etc/gastos/staging.env
```

- Usuário de sistema `gastos`, sem shell, dono de `/var/lib/gastos`.
- Usuário de deploy `deploy`, com a chave SSH do CI. Ele pode escrever em `/opt/gastos` e tem sudo restrito, só para:
  - `systemctl restart gastos`
  - `systemctl restart gastos-staging`

## systemd

`/etc/systemd/system/gastos.service` (o de staging é igual, trocando o env e o path):

```ini
[Unit]
Description=gastos.app (production)
After=network-online.target

[Service]
User=gastos
EnvironmentFile=/etc/gastos/production.env
ExecStart=/opt/gastos/production/current/gastos
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/gastos/production
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

## Script de deploy (executado pelo CI via SSH)

`scripts/remote-deploy.sh <env> <sha>`, versionado no repo e copiado junto com o binário:

1. Copia o binário para `/opt/gastos/<env>/releases/<sha>/`.
2. Guarda o alvo atual do symlink `current` como `previous`.
3. Aponta `current` para a nova release de forma atômica (`ln -sfn` + `mv -T`).
4. `sudo systemctl restart gastos[-staging]`.
5. Faz o health check com `curl -fsS localhost:<port>/healthz`, com até 10 tentativas de 1 segundo.
6. Se falhar, volta `current` para `previous`, reinicia e sai com código diferente de zero. O job fica vermelho.
7. Mantém as últimas 5 releases e apaga as outras.

O schema roda no boot do binário (via `user_version`). Antes de iniciar uma release nova, o binário faz backup automático; isso já está na spec de backend.

## Proxy reverso e TLS

**Depende da pergunta em aberto 1.** A proposta é o Caddy, porque emite e renova o TLS sozinho:

```
gastos.<dominio>          { reverse_proxy localhost:8000 }
staging.gastos.<dominio>  { reverse_proxy localhost:8001 }
```

- O SSE exige `flush_interval -1` no Caddy. No nginx, o equivalente é `proxy_buffering off`.
- Sem domínio, o staging pode ficar acessível só por túnel SSH (`ssh -L 8001:localhost:8001`).
- O Open Finance (fase 4b) precisa de HTTPS público para o webhook.

## Staging com dados

O staging começa vazio. Um botão "copiar produção para staging" fica para depois. Para testes, o staging usa o seed de demo (`GASTOS_DEV`).

## Segredos no GitHub

| Segredo | Uso |
|---|---|
| `DEPLOY_SSH_KEY` | chave privada do usuário `deploy` |
| `DEPLOY_HOST` | `46.225.89.218` |
| `DEPLOY_KNOWN_HOSTS` | saída de `ssh-keyscan` (evita `StrictHostKeyChecking=no`) |

## Setup único na VDI (documentado em `docs/ops/vdi-setup.md`)

1. Criar os usuários `gastos` e `deploy` e configurar o sudoers restrito.
2. Criar os diretórios, os arquivos `.env` e os dois units, e rodar `systemctl enable`.
3. Instalar e configurar o Caddy, se for a opção escolhida.
4. Parar e remover o serviço antigo (`gastos` com o `deploy.sh`) e o clone antigo em `/home/gustavo/gupa.dev/gastos.app`. Só depois de confirmar.

## Critérios de aceite

- Push numa branch `feature/x` atualiza o staging em menos de 5 minutos, sem tocar na produção.
- Merge em `main` atualiza a produção; um binário que não passa no `/healthz` faz rollback automático.
- Nenhuma ferramenta de build (Go, Node) é necessária na VDI.
- O repo não contém binários nem `.env`.
