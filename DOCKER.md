# Docker

This repo runs the local full stack with Docker: Postgres, the Go API, and the Vite frontend served by nginx. Run from the backend root:

```bash
docker compose up -d --build
```

Apps:

- Frontend: `http://localhost:5174`
- Backend API direct port: `http://localhost:8001/api/v1`
- Frontend nginx proxies `/api` to the backend container.

## Environment routing

The frontend should use the same API path in every environment:

```text
VITE_API_URL=/api/v1
```

That means the browser calls the same origin that served the frontend, then the local dev server or reverse proxy forwards `/api` to the backend.

| Environment | Browser calls | Proxy that forwards to backend | Backend target |
| --- | --- | --- | --- |
| Local Docker full stack | `http://localhost:5174/api/v1/...` | frontend nginx container | `http://backend:8000` |
| Local frontend dev + Docker backend | `http://localhost:5174/api/v1/...` | Vite dev proxy | `API_PROXY_TARGET=http://localhost:8001` |
| Local frontend dev + `go run .` backend | `http://localhost:5174/api/v1/...` | Vite dev proxy | `API_PROXY_TARGET=http://localhost:8000` |
| SIT/UAT/Prod same-origin | `https://<env-domain>/api/v1/...` | public nginx/load balancer/reverse proxy | backend service port `8000` |

With this setup, SIT/UAT/Prod do not need a frontend rebuild just to change the backend host. Keep `VITE_API_URL=/api/v1`; change only the environment/proxy/backend settings.

Postgres:

- Container: `payroll-postgres`
- Host inside Docker network: `postgres`
- Port inside Docker network: `5432`
- Host port: not exposed by default
- Database: `payrolldb`
- Volume: `payroll_postgres_data`

Backend `.env`:

```text
PGUSER=postgres
PGPASSWORD=<choose-a-local-postgres-password>
PGHOST=postgres
PGPORT=5432
PGDATABASE=payrolldb
PGSSLMODE=disable
DB_TIMEZONE=Asia/Bangkok
JWT_SECRET=<choose-a-long-random-secret>
FRONTEND_ORIGIN=http://localhost:5174
APP_ENV=development
COOKIE_SECURE=false
COOKIE_SAMESITE=lax
SEED_DEFAULT_PASSWORD=<optional-shared-demo-password>
SEED_USE_EMAIL_PASSWORDS=true
```

For SIT/UAT/Prod with same-origin deployment, build the frontend with `VITE_API_URL=/api/v1` and set `FRONTEND_ORIGIN` to the public frontend origin, for example `https://uat.example.com`.
For HTTPS production, use `APP_ENV=production`, `COOKIE_SECURE=true`, and `COOKIE_SAMESITE=none`. For same-origin HTTP local development, use `COOKIE_SECURE=false` and `COOKIE_SAMESITE=lax`.
Docker Compose reads `PGPASSWORD`, `JWT_SECRET`, and optional runtime values from your environment or local `.env`; secrets are required and must not be committed.
Seeded demo users use their email address as the password by default. Set `SEED_USE_EMAIL_PASSWORDS=false` with `SEED_DEFAULT_PASSWORD` only when you want one shared seed password.

Common commands:

```bash
docker compose build
docker compose ps
docker compose logs -f backend
docker compose logs -f frontend
docker compose logs -f postgres
docker compose stop
docker compose down
docker compose down -v
```

Run the backend in Docker:

```bash
docker compose up -d backend
```

Run database migrations:

```bash
go run ./migrate
docker compose --profile tools run --rm migrate
```

Seed local data:

```bash
go run ./seed
docker compose --profile tools run --rm seed
```

When running `go run .`, `go run ./migrate`, or `go run ./seed` directly from the host, override `PGHOST` with a host-reachable database address. The default Docker value `PGHOST=postgres` only works inside the Compose network.

If Docker fails with `dockerDesktopLinuxEngine` pipe errors, start Docker Desktop and retry.
