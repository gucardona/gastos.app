#!/bin/bash
set -e

APP_NAME="gastos"
BINARY="gastos-app"
MAIN="src/main.go"

echo "→ Building..."
go build -x -o "$BINARY" "$MAIN"

echo "→ Restarting service..."
sudo systemctl restart "$APP_NAME"

echo "→ Done. Status:"
sudo systemctl status "$APP_NAME" --no-pager -l
