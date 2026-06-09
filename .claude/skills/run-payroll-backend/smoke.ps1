#!/usr/bin/env powershell
# Smoke-test the payroll backend.
# Usage: pwsh smoke.ps1 [--port 8000]
# Requires: Postgres running on 127.0.0.1:5432 (or via docker compose)
# Run from: payroll-backend/

param([int]$Port = 8000)

$base = "http://localhost:$Port"
$errors = 0

function Check($label, $status, $expected) {
  if ($status -eq $expected) { Write-Host "  OK  $label ($status)" }
  else { Write-Host "  FAIL $label (got $status, want $expected)"; $script:errors++ }
}

Write-Host "`n=== payroll-backend smoke ($base) ===`n"

# 1. Build fresh binary
Write-Host "[1] Building..."
go build -o tmp\backend-smoke.exe . 2>&1 | Out-Null
if ($LASTEXITCODE -ne 0) { Write-Host "Build FAILED"; exit 1 }
Write-Host "  OK  build"

# 2. Start
$proc = Start-Process -FilePath ".\tmp\backend-smoke.exe" -WorkingDirectory $PWD -PassThru -WindowStyle Hidden
$ready = $false
for ($i = 0; $i -lt 15; $i++) {
  Start-Sleep -Seconds 1
  try { Invoke-WebRequest -Uri "$base/healthz" -UseBasicParsing -TimeoutSec 1 | Out-Null; $ready = $true; break } catch {}
}
if (-not $ready) { Write-Host "Backend never became ready"; Stop-Process -Id $proc.Id -Force; exit 1 }
Write-Host "  OK  startup"

# 3. Probes
try { $r = Invoke-WebRequest -Uri "$base/healthz" -UseBasicParsing; Check "/healthz" $r.StatusCode 200 } catch { $errors++ }
try { $r = Invoke-WebRequest -Uri "$base/api/v1/modules" -UseBasicParsing; Check "GET /modules" $r.StatusCode 200 } catch { $errors++ }
try { Invoke-WebRequest -Uri "$base/api/v1/auth/me" -UseBasicParsing | Out-Null; $errors++ }
catch { Check "GET /auth/me (no cookie)" $_.Exception.Response.StatusCode.value__ 401 }

# 4. Auth flow
$body = '{"email":"admin@admin.com","password":"admin@admin.com"}'
$l = Invoke-WebRequest -Uri "$base/api/v1/auth/login" -Method POST -Body $body -ContentType "application/json" -UseBasicParsing -SessionVariable "sess"
Check "POST /auth/login" $l.StatusCode 200
$me = Invoke-WebRequest -Uri "$base/api/v1/auth/me" -UseBasicParsing -WebSession $sess
Check "GET /auth/me (cookie)" $me.StatusCode 200
$emp = Invoke-WebRequest -Uri "$base/api/v1/employees" -UseBasicParsing -WebSession $sess
Check "GET /employees" $emp.StatusCode 200
$lo = Invoke-WebRequest -Uri "$base/api/v1/auth/logout" -Method POST -UseBasicParsing -WebSession $sess
Check "POST /auth/logout" $lo.StatusCode 200

# 5. Stop
Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue

Write-Host "`n=== $( if ($errors -eq 0) { 'ALL PASSED' } else { "$errors FAILED" } ) ===`n"
exit $errors
