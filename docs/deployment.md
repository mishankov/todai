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

That is the complete default configuration: an immutable image version and a generated database
password. The database name (`todai`), database user (`todai_app`), service addresses, cookie name,
timeouts, and internal paths already have installation-safe defaults. Keep the generated password
URL-safe because Compose uses it in the private database URL.

Pull and start the stack. Compose waits for PostgreSQL, runs migrations, and starts the application
only after the migration succeeds:

```sh
docker compose pull
docker compose up -d --wait
```

Create the single user:

```sh
docker compose run --rm backend bootstrap-user --username admin
```

The bootstrap command prompts twice without echo. It never accepts the password in an argument or
configuration file and will not silently create or replace a user during normal startup. For a
non-interactive secret source, use stdin:

```sh
secret_file=/secure/path/todai-initial-password
docker compose run --rm -T backend bootstrap-user --username admin --password-stdin < "$secret_file"
```

Verify the default installation:

```sh
curl --fail http://127.0.0.1:8080/health
docker compose ps
```

Use the configured port instead of `8080` if `TODAI_PUBLISHED_HTTP_PORT` was changed.
The frontend is the only service with a published port. PostgreSQL, the backend, and
`/internal/tools` remain private. A migration failure prevents the backend and frontend from becoming
ready.

## Agent runtime and provider state

`TODAI_AGENT_RUNTIME=fake` is a deterministic diagnostic mode. Real runs use `pi`; set all of:

```dotenv
TODAI_AGENT_RUNTIME=pi
TODAI_PI_PROVIDER=provider-name
TODAI_PI_MODEL=model-name
```

The selectable model list defaults to `TODAI_PI_MODEL`. Set `TODAI_PI_MODELS` only when more than one
model should be available. Production fails fast if the Pi agent directory, provider, or model is
missing. The container uses `/var/lib/todai/pi-agent`, backed by the `pi-agent-data` named volume.
Provision an existing Pi `auth.json` without placing it in Compose YAML, an image layer, or the
command line:

```sh
auth_file=/secure/path/auth.json
docker compose run --rm --no-deps -T --entrypoint sh backend \
  -c 'umask 077; cat > "$TODAI_PI_AGENT_DIR/auth.json"' < "$auth_file"
docker compose up -d --wait backend frontend
```

The non-root backend user owns this volume and can update token-refresh state. Restrict access to the
deployment directory and source credential file.

## Installation settings

| Variable                    | Default           | Meaning                                        |
| --------------------------- | ----------------- | ---------------------------------------------- |
| `TODAI_VERSION`             | required          | One coherent frontend/backend image tag        |
| `TODAI_POSTGRES_PASSWORD`   | required          | URL-safe database password                     |
| `TODAI_PUBLISHED_HTTP_PORT` | `8080`            | Only host-published port                       |
| `TODAI_AGENT_RUNTIME`       | `fake`            | `fake` for diagnostics or `pi` for real runs   |
| `TODAI_PI_PROVIDER`         | required for `pi` | Pi provider identifier                         |
| `TODAI_PI_MODEL`            | required for `pi` | Default and initially allowed model            |
| `TODAI_PI_MODELS`           | selected Pi model | Optional comma-separated selectable model list |

Internal settings are intentionally absent from the default `.env`. The images and Compose file use
`todai_session` for the cookie, `http://backend:8080` for the frontend proxy,
`http://127.0.0.1:8080` for runner tools, `5s`/`2m`/`2s` for runner startup/run/abort timeouts,
`1048576` bytes for runner records, and `15m` for agent tokens. The backend image also fixes the
bundled Node executable, runner entry point, and Pi data directory. Unusual installations can change
these with a separate Compose override without making the default installation carry that
complexity.

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
curl --fail http://127.0.0.1:8080/health
```

Use the configured published port for the final command if it differs from `8080`.

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
# DESTRUCTIVE: the next command replaces the `todai` database.
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
docker compose up -d --wait
docker compose ps
```

Compose runs the selected version's migration before replacing the backend. A failed migration
cannot produce a healthy frontend.

## Rollback

For a release whose notes declare its migrations backward-compatible, restore the previous immutable
tag in `.env`:

```sh
docker compose pull
docker compose up -d --wait
```

If the upgrade introduced incompatible migrations, do not start the previous application against the
new schema. Stop the application, restore the matching pre-upgrade database backup using the procedure
above, select the previous immutable image tag, run that version's migrations, and start the stack.
