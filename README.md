# Payroll Backend

Go + Gin + PostgreSQL API. It runs on `http://localhost:8000` and serves `/api/v1`.

## Setup

```powershell
Copy-Item .env.local.example .env
docker compose -f docker-compose.local.yml up -d postgres
go run ./migrate
go run ./seed --mock --reset
go run .
```

Update `.env` with local database credentials and a long random `JWT_SECRET` before starting. The frontend defaults to this API through its `/api` proxy.

## Check

```powershell
go vet ./...
go build ./...
go test ./...
```

`.env` is ignored by Git. Copy `.env.local.example` for local work. For UAT deployment, copy `.env.uat.example` to `.env` on the host and replace every placeholder, preferably injecting secrets from the deployment system. Never commit real credentials or environment values.

## Seeding

- `go run ./seed` is the production-safe core seed. It never resets the database, creates only system/reference setup, and creates the first system administrator from `BOOTSTRAP_ADMIN_EMAIL` and `BOOTSTRAP_ADMIN_PASSWORD`. It will not overwrite an existing administrator.
- `go run ./seed --mock --reset` resets the database and then creates the core setup plus disposable fixture users and payroll data. Use it for Dev and UAT only. It refuses to run when `APP_ENV` or `GO_ENV` is `production`.
- `--reset` is rejected unless `--mock` is also present.

The local Compose `seed` tool service runs the mock/reset command. UAT has a separate `seed-mock` service (`docker compose -f docker-compose.uat.yml --profile mock-tools run --rm seed-mock`); production uses the `seed` service, which invokes `/app/seed` without flags. Provide the two `BOOTSTRAP_ADMIN_*` secrets only for the initial production run.

## List pagination

Management lists return `{ "items": [], "pagination": { "page": 1,
"page_size": 25, "total": 0, "total_pages": 1 } }`. Supply `page`, `page_size`
(1–100), `search`, `sort`, and `order` (`asc` or `desc`). Sorting accepts only
repository-defined fields; counting, searching and paging retain the route's
authorization scope. Pages past the end are clamped to the last page.

This covers employee/salary, attendance, leave, payroll/adjustment, master-data,
company, configuration, employee-company access and audit management tables.
`/companies/all` accepts `is_active`; `/payroll/cycles` accepts `is_closed`.
Dropdown `/options` sources, reports and individual salary histories keep their
complete responses. `/salaries/options` is a complete active-employee target source
loaded when the bulk salary picker opens and requires `SalariesStructurePage` VIEW
permission plus the existing employee/company scope. Deploy the matching frontend
and backend together because list response envelopes have changed.

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
