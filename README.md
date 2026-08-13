# AI Slop Detector

A small Go server-rendered app for anonymous community reports on whether a website host is "AI Slop" or "Not Slop".

## Stack

- Go HTTP server with server-rendered HTML
- Postgres for sites, reports, and precomputed aggregates
- Redis for optional rate limiting
- No frontend build step

## Development environment

This repo uses the shared [dev-platform](https://github.com/link108/dev-platform)
Dev Container base image, but its PostgreSQL and Redis are **fully
isolated to this one workspace** - not shared with other repos,
worktrees, or agent sessions. Every `devcontainer up` (per checkout/
worktree directory) gets its own Postgres/Redis containers, own
generated credentials, and own data. Requirements on the host: Git,
Docker, the [Dev Container CLI](https://github.com/devcontainers/cli)
(or a compatible editor, e.g. VS Code's Dev Containers extension), and
`openssl` (used once to generate credentials). No local Go install
needed.

```sh
devcontainer up --workspace-folder .
```

That's it - `initializeCommand` generates this workspace's Postgres/Redis
credentials on first run (reused on every later run, from
`.devcontainer/secrets/`, gitignored, never committed), then Docker
Compose (`compose.dev.yml`) brings up this workspace's own backing
services and waits for them to be healthy before the container itself
starts. Inside the Dev Container, `mise install` runs automatically
(pinning Go to `1.24`, matching `go.mod`/`Dockerfile`); continue with
`just setup` below. Postgres/Redis are reachable as `postgres:5432` /
`redis:6379` only from inside this workspace's own Dev Container (no
host ports published).

## Local Setup

Inside the Dev Container:

```sh
just setup
just dev
```

Open:

```text
http://localhost:8080
```

## Just Commands

- `just setup` creates `.env.local`, then creates the app role/database if
  missing and applies migrations
- `just dev` runs setup, then starts the Go server locally
- `just migrate` or `just db-migrate` applies pending migrations
- `just db-reset` rolls back all app migrations, then reapplies them
- `just seed` or `just db-seed` inserts sample reports through the Go write path
- `just psql` / `just redis-cli` open shells against this workspace's Postgres/Redis
- `just verify` (alias `just ci`) runs typecheck, lint, test, and build - same as CI

Workspace lifecycle (run from the **host**, outside the Dev Container -
these manage the containers themselves, so they need direct Docker
access):

```sh
just status          # this workspace's container status
just workspace-logs   # follow this workspace's container logs
just workspace-stop   # stop containers; keeps data volumes + credentials
just destroy          # DESTROYS this workspace's containers, data volumes,
                       # and generated credentials - confirmation-gated,
                       # and only ever touches this checkout's own resources
```

### Testing the production image

`just docker-build`/`just run` (alias for `just docker-run`) build and run
the actual production image standalone, against a **separate**,
host-reachable Postgres/Redis (not this workspace's Dev Container
services) - see `DOCKER_DATABASE_URL`/`DOCKER_REDIS_URL` in
`.env.example`, which default to `host.docker.internal` (what Docker
Desktop usually needs to reach services exposed on the host). `just
stop`/`just logs` (aliases for `just docker-stop`/`just docker-logs`)
manage that container.

## Migrations

Migrations use `golang-migrate`. Schema changes live in versioned `up` and `down` files under `migrations/`.

Create future migrations with paired files like:

```text
migrations/000002_add_admin_fields.up.sql
migrations/000002_add_admin_fields.down.sql
```

## Environment

`DATABASE_URL` is required. Inside the Dev Container, `just` supplies it
automatically from this workspace's generated Postgres credentials - see
`.env.example` and Development environment above.

`SETUP_DATABASE_URL` is used only by `just db-setup`, to create the app
role/database if they don't already exist. Not needed inside the Dev
Container (its Postgres container already creates both via
`POSTGRES_DB`/`POSTGRES_USER_FILE` - `just db-setup`'s role-creation step
just no-ops there); set it to a local Postgres superuser role for
non-Dev-Container local setups.

`REDIS_URL` is optional. If it is unset, the app still runs but rate limiting is disabled. In production, set Redis so these rules are enforced:

- Global submissions per fingerprint per minute
- One report per fingerprint per host per 24 hours

Set `FINGERPRINT_SECRET` to a long random value in any shared environment. The development fallback is intentionally not suitable for production.

If the app runs behind a trusted reverse proxy, set `TRUST_PROXY_HEADERS=true` so fingerprints use `X-Forwarded-For` or `X-Real-IP`.

## Routes

- `GET /` home, report form, lookup form, recent and trending hosts
- `POST /report` submit a report and redirect to the host page
- `GET /lookup?input=...` normalize and redirect to the host page
- `GET /site/{host}` aggregate host view
- `GET /healthz` health check
