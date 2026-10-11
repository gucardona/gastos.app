#!/usr/bin/env bash
# End-to-end test for scripts/offsite-backup.sh, using a local "remote" repository and throwaway GPG keys.
# Needs bash, git and gpg (all present on the GitHub runner and on the VPS).
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
script=$here/offsite-backup.sh
t=$(mktemp -d)
trap 'rm -rf "$t"' EXIT
export HOME=$t/home
mkdir -p "$HOME"

fail() { echo "FAILED: $*" >&2; exit 1; }
check() { # check "description" command...
	local desc=$1
	shift
	"$@" >/dev/null 2>&1 || fail "$desc"
	echo "ok: $desc"
}
expect_failure() { # expect_failure "description" command...
	local desc=$1
	shift
	if "$@" >/dev/null 2>&1; then fail "$desc (should have failed)"; fi
	echo "ok: $desc"
}

# keys: the private one stays on the "client" (the machine that restores); the "server" only imports the public one
client=$t/gpg-client server=$t/gpg-server
mkdir -m 700 "$client" "$server"
GNUPGHOME=$client gpg --batch --passphrase '' --quick-generate-key backup-test@example.com default default never >/dev/null 2>&1
GNUPGHOME=$client gpg --export backup-test@example.com >"$t/pub.gpg"
GNUPGHOME=$server gpg --import "$t/pub.gpg" >/dev/null 2>&1

remote=$t/remote.git
git init -q --bare -b main "$remote"
backups=$t/backups work=$t/work
mkdir -p "$backups"

run() { env GNUPGHOME="$server" BACKUP_REPO="$remote" GPG_RECIPIENT=backup-test@example.com BACKUP_DIR="$backups" WORKDIR="$work" KEEP=3 "$@" bash "$script"; }
make_backup() { { printf 'SQLite format 3\000'; head -c 3000 /dev/urandom; } >"$backups/gastos-$1.db"; }
remote_files() { git --git-dir="$remote" ls-tree --name-only main; }
remote_commits() { git --git-dir="$remote" rev-list --count main; }

expect_failure "fails when there are no backups" run
echo "this is not a database" >"$backups/gastos-20260101-000000.db"
expect_failure "rejects a file that is not SQLite" run
rm "$backups"/*

# 1) first upload
make_backup 20260101-030000
run >/dev/null
check "the first upload creates 1 commit with 1 file" test "$(remote_commits)/$(remote_files | wc -l)" = "1/1"

# what is on the remote is encrypted, and decrypts back to the original
git --git-dir="$remote" show main:gastos-20260101-030000.db.gpg >"$t/blob.gpg"
expect_failure "the remote does not contain the database in plain text" grep -q "SQLite format 3" "$t/blob.gpg"
GNUPGHOME=$client gpg --batch --decrypt "$t/blob.gpg" >"$t/restored.db" 2>/dev/null
check "decrypting with the private key returns the identical backup" cmp "$t/restored.db" "$backups/gastos-20260101-030000.db"
expect_failure "the server key (public only) cannot decrypt" env GNUPGHOME="$server" gpg --batch --decrypt "$t/blob.gpg"

# 2) running again without a new backup changes nothing
before=$(git --git-dir="$remote" rev-parse main)
out=$(run)
check "with no new backup it says 'already uploaded'" grep -q "already uploaded" <<<"$out"
check "and leaves the remote alone" test "$(git --git-dir="$remote" rev-parse main)" = "$before"

# 3) rotation: with KEEP=3 only the 3 newest remain, always in a single commit
for d in 02 03 04 05; do make_backup "202601$d-030000"; run >/dev/null; done
check "keeps only the 3 newest" test "$(remote_files | tr '\n' ' ')" = "gastos-20260103-030000.db.gpg gastos-20260104-030000.db.gpg gastos-20260105-030000.db.gpg "
check "still a single commit" test "$(remote_commits)" = "1"

# 4) the remote is the source of truth: deleting the local copy does not lose old backups
rm -rf "$work"
make_backup 20260106-030000
run >/dev/null
check "recreating the local copy preserves the previous ones" test "$(remote_files | tr '\n' ' ')" = "gastos-20260104-030000.db.gpg gastos-20260105-030000.db.gpg gastos-20260106-030000.db.gpg "

# 5) failures must never destroy what is already on the remote
before=$(git --git-dir="$remote" rev-parse main)
make_backup 20260107-030000
expect_failure "an unreachable remote aborts" run BACKUP_REPO="$t/does-not-exist.git"
expect_failure "an unknown recipient aborts before publishing" run GPG_RECIPIENT=nobody@example.com
check "the remote is untouched after the failures" test "$(git --git-dir="$remote" rev-parse main)" = "$before"
check "no leftover temporary file" test -z "$(find "$work" -maxdepth 1 -name '*.tmp')"

# and once the problem is gone, the pending upload happens
run >/dev/null
check "the pending upload happens once the problem is gone" test "$(remote_files | tail -1)" = "gastos-20260107-030000.db.gpg"

echo "all good"
