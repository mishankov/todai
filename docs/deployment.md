# Self-managed deployment

Todai supports one Linux Docker host with Docker Engine and Docker Compose. The release images contain
the web application, Go backend, Node.js runtime, and compiled Pi runner; the host does not need Go,
Node.js, pnpm, or Task.

The stack serves plain HTTP. Do not expose that port directly to the public internet. Put it behind an
external HTTPS reverse proxy and firewall. The proxy must preserve `Host`, `X-Forwarded-Host`, and
`X-Forwarded-Proto`, use HTTP/1.1 upstream connections, and disable response buffering for `/api` so
server-sent events arrive immediately. DNS, TLS, firewall, server provisioning, and off-host backups
are operator responsibilities.

## Supported platforms and tags

Release CI builds and smoke-tests `linux/amd64` and `linux/arm64`. Images are published as:

- `ghcr.io/mishankov/todai-frontend:<tag>`
- `ghcr.io/mishankov/todai-backend:<tag>`

A release such as `v0.1.0` has an immutable release tag. Successful `master` builds have an immutable
`sha-<full-git-commit>` tag and update the moving `edge` tag. Pin `TODAI_VERSION` to a release or full
commit tag for deployments; do not use `edge` when rollback predictability matters.

## First install

Choose a released tag and download only its deployment files:

```sh
install -d -m 700 todai
cd todai
release=v0.1.0
curl -fsSL "https://raw.githubusercontent.com/mishankov/todai/${release}/deploy/compose.yaml" -o compose.yaml
curl -fsSL "https://raw.githubusercontent.com/mishankov/todai/${release}/deploy/.env.example" -o .env
chmod 600 .env
sed -i "s/^TODAI_VERSION=.*/TODAI_VERSION=${release}/" .env
sed -i "s/^TODAI_POSTGRES_PASSWORD=.*/TODAI_POSTGRES_PASSWORD=$(openssl rand -hex 32)/" .env
```

Review `.env`. `TODAI_VERSION`, `TODAI_POSTGRES_USER`, and `TODAI_POSTGRES_PASSWORD` are required.
Production startup rejects the development database username or password `todai`. Keep the generated
database password URL-safe because Compose constructs `TODAI_DATABASE_URL` from it and the private
service name `postgres`.

Pull the selected images, start PostgreSQL, run migrations, and create the single user:

```sh
docker compose pull
docker compose up -d postgres
docker compose run --rm migrate
docker compose run --rm backend bootstrap-user --username admin
```

The bootstrap command prompts twice without echo. It never accepts the password in an argument or
configuration file and will not silently create or replace a user during normal startup. For a
non-interactive secret source, use stdin:

```sh
secret_file=/secure/path/todai-initial-password
docker compose run --rm -T backend bootstrap-user --username admin --password-stdin < "$secret_file"
```

Start the application and wait for all health checks:

```sh
docker compose up -d --wait
published_port="$(sed -n 's/^TODAI_PUBLISHED_HTTP_PORT=//p' .env)"
curl --fail "http://127.0.0.1:${published_port:-8080}/health"
docker compose ps
```

The frontend is the only service with a published port. PostgreSQL, the backend, and
`/internal/tools` remain private. A migration failure prevents the backend and frontend from becoming
ready.

## Agent runtime and provider state

`TODAI_AGENT_RUNTIME=fake` is a deterministic diagnostic mode. Real runs use `pi`; set all of:

```dotenv
TODAI_AGENT_RUNTIME=pi
TODAI_PI_PROVIDER=provider-name
TODAI_PI_MODEL=model-name
TODAI_PI_MODELS=model-name,another-allowed-model
```

The model must appear in `TODAI_PI_MODELS`. Production fails fast if the Pi agent directory, provider,
or model is missing. The container uses `/var/lib/todai/pi-agent`, backed by the `pi-agent-data` named
volume. Provision an existing Pi `auth.json` without placing it in Compose YAML, an image layer, or
the command line:

```sh
auth_file=/secure/path/auth.json
docker compose run --rm --no-deps -T --entrypoint sh backend \
  -c 'umask 077; cat > "$TODAI_PI_AGENT_DIR/auth.json"' < "$auth_file"
docker compose up -d --wait backend frontend
```

The non-root backend user owns this volume and can update token-refresh state. Restrict access to the
deployment directory and source credential file.

## Runtime configuration

