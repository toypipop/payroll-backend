-- Reset the local database: drop everything, then re-run migrate + seed.
--
-- This wipes ALL data and schema objects in the current database. It does NOT
-- recreate tables or seed rows — those come from the Go migrate/seed steps.
--
-- Run (from payroll-backend root, Postgres must be up):
--   psql "postgresql://postgres:postgres@127.0.0.1:5432/payrolldb" -f scripts/reset-db.sql
--   go run ./migrate
--   go run ./seed
--
-- Or against the Docker container:
--   docker exec -i payroll-postgres-local psql -U postgres -d payrolldb < scripts/reset-db.sql

DROP SCHEMA public CASCADE;
CREATE SCHEMA public;

-- Restore default privileges GORM/AutoMigrate expects.
GRANT ALL ON SCHEMA public TO public;
GRANT ALL ON SCHEMA public TO postgres;
