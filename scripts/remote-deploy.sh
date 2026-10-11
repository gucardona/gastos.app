#!/usr/bin/env bash
# Roda na VDI (chamado pelo CI via SSH): ativa a release em <ROOT>/<env>/releases/<sha> e faz rollback se o /healthz falhar.
# Uso: remote-deploy.sh <production|staging> <sha> <porta>
# O binário já deve estar em <ROOT>/<env>/releases/<sha>/gastos.
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
		# mantém só as $keep releases mais recentes
		ls -1dt "$dir"/releases/*/ | tail -n +$((keep + 1)) | xargs -r rm -rf
		echo "deploy ok: $env $sha"
		exit 0
	fi
	sleep 1
done

echo "healthz falhou em $env; voltando para ${previous:-nada}" >&2
if [ -n "$previous" ]; then
	ln -sfn "$previous" "$current.new"
	mv -T "$current.new" "$current"
	$systemctl_cmd restart "gastos@$env"
fi
exit 1
