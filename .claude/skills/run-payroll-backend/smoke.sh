#!/usr/bin/env bash
# Smoke test driver for payroll-backend.
# Usage: bash .claude/skills/run-payroll-backend/smoke.sh [BASE_URL]
# Default BASE_URL: http://localhost:8000
#
# Starts nothing — assumes the server is already running.
# Runs: healthz, modules, login, auth/me, employees list.
# Exits 0 on success, 1 on any failure.

set -euo pipefail

BASE="${1:-http://localhost:8000}"
COOKIE_JAR="$(mktemp /tmp/payroll-cookies-XXXX.txt)"
PASS=0
FAIL=0

check() {
  local label="$1" url="$2" expected="$3"
  local body
  body=$(curl -s --max-time 10 -b "$COOKIE_JAR" "$url")
  if echo "$body" | grep -q "$expected"; then
    echo "[PASS] $label"
    PASS=$((PASS+1))
  else
    echo "[FAIL] $label — expected to find '$expected' in:"
    echo "       $body" | head -c 300
    FAIL=$((FAIL+1))
  fi
}

post_check() {
  local label="$1" url="$2" data="$3" expected="$4"
  local body
  body=$(curl -s --max-time 10 -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -X POST -H "Content-Type: application/json" -d "$data" "$url")
  if echo "$body" | grep -q "$expected"; then
    echo "[PASS] $label"
    PASS=$((PASS+1))
  else
    echo "[FAIL] $label — expected to find '$expected' in:"
    echo "       $body" | head -c 300
    FAIL=$((FAIL+1))
  fi
}

echo "Smoke testing $BASE"
echo "---"

check   "healthz"         "$BASE/healthz"              '"status":"ok"'
check   "modules (public)" "$BASE/api/v1/modules"      '"module_name"'
post_check "login (admin)" "$BASE/api/v1/auth/login"   '{"email":"admin@admin.com","password":"admin@admin.com"}' '"full_name"'
check   "auth/me"         "$BASE/api/v1/auth/me"        '"full_name"'
check   "employees list"  "$BASE/api/v1/employees"      '"employees_code"'

echo "---"
echo "Results: $PASS passed, $FAIL failed"
rm -f "$COOKIE_JAR"
[ "$FAIL" -eq 0 ]
