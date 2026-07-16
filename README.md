# Todai

Todai is a personal-first task tracker designed for both direct use and safe interaction with AI
agents. The current repository contains the development skeleton described in
[`prd.md`](./prd.md).

## Components

- `backend` — Go HTTP API built on Platforma.
- `frontend` — SvelteKit web application created with the official Svelte CLI.
- `infrastructure` — local PostgreSQL configuration.
- `docs/adr` — accepted architectural decisions.

Future `pi-runner` and `desktop` components will be added when their implementation begins.

## Prerequisites

- Go 1.25
- Node.js 22
- pnpm 11
- Docker with Docker Compose
- [Task](https://taskfile.dev/)

## First local run

Install frontend dependencies and start PostgreSQL:

```shell
task setup
task infrastructure:up
```

Apply migrations and create the single user:

```shell
task backend:migrate
task backend:bootstrap-user USERNAME=admin
```

The bootstrap command prompts for the password without echoing it. Public registration is not
available.

Start both development servers:

```shell
task dev
```

- Frontend: http://localhost:5173
- Backend health: http://localhost:8080/health

SvelteKit proxies `/api` requests to the backend in development.

## Commands

```shell
task check                       # backend and frontend checks
task backend:test                # Go tests
task frontend:test               # Vitest tests
task frontend:test:e2e           # Playwright tests
task infrastructure:down         # stop PostgreSQL
task infrastructure:reset        # remove PostgreSQL and local data
```

## Configuration

The backend uses environment variables listed in [`.env.example`](./.env.example). Defaults match
the local Docker Compose configuration.

Platforma's configurable auth routes, cookie policy and session-expiration behavior are tracked in
[platforma-dev/platforma#93](https://github.com/platforma-dev/platforma/issues/93). Until that work
is complete, Todai mounts the allowed auth handlers individually and does not expose `/register`.

