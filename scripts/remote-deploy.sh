#!/usr/bin/env bash
# Runs on the VPS (called by CI over SSH): activates the release in <ROOT>/<env>/releases/<sha> and rolls back if /healthz fails.
# Usage: remote-deploy.sh <env> <sha> <port>   (CI only deploys <env>=production)
# The binary must already be at <ROOT>/<env>/releases/<sha>/gastos.
set -euo pipefail

env=$1 sha=$2 port=$3
root=${ROOT:-/opt/gastos}
systemctl_cmd=${SYSTEMCTL:-sudo systemctl}
keep=5

dir=$root/$env
current=$dir/current
previous=$(readlink "$current" 2>/dev/null || true)

ln -sfn "$dir/releases/$sha" "$current.new"
mv -T "$current.new" "$current"
$systemctl_cmd restart "gastos@$env"

for _ in $(seq 10); do
	if curl -fsS "http://127.0.0.1:$port/healthz" >/dev/null 2>&1; then
		# keep only the newest $keep releases
		ls -1dt "$dir"/releases/*/ | tail -n +$((keep + 1)) | xargs -r rm -rf
		echo "deploy ok: $env $sha"
		exit 0
	fi
	sleep 1
done

echo "healthz failed on $env; rolling back to ${previous:-nothing}" >&2
if [ -n "$previous" ]; then
	ln -sfn "$previous" "$current.new"
	mv -T "$current.new" "$current"
	$systemctl_cmd restart "gastos@$env"
fi
exit 1
