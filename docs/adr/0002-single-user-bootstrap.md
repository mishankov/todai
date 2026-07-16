# ADR 0002: Bootstrap the single user through the backend CLI

Status: accepted  
Date: 2026-07-16

## Decision

The personal MVP does not expose public registration. Its only user is created with
`todai bootstrap-user` after database migrations are applied. The command refuses to create a
user when the installation is already initialized and reads passwords without placing them in
command-line arguments.

Auth handlers are mounted individually until Platforma supports configurable auth routes. The
remaining cookie and session-expiration limitations are tracked in
https://github.com/platforma-dev/platforma/issues/93.

## Consequences

- There is no first-run registration screen in the MVP.
- Operators need database and process access to initialize an installation.
- The temporary selective handler wiring can be removed after the Platforma issue is resolved.
