# Off-server backup

The app already makes a daily backup on the VDI disk (`/var/lib/gastos/production/backups`, the 14 most recent). That does not protect against losing the disk. This guide sends an **encrypted** copy to a **private** GitHub repository, at no cost.

How it works: a systemd timer runs `scripts/offsite-backup.sh` every day at 04:30. The script encrypts the most recent backup with the GPG **public** key, keeps the 14 newest and publishes a single commit (force push), so the repository does not grow. The VDI never has the private key: only you can decrypt.

> **Keep the GPG private key and its passphrase in your password manager.** Without them the backups become unreadable junk, and nobody can recover them.

The `gastos.app` repository is public. **Never** put the backup in it: use a separate, private repository.

## 1. Install once

### On your PC (Git Bash)

```bash
# GPG key: asks for a passphrase; the public one goes to the VDI, the private one stays with you
gpg --quick-generate-key "gastos-backup" default default never
gpg --armor --export gastos-backup > gastos-backup.pub.asc
gpg --armor --export-secret-keys gastos-backup > gastos-backup.secret.asc   # keep it in the password manager and delete the file
gpg --fingerprint gastos-backup                                               # note the fingerprint

# private repository (must be --private)
gh repo create gastos-backups --private --description "Backups cifrados do gastos"
```

### On the VDI (as root)

```bash
# access key for this repository only
install -d -m 700 -o gastos -g gastos /var/lib/gastos/.ssh
sudo -u gastos ssh-keygen -t ed25519 -N '' -f /var/lib/gastos/.ssh/backup_key -C gastos-offsite
ssh-keyscan -t ed25519 github.com > /var/lib/gastos/.ssh/known_hosts
chown gastos:gastos /var/lib/gastos/.ssh/known_hosts
cat /var/lib/gastos/.ssh/backup_key.pub     # register it on GitHub, below

# the GPG public key (copy gastos-backup.pub.asc from your PC)
sudo -u gastos gpg --import gastos-backup.pub.asc

# script, service and timer
install -m 755 scripts/offsite-backup.sh /usr/local/bin/gastos-offsite-backup
install -m 644 deploy/gastos-offsite-backup.service deploy/gastos-offsite-backup.timer /etc/systemd/system/
```

On your PC, register the access key **with write permission, on the backup repository only**:

```bash
gh repo deploy-key add backup_key.pub -R gucardona/gastos-backups --title vdi --allow-write
```

Back on the VDI, create `/etc/gastos/offsite.env` (permission 640, owner `root:gastos`):

```
BACKUP_REPO=git@github.com:gucardona/gastos-backups.git
GPG_RECIPIENT=<fingerprint noted above>
GIT_SSH_COMMAND="ssh -i /var/lib/gastos/.ssh/backup_key -o IdentitiesOnly=yes -o UserKnownHostsFile=/var/lib/gastos/.ssh/known_hosts -o StrictHostKeyChecking=yes"
```

Enable it:

```bash
systemctl daemon-reload
systemctl enable --now gastos-offsite-backup.timer
systemctl start gastos-offsite-backup.service   # first upload now
journalctl -u gastos-offsite-backup -n 20 --no-pager
```

The last line of the log should be `enviado: gastos-AAAAMMDD-HHMMSS.db.gpg (1 arquivos no repositório)`.

The access key (`/var/lib/gastos/.ssh`) and the working copy (`/var/lib/gastos/offsite`) belong to this service only. The app service (`gastos@.service`) runs as the same `gastos` user, but cannot see these two folders (`InaccessiblePaths`): if the app is compromised, the key with write access to the backup repository does not leak. This changes nothing in the backup service. If your server was installed before this rule, apply the step [What the server owner needs to apply](vdi-setup.md#what-the-server-owner-needs-to-apply).

## 2. Check now and then

```bash
systemctl list-timers gastos-offsite-backup.timer    # next run
journalctl -u gastos-offsite-backup --since "2 days ago" --no-pager
```

On GitHub, the `gastos-backups` repository should have **one** recent commit and up to 14 `.gpg` files. If the service fails, `systemctl status gastos-offsite-backup` shows why (network, access key removed, GPG without the public key).

## 3. Restore

Do this on your PC (which has the private key) or on any machine where you import `gastos-backup.secret.asc`.

```bash
git clone git@github.com:gucardona/gastos-backups.git
cd gastos-backups
ls                                                        # pick the file by name (date and time)
gpg --decrypt gastos-20261011-043000.db.gpg > gastos.db   # asks for the key passphrase
```

On the VDI, with the app stopped:

```bash
systemctl stop gastos@production
cd /var/lib/gastos/production
mv gastos.db gastos.db.antes              # do not delete before checking
rm -f gastos.db-wal gastos.db-shm
install -m 640 -o gastos -g gastos /path/gastos.db gastos.db
systemctl start gastos@production
curl -fsS http://127.0.0.1:8000/healthz
```

Open the app and check the latest transactions. Only then remove `gastos.db.antes`.

**Rehearse the restore once**, on another machine or in a test folder, before you actually need it.

## 4. How much data can be lost

The app makes a backup when the last one is more than 24 hours old, and the upload runs once a day. In the worst case you lose about a day and a half of transactions. If that is too much, the way forward is a configurable backup interval in the app.

## 5. Rotating the key

Generate a new key, import the public one on the VDI, change `GPG_RECIPIENT` and restart the timer. Old backups stay encrypted with the old key: **keep the old key** as long as they are the only copy.
