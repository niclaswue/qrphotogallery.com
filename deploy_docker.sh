#!/usr/bin/env bash
# Build the qr-app Docker image and run it locally.
#
# Usage:
#   ./deploy_docker.sh                  # build image + run container
#   ./deploy_docker.sh --build-only     # build image, don't run
#   ./deploy_docker.sh --no-build       # run existing image
#   PORT=9000 ./deploy_docker.sh        # custom host port
#
# Persists pb_data in a named volume so DB survives container restarts.

set -euo pipefail

cd "$(dirname "$0")"

IMAGE="${IMAGE:-qr-app:local}"
CONTAINER="${CONTAINER:-qr-app}"
PORT="${PORT:-8090}"
VOLUME="${VOLUME:-qr-app-data}"
ENV_FILE="${ENV_FILE:-.env}"

# --- prerequisites -----------------------------------------------------------
command -v docker >/dev/null 2>&1 || { echo "docker is not installed"; exit 1; }
docker info >/dev/null 2>&1 || { echo "docker daemon is not running"; exit 1; }

MODE="all"
case "${1:-}" in
    --build-only) MODE="build" ;;
    --no-build)   MODE="run"   ;;
    "")           MODE="all"   ;;
    *) echo "unknown flag: $1"; exit 1 ;;
esac

# --- build -------------------------------------------------------------------
if [[ "$MODE" != "run" ]]; then
    echo "==> building image $IMAGE"
    docker build -t "$IMAGE" .
fi

if [[ "$MODE" == "build" ]]; then
    echo "==> built $IMAGE (skipping run)"
    exit 0
fi

# --- run ---------------------------------------------------------------------
echo "==> ensuring volume $VOLUME exists"
docker volume inspect "$VOLUME" >/dev/null 2>&1 || docker volume create "$VOLUME" >/dev/null

echo "==> stopping any existing container named $CONTAINER"
docker rm -f "$CONTAINER" >/dev/null 2>&1 || true

ENV_ARGS=()
if [[ -f "$ENV_FILE" ]]; then
    ENV_ARGS=(--env-file "$ENV_FILE")
    echo "==> using env file $ENV_FILE"
fi

echo "==> starting container $CONTAINER on http://localhost:$PORT"
docker run -d \
    --name "$CONTAINER" \
    -p "$PORT:8090" \
    -v "$VOLUME:/app/pb_data" \
    "${ENV_ARGS[@]}" \
    "$IMAGE"

echo "==> tailing logs (Ctrl-C to detach; container keeps running)"
docker logs -f "$CONTAINER"
