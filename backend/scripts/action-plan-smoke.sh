#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
EMAIL="${EMAIL:-admin@morfoschools.local}"
PASSWORD="${PASSWORD:-admin123}"
EXAM_ID="${EXAM_ID:-188e909b-7386-4cf0-8f93-9a7033e12e5d}"

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

summary=$(curl -sS -b "$cookie_jar" "$BASE_URL/api/v1/ai/action-plans/current/summary?examId=$EXAM_ID")
printf 'GET current/summary: %s\n' "$summary"

current=$(curl -sS -b "$cookie_jar" "$BASE_URL/api/v1/ai/action-plans/current?examId=$EXAM_ID")
printf 'GET current: %s\n' "$current"

create_payload=$(jq -n --arg examId "$EXAM_ID" '{
  sessionId: "",
  message: "smoke kisi-kisi audit",
  scopeType: "exam",
  source: "audit",
  goal: "smoke kisi-kisi audit",
  examId: $examId,
  planned: {
    scopeType: "exam",
    source: "audit",
    goal: "smoke kisi-kisi audit",
    intentSummary: "audit kisi-kisi",
    batches: [
      {
        batchIndex: 1,
        actionType: "audit",
        workflow: "audit_blueprint_slots",
        targetType: "blueprint_slot",
        targetIds: [$examId],
        argsJson: {examId: $examId},
        preview: "Audit kisi-kisi",
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

final_summary=$(curl -sS -b "$cookie_jar" "$BASE_URL/api/v1/ai/action-plans/current/summary?examId=$EXAM_ID")
printf 'GET final summary: %s\n' "$final_summary"
