# Payroll Backend

Requires Go 1.27.2 (see `go.mod`) and Docker with Compose v2. The local database uses PostgreSQL 17.

## Install and run

```powershell
# Run from payroll-backend. Copy the template only on first setup.
Copy-Item .env.local.example .env
# Replace PGPASSWORD and JWT_SECRET in .env; keep PGHOST=127.0.0.1.
docker compose -f docker-compose.local.yml build --pull postgres
docker compose -f docker-compose.local.yml up -d postgres
go mod download
go run ./migrate
go run ./seed --mock
go run .
```

API: `http://localhost:8000/api/v1`. Mock seed is for local/UAT data only. Never commit `.env`.

## Other commands

```powershell
go test ./...
go vet ./...
go build ./...
```

On Linux/macOS, use `cp .env.local.example .env`.
