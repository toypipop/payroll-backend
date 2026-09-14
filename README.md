# Payroll Backend

Go + Gin + PostgreSQL API. It runs on `http://localhost:8000` and serves `/api/v1`.

## Setup

```powershell
Copy-Item .env.example .env
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

`.env` is ignored by Git. Copy `.env.example` after every fresh clone; never commit real credentials or environment values. `go run ./seed --reset` deletes all database rows and is only for a local non-production database.
