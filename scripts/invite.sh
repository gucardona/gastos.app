#!/usr/bin/env bash
# Prints a new single-use invite code for production. Install once as root (see docs/ops/vdi-setup.md):
#   sudo install -m 755 -o root -g root /opt/gastos/production/current/invite.sh /usr/local/sbin/gastos-invite
# then run: sudo gastos-invite [duration]. Never run the deploy-owned copy with sudo.
# The duration is optional (default 24h): 30m, 2h, 3d... from 5m to 7d.
# Root is needed only to read the root-only env file; the binary runs as the service's user so the
# database files it touches (-wal, -shm) stay owned by gastos.
set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
	echo "rode com sudo: sudo $0 [duração]" >&2
	exit 1
fi

set -a
# shellcheck source=/dev/null
. /etc/gastos/production.env
set +a
exec runuser -u gastos -- /opt/gastos/production/current/gastos invite "$@"
