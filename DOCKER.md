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
- Host: `localhost`
- Port: `5432`
- Database: `payrolldb`
- Volume: `payroll_postgres_data`

Backend `.env`:

```text
DB_USER=postgres
DB_PASSWORD=1234
DB_HOST=127.0.0.1
DB_PORT=5432
DB_NAME=payrolldb
DB_SSLMODE=disable
DB_TIMEZONE=Asia/Bangkok
```

Common commands:

```bash
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

Seed local data:

```bash
go run ./seed
```

If Docker fails with `dockerDesktopLinuxEngine` pipe errors, start Docker Desktop and retry.
