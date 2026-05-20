# ADR 0010: Assessment Blueprints (Kisi-Kisi)

**Status**: Accepted
**Date**: 2026-05-19
**Tier**: 2 (core business logic, exam-reliable)
**Depends on**: ADR-0009 (collaboration), ADR-0008 (question model)

## Context

Indonesian schools work from a **kisi-kisi** (assessment blueprint)
before writing exam questions. The kisi-kisi specifies, per question
slot:

- The competency targeted (KD in K13, CP in Merdeka)
- The material/topic
- The cognitive level (Bloom's taxonomy: C1–C6)
- The difficulty (mudah/sedang/sulit)
- The question type and points

For AKM/ANBK (national assessment), additional dimensions apply:
content category, context, cognitive process, and proficiency level
(1–5).

Phase 9 shipped freeform exams with no concept of a blueprint. Three
problems followed:

1. AI question generation has no pedagogical anchor. Bot invents the KD
   alongside the question, producing plausible content that does not
   align with curriculum standards.
2. Coverage analysis is manual. Teachers can't tell at a glance whether
   their exam tests every required KD.
3. Cross-exam consistency is impossible. Two teachers producing UTS
   Matematika kelas 10 in different semesters end up with completely
   different distributions.

We need first-class blueprint support that:

- Maps to both K13 (KD) and Merdeka (CP) terminology
- Supports AKM dimensions
- Enables reusable templates (the same exam type repeated each semester
  uses the same blueprint structure)
- Allows AI to generate questions FROM a slot specification (forward
  flow) AND to derive a blueprint FROM existing questions (reverse flow)
- Supports stimulus-based questions (AKM literacy, science) where one
  passage feeds multiple questions

Curriculum master data (the official Permendikbud KD/CP catalog) is not
available in clean structured form. We capture competencies as
free-text now, with a master `competencies` table ready to receive
preloaded data later without schema migration.

## Decision

### Core entities

```
curricula (master, preload)
  - k13, merdeka, akm_numerasi, akm_literasi
  - id, code, name, description

competencies (master, populated over time)
  - id, curriculum_id (FK → curricula, NOT NULL), subject_code,
    grade_or_phase, code, description
  - normalized_code (lowercase, dot-stripped) for dedup lookups
  - currently empty; nullable FK from blueprint_*_slots
  - lookup is "best effort" — admin can attach a master entry later
  - the curriculum_id FK is what makes the frontend's curriculum-aware
    label mapping (KD vs CP) actually selectable in dropdowns

stimuli (per tenant, library)
  - id, tenant_id, owner_user_id, type ('text' for now)
  - title, content (markdown), source (citation)
  - lifecycle: 'exam_scoped' | 'shared' | 'archived'
    • exam_scoped: created inline in question form, only visible inside
      its parent exam; reusable only after explicit "Save to library"
    • shared: visible across the tenant in the library
    • archived: hidden from new selections but kept for audit
  - usage_count (cached) for library UI

exam_question_groups (NEW — atomic shuffle units, primary AKM container)
  - id, tenant_id, exam_id, section_id (nullable)
  - stimulus_id (nullable FK → stimuli)
  - stimulus_title_snapshot, stimulus_body_snapshot (TEXT, snapshot at
    use time — frozen on exam publish/attempt; see lifecycle below)
  - group_type: 'standalone' | 'stimulus'
  - display_order INT
  - created_at
  - INDEX (exam_id, display_order)

blueprint_templates (per tenant, library, reusable)
  - id, tenant_id, owner_user_id (per ADR-0009)
  - title, description
  - curriculum_id (k13/merdeka/akm_numerasi/akm_literasi)
  - subject_code, grade_or_phase
  - blueprint_type: reguler | akm_literasi | akm_numerasi
  - total_slots, total_points, strict_coverage (bool)
  - status: draft | published | archived
  - version (advisory)

blueprint_template_slots
  - id, template_id, position
  - competency_id (nullable FK to competencies)
  - competency_code (text fallback)
  - competency_description (text fallback)
  - materi (text)
  - indikator (text)
  - cognitive_level (text: C1..C6)
  - difficulty (text: mudah | sedang | sulit)
  - question_type (text: matches exam_questions.question_type)
  - points (numeric)
  - stimulus_id (nullable FK)
  - akm_konten, akm_konteks, akm_proses (text, populated only for AKM)
  - akm_level (1..5, populated only for AKM)

exam_blueprints (1:1 with exam, snapshot of template at clone time)
  - id, exam_id, source_template_id (nullable; null for from-scratch
    or reverse-generated)
  - source_template_version (snapshot)
  - all metadata fields mirrored from blueprint_templates
  - status: draft | locked (locked = parent exam is published)

exam_blueprint_slots
  - id, exam_blueprint_id, position
  - all fields mirrored from blueprint_template_slots
  - filled (computed in queries: TRUE iff exists exam_question
    with blueprint_slot_id = this.id)

exam_questions (Phase 9, extended)
  + blueprint_slot_id UUID nullable FK exam_blueprint_slots.id
  + group_id UUID nullable FK exam_question_groups.id
  - stimulus_id is REMOVED in favor of routing through group_id
    (a question's stimulus is its group's stimulus); cleaner one-way
    data flow, removes the dual-source ambiguity
```

### Why `exam_question_groups`

In AKM (literasi + numerasi), one stimulus passage spawns 3-5 questions
that must render together and never shuffle apart. A nullable FK on
question is structurally too weak: nothing prevents two AKM questions
that share a stimulus from being separated by exam-level shuffle, and
Phase 10 (take exam) would have to scan and re-cluster on every render.

The `exam_question_groups` table makes the cluster a first-class atomic
unit:
- Phase 10 renderer: "render each group; questions inside a group never
  shuffle relative to each other; group_type='standalone' shuffles like
  a single question."
- Reverse flow can populate group_id when it detects shared stimulus.
- Blueprint slots can reference a group when they belong together (e.g.
  AKM's 3 questions for one passage).

For non-AKM exams, every question gets a `group_type='standalone'`
group with no stimulus — same row count as having NULL group_id, but
uniform shape simplifies queries.

### Curriculum-aware terminology

Backend uses generic terms (`competency_code`, `competency_description`).
Frontend chooses a label based on `curriculum.code`:

| curriculum.code | Frontend label |
|---|---|
| `k13` | "KD" |
| `merdeka` | "CP" |
| `akm_numerasi` | "Konten" (the AKM dimension takes precedence) |
| `akm_literasi` | "Konten" |

This isolates curriculum vocabulary at the UI layer, so adding Cambridge
or IB later is just a new master row, not a schema change.

### Snapshot-on-clone (templates) and snapshot-on-use (stimuli)

Two independent snapshot points, both motivated by ADR-0002's
"snapshots are immutable" principle:

**Template clone snapshot** (when user applies template to exam):

1. INSERT INTO exam_blueprints copying all fields from
   blueprint_templates, plus `source_template_id` and `source_template_version`
2. INSERT INTO exam_blueprint_slots copying all fields from
   blueprint_template_slots
3. Exam now owns its blueprint. Subsequent template edits do not
   propagate.

**Stimulus content snapshot** (when stimulus is associated with a group):

When `exam_question_groups` is created with a `stimulus_id`, we copy
the stimulus's `title` and `content` into
`stimulus_title_snapshot` and `stimulus_body_snapshot` columns at that
instant. The library entry's content can be edited freely afterwards;
the exam's frozen snapshot is what students see.

Lifecycle of the stimulus snapshot:

| Exam state | Snapshot behavior |
|---|---|
| Draft | Group can be "refreshed" — admin clicks "Sync from library", new snapshot copied, audit logged |
| Published | Snapshot locked. "Sync from library" disabled in UI |
| Has any attempt | Snapshot absolutely locked. Even unpublishing back to draft does not re-enable sync (would silently change submitted exam content) |

This matches Phase 10's score-snapshot pattern (ADR-0002): once a
student has interacted with the content, content is frozen. Library
stays the live source for new exams; exams stay frozen for fairness.

### Slot ↔ Question linking

`exam_questions.blueprint_slot_id` is nullable to support:
- Freeform exams (no blueprint)
- Mixed exams (some questions in slots, some unaligned)
- Reverse-flow exams during analysis (slots created, link in progress)

When a question is linked to a slot, the slot is "filled". A slot can
have at most one question (`UNIQUE(blueprint_slot_id)` partial index
WHERE NOT NULL). A question can be unlinked or reassigned to a
different slot at any time.

### Coverage and publish gate

Every exam blueprint computes:

```
coverage = filled_slots / total_slots
distribution = { difficulty: {mudah, sedang, sulit},
                 type: {mc, tf, sa, essay},
                 cognitive_level: {C1..C6}, ... }
gaps = list of empty slots with their target spec
```

Surfaced via `GET /api/v1/exams/{id}/blueprint/coverage`.

Publish gate behavior:

- `strict_coverage = true` (e.g. AKM) → publish blocked at coverage < 100%
- `strict_coverage = false` (default) → publish dialog warns with gap
  list, allows override

### AI tools

Forward flow (slot → soal):

- `create_blueprint_template(tenant, ...)` — propose new template
- `add_blueprint_slot(template_id, spec)` — propose single slot addition
- `bulk_add_blueprint_slots(template_id, slots[])` — batch slots
- `clone_blueprint_to_exam(exam_id, template_id)` — propose clone
- `generate_question_for_slot(slot_id)` — single question proposal that
  reads the slot spec and constructs an enriched LLM prompt
- `bulk_generate_questions_for_slots(slot_ids[])` — batch with
  all-or-nothing validation per ADR-0008 batch convention

Reverse flow (soal → blueprint) — **proposal-first, two-step**:

The reverse flow MUST NOT auto-commit. Naive auto-creation of slots and
links generates noisy near-duplicate competencies that teachers spend
more time cleaning than writing manually. Mandatory two-step protocol:

**Step 1: `analyze_questions_to_blueprint(exam_id, opts)`**

Input options:
- `min_confidence` (float, default 0.7) — below this, slot is proposed
  WITHOUT a question link
- `batch_size` (int, default 10) — questions per LLM call (cap 50 per
  invocation; larger exams require section selection)

Return shape (no DB writes):
```json
{
  "proposed_slots": [
    { "position": 1, "competency_code": "3.1", "materi": "...",
      "cognitive_level": "C2", "difficulty": "mudah",
      "question_type": "multiple_choice", "points": 4,
      "akm_*": null }
  ],
  "proposed_links": [
    { "slot_position": 1, "question_id": "uuid", "confidence": 0.92,
      "reasoning": "Question text mentions pecahan campuran; option D shows conversion step" }
  ],
  "distribution_summary": {
    "by_difficulty": { "mudah": 5, "sedang": 8, "sulit": 7 },
    "by_cognitive_level": { "C1": 0, "C2": 4, "C3": 8 },
    "by_type": { "multiple_choice": 18, "essay": 2 }
  },
  "unlinked_questions": [
    { "question_id": "uuid", "reason": "low_confidence",
      "confidence": 0.42 }
  ]
}
```

Frontend renders a diff-review screen: confidence badges, AI reasoning
alongside each classification, amber warnings for items below the
threshold, and merge controls (e.g. "these two slots have nearly
identical competency, merge?").

**Step 2: `apply_blueprint_analysis(exam_id, accepted_slot_indices, accepted_link_indices, merge_decisions)`**

Only this step writes to DB:
- INSERT exam_blueprints (with source_template_id = NULL, marker
  `created_via = 'reverse_analysis'`)
- INSERT exam_blueprint_slots from accepted indices
- UPDATE exam_questions SET blueprint_slot_id WHERE accepted

Questions not accepted remain unlinked. Slots not accepted are not
created. User can iterate the analysis later (same `exam_id` re-runs
are allowed; only unanchored questions are re-classified).

**Confidence handling**: low-confidence (< threshold) means the slot
is proposed but the question link is omitted. User assigns manually
post-apply.

**Batching at scale**: 10 questions per LLM call, post-pass clusters
identical (competency, level, difficulty) signatures across batches
before returning. Cap 50 questions per single invocation; for 100+
question UAS exams, the tool requires `section_id` filter (run
separately per section).

### Dupe guards extension

Extend `ai_dupe_guards.go`:

- Two slots in same template with identical (competency_code, materi,
  indikator) signature → propose duplicate, reject
- Two pending `add_blueprint_slot` proposals in same session for same
  template with same signature → reject
- `clone_blueprint_to_exam` against exam that already has a blueprint
  → reject unless explicit `replace: true` flag

### Permission model

Per ADR-0009:

- `blueprint_templates` follow the same owner + collaborator model as
  exams and courses. Permissions: `blueprints:read`, `blueprints:write`.
- `exam_blueprints` are owned by their parent exam — no separate
  collaboration table. Editing the exam blueprint requires editor
  access on the parent exam.
- `stimuli` are tenant-shared but creator-owned for delete purposes.
  Anyone with `exams:write` can read shared stimuli; only the owner or
  tenant admin can edit/archive. Inline-created stimuli
  (`lifecycle = 'exam_scoped'`) are visible only inside their parent
  exam until explicitly promoted to `'shared'`.

### Atomic slot-question swap

Provide a dedicated endpoint:

```
PATCH /api/v1/exam-blueprint-slots/{slotId}/assign-question
Body: { "questionId": "uuid" | null }
```

In one transaction:
- If slot already had a question linked, set its `blueprint_slot_id` to NULL
- If `questionId` provided, set that question's `blueprint_slot_id` to slotId
- All within a single SQL transaction so the slot never has two
  questions or a stale link

The two-step "clear then set" pattern via two separate API calls is
forbidden — step 2 failure leaves the slot orphaned.

### Regenerate semantics for AI question generation

`generate_question_for_slot(slot_id, exclude_content_hashes?: []string)`
accepts an optional `exclude_content_hashes` parameter. When the user
rejects a generated question and asks for another attempt, the bot
passes the rejected question's content hash so the dupe guard does not
mistake the legitimate retry for a duplicate proposal.

This turns rejection into productive iteration rather than the bot
giving up after one rejection cycle.

## Alternatives considered

- **Embed blueprint inline as JSONB on exams** — clean for read, painful
  for queries (coverage report, distribution count, dupe detection).
  Rejected.
- **Cascade template edits to exam blueprints** — confusing for users
  whose published exams suddenly change. Rejected; snapshot wins.
- **Per-question stimulus copy** — duplicates content; loses ability to
  fix a typo once for all dependents. Rejected.
- **Single blueprint per template (no cloning)** — exams edit the
  template directly. Conflicts when two exams share a template. Rejected.
- **Only forward flow** — user told us reverse flow matches their actual
  workflow ("guru nulis dulu sambil mikir"). Building both is correct.
- **Block all AI question generation when no blueprint exists** — too
  rigid for quick quizzes. Defer to tenant settings ("require blueprint
  for exam type X").
- **Treat AKM as a separate resource type** — too much surface for
  Phase 9.5. Encode AKM as a `blueprint_type` discriminator instead.

## Consequences

Positive:
- AI question generation becomes high-quality: prompt enrichment from
  slot spec replaces hallucination
- Coverage report is a deterministic check, not a manual eyeball
- Reverse flow makes adoption realistic: schools with archives can
  retroactively standardize without rewriting questions
- Stimulus library enables AKM-style assessments without per-question
  duplication
- Templates accelerate routine exams (UTS / UAS / UH)
- Subject-decoupled permissions (per ADR-0009) make collaborative
  authoring trivial

Negative:
- Schema is large (5 new tables, 2 column additions). Review burden.
- Snapshot-on-clone means template improvements don't retroactively help
  past exams. Trade-off accepted.
- AI reverse flow may misclassify questions; mitigated by confidence
  threshold and manual override.
- Free-text competency until master data lands means typo-laden KD codes
  in early data. Mitigated by master FK being nullable + later
  reconciliation job.

Neutral:
- AKM dimensions are nullable in slot rows; queries must coalesce.
- The "exam_blueprints" table has a 1:1 relationship to exams; could
  technically be denormalized into exams. Kept separate for cleaner
  queries on coverage and distinct lifecycle (template version, source).

## Implementation

- `backend/migrations/000015_blueprints_stimuli.sql` — schema
- `backend/internal/app/blueprints.go` — template CRUD
- `backend/internal/app/blueprint_slots.go` — slot CRUD on templates and
  exam blueprints
- `backend/internal/app/stimuli.go` — stimulus library
- `backend/internal/app/exams.go` — coverage report endpoint, clone
  endpoint
- `backend/internal/app/questions.go` — extend with `blueprint_slot_id`,
  `stimulus_id` handling
- `backend/internal/app/ai_blueprint_tools.go` — 6 AI capabilities
- `backend/internal/app/ai_dupe_guards.go` — extend with slot dupe
  guard
- `backend/internal/app/ai_capabilities.go` — domain detection: tambah
  `blueprints` and `stimuli` keywords
- `backend/internal/platform/devseed/devseed.go` — preload curricula
- `frontend/src/lib/modules-api.ts` — extensive
- `frontend/src/app/(app)/app/blueprints/*` — library + detail pages
- `frontend/src/app/(app)/app/stimuli/page.tsx` — library
- `frontend/src/app/(app)/app/exams/[id]/page.tsx` — Kisi-Kisi tab,
  Stimuli tab, coverage badge
- `frontend/src/components/blueprint/*` — slot editor, slot picker,
  coverage badge

## Verification

- Smoke test driver covering forward flow + reverse flow + AKM scenario
- Cross-tenant negative test
- Backend unit tests:
  - slot dupe detection
  - coverage computation
  - clone snapshot integrity (template edit does not affect exam)
  - terminology mapping (k13 → KD label)

## Future work

- Curriculum CSV importer (Permendikbud preload)
- Stimulus media (image, audio, video)
- AKM proficiency scoring rubrics
- Question bank as standalone resource that blueprint slots can pull
  from
- Cross-exam template extraction (`extend_blueprint_from_recent_exams`)
- AI tool to suggest stimulus given a topic
- Per-exam blueprint diff view ("what changed since cloned?")
