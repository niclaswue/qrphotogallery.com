#!/usr/bin/env bash
# Deploy a branch to staging.example.com by building directly on
# the VPS (no registry, no watchtower — see "Staging" in DEPLOY.md).
#
# Usage:
#   ./deploy_staging.sh              # deploy the currently checked-out branch
#   ./deploy_staging.sh my-branch    # deploy a specific branch
#
# The server builds from origin/<branch>, so push your commits first.

set -euo pipefail

cd "$(dirname "$0")"

HOST="${STAGING_HOST:-pcw@178.104.252.218}"
BRANCH="${1:-$(git rev-parse --abbrev-ref HEAD)}"

# The server pulls from origin — warn when local state won't be what ships.
if ! git diff --quiet || ! git diff --cached --quiet; then
    echo "note: uncommitted local changes will NOT be deployed"
fi
if [[ "$(git rev-parse HEAD)" != "$(git rev-parse "origin/$BRANCH" 2>/dev/null || echo missing)" ]]; then
    echo "warning: local HEAD differs from origin/$BRANCH — did you push?"
fi

echo "==> deploying origin/$BRANCH to staging on $HOST"

ssh "$HOST" "BRANCH=$(printf %q "$BRANCH") bash -s" << 'EOF'
set -euo pipefail

cd ~/qr-app-dev
git fetch origin "$BRANCH"
git checkout -q "$BRANCH" 2>/dev/null || git checkout -qb "$BRANCH" "origin/$BRANCH"
git reset --hard "origin/$BRANCH"

BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_COMMIT=$(git rev-parse --short=12 HEAD)
echo "==> building qr-app:staging ($BUILD_COMMIT)"
docker build -t qr-app:staging \
    --build-arg BUILD_TIME="$BUILD_TIME" \
    --build-arg BUILD_COMMIT="$BUILD_COMMIT" \
    .

cd ~/qr-app
docker compose up -d app-staging
echo "==> waiting for health"
for _ in $(seq 30); do
    status=$(docker inspect -f '{{.State.Health.Status}}' qr-app-staging)
    [[ "$status" == "healthy" ]] && break
    sleep 2
done
echo "==> staging is $status ($BUILD_COMMIT)"
EOF

echo "==> done: https://staging.example.com"
