---
name: run-payroll-backend
description: Use when asked to run, start, build, test, screenshot, or verify the payroll backend API. Also use when debugging backend endpoints, checking if the server starts, or confirming API changes work.
---

# run-payroll-backend

Go 1.25.5 + Gin API. Driven via PowerShell smoke script (`smoke.ps1`) that builds, launches, probes, and stops the binary in one shot. Postgres must be reachable on `127.0.0.1:5432`.

## Prerequisites

- Go 1.25.5 (`go version`)
- Postgres on `127.0.0.1:5432` — start with Docker if not running:
  ```powershell
  docker compose -f docker-compose.local.yml up -d postgres
  ```
- `.env` already present at repo root (committed example, local file is gitignored)

## Build

```powershell
go build -o tmp\backend-test.exe .
```

Build takes ~2s on this machine. Output in `tmp\` (gitignored).

## Run — agent path (smoke script)

```powershell
cd payroll-backend
powershell.exe -ExecutionPolicy Bypass -File .claude\skills\run-payroll-backend\smoke.ps1
```

Script: builds fresh binary → launches → waits for `/healthz` → runs probes → stops. Exits 0 on pass, non-zero on failure.

Verified output (2026-06-09):
```
  OK  build
  OK  startup
  OK  /healthz (200)
  OK  GET /modules (200)    ← 42 items
  OK  GET /auth/me (no cookie) (401)
  OK  POST /auth/login (200)
  OK  GET /auth/me (cookie) (200)
  OK  GET /employees (200)  ← 8 seeded employees
  OK  POST /auth/logout (200)
=== ALL PASSED ===
```

## Run — human / long-running path

```powershell
# Terminal 1: Postgres (if not already up)
docker compose -f docker-compose.local.yml up -d postgres

# Terminal 2: backend
cd payroll-backend
go run .
```

Server starts on `:8000`. Stop with Ctrl-C.

## Run — full Docker stack (local)

```powershell
cd payroll-backend
docker compose -f docker-compose.local.yml up -d
```

Ports: backend `8001`, frontend `5174`, postgres `5432`. Backend exposed on `8001` (not `8000`) when Dockerised.

After a volume reset, run migrate and seed before expecting `/modules` to work:
```powershell
docker compose -f docker-compose.local.yml --profile tools run --rm migrate
docker compose -f docker-compose.local.yml --profile tools run --rm seed
```

## Seeded credentials

| Role | Email | Password |
|------|-------|----------|
| SYSTEM_ADMIN | admin@admin.com | admin@admin.com |
| SYSTEM_STAFF | staff@admin.com | staff@admin.com |
| Manager (TechA) | alice@techa.com | alice@techa.com |

Password = email (seeded via `seed/main.go`).

## Auth model

httpOnly cookie JWT. Never add `Authorization` headers. Login sets the cookie; pass `-WebSession` (PowerShell) or `-c`/`-b` (curl) to carry it.

```powershell
$body = '{"email":"admin@admin.com","password":"admin@admin.com"}'
Invoke-WebRequest -Uri http://localhost:8000/api/v1/auth/login -Method POST `
  -Body $body -ContentType "application/json" -SessionVariable "sess"
Invoke-WebRequest -Uri http://localhost:8000/api/v1/auth/me -WebSession $sess
```

## Gotchas

- **`backend.exe` in repo root uses MySQL (stale build).** Always rebuild with `go build -o tmp\backend-test.exe .` — do not use the committed binary.
- **Docker Desktop must be running** before any `docker compose` command. Start it first; wait ~5s for the daemon.
- **Docker backend port is 8001, not 8000.** The local compose file maps `8001:8000`. `go run .` uses `8000` directly.
- **`npm` is a `.cmd` file on Windows** — `Start-Process "npm"` fails. Use `bash` or `cmd /c npm`.
- **`go run .` stderr shows migration skips** — these are informational, not errors.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `dial tcp 127.0.0.1:3306` | Using old `backend.exe` (MySQL build). Rebuild: `go build -o tmp\backend-test.exe .` |
| `dial tcp 127.0.0.1:5432: refused` | Postgres not running. `docker compose -f docker-compose.local.yml up -d postgres` |
| `/api/v1/modules` returns empty array | DB volume was reset; run migrate + seed tools |
| `{"error":"Invalid email"}` on login | Wrong email. Use `admin@admin.com`, not `admin@company.com` |
