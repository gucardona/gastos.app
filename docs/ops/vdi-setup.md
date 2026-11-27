# One-time VDI setup

Do this once, as root, on the VDI. After that, GitHub Actions handles the deploys (`.github/workflows/ci.yml`).

**The deploy runs only on a push to `master`.** Branches and pull requests only run the tests and the build; they never publish.

Layout: binaries in `/opt/gastos/production/{releases,current}`, data in `/var/lib/gastos/production`, configuration in `/etc/gastos/production.env`. The app listens on `127.0.0.1:8000`.

## 1. Users and directories

```bash
useradd --system --home /var/lib/gastos --shell /usr/sbin/nologin gastos
useradd --create-home --shell /bin/bash deploy

install -d -o deploy -g deploy /opt/gastos/production /opt/gastos/production/releases # the deploy creates the current link inside production
install -d -o gastos -g gastos -m 750 /var/lib/gastos/production
install -d /etc/gastos
```

## 2. Configuration

`/etc/gastos/production.env`:

```
GASTOS_ADDR=127.0.0.1:8000
GASTOS_DB_PATH=/var/lib/gastos/production/gastos.db
GASTOS_BACKUP_DIR=/var/lib/gastos/production/backups
```

**Registration.** The site is public, so registration is closed by default: only the **first account** is created without a code; after that nobody can sign up. To invite another person, generate a code on the server and pass it to them however you prefer.

**One time only**, install the script somewhere only root can change (the copy that comes with each deploy belongs to the deploy user; running it with `sudo` would give root to whoever had the CI deploy key):

```bash
sudo install -m 755 -o root -g root /opt/gastos/production/current/invite.sh /usr/local/sbin/gastos-invite
```

Then, for each invite:

```bash
sudo gastos-invite       # valid for 24h
sudo gastos-invite 2h    # or another duration: 30m, 2h, 3d... (from 5m to 7d)
```

The script only reads the service configuration and runs the program of the running version as the `gastos` user, so the installed copy stays valid across future deploys; reinstall it only if `scripts/invite.sh` itself changes. Never run the copy inside `/opt/gastos/production/` with `sudo`.

It prints a single line, for example `Código: K3J9-XQ2M-7PRT (vale até 15/11/2026 às 14:30, uso único)`. The code works for **one** sign-up and expires at the time shown; while there is a valid code, the sign-in screen shows the "Código de convite" (Invite code) field (uppercase, lowercase, spaces and dashes do not matter). There is no need to restart the service, and the code is not stored anywhere (the database keeps only a hash). Codes used or expired for more than 7 days are deleted automatically. The installed copy always uses the binary of the running version.

`GASTOS_INVITE_CODE` is no longer needed. It still works as an alternative (a fixed code of at least 8 characters, valid for any number of sign-ups while it is in the file, and requiring a service restart when added or removed); if you use it, **remove the line** and restart when everyone has an account.

**Assistant.** To enable the query chat, add the master key (generate it with `openssl rand -base64 32`; the value stays on the server only). Without it the assistant is unavailable and everything else works normally. Details and precautions in [Query assistant](../assistant.md).

```
GASTOS_SECRET_KEY=<output of openssl rand -base64 32>
```

`GASTOS_COOKIE_SECURE` stays at the default (`true`), so access must be over HTTPS (step 4).

The file holds secrets (invite code, master key). Make it readable by root only; systemd reads it as root before starting the app as the `gastos` user, so the app still receives the variables:

```bash
chown root:root /etc/gastos/production.env
chmod 600 /etc/gastos/production.env
```

## 3. systemd

```bash
install -m 644 deploy/gastos@.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable gastos@production   # only starts after the first deploy
```

Minimum permission for the deploy user to restart only this service, in `/etc/sudoers.d/gastos-deploy` (validate with `visudo -c`):

```
deploy ALL=(root) NOPASSWD: /usr/bin/systemctl restart gastos@production
```

The unit is already hardened: the app only writes to `/var/lib/gastos/production`, cannot see the key or the off-server backup copy (`/var/lib/gastos/.ssh` and `/var/lib/gastos/offsite`), has no root capabilities, only opens IPv4/IPv6 and local network sockets, and can only make the common system calls of services. None of this gets in the way of what it does: serving HTTP, reading and writing SQLite and calling external APIs over HTTPS.

### What the server owner needs to apply

The CI deploy only swaps the binary; **the unit and the permissions are not updated automatically**. On a server installed before this version, run once, as root, from an up-to-date clone of the repository:

```bash
# 1. secrets readable by root only (systemd reads the file as root)
chown root:root /etc/gastos/production.env
chmod 600 /etc/gastos/production.env

# 2. new unit (backup isolation and hardening)
install -m 644 deploy/gastos@.service /etc/systemd/system/
systemctl daemon-reload
systemctl restart gastos@production

# 3. check
systemctl status gastos@production --no-pager
curl -fsS http://127.0.0.1:8000/healthz; echo
systemd-analyze security gastos@production | tail -1   # the score should drop compared to the previous one
```

If the service does not start, `journalctl -u gastos@production -n 50 --no-pager` shows the reason; to roll back, reinstall the previous unit (`git log --oneline -- deploy/gastos@.service` shows the commit; `git show <commit>~1:deploy/gastos@.service` gives the file) and repeat `daemon-reload` and `restart`. The off-server backup service does not change.

## 4. HTTPS (Caddy)

The block that already exists on the VDI does not change: Caddy forwards `text/event-stream` without buffering, so SSE works.

```
gastos.gupa.dev {
	reverse_proxy 127.0.0.1:8000
}
```

## 5. CI SSH key

```bash
ssh-keygen -t ed25519 -N '' -f ci_deploy -C gastos-ci
install -d -m 700 -o deploy -g deploy /home/deploy/.ssh
cat ci_deploy.pub >> /home/deploy/.ssh/authorized_keys
ssh-keyscan -t ed25519 <vdi-ip>     # the output becomes the DEPLOY_KNOWN_HOSTS secret
```

On GitHub, under Settings → Secrets and variables → Actions, create:

| Secret | Value |
|---|---|
| `DEPLOY_HOST` | IP or name of the VDI |
| `DEPLOY_SSH_KEY` | contents of `ci_deploy` (private) |
| `DEPLOY_KNOWN_HOSTS` | output of `ssh-keyscan` |

Delete `ci_deploy` from the server afterwards.

Optional, but recommended: create the repository **variable** (not a secret) `PUBLIC_URL` with the public address, for example `https://gastos.gupa.dev`. After each deploy, CI calls `$PUBLIC_URL/healthz` through the same path as users (DNS, TLS and Caddy), and the job fails if it does not respond. Without `DEPLOY_HOST` the deploy job does nothing. To require manual approval before each deploy, configure required reviewers on the `production` environment (Settings → Environments).

## 6. Retiring the old app

The old service is named `gastos`, runs `gastos-app` on port 8000 and uses the old database, which is **not compatible** with the new schema. Before the first deploy:

```bash
systemctl disable --now gastos
cp /path/to/old/clone/gastos.db ~/gastos-antigo.db   # keep it for safety
```

Then remove `/etc/systemd/system/gastos.service` and the old clone.

## 7. Off-server backup

The app's backup lives on the same disk as the VDI. To have an encrypted copy somewhere else, at no cost, follow [backup.md](backup.md). Do this **before** using the app for real.

## First deploy

Merge into `master`. The `scripts/remote-deploy.sh` script activates the new release and rolls back on its own to the previous one if `/healthz` does not respond within 10 seconds.
