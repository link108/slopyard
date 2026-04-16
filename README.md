# AI Slop Detector

A small Go server-rendered app for anonymous community reports on whether a website host is "AI Slop" or "Not Slop".

## Stack

- Go HTTP server with server-rendered HTML
- Postgres for sites, reports, and precomputed aggregates
- Redis for optional rate limiting
- No frontend build step

## Local Setup

Create Postgres and Redis, then copy the environment example and adjust values:

```sh
cp .env.example .env
```

Apply migrations:

```sh
just db-setup
```

Run the app in Docker:

```sh
just run
```

Run the app locally without Docker:

```sh
just dev
```

Open:

```text
http://localhost:8080
```

## Just Commands

- `just docker-build` builds the app image as `slopyard:local`
- `just run` or `just docker-run` starts one named app container using `.env`
- `just stop` or `just docker-stop` stops and removes the app container
- `just logs` or `just docker-logs` follows app container logs
- `just db-setup` creates the configured local database if missing, then applies migrations
- `just dev` runs setup, then starts the Go server locally
- `just migrate` or `just db-migrate` applies pending migrations
- `just db-reset` rolls back all app migrations, then reapplies them
- `just seed` or `just db-seed` inserts sample reports through the Go write path

`just run` does not start Postgres or Redis. It expects your existing services to be reachable from the app container. By default the provided `.env.example` includes `DOCKER_DATABASE_URL` and `DOCKER_REDIS_URL` values that point at `host.docker.internal`, which is usually what Docker Desktop needs when Postgres and Redis are exposed on your host.

For non-Docker local development, run:

```sh
go run ./cmd/slopyard
```

## Migrations

Migrations use `golang-migrate`. Schema changes live in versioned `up` and `down` files under `migrations/`.

Create future migrations with paired files like:

```text
migrations/000002_add_admin_fields.up.sql
migrations/000002_add_admin_fields.down.sql
```

## Environment

`DATABASE_URL` is required.

`SETUP_DATABASE_URL` is used only by `just db-setup`. Set it to a local Postgres role that can create roles and databases. The default local `.env` uses the current machine user for setup and the `slopyard` role for the app connection.

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
