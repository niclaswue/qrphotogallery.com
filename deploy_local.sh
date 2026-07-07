#!/usr/bin/env bash
# Build and run qr-app on the local machine.
#
# Usage:
#   ./deploy_local.sh              # build + serve on 0.0.0.0:8090
#   PORT=9000 ./deploy_local.sh    # custom port
#   ./deploy_local.sh --build-only # just build, don't serve

set -euo pipefail

cd "$(dirname "$0")"

PORT="${PORT:-8090}"
BIND="${BIND:-0.0.0.0}"
BIN="./qr-app"

# --- prerequisites -----------------------------------------------------------
command -v go >/dev/null 2>&1 || { echo "go is not installed (need Go 1.25+)"; exit 1; }
command -v typst >/dev/null 2>&1 || {
    echo "typst is not on PATH — install with: brew install typst"
    exit 1
}

# --- build -------------------------------------------------------------------
echo "==> building $BIN"
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$BIN" ./cmd/app

if [[ "${1:-}" == "--build-only" ]]; then
    echo "==> built $BIN (skipping serve)"
    exit 0
fi

# --- serve -------------------------------------------------------------------
echo "==> serving on http://$BIND:$PORT"
exec "$BIN" serve --http="$BIND:$PORT"
