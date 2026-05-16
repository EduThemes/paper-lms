#!/usr/bin/env bash
# state_dpa_dry_run.sh — exercise the 5 questions a Kansas-style state
# DPA (Data Privacy Agreement) reviewer would ask. PASS = runtime
# behavior matches policy. FAIL = reviewer would reject the contract.
#
# Wave E verification artifact. Run before any state pilot kickoff.
#
# Each question has either an automated section (Go-test-driven) or a
# MANUAL: section with the curl/psql commands a human runs. The
# manual sections exist because exercising them fully requires a live
# server with a real Postgres + the audit-log trigger from migration
# 000053 + a populated test fixture set. The script's job is to make
# the walkthrough reproducible, not to substitute for the human
# reviewer's session.
#
# Exit code 0 if every automated check passes; non-zero on first
# failure (set -e).

set -euo pipefail

# Resolve repo root from the script's location so the script works
# from any CWD.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

pass() { printf '  \033[32mPASS\033[0m %s\n' "$1"; }
fail() { printf '  \033[31mFAIL\033[0m %s\n' "$1"; exit 1; }
note() { printf '  \033[33mNOTE\033[0m %s\n' "$1"; }

# run_test LOGFILE PATTERN PACKAGE... runs `go test -run` against
# the given packages and writes the log to LOGFILE. Returns:
#   0 = at least one selected test ran AND all passed
#   1 = go test exited non-zero (real failure)
#   2 = go test exited 0 but no test matched the pattern
run_test() {
  local logfile="$1"; shift
  local pattern="$1"; shift
  if ! go test -count=1 -run "$pattern" "$@" > "$logfile" 2>&1; then
    return 1
  fi
  # All "ok" lines with "[no tests to run]" = no real coverage.
  if grep -qE '^ok ' "$logfile" && \
     ! grep -E '^ok ' "$logfile" | grep -qv 'no tests to run'; then
    return 2
  fi
  return 0
}

# ---------------------------------------------------------------------------
# Q1: Show me a deletion that actually deletes.
# ---------------------------------------------------------------------------
# Policy: UserDeletionService (Wave C.1) walks dependent tables and
# nulls PII columns while preserving grade-bearing rows for academic
# record. FERPAService.ProcessDeletion wires it in.
#
# Tables touched (per the audit memo):
#   submissions             -> body, url, attachments nulled
#   submission_comments     -> comment nulled (preserve author_id)
#   conversation_messages   -> body = '[deleted]'
#   discussion_entries      -> message = '[deleted]'
#   attendance_records      -> notes nulled
#   notification_deliveries -> address, subject, body nulled
#
# NOT touched (audit trail per FERPA):
#   audit_logs, grade_change_logs, pii_access_logs,
#   data_export_requests, data_deletion_requests.

echo "Q1: Show me a deletion that actually deletes."
set +e
run_test /tmp/dpa_q1.log 'TestEraseDependents' ./internal/service/...
rc=$?
set -e
if [ $rc -eq 0 ]; then
  pass "UserDeletionService unit tests green (PII walk + grade preservation)"
elif [ $rc -eq 2 ]; then
  note "no test names matched the Q1 pattern (see /tmp/dpa_q1.log)"
  note "MANUAL fallback: from psql against a dev DB —"
  note "  INSERT INTO users (id, name, login_id, email) VALUES (9001, 'Test', 't', 't@x');"
  note "  INSERT INTO submissions (user_id, body) VALUES (9001, 'sensitive essay');"
  note "  INSERT INTO conversation_messages (author_id, body) VALUES (9001, 'sensitive DM');"
  note "  -- then run the CLI deletion task:"
  note "  go run ./cmd/server -task=ferpa-process-deletion -user=9001"
  note "  -- and verify:"
  note "  SELECT name FROM users WHERE id=9001;                  -- 'deleted_user_9001'"
  note "  SELECT body FROM submissions WHERE user_id=9001;       -- NULL"
  note "  SELECT body FROM conversation_messages WHERE author_id=9001; -- '[deleted]'"
else
  fail "Q1 go test errored (see /tmp/dpa_q1.log)"
fi

# ---------------------------------------------------------------------------
# Q2: Show me an export that downloads.
# ---------------------------------------------------------------------------
# Policy: DataExportRequest -> ProcessExport produces a signed ZIP at
# rest in object storage; GET /api/v1/data_exports/:id/download
# returns Content-Type: application/zip with a non-empty body. The
# 410 expired path is defense-in-depth.

echo "Q2: Show me an export that downloads."
set +e
run_test /tmp/dpa_q2.log 'TestDataExport|TestDownloadDataExport' ./internal/api/v1/handlers/...
rc=$?
set -e
if [ $rc -eq 0 ]; then
  pass "Data-export download handler tests green"
elif [ $rc -eq 2 ]; then
  note "no test names matched the Q2 pattern (see /tmp/dpa_q2.log)"
  note "MANUAL: against a running dev server with REVIEWER_TOKEN set —"
  note "  curl -X POST http://localhost:8080/api/v1/users/self/data_exports \\"
  note "    -H \"Authorization: Bearer \$REVIEWER_TOKEN\""
  note "  -- wait for the export job to complete (poll the request row), then:"
  note "  curl -sI http://localhost:8080/api/v1/data_exports/<id>/download \\"
  note "    -H \"Authorization: Bearer \$REVIEWER_TOKEN\""
  note "  -- expect Content-Type: application/zip, Content-Length > 0"
