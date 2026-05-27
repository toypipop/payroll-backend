# Docker

Local full stack runs Postgres, the Go API, and the Vite frontend served by nginx. Run from `payroll-backend`:

```bash
docker compose -f docker-compose.local.yml up -d --build
```

## URLs

- Frontend: `http://localhost:5174`
- Backend direct API: `http://localhost:8001/api/v1`
- UAT frontend: `http://5.223.87.23:5174`
- UAT direct API: `http://5.223.87.23:8001/api/v1`
- Frontend nginx proxies `/api` to the backend container.

## Routing

Keep the frontend API path identical in every environment:

```text
VITE_API_URL=/api/v1
```

| Environment | Browser calls | Proxy | Backend target |
| --- | --- | --- | --- |
| Local Docker full stack | `http://localhost:5174/api/v1/...` | frontend nginx | `http://backend:8000` |
| Frontend dev + Docker backend | `http://localhost:5174/api/v1/...` | Vite proxy | `API_PROXY_TARGET=http://localhost:8001` |
| Frontend dev + `go run .` | `http://localhost:5174/api/v1/...` | Vite proxy | `API_PROXY_TARGET=http://localhost:8000` |
| UAT IP/port | `http://5.223.87.23:5174/api/v1/...` | frontend nginx | `http://backend:8000` |
| SIT/UAT/Prod domain | `https://<env-domain>/api/v1/...` | public proxy | backend `:8000` |

SIT/UAT/Prod should not need a frontend rebuild just to change backend host; keep `VITE_API_URL=/api/v1` and change proxy/backend settings.

## Environment

- `.env` is runtime-only and ignored; `.env.example` lists keys only and contains no real values or secrets.
- Local Docker reads `PGUSER`, `PGPASSWORD`, `PGDATABASE`, `JWT_SECRET`, and runtime values from `.env`, but containers always use `PGHOST=postgres` on the Docker network.
- Host-run `go run .`, Air, migrate, or seed needs `.env` to use a host-reachable DB such as `PGHOST=127.0.0.1`.
- `PGPASSWORD` must match the password used when the local Postgres volume was first initialized; if it changed, recreate only the local volume with `docker compose -f docker-compose.local.yml down -v`.
- Docker Compose reads secrets from environment/local `.env`; secrets must not be committed.
- Seeded demo users use email-as-password, e.g. `admin@admin.com` / `admin@admin.com`.

Postgres defaults:

```text
Container: payroll-postgres
Docker host: postgres
Port: 5432
Database: payrolldb
Volume: payroll_postgres_data
```

Backend `.env` shape:

```text
PGUSER=postgres
PGPASSWORD=<local-password>
PGHOST=127.0.0.1
PGPORT=5432
PGDATABASE=payrolldb
PGSSLMODE=disable
DB_TIMEZONE=Asia/Bangkok
JWT_SECRET=<long-random-secret>
FRONTEND_ORIGIN=http://localhost:5174
APP_ENV=development
COOKIE_SECURE=false
COOKIE_SAMESITE=lax
```

UAT by IP/port: set `FRONTEND_ORIGIN` and `CORS_ALLOWED_ORIGINS` to `http://5.223.87.23:5174`, use `APP_ENV=uat`, `GIN_MODE=release`, `COOKIE_SECURE=false`, `COOKIE_SAMESITE=lax`.

HTTPS same-origin SIT/UAT/Prod: build frontend with `VITE_API_URL=/api/v1`, set `FRONTEND_ORIGIN` to the public origin, use `COOKIE_SECURE=true` and usually `COOKIE_SAMESITE=none` for production HTTPS.

## Commands

Local development with direct ports and no Caddy:

```bash
docker compose -f docker-compose.local.yml up -d --build
docker compose -f docker-compose.local.yml ps
docker compose -f docker-compose.local.yml logs -f backend
docker compose -f docker-compose.local.yml logs -f frontend
docker compose -f docker-compose.local.yml logs -f postgres
docker compose -f docker-compose.local.yml stop
docker compose -f docker-compose.local.yml down
docker compose -f docker-compose.local.yml down -v
```

The local file uses the same `PGUSER`, `PGPASSWORD`, and `PGDATABASE` values as host-run Go. It overrides only container-side `PGHOST` to `postgres`, so backend, migrate, and seed containers connect to the database service while host-run Go uses `127.0.0.1:5432`.

Local migrate/seed:

```bash
docker compose -f docker-compose.local.yml --profile tools run --rm migrate
docker compose -f docker-compose.local.yml --profile tools run --rm seed
```

To avoid repeating `-f docker-compose.local.yml` in a shell session:

```bash
$env:COMPOSE_FILE = "docker-compose.local.yml"
docker compose --profile tools run --rm migrate
docker compose --profile tools run --rm seed
```

UAT/SIT/production compose with Caddy:

```bash
docker compose -f docker-compose.uat.yml build
docker compose -f docker-compose.uat.yml up -d --build
docker compose -f docker-compose.uat.yml ps
docker compose -f docker-compose.uat.yml logs -f backend
docker compose -f docker-compose.uat.yml logs -f frontend
docker compose -f docker-compose.uat.yml logs -f postgres
docker compose -f docker-compose.uat.yml stop
docker compose -f docker-compose.uat.yml down
docker compose -f docker-compose.uat.yml down -v
```

Migrate/seed:

```bash
docker compose -f docker-compose.uat.yml --profile tools run --rm migrate
docker compose -f docker-compose.uat.yml --profile tools run --rm seed
go run ./migrate
go run ./seed
```

`docker compose -f docker-compose.uat.yml down -v` deletes the UAT/prod Postgres volume. Use it only for intentional fresh data, initialized DB credential/name changes, or full reseed.

## Linux/Hetzner Update

```bash
cd ~/payroll-backend && git pull
cd ~/payroll-frontend && git pull
cd ~/payroll-backend
docker compose -f docker-compose.uat.yml down
docker compose -f docker-compose.uat.yml up -d --build
docker compose -f docker-compose.uat.yml --profile tools run --rm migrate
docker compose -f docker-compose.uat.yml --profile tools run --rm seed
docker compose -f docker-compose.uat.yml up -d backend frontend
docker compose -f docker-compose.uat.yml ps
```

Fresh reset/reseed:

```bash
docker compose -f docker-compose.uat.yml down -v
docker compose -f docker-compose.uat.yml up -d --build
docker compose -f docker-compose.uat.yml --profile tools run --rm migrate
docker compose -f docker-compose.uat.yml --profile tools run --rm seed
docker compose -f docker-compose.uat.yml up -d backend frontend
```

## Notes

- Fresh DB healthchecks and `/api/v1/modules` depend on migrations and seeded `app_module` rows.
- Seed includes realistic Jan-Jun 2027 mock data; if 2026 demo data looks missing, check 2027 payroll cycles/reports.
- If Docker shows `dockerDesktopLinuxEngine` pipe errors, start Docker Desktop and retry.
