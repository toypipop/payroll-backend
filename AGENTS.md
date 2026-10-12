# payroll-backend — Codex instructions

## Project at a glance

- Go REST API (toolchain version in go.mod) using Gin, GORM, and PostgreSQL.
- API base path: `/api/v1`; backend port: `8000`; default frontend origin: `http://localhost:5174`.
- Configuration is loaded from `.env` with `godotenv`. Never hardcode or commit secrets.
- Seeded demo accounts use the email address as their password.
- When a change affects routes, modules, auth responses, permissions, seed data, or JSON shapes, inspect `../payroll-frontend/src` too.

## 1. File Organization and Responsibilities

- Keep installation, tool versions, and run commands in README.md. Keep pending discussions/tasks in the project's task file, not here.
- Models: models/ owns GORM entities, relationships, column definitions, and module capability metadata. Keep database schema constraints consistent with service validation.
- Business logic: service/ owns use cases, validation, domain rules, and repository interfaces. Shared validators include service/field_validation.go and service/salary_validation.go.
- Persistence: repository/ owns scoped queries and transaction/locking operations. Share helpers such as repository/access_scope.go rather than duplicating filters.
- HTTP presentation: handlers/ owns binding and response serialization; handlers/httpx/ owns shared HTTP helpers; routes/routes.go defines URLs, verbs, and guards; middleware/ owns authentication, permission, and CSRF checks.
- Composition/configuration: main.go and wire.go initialize and wire dependencies; config/ owns runtime configuration and database initialization.
- Database lifecycle: migrations/ owns migrations; migrate/ is the migration command; seed/ owns initial/mock data. Shared business time helpers live in timeutil/ and domain errors in apperr/.
- Go files use package-appropriate snake_case names; keep tests beside the code as *_test.go. Format only changed Go files with gofmt.
- Use the employees domain as the reference for handler/service/repository separation. Preserve route ordering and existing response envelopes when adding APIs.

### Architecture — required boundaries

Use the handler → service → repository structure. The `employees` domain is the reference pattern.

- Handlers bind input, call services, and write responses only. Use `handlers/httpx` helpers: `RequestScope`, `ParseID`, `BindJSON`, and `RespondError`.
- Handlers must not use `config.DB`; never expose `err.Error()` to clients.
- Services own business rules and depend on repository interfaces, never Gin or `config.DB`.
- Repositories implement persistence and must not import Gin.
- `wire.go` is the composition root: construct repositories, services, handlers, and middleware there. `SetupRouter` only wires routes.
- Create domain errors with `apperr`; `httpx.RespondError` is the only error-to-HTTP-status mapper. PostgreSQL unique violations must become `apperr.Conflict` (409).
- `companies_id` is immutable after creation. For nullable updates, use `repository.ClearField` rather than leaking GORM types into services.
- Tables are singular. Qualify scoped columns, for example `time_log.companies_id`.

## 2. Business Logic and API Contracts

### Auth, permission, and scoping

- Authentication is an httpOnly `auth_token` cookie; never return JWTs in response bodies.
- Protect routes with `Guard.Auth` and `CheckPermission(moduleKey, action)`.
- Resolve access through `httpx.RequestScope(c)` and `repository.AccessScope`.
- For a non-employee table, use `scope.ApplyCompany(companiesCol)`; for a table with a real employee column, use `scope.ApplyEmployee(companiesCol, employeeCol)`. Do not restore deprecated generic scope helpers.
- An empty authorized-company list must deny all data; never remove the fail-closed filter.
- A module capability in `models/module_capability.go` is the source of truth for scope level and allowed actions. It must match the route permission action and repository scope method. Category modules are headings, not pages or permission rows.
- Cross-page dropdowns need a minimal, ungated, company-scoped `GET .../options` endpoint; do not weaken CRUD permissions. Sensitive models need purpose-built endpoints instead.
- `/api/v1/modules` is public and drives the frontend UI. Module keys and icons must match frontend definitions.

### Payroll and salary invariants

- Salary history is date-effective: `is_active` means switched on, not "current". The current record is the latest effective switched-on record.
- Salary audit entries are append-only; every salary write records exactly one required-reason audit event in the same transaction.
- Salary saves, moves, and deletes lock company payroll cycles in their transaction. Do not modify salary data that a closed cycle used.
- Salary updates use optimistic locking: stale versions return `apperr.Precondition` / HTTP 412.
- A payroll run is one transaction and locks the payroll cycle before calculating. Fail on the first error; do not partially commit a run.
- Payroll history must price a cycle from the salary effective at that cycle, not today’s salary. Closed-cycle reports use `PayrollTransaction` snapshot fields, not current employee relations.
- Lateness is capped per day by the daily absence rate, not capped once for an entire cycle.
- Employee company access grants extra app access; it is not payroll ownership. Validate employee/company relationships for employee-linked APIs.

### Time handling

- Business timezone is always `Asia/Bangkok` via `timeutil`.
- For wall-clock values and business-date operations, use `timeutil.BangkokLocation`, `ToBangkok`, `DayStart`, `DayEnd`, `DateKey`, and `ParseDayStart` as applicable.
- Do not use ambient timezone, bare `UTC`, `time.Parse` for business dates, or `Truncate(24 * time.Hour)` for a Bangkok calendar day.
- Calendar-date start values are Bangkok midnight; period end values are Bangkok 23:59:59. Compare dates by Bangkok day boundaries.

