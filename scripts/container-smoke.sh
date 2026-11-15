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
field() { python3 -c 'import json,sys; v=json.load(sys.stdin)
for key in sys.argv[1].split("."): v=v[key]
print(v)' "$1"; }
start_app() {
  local volume=$1 dsn=${2:-}
  local args=()
  if [[ -n "$dsn" ]]; then args+=(-e "DATABASE_URL=$dsn"); fi
  # Linux CI uses host networking so local mode really binds to loopback.
  local port
  port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')
  docker run -d --name "$app" --network host \
    --read-only --tmpfs /tmp:rw,size=64m --cap-drop ALL \
    --security-opt no-new-privileges:true -e KRIEMHILD_AUTH_MODE=local -e "KRIEMHILD_ADDR=127.0.0.1:$port" \
    -v "$volume:/data" "${args[@]}" "$image" >/dev/null
  base="http://127.0.0.1:$port"
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
  curl -fsS "$base/api/worlds" -H 'Content-Type: application/json' \
    --data '{"name":"Container persistence check"}' > "$work/world.json"
  local id status world age job project revision
  world=$(field world.id < "$work/world.json")
  age=$(field world.currentAge < "$work/world.json")
  curl -fsS "$base/api/terrain/generation-jobs" -H 'Content-Type: application/json' \
    --data '{"spec":{"surfaceWidth":1024,"seed":"container-smoke"}}' > "$work/job.json"
  job=$(field id < "$work/job.json")
  status=running
  for ((i=0;i<240;i++)); do
    curl -fsS "$base/api/terrain/generation-jobs/$job" > "$work/job.json"
    status=$(field status < "$work/job.json")
    if [[ "$status" == ready || "$status" == failed ]]; then break; fi
    sleep 1
  done
  if [[ "$status" != ready ]]; then cat "$work/job.json" >&2; return 1; fi
  id=$(field session < "$work/job.json")
  curl -fsS "$base/api/worlds/$world/accept" -H 'Content-Type: application/json' \
    --data "{\"session\":\"$id\",\"ageId\":\"$age\",\"name\":\"Smoke terrain\"}" > "$work/accepted.json"
  project=$(field map.projectId < "$work/accepted.json")
  curl -fsS "$base/api/sessions/$id/world/state?view=terrain" > "$work/state.json"
  revision=$(field project.revision < "$work/state.json")
  python3 - "$work/state.json" "$revision" > "$work/edit.json" <<'PY'
import json,sys
s=json.load(open(sys.argv[1]))
print(json.dumps({'kind':'operation','revision':int(sys.argv[2]),'operation':{
    'id':'smoke-edit','kind':'raise','layer':'elevation',
    'center':[.45*(s['W']-1),.45*(s['H']-1)],'radius':2,'amount':120}}))
PY
  curl -fsS "$base/api/sessions/$id/terrain/sample?lon=-18&lat=9" > "$work/before.json"
  curl -fsS "$base/api/sessions/$id/world" -H 'Content-Type: application/json' \
    --data-binary "@$work/edit.json" > "$work/edited.json"
  curl -fsS "$base/api/sessions/$id/terrain/sample?lon=-18&lat=9" > "$work/sample.json"
  python3 - "$work/before.json" "$work/sample.json" <<'PY'
import json,sys
before,after=(json.load(open(p)) for p in sys.argv[1:])
assert abs(after['bed']-before['bed']-120)<1e-8, 'terrain edit was not applied'
PY
  curl -fsS "$base/api/sessions/$id/terrain/tiles/2/3/1?style=natural" > "$work/tile.png"
  curl -fsS "$base/api/sessions/$id/terrain/resources?longitude=-18&latitude=9&radiusKm=10" > "$work/resources.json"
  python3 - "$work/resources.json" <<'PY'
import json,sys
report=json.load(open(sys.argv[1]))
assert report['areaKm2']>0 and report['fields'], 'resource inspection failed'
PY
  curl -fsS "$base/api/sessions/$id/view" -H 'Content-Type: application/json' \
    --data '{"ui":{"continuousView":{"camera":{"x":0.8,"y":0.3,"w":0.1,"h":0.05},"style":"terrain","mode":"map"}},"merge":true}' > "$work/staged.json"
  # Accepted terrain and its view autosave. Restart without an explicit save.
  curl -fsS -X DELETE "$base/api/sessions/$id" >/dev/null
  stop_app
  start_app "$volume" "$dsn"
  curl -fsS "$base/api/projects/$project/open?view=terrain" -H 'Content-Type: application/json' --data '{}' > "$work/restored.json"
  id=$(field id < "$work/restored.json")
  curl -fsS "$base/api/sessions/$id/terrain/tiles/2/3/1?style=natural" > "$work/restored-tile.png"
  curl -fsS "$base/api/sessions/$id/terrain/sample?lon=-18&lat=9" > "$work/restored-sample.json"
  cmp "$work/tile.png" "$work/restored-tile.png"
  python3 - "$work/state.json" "$work/restored.json" "$work/sample.json" "$work/restored-sample.json" <<'PY'
import json,sys
original,restored,before,after=(json.load(open(p)) for p in sys.argv[1:])
assert original['continuous']['spec'] == restored['continuous']['spec'], 'geography changed on restart'
assert before['bed'] == after['bed'], 'automatic terrain save lost on restart'
assert restored['projectUI']['continuousView']['camera']['w'] == .1, 'camera state missing'
PY
  curl -fsS "$base/api/sessions/$id/project" -H 'Content-Type: application/json' --data '{}' -o "$work/world.zip"
  python3 - "$work/world.zip" <<'PY'
import sys,zipfile
assert zipfile.is_zipfile(sys.argv[1]), 'world export is not a ZIP archive'
PY
  stop_app
  echo "$driver: frontend, non-root runtime, generation, resources, autosave, restart, terrain/view persistence, ZIP export and graceful shutdown passed"
}
exercise sqlite "$prefix-sqlite"
docker run -d --name "$postgres" --network "$network" \
  -p 127.0.0.1::5432 \
  -e POSTGRES_USER=kriemhild -e POSTGRES_DB=kriemhild -e POSTGRES_PASSWORD=smoke-only-password \
  -v "$prefix-pg:/var/lib/postgresql/data" postgres:17-alpine >/dev/null
ready=false
for ((i=0;i<60;i++)); do
  if docker exec "$postgres" pg_isready -h 127.0.0.1 -p 5432 -U kriemhild -d kriemhild >/dev/null 2>&1; then ready=true; break; fi
  sleep 1
done
[[ "$ready" == true ]]
pg_mapping=$(docker port "$postgres" 5432/tcp)
exercise postgresql "$prefix-cache" "postgres://kriemhild:smoke-only-password@127.0.0.1:${pg_mapping##*:}/kriemhild?sslmode=disable"
