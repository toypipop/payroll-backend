# Payroll Backend

## Purpose and Scope

Go REST API for company-scoped employees, attendance, salary history, payroll cycles, and reports. It owns business validation, authorization, payroll calculations and PostgreSQL persistence. The companion [frontend](../payroll-frontend/README.md) provides the UI; [infra](../infra/README.md) owns edge routing.

## Requirements and Versions

- Go 1.27.2 is declared in [go.mod](go.mod) and pinned in the [Docker build](Dockerfile). Dependencies include Gin, GORM and the PostgreSQL driver; go.mod/go.sum record module versions.
- Docker with Compose v2-compatible commands is required for the documented local database setup. The repository does not pin the host Docker/Compose version.
- Local PostgreSQL 17 is built through [Dockerfile.postgres](Dockerfile.postgres) and [docker-compose.local.yml](docker-compose.local.yml).

## Installation and Configuration

Run from payroll-backend in PowerShell. Copy the template only on first setup:

~~~powershell
Copy-Item .env.local.example .env
# Replace PGPASSWORD and JWT_SECRET in .env; keep PGHOST=127.0.0.1.
docker compose -f docker-compose.local.yml build --pull postgres
docker compose -f docker-compose.local.yml up -d postgres
go mod download
~~~

The committed [.env.local.example](.env.local.example) documents PostgreSQL connection/pool settings, JWT_SECRET, PORT, frontend/CORS origins, cookie policy, trusted proxies and bootstrap administrator variables. Never commit .env. PGPASSWORD must match the password used to initialize the existing database volume. On Linux/macOS use cp instead of Copy-Item.

Initialize a disposable local database only after it is ready. These commands change schema/data; mock fixtures are for local/UAT use only:

~~~powershell
go run ./migrate
go run ./seed --mock
~~~

The production-safe core seed uses bootstrap administrator configuration; do not substitute mock/reset operations against real data.

## How to Run

~~~powershell
go run .
~~~

Default API: http://localhost:8000/api/v1. Check GET http://localhost:8000/healthz and GET http://localhost:8000/api/v1/modules, then verify login with local fixture credentials. Startup also runs registered repair migrations and synchronizes module capabilities; starting against a database can change its state.

For the local container stack (including the sibling frontend build context):

~~~powershell
docker compose -f docker-compose.local.yml up -d --build
~~~

This starts/recreates services; schema/seed preparation remains a separate step. Frontend: http://localhost:5174. The Compose seed tool is configured with --mock --reset; do not invoke it against data that must be preserved. UAT/prod uses docker-compose.uat.yml; deployment is manual and outside local setup.

## Core Business Logic

- Cookie sessions authenticate requests; module capabilities, company scope and employee ownership govern access. Extra employee company access does not transfer payroll ownership. Empty authorized scope denies data.
- Attendance, leave, salary and cycle adjustments feed payroll. Business dates and times use Asia/Bangkok through timeutil.
- Salary history is date-effective: the latest effective enabled record applies. Every salary mutation requires a reason and an append-only audit in the same transaction. Optimistic versions reject stale updates with HTTP 412; cycle locks protect salary data used by closed payroll.
- A payroll run locks its cycle and persists all employee results atomically. Calculations use the salary effective for the cycle, with lateness deductions capped per day by the daily absence rate. Unpaid leave, late and absent deductions reduce the tax and SSO bases. The Deduction/Earning Summary pages reuse the run's attendance functions, so they match payroll. Each transaction stores `sso_wage_base` (wage base after SSO caps) for the SSO report; transactions from before it existed have none and the report falls back to `salaries_at_time`. The implementation is in [service/payroll_run_service.go](service/payroll_run_service.go).
- Closing recalculates and snapshots transactions atomically. Closed cycles are immutable; reports use stored payroll snapshots rather than current employee relations. Open cycles can require recalculation.

## Verification and Useful References

Run from this repository for application changes:

~~~powershell
go test ./...
go vet ./...
go build ./...
~~~

See [AGENTS.md](AGENTS.md) for architecture, scoped-query rules, migration safety and CI checks, and [service-layer reference](docs/service-layer-refactor.md) for existing architecture detail. Never delete database volumes as a routine setup or repair step.
