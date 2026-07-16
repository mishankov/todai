# ADR 0001: Component-oriented monorepo

Status: accepted  
Date: 2026-07-16

## Decision

Todai uses a monorepo with a top-level directory for each deployable component. The initial
components are `backend` and `frontend`; `pi-runner` and `desktop` will be added only when their
development starts. Infrastructure and architectural records live in their own directories.

The repository root contains only project-wide configuration and entry points such as the root
Taskfile, README and product requirements.

## Consequences

- Each component owns its dependencies, build configuration and tests.
- Root commands aggregate component commands without hiding how a component works.
- Empty placeholder components are not committed.