else
  fail "Q2 go test errored (see /tmp/dpa_q2.log)"
fi

# ---------------------------------------------------------------------------
# Q3: Show me a tamper-evident audit.
# ---------------------------------------------------------------------------
# Policy: migration 000053 installs a trigger that raises on
# UPDATE/DELETE of audit_logs. The trigger is the load-bearing
# anti-tamper control; without it, the audit log is just a table the
# DB admin can rewrite.

echo "Q3: Show me a tamper-evident audit."
note "MANUAL (requires psql against a migrated dev DB):"
note "  psql \"\$DATABASE_URL\" -c \\"
note "    \"UPDATE audit_logs SET payload='hacked' WHERE id = (SELECT id FROM audit_logs LIMIT 1);\""
note "  -- Expected output:"
note "  -- ERROR:  audit_log is append-only"
note "  -- CONTEXT:  PL/pgSQL function audit_logs_no_update_or_delete() ..."
note ""
note "  -- And the migration that installs the trigger:"
note "  psql \"\$DATABASE_URL\" -c \"\\\\d+ audit_logs\" | grep -i trigger"
# Verify the migration file exists in the chain.
if grep -rqs "audit_log is append-only" internal/db/migrations/ ; then
  pass "audit-log immutability trigger present in migration chain"
else
  fail "migration declaring 'audit_log is append-only' not found"
fi

# ---------------------------------------------------------------------------
# Q4: Show me a leaderboard opt-out.
# ---------------------------------------------------------------------------
# Policy: users.leaderboard_opt_out=true makes the learner vanish
# from peer leaderboard views, at BOTH snapshot-write time AND
# snapshot-read time. Sprint 7-B locked this contract.

echo "Q4: Show me a leaderboard opt-out."
set +e
run_test /tmp/dpa_q4.log 'TestGetCourseLeaderboard_OptedOutStudentDroppedFromRanking|TestGetCourseLeaderboard_SnapshotReadAppliesOptOutAtReadTime' ./internal/api/v1/handlers/...
rc=$?
set -e
if [ $rc -eq 0 ]; then
  pass "Leaderboard opt-out tests green (handler + snapshot service)"
elif [ $rc -eq 2 ]; then
  note "no test names matched the Q4 pattern (see /tmp/dpa_q4.log)"
  note "MANUAL: against a running dev server with two student tokens —"
  note "  curl http://localhost:8080/api/v1/courses/1/leaderboard \\"
  note "    -H \"Authorization: Bearer \$STUDENT_A_TOKEN\" | jq '.[] | .user_id'"
  note "  -- expect student B's user_id in the result. Then opt B out:"
  note "  psql \"\$DATABASE_URL\" -c \\"
  note "    \"UPDATE users SET leaderboard_opt_out=true WHERE id=<B>;\""
  note "  -- And re-fetch:"
  note "  curl http://localhost:8080/api/v1/courses/1/leaderboard \\"
  note "    -H \"Authorization: Bearer \$STUDENT_A_TOKEN\" | jq '.[] | .user_id'"
  note "  -- expect student B's user_id absent."
else
  fail "Q4 go test errored (see /tmp/dpa_q4.log)"
fi

# ---------------------------------------------------------------------------
# Q5: Show me an AI Assist call gated by COPPA.
# ---------------------------------------------------------------------------
# Policy: ai_assist.go gates on the caller's tenant_mode + coppa_strict.
# k5 / m68 tenants refuse with 403 + "AI features unavailable for
# K-12 students under FERPA/COPPA policy."

echo "Q5: Show me an AI Assist call gated by COPPA."
set +e
run_test /tmp/dpa_q5.log 'TestCreateConversation_CoppaStrictRefuses|TestRegister_CoppaStrictUnder13Pending' ./internal/api/v1/handlers/...
rc=$?
set -e
if [ $rc -eq 0 ]; then
  pass "AI Assist COPPA-gate tests green"
elif [ $rc -eq 2 ]; then
  note "no test names matched the Q5 pattern (see /tmp/dpa_q5.log)"
  note "MANUAL: against a running dev server seeded with a k5 tenant —"
  note "  curl -X POST http://localhost:8080/api/v1/ai_assist/outline \\"
  note "    -H \"Authorization: Bearer \$K5_STUDENT_TOKEN\" \\"
  note "    -H \"Content-Type: application/json\" \\"
  note "    -d '{\"prompt\":\"essay outline about cats\"}'"
  note "  -- expect HTTP 403 with body:"
  note "  -- {\"errors\":[{\"message\":\"AI features unavailable for K-12 ...\"}]}"
else
  fail "Q5 go test errored (see /tmp/dpa_q5.log)"
fi

echo ""
echo "DPA dry run complete."
echo ""
echo "Notes for the reviewer walkthrough:"
echo "  - Q1, Q2, Q4, Q5 attempt automated probe via go test before"
echo "    falling back to manual curl/psql. If your test names differ,"
echo "    the script logs the run to /tmp/dpa_q*.log and prints the"
echo "    manual command list."
echo "  - Q3 is intentionally manual: the trigger fires inside Postgres,"
echo "    not a Go test boundary; the script verifies the migration is"
echo "    in the chain but the live behavior must be observed in psql."
echo "  - All 5 questions are designed to be answered in ~10 minutes"
echo "    against a freshly-migrated dev DB."
