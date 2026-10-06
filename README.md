# Payroll Backend

Go + Gin API at `http://localhost:8000/api/v1`.

## Install and run

```powershell
Copy-Item .env.local.example .env
# Set PGPASSWORD and JWT_SECRET in .env
docker compose -f docker-compose.local.yml up -d postgres
go mod download
go install github.com/air-verse/air@latest
go run ./migrate
go run ./seed --mock --reset
air
```

The local seed resets the local database and creates mock data. Air reloads the API when Go files change. Never commit `.env`.
