# ADR 0012: Kisi-Kisi Toggle + Stimulus Axis (3-axis authoring)

**Status**: Accepted
**Date**: 2026-05-19
**Tier**: 3 (auth-adjacent on toggle transitions; touches publish gate, AI tools, exam authoring)
**Depends on**: ADR-0008 (question model), ADR-0009 (collaboration), ADR-0010 (blueprints / kisi-kisi)

## Context

During Phase 9.6 we briefly explored a 3-mode `authoring_mode` column
on `exams` (`quick` / `structured` / `akm`) to unify the parallel
Questions + Kisi-Kisi tabs from Phase 9.5. The mode discriminator was
never shipped — within the same merge cycle three structural problems
surfaced:

1. **AKM is a property of the blueprint, not of the exam.** The exam
   only "becomes AKM" because its blueprint is `blueprint_type IN
   ('akm_literasi','akm_numerasi')`. Encoding AKM at the exam level
   duplicates state across two columns that must be kept in sync via
   handler logic (`clone_blueprint_to_exam` updates `authoring_mode`,
   reverse-flow apply updates it again, mode flips fight the blueprint
   type). The two sources of truth drift the moment any path forgets
   the side-effect.

2. **Stimulus is orthogonal to kisi-kisi.** A teacher writing a quick
   harian quiz can still want a stimulus passage with two follow-up
   questions. Today stimulus only attaches via `exam_question_groups`
   (mediated by AKM blueprint slots). Forcing the teacher into AKM
   mode just to attach a stimulus is wrong; the stimulus axis must be
   available regardless of kisi-kisi enforcement.

3. **`structured` and `akm` collapsed into the same handler logic.**
   In `handlePublishExam`, `handleCreateQuestion`, and
   `handleUpdateQuestion`, the `structured` and `akm` branches are
   indistinguishable except for the AKM branch always treating
   coverage as strict. Whenever the only difference between two enum
   values is "always strict vs honor a column", the enum is hiding a
   computed property — and that property already lives on the
   blueprint (`blueprint_type`, `strict_coverage`).

The 3-mode model also made drag-and-drop reorder muddier than it
needed to be: questions in structured/akm are already bound to a slot,
but visual reordering inside a section was conflated with slot
re-binding. The two are independent — slot binding is *pedagogical*
(this question tests this competency), section/group placement is
*visual* (this is where it lands on the rendered exam).

## Decision

Replace the 3-mode discriminator with a **3-axis model**:

```
Axis 1 (kisi-kisi)   exams.uses_kisi_kisi : bool
Axis 2 (AKM)         derived from exam_blueprints.blueprint_type
Axis 3 (stimulus)    always available; binds via direct or group path
```

### Axis 1: kisi-kisi toggle (`exams.uses_kisi_kisi`)

A boolean on `exams`. Replaces `authoring_mode` entirely.

- `false` (default): flat list. No slot enforcement on
  `exam_questions.blueprint_slot_id`. Publish gate ignores kisi-kisi
  coverage.
- `true`: every question must bind to an `exam_blueprint_slots` row.
  Publish gate enforces coverage when the blueprint has
  `strict_coverage = true`.

Transition rules (handler-enforced):

| From → To | Status `draft` | Status `published`/`archived` |
|---|---|---|
| `false` → `true` | allowed; if exam has questions but no blueprint, response includes a hint to apply a template or generate from existing | forbidden unless platform/master admin |
| `true` → `false` | allowed; slots and slot bindings are PRESERVED (just bypass enforcement) | forbidden unless platform/master admin |

Slot bindings are deliberately preserved across `true → false` flips.
A teacher who flips kisi-kisi off doesn't lose pedagogical metadata;
it just stops blocking publish. Flipping back to `true` resumes
enforcement against the existing bindings.

### Axis 2: AKM detection (no new column)

AKM is derived from the existing `exam_blueprints.blueprint_type IN
('akm_literasi','akm_numerasi')`. The frontend reads
`blueprintType` from `GET /api/v1/exams/{id}/slots-with-questions` and
renders AKM-specific labels (Konten / Konteks / Proses Kognitif,
Level 1-5) when applicable. Otherwise it shows reguler labels
(Cognitive Level / Difficulty).

There is **no exam-level AKM field**. An exam is "an AKM exam" iff its
blueprint type is AKM. This collapses the two-source-of-truth problem
that the discarded `authoring_mode` discriminator would have
introduced.

