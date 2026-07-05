#!/usr/bin/env bash
# Deploy / update Staydy: pull all 4 repos, rebuild changed images, restart, prune.
# Run on the server:  ./deploy/deploy.sh   (from the staydy-backend dir)
set -euo pipefail

# /opt/staydy (parent of all 4 repos)
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BACKEND="$ROOT/staydy-backend"

for repo in staydy-backend staydy-website staydy-app staydy-superadmin; do
  if [ -d "$ROOT/$repo/.git" ]; then
    echo "==> git pull $repo"
    git -C "$ROOT/$repo" pull --ff-only || echo "   (skipped — resolve $repo manually)"
  else
    echo "!! missing $ROOT/$repo — clone it as a sibling first"
  fi
done

cd "$BACKEND"
echo "==> building + restarting containers"
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build
docker image prune -f
docker compose --env-file .env.prod -f docker-compose.prod.yml ps
echo "==> done. Logs: docker compose --env-file .env.prod -f docker-compose.prod.yml logs -f api"
