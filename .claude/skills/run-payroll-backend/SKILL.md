---
name: run-payroll-backend
description: Run, start, build, test, or smoke-test the payroll-backend Go/Gin API server. Use when asked to launch the backend, verify an endpoint, check the API, or confirm a backend change works.
---

`payroll-backend` is a Go 1.25+ / Gin / GORM REST API. It requires PostgreSQL — start it via Docker before running the server. The smoke driver at `.claude/skills/run-payroll-backend/smoke.sh` hits five endpoints and exits 0 on success.

All paths below are relative to `payroll-backend/`.

## Prerequisites

- Go 1.25+: `go version`
- Docker: `docker --version && docker ps`
- `.env` present (copy from `.env.example`; defaults work for local dev)

## Setup (first time / after volume reset)

```bash
# 1. Start PostgreSQL (password must match .env PGPASSWORD=1234)
PGPASSWORD=1234 PGUSER=postgres PGDATABASE=payrolldb \
  docker compose -f docker-compose.local.yml up -d postgres

# 2. Wait for postgres to be healthy
until docker exec payroll-postgres-local pg_isready -U postgres -d payrolldb; do sleep 3; done

# 3. Migrate schema
go run ./migrate

# 4. Seed demo data
go run ./seed
```

## Run (agent path)

**Step 1 — start the server** (keep running in background):
```bash
go run .
# Listens on http://localhost:8000
# Logs route list on startup; "Listening and serving HTTP on :8000" = ready
```

**Step 2 — run the smoke driver:**
```bash
bash .claude/skills/run-payroll-backend/smoke.sh
# Runs: healthz, /modules, login, auth/me, employees
# Exit 0 = all pass. Override URL: bash smoke.sh http://localhost:8001
```

**Step 3 — hit specific endpoints with curl:**
```bash
# Public
curl -s http://localhost:8000/healthz
curl -s http://localhost:8000/api/v1/modules

# Login (sets auth_token cookie)
curl -s -c /tmp/c.txt -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@admin.com","password":"admin@admin.com"}'

# Authenticated (pass cookie)
curl -s -b /tmp/c.txt http://localhost:8000/api/v1/auth/me
curl -s -b /tmp/c.txt http://localhost:8000/api/v1/employees
```

## Run (human path)

```bash
go run .   # server stays in foreground — Ctrl-C to stop
```

## Build / verify

```bash
go build ./...          # compile check
gofmt -w .              # format
go vet ./...            # static analysis
go test ./...           # unit tests
```

## Seed credentials

All seeded users: **password = email**

| Email | Role |
|---|---|
| `admin@admin.com` | SYSTEM_ADMIN (access to all companies) |
| `staff@admin.com` | SYSTEM_STAFF |
| `alice@techa.com` | Manager — TechA |
| `david@bizgroup.com` | Manager — BizGroup |

## Gotchas

- **`role "postgres" does not exist`** — existing Docker volume has wrong credentials. Run `docker compose -f docker-compose.local.yml down -v` then redo Setup.
- **Password mismatch between `.env` and Docker** — docker-compose defaults `PGPASSWORD=postgres`; local `.env` uses `1234`. Always pass `PGPASSWORD=1234` when starting the postgres container (see Setup step 1).
- **Server crashes immediately on start** — DB not running. Start postgres container first.
- **Docker daemon not running** — start Docker Desktop manually, wait ~60s, then retry.
- **`/api/v1/modules` returns 404** — seed hasn't been run; run `go run ./seed`.

## Troubleshooting

**`failed SASL auth`** — password mismatch; see Gotchas above.

**`connection refused :5432`** — postgres container not started or not healthy yet.

**`bind: address already in use :8000`** — another process holds port 8000; kill it or change `PORT` in `.env`.

**`go run .` exits with migration error** — run `go run ./migrate` first; migration state must be initialized before server starts.
