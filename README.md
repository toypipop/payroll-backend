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
