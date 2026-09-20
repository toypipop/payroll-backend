# Payroll Backend

Go + Gin + PostgreSQL API. It runs on `http://localhost:8000` and serves `/api/v1`.

## Setup

```powershell
Copy-Item .env.local.example .env
docker compose -f docker-compose.local.yml up -d postgres
go run ./migrate
go run ./seed
go run .
```

Update `.env` with local database credentials and a long random `JWT_SECRET` before starting. The frontend defaults to this API through its `/api` proxy.

## Check

```powershell
go vet ./...
go build ./...
go test ./...
```

`.env` is ignored by Git. Copy `.env.local.example` for local work. For UAT deployment, copy `.env.uat.example` to `.env` on the host and replace every placeholder, preferably injecting secrets from the deployment system. Never commit real credentials or environment values. `go run ./seed --reset` deletes all database rows and is only for a local non-production database.

## Database connection pool

`ConnectDatabase()` configures the shared SQL pool for API, migration, and seed processes.
The defaults are 20 open connections, 5 idle connections, a 30-minute maximum lifetime,
and a 5-minute idle timeout. Override them with `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`,
`DB_CONN_MAX_LIFETIME`, and `DB_CONN_MAX_IDLE_TIME` (see the environment templates).
Open connections must be at least 2; idle connections must be between 0 and max open;
durations must be positive Go durations such as `30m`. Invalid configuration stops startup.

The API admits at most `floor(max_open / 2)` requests simultaneously because company
access middleware holds one connection while domain work may use another. All API
routes share this budget, including login and company status updates. Waiting requests
hold no connections and receive HTTP 503 with `Retry-After: 1` after 5 seconds.
`/healthz` bypasses this queue. Client cancellation also stops waiting. The slot is
released only after the request finishes, including rollback of company locks.

These are starting limits, not measured optimal values. Budget connections across all
API instances and concurrent migration/seed processes, leaving capacity for PostgreSQL
administration and other clients. Restart the process after changing pool settings.

## Payroll transaction indexes

The existing unique `idx_txn_cycle_employee` covers `(deleted_at, cycle_id, employees_id)`
and supports live cycle queries and snapshot upserts. My Payslips searches one employee
across cycles, so `idx_txn_employee_live` adds `(employees_id, id DESC) WHERE deleted_at = 0`.
It supports that employee filter and newest-first ordering without changing payroll values
or uniqueness rules. PostgreSQL still chooses the execution plan based on data statistics.

Apply schema changes through the normal `go run ./migrate` workflow; restarting the API
alone does not run AutoMigrate. This index uses the existing GORM schema migration path,
which builds a regular index and can block writes while it is created on a large table.
The source change itself does not apply the migration to a running database.