### Axis 3: stimulus attachment (always available)

A question can carry a stimulus through one of two mutually-exclusive
binding paths. The check sits in handler validation, not in the DB
(both columns share a domain — group's stimulus is the snapshot, the
direct stimulus is the raw library link — but a question must pick at
most one path).

```sql
-- Direct: solo question with its own stimulus reference
ALTER TABLE exam_questions ADD COLUMN stimulus_id UUID
  REFERENCES stimuli(id) ON DELETE SET NULL;

-- Group-mediated: existing column from migration 000015
-- exam_questions.group_id → exam_question_groups → stimulus snapshot

-- Mutex enforced by both DB CHECK and handler validation
ALTER TABLE exam_questions ADD CONSTRAINT exam_questions_stimulus_xor_group_chk
  CHECK (stimulus_id IS NULL OR group_id IS NULL);
```

Selection rules:

- `stimulusId` set, `groupId` null → solo: this question shows that
  stimulus header above its body.
- `stimulusId` null, `groupId` set → group-mediated: this question
  belongs to a stimulus cluster. Multiple questions in the same group
  share the snapshot.
- Both null → no stimulus.
- Both set → handler returns 422 with structured field error.

### Schema diff (migration 000016)

```sql
-- New flag (boolean axis)
ALTER TABLE exams ADD COLUMN uses_kisi_kisi BOOLEAN NOT NULL DEFAULT false;

-- Backfill from blueprint presence (every existing blueprint → on)
UPDATE exams e SET uses_kisi_kisi = true
 WHERE EXISTS (SELECT 1 FROM exam_blueprints b WHERE b.exam_id = e.id);

-- Index for filtering
CREATE INDEX exams_uses_kisi_kisi_idx ON exams (tenant_id, uses_kisi_kisi);

-- Direct stimulus link on questions (solo / non-group attachment)
ALTER TABLE exam_questions ADD COLUMN stimulus_id UUID
  REFERENCES stimuli(id) ON DELETE SET NULL;
CREATE INDEX exam_questions_stimulus_idx ON exam_questions (tenant_id, stimulus_id)
  WHERE stimulus_id IS NOT NULL;

-- Mutual exclusion: question has stimulus_id OR group_id, never both
ALTER TABLE exam_questions ADD CONSTRAINT exam_questions_stimulus_xor_group_chk
  CHECK (stimulus_id IS NULL OR group_id IS NULL);
```

Forward-only; dev data is allowed to drop `authoring_mode` since the
information is fully reconstructible from `exam_blueprints` presence
and `blueprint_type`.

### Drag-and-drop semantics

The slot-first canvas (and the flat list when kisi-kisi is off) uses
@dnd-kit/core + @dnd-kit/sortable to reorder, group, and section
questions. The endpoint is `POST /api/v1/exams/{id}/questions/move`
with body `{questionId, sectionId?, groupId?, sortOrder?}`. Atomic
move in a single transaction.

Critical invariant: **slot binding (`blueprint_slot_id`) is PRESERVED
across moves**. Kisi-kisi position is pedagogical, not visual. A
question moved from one section to another, or into a stimulus group,
keeps its slot. To re-bind a slot, the user uses the existing
`PATCH /api/v1/exam-blueprint-slots/{slotId}/assign-question` flow.

Reordering policy: the caller (frontend) is responsible for sortOrder
values. When multiple questions need to shift, the frontend computes
the new order locally and sends one move call per affected question.
This keeps the backend endpoint cheap (no transitive shifts) and
avoids the "reorder a single question, server rewrites N rows" pattern.

### Section vs Group nesting

The visual hierarchy supports both axes simultaneously:

```
Exam
├── Section (visual cluster, optional)
│   ├── Question (root in section)
│   ├── Group (stimulus cluster, optional, can nest inside a section)
│   │   ├── Question
│   │   └── Question
│   └── Question
├── Group (stimulus cluster, optional, at root level)
│   └── Question
└── Question (root)
```

A question can be: root, in-section, in-group, or in-group-in-section.
A group can nest inside a section. A section never nests inside a
group. The `exam_question_groups.section_id` column already supports
this from migration 000015.

### AKM auto-grouping policy

When a user applies an AKM template (blueprint_type in akm_*) via
`clone_blueprint_to_exam`:

