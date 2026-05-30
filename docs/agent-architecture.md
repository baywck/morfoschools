# Agent Architecture Reset

Status: draft baseline after legacy AI cleanup.

## Goals

- Keep chat useful for discussion.
- Rebuild write abilities as explicit workflows, not a generic tool loop.
- Prevent stale active context, accidental mutations, and fake confirmations.
- Make exam authoring reliable before adding question generation.

## Non-negotiables

1. LLM may discuss and extract intent/arguments only.
2. LLM never decides tenant, user, ownership, or write authorization.
3. Every write is `proposal -> confirm -> transaction`.
4. No generic tool registry and no large tool catalog sent every turn.
5. Workflows are explicit backend code with typed validation.
6. Backend validates RBAC, tenant ownership, required fields, and state.
7. Active page context is server-verified; client shadow is only a hint.
8. Kisi-kisi metadata is non-blocking for early workflows.

## Initial workflow scope

1. `create_exam`
2. `create_exam_section`
3. `create_question_group`
4. `create_stimulus`
5. `attach_stimulus_to_group`

## Kisi-kisi policy

`create_exam` supports `usesKisiKisi` from the start.

When `usesKisiKisi=true`, confirmation creates:

- `exams` row
- default `exam_sections` row
- empty/default `exam_blueprints` container

It does not generate blueprint slots, CP/TP, indicators, or questions yet.
Those are separate future workflows.

## Proposed backend files

- `agent_chat.go` — discussion + routing entrypoint
- `agent_intent.go` — LLM JSON intent extraction
- `agent_proposals.go` — proposal lifecycle
- `agent_workflows.go` — explicit workflow dispatch
- `agent_exam_create.go` — create exam workflow
- future: `agent_section_create.go`, `agent_group_create.go`, `agent_stimulus_create.go`

## First implementation slice

Implement only `create_exam` with tests:

- creating a proposal does not mutate data
- confirming proposal creates exam
- confirming with `usesKisiKisi=true` creates empty blueprint
- missing subject/title produces clarification/validation
- workflow requires authenticated user, tenant, CSRF, and `exams:write`

## Kisi-kisi execution planner

For larger kisi-kisi work, the agent can use the action plan layer.

Current supported backend runners:

- `audit_blueprint_slots`
- `repair_kisi_kisi_slots`
- `complete_kisi_kisi_slots`

Current supported plan APIs:

- `GET /api/v1/ai/action-plans/current?examId=...`
- `GET /api/v1/ai/action-plans/current/summary?examId=...`
- `GET /api/v1/ai/action-plans/{planId}`
- `POST /api/v1/ai/action-plans`
- `POST /api/v1/ai/action-plans/{planId}/run-next`

Behavior:

- Small 1-5 slot actions can stay proposal-first.
- Large audit/repair/completion actions can use plan-first execution.
- Failed batches can be retried without losing prior completed batches.
- Plan status can reactivate when user requests retry.
- Plan summary now exposes domain-specific kisi-kisi health: totalSlots, missingTP, missingMateri, missingIndikator, disconnected.
