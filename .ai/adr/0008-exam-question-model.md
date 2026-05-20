# ADR 0008: Exam Question Model and AI Authoring Guards

**Status**: Accepted
**Date**: 2026-05-19
**Tier**: 2 (core business logic, exam-reliable)

## Context

Phase 9 introduces exam authoring. The skeleton in `000006_exams.sql`
(from Phase 1) had basic shapes but missed three critical things for
real-world authoring:

1. **Flexible MCQ scoring** — Indonesian schools commonly use partial-credit
   MCQs (pick 2 of 3 correct → 67%) for skill assessments, not just the
   binary "exact match = full points" model.
2. **Per-question shuffle override** — exam-level shuffle is too coarse;
   ordering questions ("place these events in order") need fixed order
   even when the rest of the exam shuffles.
3. **AI batch authoring without duplicates** — the chatbot is the primary
   authoring surface for power users. Generating 50 questions in batches
   is the canonical workflow, and the bot will hallucinate duplicate
   content unless we hard-block it.

We also needed to preserve the existing `000006` skeleton because some
deployments may already have data in those tables.

## Decision

### 1. Additive migration (`000013_exams_questions.sql`)

All changes are `ALTER TABLE ADD COLUMN IF NOT EXISTS` plus new indexes
and a single trigger. No schema breaks. The original `000006` skeleton
remains in place; clients on either schema version coexist until the
new columns are populated.

### 2. Three MCQ scoring modes

```
correct_all  default. Score full points only when student selected
             EXACTLY all correct options (no extras, no missing).
             Back-compatible with the simple `correct_answer` column.

correct_one  Pick any single correct option for full points. Used for
             "find the one true statement" style questions where the
             student should not feel penalized for not picking all
             defensible options.

percentage   score = points * (correct_selected / total_correct)
             - per-option `points_weight` overrides equal-share when set
             - optional `wrong_penalty_pct` (0..1) subtracts
               `points * wrong_penalty_pct` per wrong selection
             - score clamped >= 0
```

The choice is per-question, not per-exam. A typical mix: easy MCQs use
`correct_all`, complex multi-correct MCQs use `percentage`.

### 3. Per-question option shuffle override

Resolution rule: `exam_questions.shuffle_options_override IS NOT NULL`
wins over `exams.shuffle_options`. Default to inheriting from the exam.

Rationale: ordering questions (chronological events, ranked priorities)
break when shuffled, but other questions in the same exam should still
shuffle. Per-question override is the right granularity.

### 4. Content-hash duplicate prevention for AI batches

Two independent guards, both required:

**Committed guard**
```
content_hash = md5(lower(trim_whitespace_collapsed(content)))
```
stored on insert. Pre-create check rejects duplicates at the API layer
with a `422` validation error (committed dupe scoped to one exam).

**In-flight guard**
The AI dupe-guard inspects `ai_pending_actions` for the same session,
walking BOTH `create_question` proposals AND items inside
`batch_create_questions` proposals. This catches the canonical bot
loop: propose 5 questions, user confirms, propose 5 more — without
this check, the bot reuses plausible content and trips the unique
constraint at execute time, after the user already confirmed.

Within a single `batch_create_questions` call, a `seenHashes` map
catches the bot proposing the same content twice in one batch.

The whole pattern is documented at `.ai/standards/ai-tool-guards.md`
so Phase 10+ inherits it.

### 5. All-or-nothing batch validation

`batch_create_questions` validates every item up-front. If even one
fails (duplicate, invalid type, missing options), the whole batch is
rejected with `BATCH_VALIDATION_FAILED` and a per-item `failures` array.
The bot then has explicit per-index hints to retry.

Rationale: partial-success batches in this domain create messy
half-states ("28 of 50 questions added, retry the rest"). The bot
handles all-or-nothing far more cleanly because the recovery is
"fix the listed items and retry the whole batch".

### 6. Cascade rule on section delete