- **If the template's slots carry `stimulus_id` references**, the
  clone executor auto-creates `exam_question_groups` rows for each
  unique stimulus and links the cloned slots to those groups via
  `group_id` on the slot or via the slot's stimulus_id reference.
  Best-effort.
- **If the template has no stimulus pre-assignments**, no groups are
  auto-created. The user manually creates groups from the canvas
  toolbar.

The current `blueprint_template_slots.stimulus_id` is a live FK to the
library; there is no slot-level `group_id` today. AKM auto-grouping
therefore reads `stimulus_id` across slots, dedupes, and creates one
group per unique stimulus. Slot question authoring then attaches new
questions to that group via `groupId`.

### Mode/toggle transitions on archived/published exams

Forbidden by default; only `master_admin` / `platform_admin` can
override. `school_admin` is intentionally NOT enough — flipping
kisi-kisi off on a published exam silently changes the publish
contract for students who already started. This follows the same
gate philosophy used elsewhere in Phase 9 — publish-time invariants
are master/platform-only.

### AI tool deltas

- `set_authoring_mode` capability → renamed to `set_uses_kisi_kisi`,
  args `{examId, enabled: bool}`. Description updated to "Enable or
  disable kisi-kisi (assessment blueprint) enforcement on an exam".
- `convert_questions_to_kisi_kisi` capability stays — internally now
  calls `set_uses_kisi_kisi(true)` before/after analyze + apply
  instead of `set_authoring_mode(structured)`.
- `clone_blueprint_to_exam` executor: also sets `uses_kisi_kisi = true`
  on the parent exam (replacing the old authoring_mode promotion). AKM
  detection follows from the cloned blueprint type, no exam column to
  flip.

### Frontend deltas

- `mode-switch-modal.tsx` (3-radio picker) → replaced by
  `kisi-kisi-toggle.tsx` (single toggle button with confirm).
- `exam-blueprint-section.tsx` (unimported leftover from Phase 9.5b) →
  deleted.
- `slot-first-canvas.tsx` rewritten with @dnd-kit DndContext +
  SortableContext, AKM-aware labels read from
  `blueprint.blueprintType`, drag handle on slot cards, drop targets
  for sections / groups / root.
- Question form gains a stimulus picker section visible regardless of
  kisi-kisi state, with three radio paths: no stimulus / direct /
  group.

## Alternatives considered

- **Keep `authoring_mode`, just collapse `structured` + `akm`** —
  rejected. Still leaves AKM duplicated across two sources, and
  doesn't address the stimulus orthogonality problem.
- **Auto-detect kisi-kisi from blueprint presence (no column)** —
  rejected. "Has a blueprint but doesn't enforce coverage" is a real
  state (downgrade path). An explicit boolean is cheaper than
  reconstructing intent from blueprint + slot binding count every
  request.
- **Promote stimulus to a top-level question relationship table
  (`question_stimuli`)** — overkill. A question has at most one
  stimulus path, and the existing `exam_question_groups` already
  mediates the multi-question case. Mutex column + check constraint
  is sufficient.
- **Drop sections, keep only groups** — rejected. Sections are visual
  clusters teachers want for "Bagian A: Pilihan Ganda / Bagian B:
  Essay". Groups are stimulus-bound clusters. Different concepts;
  both stay.

## Consequences

Positive:

- Single source of truth for AKM (`blueprint_type`). No more
  side-effect-fully syncing two columns across forward and reverse
  flows.
- Stimulus axis is genuinely orthogonal. A quick harian quiz can
  attach a passage; an AKM exam isn't required to have one.
- Drag-and-drop reorder/group/section is decoupled from slot binding.
  Visual layout and pedagogical anchoring stop fighting.
- Toggle UX is simpler — one button instead of a 3-radio picker.
- Backend handler logic loses the `structured`/`akm` bifurcation; any
  AKM-specific behavior reads `blueprint_type` directly.

Negative:

- Forward-only migration. Any tooling that read `exams.authoring_mode`
  must be updated (we audit-grepped to zero before merging).
- Two stimulus paths (direct + group-mediated) means the frontend
  picker has 3 mutually-exclusive options; tests must cover all
  combinations including the 422 path.
- AKM auto-grouping is best-effort; when a template lacks
  `stimulus_id` on slots, the user creates groups manually. Documented
  in the clone executor and surfaced as an empty-state hint on the
  canvas.

