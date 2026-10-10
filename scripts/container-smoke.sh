#!/usr/bin/env bash
# Temporary containers only; no registry push and no production volume access.
set -euo pipefail
image=${1:-kriemhild:ci}
prefix="kriemhild-smoke-${RANDOM}-$$"
network="$prefix-net"
app="$prefix-app"
postgres="$prefix-postgres"
work=$(mktemp -d)
cleanup() {
  docker rm -f "$app" "$postgres" >/dev/null 2>&1 || true
  docker volume rm "$prefix-sqlite" "$prefix-cache" "$prefix-pg" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  rm -rf -- "$work"
}
trap cleanup EXIT
docker network create "$network" >/dev/null
field() { python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"; }
start_app() {
  local volume=$1 dsn=${2:-}
  local args=()
  if [[ -n "$dsn" ]]; then args+=(-e "DATABASE_URL=$dsn"); fi
  docker run -d --name "$app" --network "$network" \
    --read-only --tmpfs /tmp:rw,size=64m --cap-drop ALL \
    --security-opt no-new-privileges:true -p 127.0.0.1::8124 \
    -v "$volume:/data" "${args[@]}" "$image" >/dev/null
  local mapping
  mapping=$(docker port "$app" 8124/tcp)
  base="http://127.0.0.1:${mapping##*:}"
  for ((i=0;i<60;i++)); do
    if curl -fsS "$base/api/ready" >/dev/null 2>&1; then
      docker exec "$app" /app/kriemhild -healthcheck
      return
    fi
    sleep 1
  done
  docker logs "$app" >&2
  return 1
}
stop_app() {
  docker stop -t 40 "$app" >/dev/null
  [[ $(docker inspect -f '{{.State.ExitCode}}' "$app") == 0 ]]
  docker rm "$app" >/dev/null
}
exercise() {
  local driver=$1 volume=$2 dsn=${3:-}
  start_app "$volume" "$dsn"
  [[ $(docker inspect -f '{{.Config.User}}' "$app") == '10001:10001' ]]
  curl -fsS "$base/" > "$work/index.html"
  grep -q 'id="root"' "$work/index.html"
  local asset
  asset=$(python3 -c 'import re,sys; print(re.search(r"src=\"(/assets/[^\"]+)\"",open(sys.argv[1]).read()).group(1))' "$work/index.html")
  curl -fsS "$base$asset" >/dev/null
  [[ $(curl -fsS "$base/api/projects" | field database) == "$driver" ]]
  curl -fsS "$base/api/sessions" -H 'Content-Type: application/json' \
    --data '{"environment":{"columns":16,"rows":16,"seed":"container-smoke","realism":true}}' > "$work/state.json"
  local id status world
  id=$(field id < "$work/state.json")
  status=$(field status < "$work/state.json")
  for ((i=0;i<100;i++)); do
    if [[ "$status" == done ]]; then break; fi
    curl -fsS "$base/api/sessions/$id/step" -H 'Content-Type: application/json' --data '{"count":10000}' > "$work/step.json"
    status=$(python3 -c 'import json,sys;print(json.load(sys.stdin)["state"]["status"])' < "$work/step.json")
  done
  [[ "$status" == done ]]
  curl -fsS "$base/api/sessions/$id/detail/2/0/0" > "$work/tile.json"
  curl -fsS "$base/api/sessions/$id/view" -H 'Content-Type: application/json' --data '{"ui":{"seedUsed":42,"camera2d":{"scale":120}}}' > "$work/staged.json"
  curl -fsS "$base/api/sessions/$id/save" -H 'Content-Type: application/json' --data '{}' > "$work/saved.json"
  world=$(curl -fsS "$base/api/projects" | python3 -c 'import json,sys;print(json.load(sys.stdin)["projects"][0]["id"])')
  curl -fsS -X DELETE "$base/api/sessions/$id" >/dev/null
  stop_app
  start_app "$volume" "$dsn"
  curl -fsS "$base/api/projects/$world/open" -H 'Content-Type: application/json' --data '{}' > "$work/restored.json"
  id=$(field id < "$work/restored.json")
  curl -fsS "$base/api/sessions/$id/detail/2/0/0" > "$work/restored-tile.json"
  cmp "$work/tile.json" "$work/restored-tile.json"
  python3 - "$work/state.json" "$work/restored.json" <<'PY'
import json,sys
original,restored=(json.load(open(p)) for p in sys.argv[1:])
assert original['environment'] == restored['environment'], 'geography changed on restart'
assert restored['storedDetailCount'] >= 3, 'detail hierarchy missing'
assert restored['projectUI']['camera2d']['scale'] == 120, 'camera state missing'
PY
  curl -fsS "$base/api/sessions/$id/project" -H 'Content-Type: application/json' --data '{"ui":{"seedUsed":42}}' -o "$work/world.zip"
  stop_app
  echo "$driver: frontend, non-root runtime, explicit save, restart, stored detail, ZIP export and graceful shutdown passed"
}
exercise sqlite "$prefix-sqlite"
docker run -d --name "$postgres" --network "$network" \
  -e POSTGRES_USER=kriemhild -e POSTGRES_DB=kriemhild -e POSTGRES_PASSWORD=smoke-only-password \
  -v "$prefix-pg:/var/lib/postgresql/data" postgres:17-alpine >/dev/null
ready=false
for ((i=0;i<60;i++)); do
  if docker exec "$postgres" pg_isready -h 127.0.0.1 -p 5432 -U kriemhild -d kriemhild >/dev/null 2>&1; then ready=true; break; fi
  sleep 1
done
[[ "$ready" == true ]]
exercise postgresql "$prefix-cache" "postgres://kriemhild:smoke-only-password@$postgres:5432/kriemhild?sslmode=disable"

