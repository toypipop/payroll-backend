# Docker

This repo runs the local full stack with Docker: Postgres, the Go API, and the Vite frontend served by nginx. Run from the backend root:

```bash
docker compose up -d --build
```

Apps:

- Frontend: `http://localhost:5174`
- Backend API direct port: `http://localhost:8001/api/v1`
- Frontend nginx proxies `/api` to the backend container.

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
PGHOST=127.0.0.1
PGPORT=5432
PGDATABASE=payrolldb
PGSSLMODE=disable
DB_TIMEZONE=Asia/Bangkok
JWT_SECRET=<choose-a-long-random-secret>
SEED_DEFAULT_PASSWORD=<optional-local-demo-password>
SEED_USE_EMAIL_PASSWORDS=false
```

Docker Compose reads `PGPASSWORD` and `JWT_SECRET` from your environment or local `.env`; both are required.

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

Run the backend locally against Docker Postgres:

```bash
go run .
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

If Docker fails with `dockerDesktopLinuxEngine` pipe errors, start Docker Desktop and retry.