Deleting a section sets `section_id = NULL` on its questions (via the
schema's existing `ON DELETE SET NULL`). Questions become "unsectioned"
rather than being deleted.

Rejected alternative: cascade-delete questions when their section is
deleted. Deleting a section is usually an organizational change ("move
these questions to a different group"), not a content removal. Losing
the questions silently would be a serious data-loss footgun.

### 7. Publish gate

`POST /exams/{id}/publish` returns `422` when the exam has zero
questions. The empty-published state is never useful (a takeable exam
with nothing to take), so we hard-block it.

## Alternatives considered

- **Single boolean `is_correct` per option without scoring modes** — too rigid for real-world MCQs.
- **Free-form `scoring_function` JSONB** — extreme flexibility, but every grader needs to interpret it. Kills determinism, complicates Phase 11 grading.
- **Per-exam shuffle only** — rejected because ordering questions break.
- **Soft-cap batch at 20 questions** — would hide the real problem (no dedup). Better to allow up to 100 with hard guards than to artificially narrow the API.
- **Hash content + options together** — over-strict; the same question with one rephrased option should still be considered a dupe.
- **Hash includes question_type** — unnecessary; the same content as both an MCQ and an essay is genuinely a different question, but realistically nobody does this. The looser rule (hash content only) catches the real failure case (bot regenerating the same MCQ text) without overhead.
- **Rate-limit AI batch_create_questions per session** — would be reactive, not preventive. The dupe guard prevents the harm directly.

## Consequences

**Positive**
- Authors can express real exam scoring requirements without app-side hacks
- 50-question AI authoring works without polluting the exam with duplicates
- Per-question shuffle override removes the "always shuffle / never shuffle" false dichotomy
- Skeleton from 000006 is preserved; rollback to 000012 is safe (the new columns are nullable and the legacy `password` column on gate windows is still populated)

**Negative**
- More state per question (8 new columns, one trigger, three indexes). Fine for OLTP at the row counts an LMS will see (millions, not billions).
- Two scoring-mode dimensions (mode + per-option weight + wrong penalty) is more surface area for bugs in the Phase 11 grader. Mitigation: grader will be unit-tested per mode with golden test cases.
- Content-hash is fingerprint-strong (md5 collision rate is fine for this scale) but not cryptographic. We're not relying on it for security; only correctness.

**Neutral**
- The `correct_count` column is denormalized (cached from option rows) and maintained by a trigger. Future work could replace it with a generated column if Postgres adds one for cross-row aggregates.

## Implementation

- `backend/migrations/000013_exams_questions.sql` — schema additions + trigger
- `backend/internal/app/exams.go` — exam CRUD, publish gate, archive, restore, RBAC by subject
- `backend/internal/app/exam_sections.go` — sections nested under exam
- `backend/internal/app/questions.go` — questions + options CRUD, validation per type, content-hash dedup
- `backend/internal/app/exam_gates.go` — gate windows with `is_open` computed
- `backend/internal/app/ai_exam_tools.go` — 7 capabilities, single + batch executors
- `backend/internal/app/ai_dupe_guards.go` — `checkExamDuplicate`, `checkQuestionDuplicate`
- `backend/internal/app/questions_test.go` — content-hash + validateQuestionPayload tests
- `frontend/src/app/(app)/app/exams/page.tsx` — list + create/edit/archive/publish
- `frontend/src/app/(app)/app/exams/[id]/page.tsx` — detail with sections tree, question authoring overlay (4 type-specific forms), gate window scheduler
- `frontend/src/lib/modules-api.ts` — API client extension
- `.ai/api/exams.md` — endpoint contracts
- `.ai/standards/ai-tool-guards.md` — established last sprint, now references question dedup as the canonical content-hash example

## Verification

- `go vet ./...` clean
- `go build ./...` clean
- `go test ./internal/app/` passes (16 tests, including new question + hash tests)
- `tsc --noEmit` clean (frontend/symlink to main repo node_modules)

## Future work

- Question bank (reusable across exams) — out of scope for Phase 9
- Anti-cheat (camera, tab focus) — Phase 11 / separate feature
- Auto-grade short_answer with fuzzy match against reference — needs careful design (false-positives in Indonesian-language synonym handling)
- AI tool to import questions from a PDF / text dump — high-value but pulls in OCR / parsing concerns; defer until Phase 12 AI runtime
