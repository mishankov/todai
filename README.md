# Todai

Todai is a personal-first task tracker designed for both direct use and safe interaction with AI
agents. The current repository contains the development skeleton described in
[`prd.md`](./prd.md).

## Components

- `backend` — Go HTTP API built on Platforma.
- `frontend` — SvelteKit web application created with the official Svelte CLI.
- `pi-runner` — isolated TypeScript process that exposes the stable agent JSONL protocol.
- `infrastructure` — local PostgreSQL configuration.
- `docs/adr` — accepted architectural decisions.

The runner supports a deterministic fake runtime for tests and an opt-in Pi SDK runtime. The `desktop`
component will be added later.

## Prerequisites

- Go 1.25
- Node.js 22.19 or newer
- pnpm 11
- Docker with Docker Compose
- [Task](https://taskfile.dev/)

## First local run

Install frontend dependencies:

```shell
task setup
```

Start PostgreSQL, apply migrations, and run both development servers:

```shell
task dev
```

Create the single user from another terminal:

```shell
task backend:bootstrap-user USERNAME=admin
```

The bootstrap command prompts for the password without echoing it. Public registration is not
available.

- Frontend: http://localhost:5173
- Backend health: http://localhost:8080/health

SvelteKit proxies `/api` requests to the backend in development.

## Commands

```shell
task check                       # backend and frontend checks
task setup:test                  # dependencies and Playwright browsers
task backend:test                # Go tests
task backend:lint                # golangci-lint
task frontend:test               # Vitest tests
task frontend:test:e2e           # Playwright tests
task pi-runner:check             # runner lint, types, tests, and build
task infrastructure:down         # stop PostgreSQL
task infrastructure:reset        # remove PostgreSQL and local data
```

## Configuration

The backend uses environment variables listed in [`.env.example`](./.env.example). Defaults match
the local Docker Compose configuration.

Agent runs use the deterministic `fake` runtime by default. To use Pi, set
`TODAI_AGENT_RUNTIME=pi` and optionally set `TODAI_PI_AGENT_DIR`, `TODAI_PI_PROVIDER`,
`TODAI_PI_MODEL`, and the comma-separated user-selectable `TODAI_PI_MODELS`. Pi reads provider
authentication from the selected agent directory's `auth.json`;
the backend never sends provider credentials to the runner. Each run receives a separate short-lived
token that can call only Todai's internal task tools.

The backend uses the globally installed `golangci-lint` with its default configuration.

Platforma's configurable auth routes, cookie policy and session-expiration behavior are tracked in
[platforma-dev/platforma#93](https://github.com/platforma-dev/platforma/issues/93). Until that work
is complete, Todai mounts the allowed auth handlers individually and does not expose `/register`.