Neutral:

- The dnd-kit dependency is added to the frontend bundle. Pinned
  exact versions per repo convention. Tree-shaking should keep impact
  small (~15kb gzipped for core+sortable+utilities).
- The `mode-switch-modal.tsx` component is deleted, not deprecated.
  No callers remain.

## Implementation

- `backend/migrations/000017_kisi_kisi_toggle_and_stimulus.sql`
- `backend/internal/app/exams.go` — examRow JSON renamed
  `authoringMode` → `usesKisiKisi`; create/update accept
  `usesKisiKisi`; transition gate; publish-gate simplified.
- `backend/internal/app/questions.go` — replaces
  `loadExamAuthoringMode` with `loadExamUsesKisiKisi`; mutex check on
  stimulusId/groupId; embedded summaries in list response.
- `backend/internal/app/exam_sections.go` (no change beyond unused
  helpers) and new endpoint family in
  `backend/internal/app/question_moves.go` for `POST
  /api/v1/exams/{id}/questions/move` and group CRUD.
- `backend/internal/app/blueprint_clone.go` — replaces authoring_mode
  promotion with `uses_kisi_kisi = true`; AKM auto-grouping when
  template carries stimulus refs.
- `backend/internal/app/blueprint_slots.go` — `slots-with-questions`
  response includes `blueprintType` and per-question `stimulus` /
  `group` summaries.
- `backend/internal/app/ai_blueprint_tools.go` /
  `backend/internal/app/ai_write_tools.go` — `set_authoring_mode` →
  `set_uses_kisi_kisi`; executor switch updated;
  `clone_blueprint_to_exam` and `convert_questions_to_kisi_kisi` flip
  `uses_kisi_kisi = true`.
- `backend/internal/app/kisi_kisi_test.go` (renamed from
  authoring_mode_test.go) — tests rewritten around boolean toggle +
  stimulus-mutex.
- `frontend/package.json` — pin
  `@dnd-kit/core 6.1.0`, `@dnd-kit/sortable 8.0.0`,
  `@dnd-kit/utilities 3.2.2`.
- `frontend/src/lib/modules-api.ts` — `usesKisiKisi`,
  `updateExamKisiKisi`, `moveQuestion`, group CRUD helpers, extended
  Question type.
- `frontend/src/components/blueprint/kisi-kisi-toggle.tsx` (new) —
  replaces `mode-switch-modal.tsx`.
- `frontend/src/components/blueprint/slot-first-canvas.tsx` — full
  rewrite with @dnd-kit, AKM-aware labels, group rendering, root /
  section / group drop targets.
- `frontend/src/app/(app)/app/exams/[id]/page.tsx` — header swap,
  AKM badge, stimulus picker in question form.
- DELETE: `frontend/src/components/blueprint/mode-switch-modal.tsx`,
  `frontend/src/components/blueprint/exam-blueprint-section.tsx`.

## Verification

- `TestCreateQuestion_RequiresSlotWhenKisiKisiOn`
- `TestCreateQuestion_AllowsNoSlotWhenKisiKisiOff`
- `TestUpdateExam_DisableKisiKisi_DraftOnly`
- `TestUpdateExam_KisiKisiToggleImmutableAfterPublish`
- `TestQuestion_StimulusAndGroupMutuallyExclusive`
- `TestMoveQuestion_PreservesSlotBinding`
- `go vet ./... && go build ./... && go test ./...` clean
- `npx tsc --noEmit` clean
- Manual smoke: create new exam → kisi-kisi off → flat list works →
  toggle on → empty canvas + apply template flow → fill slot via
  drag-and-drop into a group → verify slot binding preserved → toggle
  off → publish → verify coverage gate bypassed.

## Future work

- Question Bank (council Idea #5) — once tenants have 5+ exams using
  similar kisi-kisi, lift questions out of `exam_questions` into a
  reusable `question_bank` resource. ADR pending.
- Stimulus media (image/audio/video) — schema in 000015 already has a
  `type` column; expand the CHECK enum and add storage references.
- Multi-question move endpoint (single payload, multiple shifts) —
  defer until profiling shows the per-question-call pattern is a
  bottleneck.
- AKM auto-grouping by inferring shared topic when the template has
  no stimulus refs (LLM-assisted; out of scope for 9.7).
