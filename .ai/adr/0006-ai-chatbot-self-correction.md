# ADR 0006: AI Chatbot Self-Correction Protocol

**Status**: Accepted
**Date**: 2026-05-18
**Tier**: 2 (core business logic)

## Context

The AI chatbot in Morfoschools assists teachers and admins with operational tasks (enroll students, archive accounts, create classes) by calling backend APIs as tools. The original implementation had three problems:

1. **Token waste on startup** — every request loaded the full system prompt with all tool schemas, regardless of intent.
2. **Generic error messages** — when a tool failed (invalid UUID, entity not found), the response was `{"error":"Student not found"}` with no guidance for the bot. The bot either gave up or hallucinated.
3. **No retry-with-correction** — bot had no protocol for handling failures. It would either ask the user dumb questions or fabricate a response.

A user request like "archive Budi" would fail silently if the bot guessed the wrong UUID, and the bot would respond "Done!" without confirming.

## Decision

Three changes layered together:

### 1. Structured error envelopes (`ToolError`)

All tool handlers return errors as machine-readable JSON with explicit recovery hints:

```json
{
  "error": {
    "code": "ENTITY_NOT_FOUND",
    "message": "student not found for studentName='Budi'",
    "field": "studentName",
    "entity": "student",
    "recoverable": true,
    "recovery": {
      "tool": "search_students",
      "args": {"search": "Budi"},
      "hint": "Search for the correct student first, then retry with the UUID from results"
    },
    "suggestions": ["Budi Santoso", "Budi Pratama"]
  }
}
```

Error codes: `ENTITY_NOT_FOUND`, `INVALID_UUID`, `VALIDATION_FAILED`, `DUPLICATE_ENTRY`, `PERMISSION_DENIED`.

### 2. Self-correction protocol in system prompt (~100 tokens)

```
ON TOOL ERROR:
1. Read error.recovery — call suggested tool — retry original action (silently)
2. If second failure on same action — ask user a focused clarification question
3. If third failure — apologize with specific reason
NEVER: show error codes to user, retry >2x same error, guess without data
```

### 3. Enriched search results

All `search_*` and `list_*` tools now return UUIDs in their results. This allows the bot to chain calls: search by name → get UUID → retry mutation.

## Consequences

**Positive:**
- Bot self-corrects on first failure without bothering the user
- "Invalid UUID" errors point the bot toward the search tool with the original input
- Suggestions field handles fuzzy-match cases (user typed "Bdi" → suggestions: ["Budi"])
- Token budget unchanged in the happy path; ~100 token overhead in system prompt is paid back many times when avoiding redundant clarification turns
- Frontend auto-refreshes via `morfoschools:data-changed` event after both AI mutations and manual CRUD operations

**Negative:**
- Backend tool handlers now have more code (validation + error construction)
- Two registry systems still coexist (`ToolRegistry` for legacy, `CapabilityRegistry` for domain-routed). Consolidation deferred.

**Neutral:**
- Tool result JSON is now larger (includes `id` field), but this enables chaining which saves more tokens overall.

## Implementation

- `backend/internal/app/ai_errors.go` — `ToolError` type + factory functions (`errEntityNotFound`, `errInvalidUUID`, `errValidationFailed`, `errDuplicateEntry`, `errWithSuggestions`)
- `backend/internal/app/ai_chat.go` — system prompt updated with retry protocol, response includes `mutated: true` flag for frontend
- `backend/internal/app/ai_write_tools.go` — all write tool handlers return structured errors
- `backend/internal/app/ai_tools.go` — search tool results include `id` field
- `frontend/src/components/layout/app-shell.tsx` — global `morfoschools:data-changed` listener triggers `router.refresh()`
- `frontend/src/components/layout/ai-chat-panel.tsx` — dispatches event when `data.mutated === true`
- `frontend/src/lib/use-crud.ts` — dispatches event after manual create/update/archive

## Future work

- Consolidate `ToolRegistry` and `CapabilityRegistry` into one
- Add `list_capabilities` meta-tool for true lazy tool discovery
- Add session-level entity cache (resolved UUIDs persist across turns)
- Route-aware context injection (frontend sends active entity IDs, bot skips redundant lookups)
