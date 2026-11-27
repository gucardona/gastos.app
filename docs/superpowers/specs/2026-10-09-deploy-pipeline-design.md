# Deploy pipeline — local → GitHub → VDI

**Date:** 2026-10-09
**Status:** Draft — depends on open questions 1 and 2 of the roadmap
**Roadmap:** [2026-10-09-roadmap.md](2026-10-09-roadmap.md)

> **Later decision (2026-10-11):** the deploy runs **only on a push to `master`**, for production only. There is no staging environment; the staging mentions below are kept as history. What applies is in `.github/workflows/ci.yml` and `docs/ops/vdi-setup.md`.

## Goal

Develop everything locally (Windows), push to GitHub and have staging and production updated on the Hetzner VDI (`46.225.89.218`) without compiling anything there.

## Flow

```
local ──push──▶ GitHub ──Actions──▶ build (web + go) ──scp+ssh──▶ VDI
                  │                                               ├─ gastos-staging (:8001)  ← any push to a branch ≠ main
                  │                                               └─ gastos          (:8000)  ← push to main (or tag)
                  └─ PR: tests only, no deploy
```

## Local development

- `go run ./cmd/gastos` with `GASTOS_DEV=true GASTOS_COOKIE_SECURE=false`.
- `npm run dev` in `web/`: Vite on `:5173` with a proxy from `/api` to `:8000`. Cookies work because, from the browser's point of view, everything comes from `localhost:5173`.
- A `Makefile` (or `justfile`) and a `scripts/dev.ps1` start both together.
- Prerequisites on Windows: Git, Go and Node LTS (already installed or installable via `winget`).

## Build (GitHub Actions)

One workflow `.github/workflows/ci.yml` with three jobs.

1. **test**, on every push and PR:
   - `go vet ./...`, `go test ./...`;
   - in `web/`: `npm ci`, `npm run lint`, `npm run typecheck`, `npm test`.
2. **build**, on push only:
   - `npm ci && npm run build` in `web/`, producing `web/dist`;
   - `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$GITHUB_SHA" -o gastos ./cmd/gastos`. The `modernc.org/sqlite` driver is pure Go, so cross-compiling works without CGO.
   - The binary is available as an artifact.
3. **deploy**, on push only and with `needs: build`:
   - the environment comes from the branch: `main` goes to `production`, the rest to `staging`;
   - uses GitHub Environments, with optional manual approval protection on `production`.

## Layout on the VDI

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
/etc/gastos/production.env    # GASTOS_ADDR, GASTOS_DB_PATH, ... (outside git)
/etc/gastos/staging.env
```

- System user `gastos`, no shell, owner of `/var/lib/gastos`.
- Deploy user `deploy`, with the CI SSH key. It can write to `/opt/gastos` and has restricted sudo, only for:
  - `systemctl restart gastos`
  - `systemctl restart gastos-staging`

## systemd

`/etc/systemd/system/gastos.service` (the staging one is the same, swapping the env and the path):

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

## Deploy script (run by CI over SSH)

`scripts/remote-deploy.sh <env> <sha>`, versioned in the repo and copied along with the binary:

1. Copies the binary to `/opt/gastos/<env>/releases/<sha>/`.
2. Saves the current target of the `current` symlink as `previous`.
3. Points `current` at the new release atomically (`ln -sfn` + `mv -T`).
4. `sudo systemctl restart gastos[-staging]`.
5. Runs the health check with `curl -fsS localhost:<port>/healthz`, up to 10 attempts 1 second apart.
6. On failure, points `current` back at `previous`, restarts and exits with a non-zero code. The job turns red.
7. Keeps the last 5 releases and deletes the others.

The schema runs at binary boot (via `user_version`). Before starting a new release, the binary makes an automatic backup; this is already in the backend spec.

## Reverse proxy and TLS

**Depends on open question 1.** The proposal is Caddy, because it issues and renews TLS on its own:

```
gastos.<dominio>          { reverse_proxy localhost:8000 }
staging.gastos.<dominio>  { reverse_proxy localhost:8001 }
```

- SSE requires `flush_interval -1` in Caddy. In nginx, the equivalent is `proxy_buffering off`.
- Without a domain, staging can be reachable only through an SSH tunnel (`ssh -L 8001:localhost:8001`).
- Open Finance (phase 4b) needs public HTTPS for the webhook.

## Staging with data

Staging starts empty. A "copy production to staging" button is left for later. For tests, staging uses the demo seed (`GASTOS_DEV`).

## GitHub secrets

| Secret | Use |
|---|---|
| `DEPLOY_SSH_KEY` | private key of the `deploy` user |
| `DEPLOY_HOST` | `46.225.89.218` |
| `DEPLOY_KNOWN_HOSTS` | output of `ssh-keyscan` (avoids `StrictHostKeyChecking=no`) |

## One-time VDI setup (documented in `docs/ops/vdi-setup.md`)

1. Create the `gastos` and `deploy` users and configure the restricted sudoers.
2. Create the directories, the `.env` files and the two units, and run `systemctl enable`.
3. Install and configure Caddy, if that is the chosen option.
4. Stop and remove the old service (`gastos` with `deploy.sh`) and the old clone in `/home/gustavo/gupa.dev/gastos.app`. Only after confirming.

## Acceptance criteria

- A push to a `feature/x` branch updates staging in under 5 minutes, without touching production.
- A merge into `main` updates production; a binary that fails `/healthz` triggers an automatic rollback.
- No build tool (Go, Node) is needed on the VDI.
- The repo contains no binaries or `.env` files.
