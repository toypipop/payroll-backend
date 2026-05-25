# Docker

This repo uses Docker only for local Postgres. Run from the backend root:

```bash
docker compose up -d
```

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
docker compose logs -f postgres
docker compose stop
docker compose down
docker compose down -v
```

With Postgres running:

```bash
go run .
```

If Docker fails with `dockerDesktopLinuxEngine` pipe errors, start Docker Desktop and retry.
