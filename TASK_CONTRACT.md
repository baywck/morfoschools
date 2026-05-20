# TASK_CONTRACT.md

Feature: **phase-9-5-blueprints-collab**
Branch: `feature/phase-9-5-blueprints-collab`
Worktree: `/home/bayw/Documents/Morfosis/morfoschools-phase-9-5-blueprints-collab`
Forked from: `feature/phase-9-exams` (depends on Phase 9 schema)
Size: L
Created: 2026-05-19

## Goal

Add Indonesian curriculum-aware **kisi-kisi (assessment blueprints)** to
exams, with reusable templates, AKM/ANBK structural support, text
stimuli library, AI generation in both directions (slot→soal forward
flow and soal→blueprint reverse flow), and an explicit ownership +
collaboration model on exams, courses, and blueprint templates.

## Scope

In scope:

### Collaboration model (ADR-0009)

- `*.owner_user_id` columns on exams, courses, blueprint_templates
  (defaults to created_by, backfilled in migration after pre-flight audit)
- Pre-flight audit: detect rows with NULL `created_by` or `created_by`
  pointing to archived/missing users. Migration aborts loud with a
  list of orphan rows. Operator resolves via `/admin/orphaned-resources`
  before re-running migration.
- Collaborator tables: `exam_collaborators`, `course_collaborators`,
  `blueprint_template_collaborators` with role `editor` | `viewer`
- Permission helpers: `requireExamAccess(action)`,
  `requireCourseAccess(action)`, `requireBlueprintAccess(action)` that
  evaluate layered access:
  tenant admin > owner > editor > viewer > subject institutional
  fallback (read-only, exam-only) > 404
- Subject-based RBAC is **preserved** (not deleted) as institutional
  read fallback for waka kurikulum / ketua MGMP patterns. To grant
  edit access via subject membership, school must promote them to
  `academic_admin` role.
- Endpoints to manage collaborators (invite, change role, remove,
  transfer ownership)
- `canAccess` field on every list response (exams, courses, blueprints)
  computed server-side; frontend uses to disable cards/show lock icons
- Take-exam path (Phase 10) is OUT of collaboration scope: enrollment-
  based access only, must not call collab helpers
- Permissions: `blueprints:read`, `blueprints:write` added to seed

### Blueprints (ADR-0010)

- `curricula` master table (preload K13, Merdeka, AKM_Numerasi, AKM_Literasi)
- `competencies` table with NOT NULL FK to curricula, plus
  `normalized_code` for dedup
- `stimuli` table with lifecycle states: `exam_scoped` | `shared` |
  `archived`. Inline-created defaults to `exam_scoped`; explicit
  promotion to `shared` makes it library-wide. `usage_count` cached.
- `exam_question_groups` (NEW): atomic shuffle units. Required for
  AKM (3-5 questions per stimulus must render together). For non-AKM,
  every question gets a `standalone` group for query uniformity.
- `blueprint_templates` (library, reusable per tenant)
- `blueprint_template_slots` (the structural definition)
- `exam_blueprints` (clone-on-create from template, owned by exam)
- `exam_blueprint_slots` (clone of template slots)
- `exam_questions` extended:
  + `blueprint_slot_id` (additive nullable FK)
  + `group_id` (additive nullable FK — replaces direct `stimulus_id`
    so stimulus is sourced through group)
- Stimulus content snapshot: `exam_question_groups` carries
  `stimulus_title_snapshot` and `stimulus_body_snapshot`. Lifecycle:
  draft (refresh allowed), published (locked), attempted (absolutely
  locked).
- Status field for blueprint: `draft` | `locked` (locked = published exam)
- Coverage report: filled vs total, distribution check vs target
- Atomic slot-question swap endpoint:
  `PATCH /exam-blueprint-slots/{id}/assign-question` (single tx)

### AI tools (Must + Should scope from earlier discussion)

Must:
- `create_blueprint_template` — fresh template
- `add_blueprint_slot` / `bulk_add_blueprint_slots` — fill template
- `clone_blueprint_to_exam` — apply template to exam
- `generate_question_for_slot(slot_id, exclude_content_hashes?)` —
  forward flow with regenerate semantic for productive retry after
  rejection

Should:
- `bulk_generate_questions_for_slots` — batch forward flow with
  all-or-nothing validation
- `analyze_questions_to_blueprint(exam_id, min_confidence?, batch_size?)`
  — PROPOSAL ONLY, no DB writes. Returns proposed_slots,
  proposed_links with confidence + reasoning, distribution_summary,
  unlinked_questions. Cap 50 questions per invocation.
