# payroll-backend — Codex instructions

## Project at a glance

- Go 1.25+ REST API using Gin, GORM, and PostgreSQL.
- API base path: `/api/v1`; backend port: `8000`; default frontend origin: `http://localhost:5174`.
- Configuration is loaded from `.env` with `godotenv`. Never hardcode or commit secrets.
- Seeded demo accounts use the email address as their password.
- When a change affects routes, modules, auth responses, permissions, seed data, or JSON shapes, inspect `../payroll-frontend/src` too.

## Architecture — required boundaries

Use the handler → service → repository structure. The `employees` domain is the reference pattern.

- Handlers bind input, call services, and write responses only. Use `handlers/httpx` helpers: `RequestScope`, `ParseID`, `BindJSON`, and `RespondError`.
- Handlers must not use `config.DB`; never expose `err.Error()` to clients.
- Services own business rules and depend on repository interfaces, never Gin or `config.DB`.
- Repositories implement persistence and must not import Gin.
- `wire.go` is the composition root: construct repositories, services, handlers, and middleware there. `SetupRouter` only wires routes.
- Create domain errors with `apperr`; `httpx.RespondError` is the only error-to-HTTP-status mapper. PostgreSQL unique violations must become `apperr.Conflict` (409).
- `companies_id` is immutable after creation. For nullable updates, use `repository.ClearField` rather than leaking GORM types into services.
- Tables are singular. Qualify scoped columns, for example `time_log.companies_id`.

## Auth, permission, and scoping

- Authentication is an httpOnly `auth_token` cookie; never return JWTs in response bodies.
- Protect routes with `Guard.Auth` and `CheckPermission(moduleKey, action)`.
- Resolve access through `httpx.RequestScope(c)` and `repository.AccessScope`.
- For a non-employee table, use `scope.ApplyCompany(companiesCol)`; for a table with a real employee column, use `scope.ApplyEmployee(companiesCol, employeeCol)`. Do not restore deprecated generic scope helpers.
- An empty authorized-company list must deny all data; never remove the fail-closed filter.
- A module capability in `models/module_capability.go` is the source of truth for scope level and allowed actions. It must match the route permission action and repository scope method. Category modules are headings, not pages or permission rows.
- Cross-page dropdowns need a minimal, ungated, company-scoped `GET .../options` endpoint; do not weaken CRUD permissions. Sensitive models need purpose-built endpoints instead.
- `/api/v1/modules` is public and drives the frontend UI. Module keys and icons must match frontend definitions.

## Payroll and salary invariants

- Salary history is date-effective: `is_active` means switched on, not "current". The current record is the latest effective switched-on record.
- Salary audit entries are append-only; every salary write records exactly one required-reason audit event in the same transaction.
- Salary saves, moves, and deletes lock company payroll cycles in their transaction. Do not modify salary data that a closed cycle used.
- Salary updates use optimistic locking: stale versions return `apperr.Precondition` / HTTP 412.
- A payroll run is one transaction and locks the payroll cycle before calculating. Fail on the first error; do not partially commit a run.
- Payroll history must price a cycle from the salary effective at that cycle, not today’s salary. Closed-cycle reports use `PayrollTransaction` snapshot fields, not current employee relations.
- Lateness is capped per day by the daily absence rate, not capped once for an entire cycle.
- Employee company access grants extra app access; it is not payroll ownership. Validate employee/company relationships for employee-linked APIs.

## Time handling

- Business timezone is always `Asia/Bangkok` via `timeutil`.
- For wall-clock values and business-date operations, use `timeutil.BangkokLocation`, `ToBangkok`, `DayStart`, `DayEnd`, `DateKey`, and `ParseDayStart` as applicable.
- Do not use ambient timezone, bare `UTC`, `time.Parse` for business dates, or `Truncate(24 * time.Hour)` for a Bangkok calendar day.
- Calendar-date start values are Bangkok midnight; period end values are Bangkok 23:59:59. Compare dates by Bangkok day boundaries.

## Development and verification

Run commands from `payroll-backend`:

```bash
go run .
go run ./migrate
go run ./seed
go vet ./...
go build ./...
go test ./...
```

- Run `gofmt -w` only on files or directories you changed; formatting the whole tree rewrites CRLF line endings.
- Run `go vet ./...` and `go build ./...` after edits. Also run `go test ./...` after touching `service/`.
- Local database setup: start PostgreSQL with `docker compose -f docker-compose.local.yml up -d postgres`, then migrate and seed. Host-run Go uses `PGHOST=127.0.0.1`; containers use `PGHOST=postgres`.
- After a database volume reset, migrate and seed before checking `/api/v1/modules`.
- To smoke-test a running server, verify: `/healthz`, `/api/v1/modules`, login, `/auth/me`, and `/employees`. Use an admin login of `admin@admin.com` / `admin@admin.com`.
- If port 8000 is already used, do not terminate an unknown process; identify it and ask before stopping it.

## Docker and environments

- Local full stack: `docker compose -f docker-compose.local.yml up -d --build`.
- Local frontend and backend are exposed at `http://localhost:5174` and `http://localhost:8000/api/v1`.
- Keep `VITE_API_URL=/api/v1` in every environment; change the proxy/backend target rather than rebuilding solely to change the API host.
- `.env` is local runtime configuration. `PGPASSWORD` must match the password used when the Postgres volume was initialized.
- `docker compose ... down -v` deletes database data. Never run it unless the user explicitly requests a reset or the task clearly requires one.
- UAT/prod uses `docker-compose.uat.yml`; it can delete real environment data with `down -v`. Treat all production-like Docker actions as externally impactful and seek confirmation when needed.

## Delivery and deployment

- Keep changes small and domain-scoped; preserve route ordering in `routes/routes.go`.
- CI enforces formatting, vet, build, tests, and layering import rules.
- Backend deployment is manual through the backend repository workflow and deploys both backend and frontend from the same branch. Do not deploy unless explicitly asked.

## Engineering standards for every change

- Refactor within the touched domain: keep handler, service, repository, and model responsibilities clear; reuse existing abstractions; and remove newly introduced duplication. Do not combine an unrelated rewrite with a feature or fix.
- Preserve data and transaction integrity. Validate API input before use, keep dependent multi-record writes atomic, use transactions and locks where consistency requires them, make retryable mutations idempotent where the contract requires it, and return explicit concurrency conflicts.
- Treat security as part of the feature: enforce authentication, authorization, company scoping, and ownership server-side; validate untrusted input; use parameterized GORM queries; and never expose or log credentials, tokens, cookies, or sensitive personal data.
- Avoid unnecessary database, API, and network work. Scope and paginate reads, prevent N+1 loading and duplicate queries, select only needed relations or columns, and avoid broad cache or state invalidation when a targeted update is sufficient.
- Add useful observability without noise. Log actionable failures with safe request and domain context at an appropriate severity, preserve error wrapping for traceability, and avoid logging secrets or excessive success-path detail.
- Apply these standards to the active product work, including System Permissions, leave-type annual limits, permission-save refresh behavior, and paired date inputs.
