#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
EMAIL="${EMAIL:-admin@morfoschools.local}"
PASSWORD="${PASSWORD:-admin123}"
EXAM_ID="${EXAM_ID:-148ba6ec-a7ef-4c41-8a31-a4652e36b506}"

cookie_jar=$(mktemp)
trap 'rm -f "$cookie_jar"' EXIT

login_resp=$(curl -sS -c "$cookie_jar" -X POST "$BASE_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")
csrf_token=$(printf '%s' "$login_resp" | jq -r '.csrfToken // empty')
if [ -z "$csrf_token" ]; then
  echo "login_failed" >&2
  printf '%s\n' "$login_resp" >&2
  exit 1
fi

summary_before=$(curl -sS -b "$cookie_jar" "$BASE_URL/api/v1/ai/action-plans/current/summary?examId=$EXAM_ID")
printf 'GET summary before: %s\n' "$summary_before"

create_payload=$(jq -n --arg examId "$EXAM_ID" '{
  sessionId: "",
  message: "Audit semua kisi-kisi yang tersedia",
  scopeType: "exam",
  source: "audit",
  goal: "Audit semua kisi-kisi yang tersedia",
  examId: $examId,
  planned: {
    scopeType: "exam",
    source: "audit",
    goal: "Audit semua kisi-kisi yang tersedia",
    intentSummary: "audit kisi-kisi",
    batches: [
      {
        batchIndex: 1,
        actionType: "audit",
        workflow: "audit_blueprint_slots",
        targetType: "blueprint_slot",
        targetIds: [$examId],
        argsJson: {examId: $examId},
        preview: "Audit semua kisi-kisi",
        progressUnits: 1
      }
    ]
  }
}')
created=$(curl -sS -b "$cookie_jar" -X POST "$BASE_URL/api/v1/ai/action-plans" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf_token" \
  -d "$create_payload")
printf 'POST action-plans: %s\n' "$created"

plan_id=$(printf '%s' "$created" | jq -r '.planId // empty')
if [ -z "$plan_id" ]; then
  echo "plan_create_failed" >&2
  printf '%s\n' "$created" >&2
  exit 1
fi

run_next=$(curl -sS -b "$cookie_jar" -X POST "$BASE_URL/api/v1/ai/action-plans/$plan_id/run-next" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf_token")
printf 'POST run-next: %s\n' "$run_next"

if ! printf '%s' "$run_next" | jq -e '.plan.status == "completed"' >/dev/null; then
  echo "plan_not_completed_after_run" >&2
  printf '%s\n' "$run_next" >&2
  exit 1
fi

final_summary=$(curl -sS -b "$cookie_jar" "$BASE_URL/api/v1/ai/action-plans/current/summary?examId=$EXAM_ID")
printf 'GET final summary: %s\n' "$final_summary"

if ! printf '%s' "$final_summary" | jq -e '.status == "completed"' >/dev/null; then
  echo "final_summary_not_completed" >&2
  printf '%s\n' "$final_summary" >&2
  exit 1
fi
