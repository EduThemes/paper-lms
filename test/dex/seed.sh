#!/usr/bin/env bash
# Phase 10-A.7 — register Dex as an OIDC provider in Paper LMS.
#
# Requires:
#   - Paper LMS backend running at http://localhost:3000
#   - Dex running at http://localhost:5556 (make dex-up)
#   - An admin session cookie in $PAPER_ADMIN_COOKIE, e.g.:
#       PAPER_ADMIN_COOKIE=$(curl -sc - -X POST http://localhost:3000/api/v1/login \
#         -H 'Content-Type: application/json' \
#         -d '{"login_id":"admin@paper.test","password":"paperpaper"}' \
#         | awk '/paper_session/ {print $NF}')
#
# Idempotent: the Create call will 400 if the provider already exists
# for the account; you can ignore that, or DELETE the existing row first.

set -euo pipefail

API="${API:-http://localhost:3000/api/v1}"
ACCOUNT_ID="${ACCOUNT_ID:-1}"
COOKIE="${PAPER_ADMIN_COOKIE:?set PAPER_ADMIN_COOKIE to an admin session token}"

curl -fsS -X POST "$API/accounts/$ACCOUNT_ID/authentication_providers" \
  -H "Content-Type: application/json" \
  -H "Cookie: paper_session=$COOKIE" \
  --data-binary @- <<'JSON'
{
  "auth_type": "oidc",
  "position": 100,
  "oidc_preset": "generic",
  "oidc_issuer_url": "http://localhost:5556/dex",
  "oidc_client_id": "paper-lms-dev",
  "oidc_client_secret": "dev-secret-not-prod",
  "oidc_scopes": ["openid", "email", "profile"],
  "auto_provision": true
}
JSON

echo
echo "Dex provider registered. Visit /login → 'Sign in with Generic OIDC' to test."
