# Docker

Local full stack runs Postgres, the Go API, and the Vite frontend served by nginx. Run from `payroll-backend`:

```bash
docker compose up -d --build
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
- Docker backend uses `PGHOST=postgres`; host-run `go run .`, migrate, or seed needs a host-reachable DB such as `PGHOST=127.0.0.1`.
- Docker Compose reads `PGPASSWORD`, `JWT_SECRET`, and optional runtime values from environment/local `.env`; secrets must not be committed.
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
PGHOST=postgres
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

```bash
docker compose build
docker compose up -d --build
docker compose ps
docker compose logs -f backend
docker compose logs -f frontend
docker compose logs -f postgres
docker compose stop
docker compose down
docker compose down -v
```

Migrate/seed:

```bash
docker compose --profile tools run --rm migrate
docker compose --profile tools run --rm seed
go run ./migrate
go run ./seed
```

`docker compose down -v` deletes the Postgres volume. Use it only for intentional fresh data, initialized DB credential/name changes, or full reseed.

## Linux/Hetzner Update

```bash
cd ~/payroll-backend && git pull
cd ~/payroll-frontend && git pull
cd ~/payroll-backend
docker compose down
docker compose up -d --build
docker compose --profile tools run --rm migrate
docker compose --profile tools run --rm seed
docker compose up -d backend frontend
docker compose ps
```

Fresh reset/reseed:

```bash
docker compose down -v
docker compose up -d --build
docker compose --profile tools run --rm migrate
docker compose --profile tools run --rm seed
docker compose up -d backend frontend
```

## Notes

- Fresh DB healthchecks and `/api/v1/modules` depend on migrations and seeded `app_module` rows.
- Seed includes realistic Jan-Jun 2027 mock data; if 2026 demo data looks missing, check 2027 payroll cycles/reports.
- If Docker shows `dockerDesktopLinuxEngine` pipe errors, start Docker Desktop and retry.
