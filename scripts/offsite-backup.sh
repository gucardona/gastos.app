#!/usr/bin/env bash
# Uploads the most recent backup, encrypted with GPG, to a PRIVATE git repository (no cost).
# Keeps only the newest KEEP files and publishes a single commit (force push), so the repository never grows.
# The remote repository is the source of truth: losing the local disk does not lose the set of backups.
#
# Variables: BACKUP_REPO (required, git URL), GPG_RECIPIENT (required, the PUBLIC key that encrypts),
#            BACKUP_DIR, WORKDIR, KEEP.
set -euo pipefail

: "${BACKUP_REPO:?set BACKUP_REPO to the git URL of the private repository}"
: "${GPG_RECIPIENT:?set GPG_RECIPIENT to the id of the public key that encrypts the backups}"
backup_dir=${BACKUP_DIR:-/var/lib/gastos/production/backups}
work=${WORKDIR:-/var/lib/gastos/offsite}
keep=${KEEP:-14}

shopt -s nullglob
files=("$backup_dir"/gastos-*.db) # the name carries the timestamp, so alphabetical order is chronological
((${#files[@]})) || { echo "no backups in $backup_dir" >&2; exit 1; }
latest=${files[-1]}
[ "$(head -c 15 "$latest")" = "SQLite format 3" ] || { echo "$latest does not look like a SQLite database" >&2; exit 1; }
name=$(basename "$latest").gpg

mkdir -p "$work"
cd "$work"
[ -d .git ] || git init -q -b main
git config user.name gastos-backup
git config user.email gastos-backup@localhost
if git remote get-url origin >/dev/null 2>&1; then git remote set-url origin "$BACKUP_REPO"; else git remote add origin "$BACKUP_REPO"; fi

# Fetch the current set from the remote before changing anything. If the remote is unreachable, abort:
# carrying on with an empty copy and force-pushing would wipe the old backups.
set +e
git ls-remote --exit-code -q origin main >/dev/null 2>&1
rc=$?
set -e
case $rc in
0)
	git fetch -q origin main
	git checkout -q -f -B main FETCH_HEAD
	git clean -fdxq
	;;
2) # the repository is still empty
	find . -maxdepth 1 -name '*.gpg*' -delete
	;;
*)
	echo "could not reach the backup repository (exit code $rc); nothing was changed" >&2
	exit 1
	;;
esac

if [ -e "$name" ]; then
	echo "already uploaded: $name"
	exit 0
fi

gpg --batch --yes --trust-model always --recipient "$GPG_RECIPIENT" --output "$name.tmp" --encrypt "$latest"
mv "$name.tmp" "$name"

# prune: keep only the newest $keep files
ls -1 -- *.gpg | sort | head -n "-$keep" | xargs -r rm -f --

git checkout -q --orphan next
git add -A
git commit -q -m "backup $name"
git branch -M main
git push -q --force origin main
git reflog expire --expire=now --all
git gc -q --prune=now

echo "uploaded: $name ($(ls -1 -- *.gpg | wc -l) files in the repository)"
