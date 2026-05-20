# Docker

This repo uses Docker only for local MySQL. Run from the backend root:

```bash
docker compose up -d
```

MySQL:

- Container: `payroll-mysql`
- Host: `localhost`
- Port: `3306`
- Database: `PayrollDB`
- Volume: `payroll_mysql_data`

Backend `.env`:

```text
DB_USER=root
DB_PASSWORD=1234
DB_HOST=127.0.0.1
DB_PORT=3306
DB_NAME=PayrollDB
```

Common commands:

```bash
docker compose ps
docker compose logs -f mysql
docker compose stop
docker compose down
docker compose down -v
```

With MySQL running:

```bash
go run .
```

If Docker fails with `dockerDesktopLinuxEngine` pipe errors, start Docker Desktop and retry.
