#!/usr/bin/env bash
# Deploy / update Staydy: pull all 4 repos, rebuild changed images, restart, prune.
# Run on the server:  ./deploy/deploy.sh   (from the staydy-backend dir)
set -euo pipefail

# Serialize deploys. Each repo has its own CI workflow, so two pushes close together
# fire two deploys at once — without this lock, two concurrent `docker compose up --build`
# race on the same project and one can no-op (stale image ships). Wait up to 10 min for
# any in-flight deploy to finish, then run.
exec 9>/tmp/staydy-deploy.lock
flock -w 600 9 || { echo "!! another deploy is running; giving up"; exit 1; }

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
docker compose -f docker-compose.prod.yml up -d --build
docker image prune -f
docker compose -f docker-compose.prod.yml ps
echo "==> done. Logs: docker compose -f docker-compose.prod.yml logs -f api"