| Variable                       | Default                 | Meaning                                              |
| ------------------------------ | ----------------------- | ---------------------------------------------------- |
| `TODAI_VERSION`                | required                | One coherent frontend/backend image tag              |
| `TODAI_PUBLISHED_HTTP_PORT`    | `8080`                  | Only host-published port                             |
| `TODAI_POSTGRES_DB`            | `todai`                 | Internal database name                               |
| `TODAI_POSTGRES_USER`          | required                | Non-development database user                        |
| `TODAI_POSTGRES_PASSWORD`      | required                | URL-safe non-development database password           |
| `TODAI_SESSION_COOKIE_NAME`    | `todai_session`         | Browser session cookie name                          |
| `TODAI_BACKEND_URL`            | `http://backend:8080`   | Frontend proxy upstream on the Compose network       |
| `TODAI_INTERNAL_API_URL`       | `http://127.0.0.1:8080` | Runner-only tool API in the shared backend container |
| `TODAI_AGENT_RUNTIME`          | `fake`                  | `fake` for diagnostics or `pi` for real agents       |
| `TODAI_RUNNER_STARTUP_TIMEOUT` | `5s`                    | Runner ready-event deadline                          |
| `TODAI_RUNNER_RUN_TIMEOUT`     | `2m`                    | Maximum agent run duration                           |
| `TODAI_RUNNER_ABORT_TIMEOUT`   | `2s`                    | Graceful runner abort deadline                       |
| `TODAI_RUNNER_MAX_LINE_BYTES`  | `1048576`               | Maximum JSONL protocol record                        |
| `TODAI_AGENT_TOKEN_TTL`        | `15m`                   | Short-lived internal tool token lifetime             |
| `TODAI_PI_PROVIDER`            | required for `pi`       | Pi provider identifier                               |
| `TODAI_PI_MODEL`               | required for `pi`       | Default Pi model                                     |
| `TODAI_PI_MODELS`              | selected Pi model       | Comma-separated user-selectable model allow-list     |

The image fixes `TODAI_RUNNER_EXECUTABLE=/usr/local/bin/node`,
`TODAI_RUNNER_ENTRY=/opt/todai/pi-runner/dist/cli/main.js`, and
`TODAI_PI_AGENT_DIR=/var/lib/todai/pi-agent`. Change neither path on the host.

## Logs and diagnosis

All containers log to stdout/stderr. View the complete stack or one service:

```sh
docker compose logs --tail=200 -f
docker compose logs --tail=200 -f backend
docker compose logs --tail=200 -f postgres
```

Backend logs are JSON with `component=backend`. Runner stderr is re-emitted one line at a time with
`subsystem=pi-runner`, `run_id`, and `stream=stderr`; runner protocol stdout is never copied to logs.
With `jq`, separate the two sources:

```sh
docker compose logs --no-color --no-log-prefix backend |
  jq -Rr 'fromjson? | select(.component == "backend" and .subsystem != "pi-runner")'
docker compose logs --no-color --no-log-prefix backend |
  jq -Rr 'fromjson? | select(.subsystem == "pi-runner")'
```

Check both container and application readiness:

```sh
docker compose ps
docker compose exec -T postgres sh -c 'pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
docker compose exec -T backend wget -q -O - http://127.0.0.1:8080/health
published_port="$(sed -n 's/^TODAI_PUBLISHED_HTTP_PORT=//p' .env)"
curl --fail "http://127.0.0.1:${published_port:-8080}/health"
```

## Stop, restart, and persistence

```sh
docker compose stop
docker compose start
docker compose up -d --wait
```

`docker compose down`, image pulls, and container recreation preserve `postgres-data` and
`pi-agent-data`. Removing them is separate and destructive:

```sh
# DESTRUCTIVE: permanently removes the database and Pi credentials/state.
docker compose down --volumes
```

## Backup and restore

Write a compressed PostgreSQL archive outside the database volume:

```sh
backup="todai-$(date -u +%Y%m%dT%H%M%SZ).dump"
docker compose exec -T postgres sh -c \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom' > "$backup"
test -s "$backup"
chmod 600 "$backup"
```

Restore replaces the selected database. Verify `restore_file`, stop application processes, and take a
second backup first:

```sh
restore_file=/secure/backups/todai-YYYYMMDDTHHMMSSZ.dump
test -s "$restore_file"
docker compose stop frontend backend
# DESTRUCTIVE: the next command replaces TODAI_POSTGRES_DB.
docker compose exec -T postgres sh -c \
  'dropdb --if-exists -U "$POSTGRES_USER" "$POSTGRES_DB" &&
   createdb -U "$POSTGRES_USER" "$POSTGRES_DB" &&
   pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < "$restore_file"
docker compose run --rm migrate
docker compose up -d --wait
```

CI verifies backup and restore against a second disposable database on every image smoke test.
Automated scheduling and off-host retention are intentionally not included.

## Upgrade

Read the release notes first; they declare whether migrations remain backward-compatible. Always
make a pre-upgrade backup. Then set the new immutable version in `.env` and run:

```sh
docker compose pull
docker compose stop frontend backend
docker compose run --rm migrate
docker compose up -d --wait
docker compose ps
```

The explicit migration must succeed before the upgraded backend starts. Compose also keeps migration
as a readiness dependency, so a failed migration cannot produce a healthy frontend.

## Rollback

For a release whose notes declare its migrations backward-compatible, restore the previous immutable
tag in `.env`:

```sh
docker compose pull
docker compose stop frontend backend
docker compose run --rm migrate
docker compose up -d --wait
```

If the upgrade introduced incompatible migrations, do not start the previous application against the
new schema. Stop the application, restore the matching pre-upgrade database backup using the procedure
above, select the previous immutable image tag, run that version's migrations, and start the stack.
