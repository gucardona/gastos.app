# Off-server backup — plan

**Date:** 2026-10-11
**Status:** script and test ready; the VDI installation is missing (done by you)

## Problem

The app's rotating backup lives in `/var/lib/gastos/production/backups`, on the same disk as the database. If the VDI disk dies, both go together.

## Constraints

No additional cost. The `gastos.app` repository is **public**, so the backup goes to a **separate, private** and **encrypted** repository: the database holds financial transactions and password hashes.

## Decision

A script with a `systemd timer` uploads, once a day, the most recent backup to `gastos-backups` (private), encrypted with GPG.

- The VDI keeps only the **public** key. Whoever breaks into the VDI cannot decrypt old backups. The private key stays with you, protected by a passphrase.
- Each upload force-pushes **a single commit** with the 14 newest backups. The repository does not grow forever, and GitHub Free does not charge for private repositories.
- The remote is the source of truth: the script fetches the current set before touching anything, so recreating the local copy loses nothing.
- A network or encryption failure aborts without touching the remote (a network error must not turn into "start over and wipe everything").
- The deploy key of the backup repository is **its own**, with write permission only on it.

Discarded alternatives: Litestream and Storage Box (cost or extra service), unencrypted repository (financial data readable by whoever has access to the repository).

## Files

| File | Role |
|---|---|
| `scripts/offsite-backup.sh` | encrypts and publishes the most recent backup, prunes, ensures 1 commit |
| `scripts/offsite-backup.test.sh` | end-to-end test with a local remote and disposable GPG keys |
| `deploy/gastos-offsite-backup.service` / `.timer` | daily run (04:30, with random delay and `Persistent`) |
| `docs/ops/backup.md` | installation, verification and **restore** |

## Tasks

- [x] **1. Script** with SQLite header check, GPG encryption, pruning and a single commit.
- [x] **2. Test** (16 checks, run on Ubuntu): first upload, content encrypted and identical after decrypting, public key cannot decrypt, idempotence, rotation, remote as source of truth, unreachable remote and invalid recipient without touching the remote.
- [ ] **3. systemd** (unit and timer) and `docs/ops/backup.md`.
- [ ] **4. CI**: the script test runs in the `backend` job.
- [ ] **5. Installation on the VDI (manual, after the merge):** create the private repository, generate the GPG key on your PC, deploy key, install the script and enable the timer.
- [ ] **6. Restore rehearsal** on a clean machine before trusting the backup.

## Risk and loss window

The app makes a backup when the last one is more than 24 hours old; the upload runs daily. The worst case is losing about a day and a half of transactions. If that is too much, the next step is a configurable backup interval in the app (`GASTOS_BACKUP_EVERY`).

**Losing the GPG private key makes all backups unreadable.** Keep it, and its passphrase, in the password manager.