- `apply_blueprint_analysis(exam_id, accepted_slot_indices, accepted_link_indices, merge_decisions)`
  — the second step that actually persists. Without this companion
  tool, reverse flow does not complete.

Defer to Phase 9.6+:
- `extend_blueprint_from_recent_exams`
- `suggest_stimulus_from_text`

### Stimuli (text-only)

- CRUD endpoints, library page
- Stimulus is associated with `exam_question_groups`, not directly
  with questions (a question's stimulus is its group's stimulus)
- Snapshot semantics: title + body copied into `exam_question_groups`
  at use time; library can be edited, exam stays frozen
- Markdown content, optional `source` citation field
- Lifecycle: `exam_scoped` (private to parent exam) | `shared` (library-
  wide reusable) | `archived`. Inline-created default to exam_scoped
  to avoid library pollution; explicit promotion needed for sharing.

### Frontend

- `/app/blueprints` — template library page
- `/app/blueprints/[id]` — template detail with slot table editor
- `/app/exams/[id]` — new tab "Kisi-Kisi" + new tab "Stimuli"
- `/app/stimuli` — library page
- Question form: when exam has a blueprint, dropdown "Slot" appears,
  selecting a slot auto-fills metadata fields (KD, level, etc.) and
  locks them
- Share dialog: invite teacher as editor/viewer, list current
  collaborators, transfer ownership (owner only), leave (collaborator
  only)
- Coverage indicator on exam header: `Coverage: 18/20 (90%)` with
  warning color when below 100%

### Documentation

- ADR-0009-collaboration-model.md
- ADR-0010-blueprints-kisi-kisi.md
- `.ai/api/blueprints.md` (new)
- `.ai/api/stimuli.md` (new)
- `.ai/api/collaborators.md` (new — shared section across resources)
- `.ai/api/exams.md` updates (new tabs, collab endpoints, blueprint FK)
- `.ai/api/_index.md` updates
- `.ai/standards/ai-tool-guards.md` updated with blueprint dedup pattern

## Out of scope (defer)

- **Stimulus media** (image/audio/video upload) — Phase 9.6
- **Curriculum CSV importer** — Phase 9.6 or 10
- **Cross-exam aggregation tools** (`extend_blueprint_from_recent_exams`)
- **Question bank** as standalone resource
- **Take-exam flow** (Phase 10)
- **Grading + monitor** (Phase 11)
- **AKM official rubrics** — we capture metadata fields but no automatic
  scoring rubric application

## Acceptance criteria

### Collaboration

- [ ] Owner can invite teacher A as editor, teacher A appears in `/app/exams/[id]` collaborators dialog
- [ ] Teacher A can edit exam metadata, sections, questions, blueprint
- [ ] Teacher A cannot invite teacher B (only owner can)
- [ ] Teacher B (no role) sees the exam in list but gets 404 on detail
- [ ] Tenant admin sees + can edit all exams in tenant regardless of ownership
- [ ] Owner can transfer ownership to teacher A; teacher A becomes new owner, original owner becomes editor
- [ ] Same flow works for courses and blueprint templates
- [ ] AI bot acting as teacher A respects the same access rules — 403 returned and bot self-corrects

### Blueprints — forward flow

- [ ] Admin creates blueprint template "UTS Matematika kelas 10 K13" with 20 slots
- [ ] Admin clones the template into a fresh exam → exam has 20 empty slots
- [ ] Admin opens question form, selects slot 1 → KD/materi/level/type/points fields auto-filled and locked
- [ ] Admin saves → question is linked to slot 1, slot status flips to `filled`
- [ ] Coverage indicator updates `1/20 (5%)`
- [ ] Strict-coverage blueprint (e.g. AKM type) blocks publish at 95% coverage
- [ ] Lenient blueprint shows publish dialog with empty-slot list, allows override

### Blueprints — reverse flow

- [ ] Existing exam with 20 questions, no blueprint
- [ ] User clicks "Generate kisi-kisi from questions (AI)"
- [ ] AI proposes blueprint with 20 slots, each linked to its source question
- [ ] User confirms → blueprint becomes part of exam, all 20 questions auto-linked to their slot
- [ ] Coverage indicator shows `20/20 (100%)`
- [ ] Distribution report displayed: count by KD, level, difficulty, type

### AKM/ANBK

- [ ] Blueprint template type can be set to `akm_literasi` or `akm_numerasi`
- [ ] Slot UI shows AKM-specific fields (Konten, Konteks, Proses Kognitif, Level 1-5)
- [ ] Stimulus dropdown is required-feeling for AKM slots (warned if empty)

### Stimuli

