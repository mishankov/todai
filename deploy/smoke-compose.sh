#!/usr/bin/env bash
set -euo pipefail

: "${TODAI_BACKEND_IMAGE:?set TODAI_BACKEND_IMAGE}"
: "${TODAI_FRONTEND_IMAGE:?set TODAI_FRONTEND_IMAGE}"

export TODAI_VERSION="${TODAI_VERSION:-smoke}"
export TODAI_PUBLISHED_HTTP_PORT="${TODAI_SMOKE_PORT:-18080}"
export TODAI_POSTGRES_DB=todai
export TODAI_POSTGRES_USER=todai_smoke_app
export TODAI_POSTGRES_PASSWORD=smoke_database_password
export TODAI_AGENT_RUNTIME=fake
export TODAI_BACKEND_URL=http://backend:8080
export TODAI_INTERNAL_API_URL=http://127.0.0.1:8080
export TODAI_PI_PROVIDER=
export TODAI_PI_MODEL=
export TODAI_PI_MODELS=

compose_project="todai-smoke-${RANDOM}-${RANDOM}"
compose=(docker compose --project-name "$compose_project" --file deploy/compose.yaml)
cookie_file="$(mktemp)"
sse_file="$(mktemp)"
backup_file="$(mktemp)"

cleanup() {
  status=$?
  trap - EXIT
  if ((status != 0)); then
    "${compose[@]}" ps || true
    "${compose[@]}" logs --no-color --tail=200 || true
  fi
  "${compose[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -f "$cookie_file" "$sse_file" "$backup_file"
  exit "$status"
}
trap cleanup EXIT

"${compose[@]}" config --quiet
"${compose[@]}" up -d --wait --wait-timeout 240

base_url="http://127.0.0.1:${TODAI_PUBLISHED_HTTP_PORT}"
curl --silent --fail "${base_url}/health" >/dev/null
curl --silent --fail "${base_url}/login" | grep -q '<div'
test "$(curl --silent --output /dev/null --write-out '%{http_code}' \
  "${base_url}/internal/tools")" = "404"
if backend_port="$("${compose[@]}" port backend 8080 2>/dev/null)" &&
  [[ -n "$backend_port" && "$backend_port" != ":0" ]]; then
  exit 1
fi
if postgres_port="$("${compose[@]}" port postgres 5432 2>/dev/null)" &&
  [[ -n "$postgres_port" && "$postgres_port" != ":0" ]]; then
  exit 1
fi

printf '%s\n' smoke-password |
  "${compose[@]}" run --rm -T backend bootstrap-user --username smoke-admin --password-stdin
curl --silent --fail --cookie-jar "$cookie_file" \
  --header 'Content-Type: application/json' \
  --data '{"login":"smoke-admin","password":"smoke-password"}' \
  "${base_url}/api/auth/login" >/dev/null

project_id="$(
  curl --silent --fail --cookie "$cookie_file" \
    --header 'Content-Type: application/json' \
    --data '{"name":"Image smoke project"}' \
    "${base_url}/api/projects" | jq -er '.id'
)"
session_id="$(
  curl --silent --fail --cookie "$cookie_file" \
    --request POST "${base_url}/api/agent/sessions" | jq -er '.id'
)"
curl --silent --fail --cookie "$cookie_file" \
  --header 'Content-Type: application/json' \
  --data "{\"projectId\":\"${project_id}\",\"message\":\"Exercise the packaged runner\"}" \
  "${base_url}/api/agent/sessions/${session_id}/messages" >/dev/null

set +e
curl --silent --show-error --max-time 10 --no-buffer --cookie "$cookie_file" \
  "${base_url}/api/agent/sessions/${session_id}/events" >"$sse_file"
sse_status=$?
set -e
test "$sse_status" -eq 0 || test "$sse_status" -eq 28
grep -q 'agent.run.completed' "$sse_file"
"${compose[@]}" logs --no-color --no-log-prefix backend |
  grep -q '"subsystem":"pi-runner"'

"${compose[@]}" exec -T backend sh -ec \
  'printf persisted > "$TODAI_PI_AGENT_DIR/smoke-persistence-marker"'
"${compose[@]}" exec -T postgres sh -c \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom' >"$backup_file"
test -s "$backup_file"
"${compose[@]}" exec -T postgres sh -c \
  'createdb -U "$POSTGRES_USER" todai_restore_smoke'
"${compose[@]}" exec -T postgres sh -c \
  'pg_restore -U "$POSTGRES_USER" -d todai_restore_smoke' <"$backup_file"
test "$("${compose[@]}" exec -T postgres sh -c \
  'psql -U "$POSTGRES_USER" -d todai_restore_smoke -Atc "select count(*) from projects"')" = "1"
"${compose[@]}" exec -T postgres sh -c \
  'dropdb -U "$POSTGRES_USER" todai_restore_smoke'

"${compose[@]}" down
"${compose[@]}" up -d --wait --wait-timeout 240
"${compose[@]}" exec -T backend test -f \
  /var/lib/todai/pi-agent/smoke-persistence-marker
curl --silent --fail --cookie-jar "$cookie_file" \
  --header 'Content-Type: application/json' \
  --data '{"login":"smoke-admin","password":"smoke-password"}' \
  "${base_url}/api/auth/login" >/dev/null
