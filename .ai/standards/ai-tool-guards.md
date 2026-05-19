# AI Tool Guards — Standard for `create_*` Tools

> Tier 2. Required for any new write tool that creates entities the bot
> can batch-generate (students, teachers, classes, subjects, programs,
> courses, exams, questions, etc).

## Why this exists

The AI bot creates entities in batches. Every batch goes through
**propose → user confirms → execute**. Without explicit duplicate guards,
two failure modes appear:

1. **Cross-batch collision** — User asks "buat 5 siswa lagi", bot reuses
   plausible names/emails (`siti.aminah@example.com`) that were already
   created last batch. Unique constraint trips at execute time, after the
   user already confirmed.
2. **In-flight collision** — Bot proposes 10 questions in one turn, two of
   them have identical text or order. Both proposals sit in
   `ai_pending_actions` simultaneously; only the first one to execute
   succeeds.

The fix is layered: a system-prompt nudge is not enough on its own
because the LLM will sometimes skip the lookup. A pre-propose guard
prevents the bad proposal from ever reaching the user.

## Required pattern

For every new `create_<entity>` tool:

### 1. Validation block (existing pattern)

Reject empty required fields with `errValidationFailed`. Reject malformed
UUIDs with `errInvalidUUID`. Both must run before the duplicate guard.

### 2. Pre-propose duplicate guard

Add a helper in `backend/internal/app/ai_dupe_guards.go` named
`check<Entity>Duplicate(ctx, tenantID, ...uniqueFields) string`. The
helper must:

1. Check **in-flight pending proposals** in this session via
   `pendingHasField` (or a custom composite walk on
   `pendingArgsForSession`)
2. Check **committed rows** in the database, scoped to `tenant_id` and
   excluding `status = 'archived'` (archived rows free their slot per
   ADR-0007)
3. Return either `""` (clean) or
   `errDuplicateEntryWithRecovery(entity, field, value, hint)` so the bot
   knows what to fix

### 3. Wire into the tool handler

```go
func (a *App) toolCreateThing(ctx, tenantID, userID, args) (string, error) {
    // ... unmarshal + validation block ...

    if dup := a.checkThingDuplicate(ctx, tenantID, params.Email, ...); dup != "" {
        return dup, nil
    }

    confirmText := fmt.Sprintf(...)
    sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
    return a.createProposal(ctx, sessionID, tenantID, userID, "create_thing", args, confirmText)
}
```

### 4. Execute layer keeps its own dupe check

The execute layer (`exec*` or composite endpoint) must keep its own
unique-constraint check. The propose-time guard catches 99% of cases but
race conditions, multi-window admins, and direct API users can still
slip through.

## What to use as the "unique" key

| Entity | DB unique key | In-flight key |
|--------|--------------|---------------|
| student | `users.email`, `students.student_id_number` | `email`, `studentIdNumber` |
| teacher | `users.email`, `teachers.employee_id` | `email`, `employeeId` |
| staff | `users.email`, `staff_profiles.employee_id` | `email`, `employeeId` |
| class | `(tenant_id, academic_year_id, name)` | composite via `pendingArgsForSession` |
| subject | `(tenant_id, code)`, case-insensitive name | `code`, `name` |
| program | `(tenant_id, slug)` (when added) | `slug`, `title` |
| question (Phase 9) | `(exam_id, position)` AND content-hash for soft-dupe | `position`, content hash |

For **content-shaped** entities like exam questions, hashing the
normalized question text (lowercase, whitespace-collapsed, trimmed
punctuation) and comparing against existing rows in the same exam is the
right move — semantic dedup matters more than exact-string for
LLM-generated content.

## What NOT to guard against

- **Display name collisions on people**. Two students named "Ahmad" is
  legitimate. Email and student ID are the real uniqueness keys.
- **Soft-deleted (archived) collisions**. The partial unique index on
  `users.email` already exempts archived rows; the guards must mirror
  that with `status != 'archived'`.

## System prompt rules (enforced server-side)

The chat system prompt includes:

```
SEBELUM batch-create (>1 item dari jenis sama), WAJIB panggil list_*
atau search_* dulu untuk lihat data existing. JANGAN mengusulkan
nama/email/kode yang sudah ada.
```

This is a soft instruction. The pre-propose guard is the hard gate.

## Telemetry

When a duplicate guard fires it should be observable, but is not yet
emitted. Future work: log `ai.duplicate_blocked` with `{tool_name,
field, value_hash}` so we can spot when the bot is repeatedly blocked
on the same key (signal of a retry loop).

## See also

- `backend/internal/app/ai_dupe_guards.go` — implementations
- `backend/internal/app/ai_errors.go` — `errDuplicateEntryWithRecovery`
- ADR-0006 — self-correction protocol that makes bot retry on
  recoverable errors
- ADR-0007 — archive policy that frees `users.email` for reuse, which
  the guards rely on