- [ ] Admin creates a stimulus "Sampah Plastik di Laut" (markdown article)
- [ ] Question creation form has stimulus dropdown
- [ ] Multiple questions in same exam share one stimulus
- [ ] Stimulus listed in stimuli library page
- [ ] Stimulus archive does not delete linked questions

## Likely affected files

Backend:
- `backend/migrations/000014_collaboration.sql` (new)
- `backend/migrations/000015_blueprints_stimuli.sql` (new)
- `backend/internal/app/collaborators.go` (new — shared helpers)
- `backend/internal/app/blueprints.go` (new)
- `backend/internal/app/blueprint_slots.go` (new)
- `backend/internal/app/stimuli.go` (new)
- `backend/internal/app/exams.go` (modify — owner field, replace
  requireExamSubjectAccess with new helpers)
- `backend/internal/app/courses.go` (modify — owner field)
- `backend/internal/app/questions.go` (modify — slot_id FK, stimulus_id FK)
- `backend/internal/app/ai_blueprint_tools.go` (new)
- `backend/internal/app/ai_dupe_guards.go` (extend with blueprint slot
  dedup)
- `backend/internal/app/ai_capabilities.go` (extend domain detection)
- `backend/internal/app/ai_cap_registry.go` (register blueprint capabilities)
- `backend/internal/app/app.go` (route registration)
- `backend/internal/platform/devseed/devseed.go` (seed permissions
  blueprints:read, blueprints:write; preload curricula)

Frontend:
- `frontend/src/lib/modules-api.ts` (extensive additions)
- `frontend/src/app/(app)/app/blueprints/page.tsx` (new)
- `frontend/src/app/(app)/app/blueprints/[id]/page.tsx` (new)
- `frontend/src/app/(app)/app/stimuli/page.tsx` (new)
- `frontend/src/app/(app)/app/exams/[id]/page.tsx` (modify — add
  Kisi-Kisi tab, Stimuli tab, share dialog, coverage indicator)
- `frontend/src/app/(app)/layout.tsx` (modify — sidebar nav: Blueprints,
  Stimuli)
- `frontend/src/components/share-dialog.tsx` (new — reusable for
  exam/course/blueprint)
- `frontend/src/components/blueprint/slot-editor.tsx` (new)
- `frontend/src/components/blueprint/coverage-badge.tsx` (new)
- `frontend/src/components/blueprint/slot-picker.tsx` (new — dropdown
  for question form)

Docs:
- `.ai/adr/0009-collaboration-model.md` (new)
- `.ai/adr/0010-blueprints-kisi-kisi.md` (new)
- `.ai/api/blueprints.md` (new)
- `.ai/api/stimuli.md` (new)
- `.ai/api/collaborators.md` (new)
- `.ai/api/exams.md` (extend)
- `.ai/api/_index.md` (extend)
- `.ai/standards/ai-tool-guards.md` (extend)

## Forbidden changes

- [ ] Do not refactor unrelated modules
- [ ] Do not change migrations 000001 through 000013 — additive only
- [ ] Do not modify Phase 9 exam handlers beyond owner field + new
  permission helper integration
- [ ] Do not introduce stimulus media upload — text only
- [ ] Do not introduce curriculum master CSV importer — admin entry only
- [ ] Do not add take-exam endpoints — Phase 10
- [ ] Do not change UI primitives (`InputField`, `SelectField`,
  `RightPullSheet`, `DateTimePicker`, etc.) beyond extending props if
  needed; reuse existing components
- [ ] No native form controls; floating labels only

## Verification plan

Tier 3 (auth-adjacent because of permission model change). Required:

- `go vet ./...` clean
- `go build ./...` clean
- `go test ./...` clean — including new unit tests for `requireExamAccess`
  variants and blueprint slot validation
- `tsc --noEmit` clean
- Smoke test driver `/tmp/smoke-blueprints.sh` covering all acceptance
  criteria
- Manual UI smoke for share dialog and coverage indicator
- Cross-tenant negative test re-run (already passing in Phase 9, ensure
  no regression)
- AI bot dry run: chat scenarios:
  - "buatkan template kisi-kisi UTS Matematika kelas 10 K13"
  - "isi semua slot kosong di exam ini"
  - "generate kisi-kisi dari soal exam ini"

Manual confirmation before commit per Tier 3 protocol.

## Notes

- Backend code style: continue using raw `database/sql`, no ORM
- Schema: free-text competency fields with FK columns (nullable) to
  competency master so future migration to master-data does not break
- Blueprint clone-on-create is snapshot, not reference — template edits
  do not affect existing exam blueprints
- AI write tools follow existing `propose → confirm → execute` flow
  with dupe guards from `ai_dupe_guards.go`
- All collaborator tables have `(resource_id, user_id)` unique
  constraint so a user can have at most one role per resource