## 3. Engineering Principles

Apply all five principles to every change, including System Permissions, leave-type annual limits, permission-save refresh behavior, and paired date inputs.

1. [Structure and Readability](#principle-1-structure-and-readability)
2. [Data Integrity, Validation, and Transactions](#principle-2-data-integrity-validation-and-transactions)
3. [Logging and HTTP Status](#principle-3-logging-and-http-status)
4. [Performance](#principle-4-performance)
5. [Security](#principle-5-security)

## Principle 1: Structure and Readability

- Prioritize readable code, explicit names, and small responsibilities. Use Go interfaces and composition for useful OOP boundaries; avoid unnecessary abstraction.
- Keep handler, service, repository, and model responsibilities separate. Reuse existing abstractions and remove newly introduced duplication within the touched domain; avoid unrelated rewrites.

## Principle 2: Data Integrity, Validation, and Transactions

- Keep frontend, service/API, and database constraints consistent for required fields, types, lengths, ranges, numeric precision, references, and business dates/times. The API must reject invalid input even when the UI already validates it.
- Reuse shared validators. Preserve meaningful zero values and distinguish omitted/null updates according to the API contract.
- Keep dependent multi-record writes atomic. Use ACID transactions, appropriate locks, and constraints; fail rather than commit partial results.
- Preserve optimistic concurrency and make retryable mutations idempotent where required. Return explicit conflicts instead of silently overwriting stale data.
- Review migrations for data preservation and registration in the relevant entrypoints. Do not truncate or reset real data without explicit authorization.

## Principle 3: Logging and HTTP Status

- Use apperr and httpx.RespondError for consistent errors. Preserve 400 invalid input, 401 authentication, 403 permission, 404 missing/scoped-out records, 409 conflict, 412 stale-version preconditions, and 500 internal failures according to the API contract.
- Do not leak record existence through unauthorized responses or return raw internal errors. Preserve error wrapping internally for traceability.
- Request logs should carry method, route template, status, duration, and request/correlation ID where supported. Use appropriate severity and avoid duplicate logs; do not treat every 4xx as a server error.
- Audit important mutations separately with actor, action, target, time, and reason as required. Keep salary audit writes in the same transaction as the mutation.
- Never log secrets, credentials, tokens, cookies, unnecessary personal data, or entire request/response bodies.

## Principle 4: Performance

- Scope and paginate reads, prevent N+1 loading and duplicate queries, and select only needed relations/columns.
- Inspect indexes and query plans when relevant; keep database work and transaction duration proportional to the operation.
- Avoid broad cache/state invalidation when a targeted update is sufficient. Do not trade correctness or authorization for speed.

## Principle 5: Security

- Enforce authentication, authorization, company scope, and ownership on every relevant endpoint. Never bypass scoped repositories or fail-closed filters.
- Use parameterized GORM queries and validate untrusted input/references. Inspect XSS, upload/export, and information disclosure as relevant.
- Preserve sid-based cookie sessions, revocation behavior, strict CORS, and X-Requested-With: XMLHttpRequest checks on mutations, including authenticated logout.
- Never expose or log credentials, tokens, cookies, or secrets. Trust forwarded IP/HTTPS only from configured proxy CIDRs.
- Apply least privilege to runtime/container/database access. Check dependency/image advisories and compatibility when updating versions.

## Verification and Delivery

- See README.md for setup/run commands. Run verification from payroll-backend: go vet ./..., go build ./..., and go test ./... for application code changes.
- Add/update meaningful unit tests for changed behavior. If tests fail, fix within approved scope and rerun until all relevant tests pass. Do not skip or weaken tests merely to pass, or declare completion while required checks are blocked.
- Documentation-only changes require checking documented paths, contracts, and commands; do not add artificial unit tests for prose.
- CI enforces gofmt, vet, build, tests, and layering import rules. Run gofmt -w only on changed files/directories; formatting the whole tree can rewrite CRLF endings.
- Smoke-test /healthz, /api/v1/modules, login, /auth/me, and /employees against a disposable local environment with seeded mock credentials.
- If port 8000 is occupied, identify the process and ask before stopping an unknown process.
- Keep changes domain-scoped. Backend deployment is manual, deploys backend and frontend from the same branch, and requires explicit user authorization.

## Docker and environments

- Local full stack: `docker compose -f docker-compose.local.yml up -d --build`.
- Local frontend and backend are exposed at `http://localhost:5174` and `http://localhost:8000/api/v1`.
- Keep `VITE_API_URL=/api/v1` in every environment; change the proxy/backend target rather than rebuilding solely to change the API host.
- `.env` is local runtime configuration. `PGPASSWORD` must match the password used when the Postgres volume was initialized.
- `docker compose ... down -v` deletes database data. Never run it unless the user explicitly requests a reset or the task clearly requires one.
- UAT/prod uses `docker-compose.uat.yml`; it can delete real environment data with `down -v`. Treat all production-like Docker actions as externally impactful and seek confirmation when needed.
