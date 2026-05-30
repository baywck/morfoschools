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

create_payload=$(jq -n --arg examId "$EXAM_ID" '{
  sessionId: "",
  message: "smoke kisi-kisi fail-retry",
  scopeType: "exam",
  source: "repair",
  goal: "smoke kisi-kisi fail-retry",
  examId: $examId,
  planned: {
    scopeType: "exam",
    source: "repair",
    goal: "smoke kisi-kisi fail-retry",
    intentSummary: "fail then retry",
    batches: [
      {
        batchIndex: 1,
        actionType: "audit",
        workflow: "audit_blueprint_slots",
        targetType: "blueprint_slot",
        targetIds: [$examId],
        argsJson: {examId: $examId},
        preview: "Audit ok",
        progressUnits: 1
      },
      {
        batchIndex: 2,
        actionType: "repair",
        workflow: "smoke_nonexistent_workflow",
        targetType: "blueprint_slot",
        targetIds: [$examId],
        argsJson: {examId: $examId},
        preview: "Planned failure",
        progressUnits: 1
      }
    ]
  }
}')
created=$(curl -sS -b "$cookie_jar" -X POST "$BASE_URL/api/v1/ai/action-plans" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf_token" \
  -d "$create_payload")
plan_id=$(printf '%s' "$created" | jq -r '.planId // empty')
if [ -z "$plan_id" ]; then
  echo "plan_create_failed" >&2
  printf '%s\n' "$created" >&2
  exit 1
fi

batch1=$(curl -sS -b "$cookie_jar" -X POST "$BASE_URL/api/v1/ai/action-plans/$plan_id/run-next" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf_token")
printf 'batch1: %s\n' "$batch1"

batch2=$(curl -sS -b "$cookie_jar" -X POST "$BASE_URL/api/v1/ai/action-plans/$plan_id/run-next" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf_token" || true)
printf 'batch2: %s\n' "$batch2"

# Patch batch 2 to workflow that can succeed, then retry.
patch_sql="UPDATE agent_action_plan_batches SET workflow='audit_blueprint_slots', status='pending', error='' WHERE plan_id='$plan_id' AND batch_index=2"
docker compose exec -T postgres psql -U morfoschools -d morfoschools -c "$patch_sql" >/dev/null

retry=$(curl -sS -b "$cookie_jar" -X POST "$BASE_URL/api/v1/ai/action-plans/$plan_id/run-next" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf_token")
printf 'retry: %s\n' "$retry"

plan=$(curl -sS -b "$cookie_jar" "$BASE_URL/api/v1/ai/action-plans/$plan_id")
printf 'plan: %s\n' "$plan"
