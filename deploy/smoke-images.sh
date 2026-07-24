#!/usr/bin/env bash
set -euo pipefail

: "${BACKEND_IMAGE:?set BACKEND_IMAGE}"
: "${FRONTEND_IMAGE:?set FRONTEND_IMAGE}"

backend_user="$(docker image inspect "$BACKEND_IMAGE" --format '{{.Config.User}}')"
frontend_user="$(docker image inspect "$FRONTEND_IMAGE" --format '{{.Config.User}}')"
test -n "$backend_user" && test "$backend_user" != "0" && test "$backend_user" != "root"
test -n "$frontend_user" && test "$frontend_user" != "0" && test "$frontend_user" != "root"

docker run --rm --entrypoint sh "$BACKEND_IMAGE" -ec '
  test "$(id -u)" -ne 0
  test "$(node --version)" = "v22.22.0"
  test -x /usr/local/bin/todai
  test -f /opt/todai/pi-runner/dist/cli/main.js
  test -d /opt/todai/pi-runner/node_modules
  test ! -e /opt/todai/pi-runner/src
  test ! -e /opt/todai/pi-runner/tests
  ! command -v npm
  ! command -v pnpm
  ! command -v corepack
  test ! -d /opt/todai/pi-runner/node_modules/typescript
  test ! -d /opt/todai/pi-runner/node_modules/vitest
'
docker run --rm --entrypoint sh "$FRONTEND_IMAGE" -ec '
  test "$(id -u)" -ne 0
  test "$(node --version)" = "v22.22.0"
  test ! -e /opt/todai/frontend/server.mjs
  test -f /opt/todai/frontend/build/index.js
  test -f /opt/todai/frontend/build/handler.js
  test ! -d /opt/todai/frontend/node_modules
  ! command -v npm
  ! command -v pnpm
  ! command -v corepack
'

runner_input='{"protocol":"todai.runner","version":4,"type":"run.start","requestId":"smoke-request","sessionId":"smoke-session","runId":"smoke-run","message":"Image smoke test","history":[],"runtimeName":"fake","toolAccess":{"baseUrl":"http://127.0.0.1:8080","token":"smoke-token","allowedTools":["task_get"]},"pi":{}}'
runner_output="$(
  printf '%s\n' "$runner_input" |
    docker run --rm -i --entrypoint /usr/local/bin/node "$BACKEND_IMAGE" \
      /opt/todai/pi-runner/dist/cli/main.js 2>/dev/null
)"
grep -q '"type":"runner.ready"' <<<"$runner_output"
grep -q '"type":"run.completed"' <<<"$runner_output"

container_name="todai-frontend-smoke-${RANDOM}-${RANDOM}"
cleanup() {
  docker rm -f "$container_name" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker run -d --name "$container_name" -e TODAI_BACKEND_URL=http://127.0.0.1:9 \
  -p 127.0.0.1::8080 "$FRONTEND_IMAGE" >/dev/null
frontend_address="$(docker port "$container_name" 8080/tcp | head -n 1)"
for _ in $(seq 1 30); do
  if curl --silent --fail "http://${frontend_address}/health" >/dev/null; then
    break
  fi
  sleep 1
done
test "$(curl --silent --output /dev/null --write-out '%{http_code}' \
  "http://${frontend_address}/projects/direct/overview")" = "200"
test "$(curl --silent --output /dev/null --write-out '%{http_code}' \
  "http://${frontend_address}/a-page-that-does-not-exist")" = "404"
test "$(curl --silent --output /dev/null --write-out '%{http_code}' \
  "http://${frontend_address}/api")" = "502"
test "$(curl --silent --output /dev/null --write-out '%{http_code}' \
  "http://${frontend_address}/internal/tools")" = "404"
test "$(curl --silent --output /dev/null --write-out '%{http_code}' \
  "http://${frontend_address}/internal/tools/task_get")" = "404"
