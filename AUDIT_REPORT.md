# AUDIT REPORT — Phase 9.5 Blueprints + Collaboration

Base: `05a05fc`  
Branch: `feature/phase-9-5-blueprints-collab`  
Scope: 36 changed files, +9347/-138. Diff is large (>1000 lines); merge review should be split by backend auth/schema, AI tools, and frontend UI if time allows.

## PHASE 9.9 — Exam authoring polish (rich editor + LaTeX + proper toggle + accordion polish + group fix) — 2026-05-19

Four interlocking polish items shipped in one commit on top of Phase 9.8:

1. **Add Group fix.** Pre-9.9 `handleAddGroup` discarded the section context (`void sectionId`) so newly created groups landed without a `section_id`. Backend `handleCreateQuestionGroup` now accepts an optional `sectionId` in the body, validates it belongs to the same exam + tenant, and falls back to the exam's first section (lowest sort_order, oldest created_at) when omitted — matching the Phase 9.8 section-mandatory rule. `handleUpdateQuestionGroup` accepts a `sectionId` in PATCH bodies for cross-section group moves (also unlocks the deferred "group → section drag-drop" item from Phase 9.7). Frontend `createQuestionGroup` helper exposes the new field; `ExamCanvas.handleAddGroup(sectionId)` now actually forwards the value. The new group renders inside its owning section card via the existing `reload()` path. UX fallback option (a) chosen: empty group card with `+ Tambah stimulus` placeholder, no naming prompt — consistent with `+ Tambah Soal`.

2. **Question accordion field order + collapsible Kisi-Kisi.** New strict order: Tipe Soal + Poin (top-right metadata row) → Stimulus (collapsible) → Konten Soal (rich editor) → Opsi (MCQ/TF only) → Penjelasan (rich editor) → Kisi-Kisi (collapsible, default closed). The `KisiKisiSection` wrapper carries a ClipboardList icon, a chevron that rotates on toggle, and a counter pill `X/N` showing how many fields are filled (5 fields for reguler, 7 for AKM). When the slot is locked from a template the header surfaces a Lock icon + `Terkunci` chip and the inner `KisiKisiInline` fields render read-only. Points input relocated from the metadata grid to a 1fr/120px split next to Tipe Soal so type + points sit on the same row.

3. **Rich text editor with LaTeX support.** New primitive `frontend/src/components/ui/rich-editor.tsx` based on TipTap (`@tiptap/react@2.10.3` + StarterKit + Image + Link + Placeholder), with inline + block math via `@aarkue/tiptap-math-extension@1.3.6` and KaTeX rendering. Toolbar buttons: Bold / Italic / Strike / H2 / H3 / Bullet / Numbered / Link / Image / Math (inline) / Math (block) / Clear formatting. Storage shape is HTML via TipTap's `getHTML()`; the math extension preserves the raw LaTeX in the document JSON so reload re-renders intact. New companion `rendered-content.tsx` sanitizes HTML via DOMPurify (3.2.4) and falls back to `<p>` wrapping for legacy plain-text content (`isHtmlContent` heuristic). `stripHtmlPreview` helper strips tags + truncates for the collapsed accordion header so HTML never leaks into compact previews. KaTeX CSS is imported once in `app/layout.tsx` (and again at module level in the editor / renderer for type safety; the bundler dedupes). Migrated surfaces:
   - `QuestionAccordion` content + explanation textareas → RichEditor.
   - `StimulusPicker` "Tulis Baru" body textarea → RichEditor; library row preview uses RenderedContent.
   - `GroupCard` snapshot details preview → RenderedContent; collapsed header truncation via `stripHtmlPreview`.
   - `app/(app)/app/stimuli/page.tsx` view-sheet body → RenderedContent.

4. **Proper Toggle component for Kisi-Kisi.** New primitive `frontend/src/components/ui/toggle-switch.tsx` (iOS-style pill track + sliding thumb, `role=switch`, `aria-checked`, keyboard space/enter, sm + md sizes, optional label + trailing slot). `KisiKisiToggle` rebuilt around it: "Kisi-Kisi" label on the left, switch on the right, lock icon trailing when the exam isn't draft. Confirm-dialog flow on turn-off preserved.

### What ships

**Backend** (small surface): `backend/internal/app/question_moves.go` (sectionId in create + update bodies, default-to-first-section fallback, validation against exam + tenant). New tests `backend/internal/app/group_section_test.go` (5 helper assertions: defaults, foreign-section rejection, valid-section acceptance, no-section error, update accept/reject/no-op/clear).

**Frontend deps** (all pinned, no carets): `@tiptap/react@2.10.3`, `@tiptap/starter-kit@2.10.3`, `@tiptap/extension-image@2.10.3`, `@tiptap/extension-link@2.10.3`, `@tiptap/extension-placeholder@2.10.3`, `@aarkue/tiptap-math-extension@1.3.6`, `katex@0.16.11`, `dompurify@3.2.4`, dev `@types/dompurify@3.0.5`.

**Frontend new primitives**: `components/ui/rich-editor.tsx`, `components/ui/rendered-content.tsx`, `components/ui/toggle-switch.tsx`.

**Frontend wiring**: `lib/modules-api.ts` (`CreateQuestionGroupPayload.sectionId`, `UpdateQuestionGroupPayload.sectionId`, response carries `sectionId`), `components/exams/exam-canvas.tsx` (`handleAddGroup` forwards sectionId, `void sectionId` hack removed), `components/exams/question-accordion.tsx` (field reorder + collapsible KisiKisi + RichEditor on content/explanation + KK fill counter), `components/exams/stimulus-picker.tsx` (RichEditor on Tulis-Baru body, RenderedContent on library row preview, blank-doc detection for the disabled-button check), `components/exams/group-card.tsx` (RenderedContent on snapshot, `stripHtmlPreview` on header), `components/exams/kisi-kisi-toggle.tsx` (rewritten around ToggleSwitch), `app/(app)/app/stimuli/page.tsx` (view-sheet preview), `app/layout.tsx` (KaTeX CSS import).

### Verification

- `go vet ./...` clean.
- `go build ./...` clean.
- `go test ./...` clean (5 new helper tests in `group_section_test.go`).
- `npx tsc --noEmit` clean.
- `docker compose up -d --build backend` succeeded; container restarted clean.
- `curl http://127.0.0.1:8080/readyz` → 200.

### Audit/safety notes

- Group create now enforces section-membership at the validation layer; foreign-section ids return a structured 422 with `fields.sectionId`.
- Group update gains the same gate for the section-move path; empty string clears the binding to match the question-move semantics.
- All HTML rendered from user-authored fields is run through DOMPurify before injection; the allow-list permits the KaTeX MathML node names so rendered formulae keep their layout.
- TipTap's `Link` extension is configured with `target=_blank rel=noopener noreferrer` so library content doesn't open windows back into the host document.
- KaTeX CSS imported once at the app layout level (and idempotently at module scope where needed for bundler graph reasons).
- `Lint` step skipped: project's `npm run lint` triggers the legacy interactive `next lint` setup that the repo hasn't migrated yet; documented in audit but not blocking for this Tier 2 polish.

### Deferred

- Migrating `app/(app)/app/stimuli/page.tsx` create / edit textareas to RichEditor (out of scope for the four targeted changes; the read surface is migrated, write surface remains plain textarea with markdown).
- ESLint migration to the new Next.js v16 CLI flow (separate infra task).

## PHASE 9.7 — Kisi-Kisi Toggle + Stimulus Axis (ADR-0012, supersedes ADR-0011) — 2026-05-19

The 3-mode `authoring_mode` model from ADR-0011 / Phase 9.6 was replaced one
week into use by a cleaner 3-axis model:

- **Axis 1** — `exams.uses_kisi_kisi` (boolean toggle). Replaces the
  authoring_mode discriminator. `false` = flat list, no slot enforcement;
  `true` = every question must bind to a blueprint slot, publish gate
  enforces coverage when the blueprint has `strict_coverage=true`.
- **Axis 2** — AKM detection from existing `exam_blueprints.blueprint_type IN
  ('akm_literasi','akm_numerasi')`. No exam-level AKM column. The frontend
  reads `blueprintType` from `/slots-with-questions` and renders
  Konten/Konteks/Proses Kognitif/Level labels when applicable.
- **Axis 3** — Universal stimulus attachment: every question can carry a
  stimulus through `exam_questions.stimulus_id` (direct, new column) OR
  `exam_questions.group_id` (group-mediated, ADR-0010 path). Mutually
  exclusive at both the SQL CHECK and handler-validation layers.

Why the rewrite: AKM is structurally a property of the blueprint, not the
exam; encoding it at the exam level forced two sources of truth that drifted
on every clone / reverse-flow / mode flip. Stimulus needed to be orthogonal so
quick harian quizzes can attach a passage without forcing AKM mode. Drag-and-
drop reorder also wanted to be visual-only with slot binding preserved—
something the 3-mode model conflated.

Deliverables:

- **Schema (Phase A):** `backend/migrations/000017_kisi_kisi_toggle_and_stimulus.sql`
  drops the `authoring_mode` column + index, adds `uses_kisi_kisi BOOLEAN
  NOT NULL DEFAULT false` (backfilled from blueprint presence), adds
  `exam_questions.stimulus_id UUID REFERENCES stimuli`, partial index, and a
  CHECK constraint enforcing `stimulus_id` XOR `group_id`. Forward-only;
  dev data is reconstructible from blueprint presence + blueprint_type.
  ADR-0012 records the decision; ADR-0011 receives a SUPERSEDED banner for
  traceability.
- **Backend (Phase B):** `exams.go` examRow JSON exposes `usesKisiKisi`;
  create/update accept the bool; transition rules locked behind
  draft-or-platform-admin; publish gate simplified to
  `usesKisiKisi && bpStrict && filled<total → 422`. `questions.go` replaces
  `loadExamAuthoringMode` with `loadExamUsesKisiKisi`; create/update accept
  `stimulusId` OR `groupId` with mutex 422; list response embeds `stimulus`
  and `group` summaries when set. New `question_moves.go` adds
  `POST /api/v1/exams/{id}/questions/move` (atomic move; slot binding
  preserved), `POST /api/v1/exams/{id}/groups`, `PATCH /api/v1/groups/{id}`,
  `DELETE /api/v1/groups/{id}`. `blueprint_clone.go` flips uses_kisi_kisi
  on clone and runs best-effort AKM auto-grouping when the template's slots
  carry stimulus_id refs (via new `autoCreateAkmGroups` helper).
  `blueprint_slots.go` /slots-with-questions response carries
  `blueprintType` and per-question stimulus/group summaries.
  `ai_blueprint_tools.go` renames `set_authoring_mode` capability →
  `set_uses_kisi_kisi` (args `{examId, enabled}`); `clone_blueprint_to_exam`
  and `convert_questions_to_kisi_kisi` flip `uses_kisi_kisi=true` instead of
  rotating the old enum. Tests rewritten in `kisi_kisi_test.go`
  (TestCreateQuestion_RequiresSlotWhenKisiKisiOn,
  TestCreateQuestion_AllowsNoSlotWhenKisiKisiOff,
  TestUpdateExam_DisableKisiKisi_DraftOnly,
  TestUpdateExam_KisiKisiToggleImmutableAfterPublish,
  TestQuestion_StimulusAndGroupMutuallyExclusive,
  TestMoveQuestion_PreservesSlotBinding).
- **Frontend (Phase C):** `package.json` pins `@dnd-kit/core@6.1.0`,
  `@dnd-kit/sortable@8.0.0`, `@dnd-kit/utilities@3.2.2` (exact versions per
  repo convention). `modules-api.ts` swaps `AuthoringMode` for
  `usesKisiKisi`/`updateExamKisiKisi`, adds `moveQuestion`,
  `createQuestionGroup`, `updateQuestionGroup`, `deleteQuestionGroup`, and
  extends `Question` + `SlotsWithQuestionsResponse` with stimulus/group
  summaries plus `blueprintType`. New `kisi-kisi-toggle.tsx` replaces the
  3-radio `mode-switch-modal.tsx` (deleted). `slot-first-canvas.tsx` rewired
  with @dnd-kit DndContext + SortableContext, drag handle on each card,
  AKM-aware metadata header (Konten/Konteks/Proses/Level vs Cog/Diff), and
  stimulus/group summaries on filled question rows. Drag reorders only
  `sortOrder` via `moveQuestion`; slot binding never changes via D&D.
  `/app/exams/[id]/page.tsx` swaps the mode pill for the toggle, adds an
  AKM badge derived from `blueprintType`, and adds a stimulus picker
  (radio: none / direct / group) to the question form regardless of
  kisi-kisi state. The unimported `exam-blueprint-section.tsx` is deleted.
- **AI tools (Phase D):** `set_authoring_mode` removed. `set_uses_kisi_kisi`
  registered with proposal-then-execute flow (capSetUsesKisiKisi /
  execSetUsesKisiKisi); switch in `ai_write_tools.go` updated.
  `convert_questions_to_kisi_kisi` and `clone_blueprint_to_exam` now flip
  `uses_kisi_kisi=true` and rely on blueprint_type for AKM signaling. System
  prompt was already toggle-agnostic; no edit needed.

Verification: `go vet ./...` clean, `go build ./...` clean, `go test ./...`
clean (6 unit tests in `kisi_kisi_test.go`), `npx tsc --noEmit` clean,
migration 000017 applied on `docker compose up -d --build backend`,
`/readyz` returns 200. No `authoring_mode` / `authoringMode` references
remain anywhere in live code (only historical ADR-0011 + migrations 000016
and 000017 reference the old name for traceability).

Open items deferred:

- **Group list endpoint for the question form picker** — the form's group
  dropdown only shows the currently-bound group (when editing). New groups
  must be created via the canvas toolbar ("+ Group" UI is present in the
  ADR but the canvas-side toolbar wiring is follow-up polish; the API
  endpoints are live and reachable from chat).
- **Section + group canvas toolbar** — the /move endpoint, group CRUD, and
  AKM auto-grouping all ship; the canvas currently exercises drag-reorder
  within slots only. Section-band drop targets and group-as-container
  drop targets are visual additions tracked as the next polish pass.
- **Question Bank** (Council Idea #5) — still deferred per ADR-0011 future
  work; not a 9.7 deliverable.

## PHASE 9.6 — Authoring Modes (ADR-0011, SUPERSEDED) — 2026-05-19

Unified Questions + Kisi-Kisi authoring landed on this branch. Single content
area at `/app/exams/[id]` now switches presentation by `exams.authoring_mode`
(quick / structured / akm) instead of forcing the teacher between two parallel
tabs.

Deliverables (all phases A–D shipped, verification clean):

- **Schema (Phase A):** `backend/migrations/000016_authoring_mode.sql` adds the
  `authoring_mode TEXT NOT NULL DEFAULT 'quick'` column with CHECK enum +
  backfill from existing blueprints (AKM blueprints → 'akm', other blueprints
  → 'structured', everything else → 'quick'). Index on (tenant_id,
  authoring_mode). ADR-0011 records the decision (Option A1 — nullable column,
  handler enforcement) and the alternatives considered (A2 partial CHECK, A3
  table split).
- **Backend (Phase B):** `exams.go` examRow JSON exposes `authoringMode`;
  create/update accept it; mode transitions on non-draft exams require platform
  admin override; downgrade structured→quick keeps slots and surfaces a
  warning. `questions.go` enforces slot binding in structured/akm and adds the
  `POST /api/v1/exams/{id}/questions/from-slot` endpoint with the slot's
  questionType + points as defaults. `blueprint_slots.go` adds
  `GET /api/v1/exams/{id}/slots-with-questions` for the slot-first canvas.
  `blueprint_clone.go` and the AI clone executor auto-promote authoring_mode.
  Publish gate consults the mode (quick bypasses kisi-kisi gate; structured
  honors strict_coverage; akm always strict).
- **Frontend (Phase C):** `modules-api.ts` extended with `AuthoringMode`,
  `updateExamMode`, `getSlotsWithQuestions`, `createQuestionFromSlot`, plus a
  `slot` nested object on Question. New components
  `components/blueprint/slot-first-canvas.tsx` and
  `components/blueprint/mode-switch-modal.tsx`. `/app/exams/[id]/page.tsx`
  rebuilt around mode-driven content area + Mode pill in header. The legacy
  Kisi-Kisi tab is retired; ExamBlueprintSection component is no longer wired
  in but kept on disk for reference (no delete to honor "don't refactor
  unrelated code").
- **AI tools (Phase D):** new caps `set_authoring_mode` (write, exams:write)
  and `convert_questions_to_kisi_kisi` (write, blueprints:write + exams:write).
  Reverse-flow wrapper auto-accepts proposals at confidence ≥ minConfidence
  (default 0.7) and delegates to `apply_blueprint_analysis`. Both capabilities
  use the existing `checkAIExamAccess` / `loadAIAuth` helpers and create
  proposals via `createProposal` for user confirmation. Switch in
  `ai_write_tools.go` extended.

Verification: `go vet ./...` clean, `go build ./...` clean, `go test ./...`
clean (6 new unit tests in `authoring_mode_test.go`), `npx tsc --noEmit` clean.
Migration 16 backfill is idempotent; existing seed exams without blueprints
stay in 'quick'.

Open items deferred (per ADR-0011 Future Work + council Idea #5):

- **Question Bank** as a standalone resource that blueprint slots can pull
  from. Defer until a tenant has 5+ exams reusing similar kisi-kisi.
- **AI "Generate via AI" button on empty slot cards** is currently a passive
  placeholder — the chat panel handles the proposal flow. A direct in-canvas
  trigger that opens the chat with `generate_question_for_slot` pre-armed is
  follow-up polish.
- **Mode-filtered list pages** (e.g. "all akm exams") not yet wired into
  `/app/exams`. The index column + DB index are in place; UI filter follow-up.

## FIX STATUS — 2026-05-19 follow-up commit

All 8 Tier 3 blockers below have been remediated on this branch. Verification re-ran clean: `go vet`, `go build`, `go test`, `tsc --noEmit`.

| # | Blocker | Fix |
|---|---------|-----|
| 1 | AI authz bypass in blueprint capabilities | New `backend/internal/app/ai_blueprint_access.go`: `loadAIAuth`, `checkAIExamAccess`, `checkAIBlueprintAccess`, `resolveSlotParentExam`. Wired into `capAnalyzeQuestionsToBlueprint` (`ai_blueprint_tools.go:540` Read), `capGetBlueprintTemplate` (Read), `capGetExamBlueprintAI` (Read), `capListExamBlueprintSlotsAI` (Read), `capAddBlueprintSlot` (Write template), `capBulkAddBlueprintSlots` (Write template), `capCloneBlueprintToExamAI` (Write exam + Read template), `capGenerateQuestionForSlot` (Write exam — slot→exam resolution), `capApplyBlueprintAnalysis` (Write exam). Also wired into the executors (`execAddBlueprintSlot`, `execBulkAddBlueprintSlots`, `execCloneBlueprintToExam`, `execGenerateQuestionForSlot`, `execApplyBlueprintAnalysis`) so a stale propose-then-revoke window cannot leak access. Permission string on `get_blueprint_template` capability bumped from `exams:read` to `blueprints:read`. |
| 2 | Stimulus exam-scoped privacy broken | `backend/internal/app/stimuli.go` `handleListStimuli`: `lifecycle=all` and `lifecycle=archived` now restricted to tenant admins; `lifecycle=exam_scoped` requires `parentExamId` AND `requireExamAccess(ActionRead)` on the parent exam; unknown values rejected with 400. `handleGetStimulus`: lifecycle-aware gate at the read site — shared is open to `exams:read`, exam_scoped requires parent-exam read access (defensive 404 when parent missing), archived owner-or-admin only. |
| 3 | Publish gate strict coverage + lock | `backend/internal/app/exams.go` `handlePublishExam`: computes `filled vs total` for the exam's draft `exam_blueprints` row; if `strict_coverage=true` and `filled < total`, returns 422 with `{coverage: "X filled of Y …"}` field. On successful publish, the exam status flip and `exam_blueprints SET status='locked'` happen inside one transaction. |
| 4 | Reverse-flow analyze shape | `ai_blueprint_tools.go` `capAnalyzeQuestionsToBlueprint`: response now matches ADR-0010 — `proposedSlots`, `proposedLinks` (with `questionIndex`/`slotIndex`/`confidence`/`reasoning`), `distributionSummary` (`byKD`, `byLevel`, `byDifficulty`, `byType`), `unlinkedQuestions` (low-confidence questions are NOT linked but DO produce a slot), `questionLimit:50`, `minConfidence` (default 0.5), `batchSize` (default 50, cap 50). Read-only — no DB writes. `capApplyBlueprintAnalysis` schema accepts the ADR's `acceptedSlotIndices`, `acceptedLinkIndices`, `mergeDecisions` (validated for index range and self-merge) in addition to the existing `acceptedSlots`. |
| 5 | Stimulus lifecycle locks | `stimuli.go` `handleUpdateStimulus`: title/content edits go through `stimulusContentLocked` which blocks any change while a linked `exam_question_groups` row sits inside a published exam OR an exam with at least one row in `exam_attempts`. Source-only edits remain free (snapshot doesn't store source). New endpoint `POST /api/v1/stimuli/{id}/sync-snapshot` (`handleSyncStimulusSnapshot`) refreshes every linked group's snapshot from the live library — allowed only when ALL linked exams are draft AND have zero attempts. |
| 6 | Phase 9 retrofit incomplete | `exams.go` `handleCreateExam`: `requireExamSubjectAccess` removed from create. The only access gates are `RequirePermission("exams:write")`, `RequireEffectiveTenant`, `RequireCSRF`. Subject is now optional metadata; we still validate it belongs to the tenant (existence check) so the FK insert can't fail and so the read fallback in `resolveExamAccess` continues to work. The created exam's `owner_user_id` is the caller. |
| 7 | Slot dupe guards | `backend/internal/app/ai_dupe_guards.go` `checkBlueprintSlotDuplicate(ctx, tenantID, templateID, competencyCode, materi, indikator)` checks: (a) committed `blueprint_template_slots` with matching `(competency_code, materi, indikator)` signature, (b) pending `add_blueprint_slot` proposals in the same session, (c) pending `bulk_add_blueprint_slots` payloads. Wired into `capAddBlueprintSlot`, `capBulkAddBlueprintSlots` (in-batch + committed + pending), `execAddBlueprintSlot`, `execBulkAddBlueprintSlots`. Empty signatures (all three fields blank) are skipped. |
| 8 | ShareDialog native `<select>` | `frontend/src/components/share-dialog.tsx:250`: native `<select>` for collaborator role replaced with `SelectField` primitive (wrapped in a `w-32` div for layout parity with surrounding row-action buttons). Search field now has a visible "Find user" label above it instead of relying on placeholder copy. |

Remaining medium/high items from the original report (12, 13, 14, 15, 17, 18) are out of scope for this fix pass — they're documentation and schema-tightening tasks that don't gate the Tier 3 merge.

## SPEC COMPLIANCE

**Partial / not compliant for Tier 3.** The implementation covers a large portion of the requested shape: collaboration helpers exist, list responses include `canAccess` for exams/blueprints, migrations add owner/collaborator/blueprint/stimulus schema, preflight migration audit exists, seeded `blueprints:read/write` permissions exist, and the slot assignment endpoint uses a transaction.

Blocking spec gaps:

1. **Layered access is not consistently enforced in AI tools.** Multiple AI blueprint capabilities query exams/templates/slots by tenant only and never resolve user-level collaborator access. A teacher with `exams:read`/`blueprints:write` in the tenant can analyze or propose writes against resources they cannot access.
2. **Phase 9 retrofit is incomplete/ambiguous.** `requireExamSubjectAccess` remains and is still used on exam creation, preserving subject-based write gating instead of making subject access read-only fallback after resource creation.
3. **Stimulus exam-scoped privacy is broken.** List/get allow users with `exams:read` to fetch exam-scoped stimuli from exams they cannot access, as long as they know/filter the parent exam/stimulus ID.
4. **Stimulus lifecycle locks are not implemented.** Shared stimulus updates are allowed regardless of published/attempted linked exams, and there is no refresh/sync operation with draft-only guard. Snapshot columns exist, but lifecycle behavior is not enforced.
5. **Publish gate does not enforce strict blueprint coverage and does not lock blueprints.** Existing publish only checks question count; strict coverage at <100% is not blocked, and `exam_blueprints.status` is not set to `locked`.
6. **AI reverse analysis output does not match ADR-0010 shape.** `analyze_questions_to_blueprint` returns heuristic `proposedSlots` only, no `proposed_links`, no `distribution_summary`, no `unlinked_questions`, no min_confidence/batch_size cap behavior.
7. **AI slot dupe guards are insufficient.** `add_blueprint_slot` has no in-flight or committed duplicate guard on `(template_id, competency_code, materi, indikator)`, and bulk only checks duplicated explicit positions.
8. **AKM support is schema-level only.** AKM fields exist, but enforcement/UI behavior appears incomplete; no “stimulus required-feeling” server or meaningful validation was found.

## STANDARDS COMPLIANCE

**Partial.**

Positive:
- Backend mostly follows raw `database/sql` and handler style.
- Structured validation is used in many handlers.
- Additive migrations only; earlier migrations were not modified.
- Preflight owner audit is implemented and aborts loudly.
- Routes are registered through module route registration.
- `blueprints:read` and `blueprints:write` are seeded in `devseed.go`.
- `canAccess` exists in exam and blueprint list response models.
- Atomic assign-question endpoint uses a DB transaction.

Non-compliance / concerns:
- Tier 3 access helpers are not used from AI capability handlers. Tool-level permission strings are not a substitute for resource-level collaborator checks.
- Old `requireExamSubjectAccess` remains as write authorization for create flow, contrary to the accepted collaboration model’s direction that subject RBAC is read-only institutional fallback.
- Audit events are present for many writes, but security-sensitive collaborator/ownership behavior needs targeted test coverage; no evidence of complete RBAC test matrix was found.
- Frontend `ShareDialog` uses a native `<select>` at `frontend/src/components/share-dialog.tsx:250`, violating “no native form controls; reuse primitives/floating labels.” Search placeholders are still used as labels/help text in new UI paths.
- Native question-form inputs in the modified exam page remain (`frontend/src/app/(app)/app/exams/[id]/page.tsx:884`, `:893`), inherited from Phase 9 but now in the touched UI surface.

## BUGS / RISKS

### Blocking / Critical

1. **backend/internal/app/ai_blueprint_tools.go:540** — `capAnalyzeQuestionsToBlueprint` reads questions by `exam_id` + `tenant_id` only and never checks `resolveExamAccess(..., ActionRead)`.
   - Severity: Critical (Tier 3 authorization bypass).
   - Risk: Any tenant user/bot with `exams:read` can analyze questions from an exam they should receive 404 for, leaking exam content.
   - Fix: Load auth/user context into capability handlers and call a non-HTTP access resolver (`resolveExamAccess(ctx, tenantID, auth, examID)`), requiring `ActionRead` before any query. If capability context lacks roles, pass `AuthContext` into capability execution.

2. **backend/internal/app/ai_blueprint_tools.go:721** — `capAddBlueprintSlot` only verifies the template exists in the tenant; it does not require `requireBlueprintAccess`/`resolveBlueprintAccess(ActionWrite)`.
   - Severity: Critical.
   - Risk: Viewer or unrelated teacher with `blueprints:write` can propose slot changes against any template in the tenant.
   - Fix: Enforce template owner/editor/admin write access before proposal and again in executor.

3. **backend/internal/app/ai_blueprint_tools.go:863** — `capGenerateQuestionForSlot` resolves slot→exam and checks slot state without exam collaborator write access.
   - Severity: Critical.
   - Risk: Bot can propose generated questions for a slot in an exam the user cannot edit. If executor also misses the helper, this becomes direct unauthorized mutation; even at proposal stage it leaks slot/exam metadata.
   - Fix: Resolve parent exam, then require `resolveExamAccess(..., ActionWrite)` before validation/proposal and again in executor.

4. **backend/internal/app/stimuli.go:83** — `handleListStimuli` allows `lifecycle=all` or `lifecycle=exam_scoped&parentExamId=<id>` with only tenant-level `exams:read` and no parent exam access check.
   - Severity: Critical.
   - Risk: Exam-scoped private stimuli leak across collaborators/non-collaborators in same tenant.
   - Fix: Do not expose `lifecycle=all` except tenant admin. For `exam_scoped`/`parentExamId`, require `requireExamAccess(ActionRead)` on parent exam and filter to that exam. Shared library can remain tenant-readable.

5. **backend/internal/app/stimuli.go:166** — `handleGetStimulus` returns any tenant stimulus by ID, including `exam_scoped`, without checking parent exam access.
   - Severity: Critical.
   - Risk: Direct-ID private stimulus disclosure.
   - Fix: If lifecycle is `exam_scoped`, require parent exam read access before returning; if `archived`, decide owner/admin or linked-exam reader policy explicitly.

6. **backend/internal/app/exams.go:415** — Publish gate only checks `questionCount`; it does not enforce strict blueprint coverage or lock the blueprint.
   - Severity: Critical for exam reliability/spec compliance.
   - Risk: Strict AKM blueprint can be published at <100%; blueprint remains mutable after publish despite ADR-0010.
   - Fix: In publish transaction, compute coverage for existing `exam_blueprints`; if `strict_coverage=true` and filled < total, return 422 fields object. On successful publish, update `exam_blueprints.status='locked'` in the same transaction.

7. **backend/internal/app/stimuli.go:306** — Stimulus content can be edited by owner/admin even when used by groups in published/attempted exams; no snapshot refresh/lock semantics are implemented.
   - Severity: Critical/High.
   - Risk: Library content changes may not affect frozen snapshots, but there is no controlled draft refresh path, no published/attempted sync lock, and no attempt-aware guard for snapshot changes.
   - Fix: Add explicit `sync stimulus snapshot` endpoint that checks parent exam `draft` and zero attempts; keep library edit separate. Ensure group snapshot updates are impossible after publish/attempt.

8. **backend/internal/app/ai_blueprint_tools.go:531** — Reverse analysis does not implement proposal contract: no `proposed_links`, `distribution_summary`, `unlinked_questions`, `min_confidence`, `batch_size`, or 50-question cap.
   - Severity: High / spec-blocking.
   - Risk: Frontend/user cannot perform the required review of links/confidence; apply step accepts a different shape than ADR-0010.
   - Fix: Match ADR return schema exactly and enforce caps; make low-confidence questions proposed-without-link.

### High / Should Fix

9. **backend/internal/app/exams.go:245** — Exam creation still calls `requireExamSubjectAccess`, a Phase 9 subject-based write gate.
   - Severity: High.
   - Risk: The new model says subject membership is read-only fallback, while ownership should govern existing resources. This keeps subject membership as a write gate and may block legitimate non-subject co-author/admin workflows with teacher profiles.
   - Fix: Replace with role/permission-only create policy (e.g. `exams:write` can create and becomes owner) plus optional subject validation. Keep subject fallback only inside `resolveExamAccess` for reads.

10. **backend/internal/app/ai_blueprint_tools.go:711** — Missing blueprint slot duplicate guard for single add.
   - Severity: High.
   - Risk: Bot can propose duplicate slots with identical `(competency_code, materi, indikator)` in same template/session.
   - Fix: Add `checkBlueprintSlotDuplicate(ctx, tenantID, templateID, competencyCode, materi, indikator)` that checks pending proposals and committed rows.

11. **backend/internal/app/ai_blueprint_tools.go:743** — Bulk slot addition only detects duplicate explicit positions, not duplicate signatures or committed positions/signatures.
   - Severity: High.
   - Risk: Duplicate pedagogical slots and/or DB unique failures at execute time.
   - Fix: Validate all in-batch signatures, existing committed signatures, pending proposals, and existing positions before proposal.

12. **backend/internal/app/blueprint_slots.go:880** — Assign-question transaction verifies question belongs to same exam but not `tenant_id` in the query.
   - Severity: Medium/High.
   - Risk: UUID guessing is unlikely, and same-exam check mitigates, but Tier 3 tenant-scoped SQL standard expects tenant filters on domain reads.
   - Fix: Add `AND tenant_id = $tenantID` to question lookup/update inside tx.

13. **backend/migrations/000015_blueprints_stimuli.sql:320** — `exam_questions.group_id` remains nullable after backfill.
   - Severity: Medium.
   - Risk: ADR says every question lives inside a group; future code can create questions with NULL group_id unless handlers always backfill.
   - Fix: Either make `group_id SET NOT NULL` after handler retrofit, or enforce group creation in all question creation paths and add tests.

14. **backend/migrations/000015_blueprints_stimuli.sql:65** — No DB constraint ties `lifecycle='exam_scoped'` to `parent_exam_id IS NOT NULL`.
   - Severity: Medium.
   - Risk: Data drift can produce globally invisible orphan stimuli or shared stimuli with parent exam still set.
   - Fix: Add CHECK: `(lifecycle='exam_scoped') = (parent_exam_id IS NOT NULL)` unless archived semantics require a separate allowed case.

15. **backend/internal/platform/migrate/preflight.go:31** — Preflight only audits exams/courses, not blueprint templates.
   - Severity: Medium.
   - Risk: Acceptable for initial migration ordering because templates are new, but ADR-0009 includes blueprint owner semantics; future imported templates need equivalent audit.
   - Fix: Document why new table is excluded or add audit when backfilling existing blueprint data.

### UI / Frontend

16. **frontend/src/components/share-dialog.tsx:250** — Native `<select>` used for collaborator role.
   - Severity: Medium.
   - Risk: Violates project UI standard; inconsistent focus/disabled styling and no floating label primitive.
   - Fix: Replace with existing `SelectField` or Morfosis select primitive.

17. **frontend/src/components/share-dialog.tsx:307** — Search field uses placeholder text as instruction.
   - Severity: Low/Medium.
   - Risk: Placeholder-as-label pattern conflicts with Morfosis form standard.
   - Fix: Add visible label/help text and keep placeholder non-essential or remove.

18. **frontend/src/app/(app)/app/exams/[id]/page.tsx:884 / :893** — Native inputs remain in the touched question form path.
   - Severity: Medium.
   - Risk: Violates “no native form controls” and floating-label standard; touched surface should be remediated.
   - Fix: Replace with `InputField`/Morfosis primitives and typed controlled components.

## MERGE READINESS

**Tier 3: BLOCKED.**

This should not merge until the authorization bypasses are fixed and independently security-reviewed. Minimum conditions:

1. All AI blueprint capabilities and executors enforce resource-level access (`resolveExamAccess`, `resolveBlueprintAccess`) in addition to coarse permission strings.
2. Stimulus list/get privacy for `exam_scoped` resources is fixed.
3. Publish gate enforces strict coverage and locks blueprints transactionally.
4. Reverse analysis/apply schema aligns with ADR-0010 or the contract/ADR is explicitly amended.
5. Slot duplicate guards are implemented for single and bulk AI tools.
6. Targeted tests cover: editor cannot invite, viewer cannot edit, subject fallback read-only, cross-tenant negative, AI tool negative access, strict coverage publish block, transfer ownership role flip, exam-scoped stimulus privacy.
7. Lint blockage is resolved; parent reported go vet/build/test and tsc clean, but lint is still blocked.

## OPEN QUESTIONS

1. Should users with tenant-level `exams:read` see shared stimuli content, or should shared stimuli also be gated by a new `stimuli:read`/`blueprints:read` permission?
2. Is exam creation intended to remain subject-gated for teachers, or should any actor with `exams:write` create and own exams regardless of subject membership?
3. Where are “attempted exam” checks modeled in Phase 9/10? Stimulus snapshot lock needs a canonical `exam_attempts`/submission source.
4. Should `exam_questions.group_id` become NOT NULL now, or remain nullable until all Phase 9 question creation paths are fully group-aware?
5. Should `apply_blueprint_analysis` accept indices from the analysis response as ADR says, or accepted slot objects as currently implemented? Pick one and update code/docs/tests consistently.
6. Should collaborator management require `blueprints:write`/`courses:write` per resource type in addition to `manage` role, or is role-level access sufficient once the user has a coarse module permission?

---

## Phase 9.8 — Section-mandatory + inline kisi-kisi UX (2026-05-19)

The 9.7 canvas worked but had three sharp edges users hit immediately:

1. **Sections were optional**, so the empty state showed a global toolbar with three competing buttons (+ Section / + Group / + Soal). New users didn't know where to start.
2. **Kisi-Kisi=ON forced an empty "load template" prompt.** A teacher who flipped the toggle on couldn't write a question until they applied a template, which inverted the natural authoring flow ("guru nulis dulu sambil mikir" — write first, structure later).
3. **Pedagogical metadata lived behind a tab** (in the slot-first canvas) instead of next to the question content. Editing a question's KD/materi/indikator required mentally context-switching between two surfaces.

This phase rewrites the authoring surface around the principle that **a section is the unit of containment** and **kisi-kisi metadata is first-class on every question** when the toggle is on.

### Three interlocking changes

**Change 1 — Section becomes the mandatory container.** Migration 18 backfills "Section 1" for any exam that didn't have one, reassigns orphan questions/groups to that section, then locks `exam_questions.section_id NOT NULL`. `handleCreateExam` creates the default section in the same transaction as the exam row, so the canvas always lands users on a usable empty surface. `handleDeleteExamSection` blocks delete when the candidate is the last remaining section (structured 422 with `fields.section`); when allowed, it reassigns members to the first remaining section in one tx so the new NOT NULL invariant holds. The frontend SectionCard exposes a pencil-icon inline title edit and a trash icon that disables when `isOnlySection`. The old global toolbar is removed; the global "+ Section" CTA becomes a full-width dashed button below all sections; `+ Tambah Soal` and `+ Tambah Group` now live in each section's footer.

**Change 2 — Reverse-flow as default + simplified toggle UX.** The KisiKisiToggle is now a single pill (filled-on vs ghost-off), no verbose "Kisi-kisi: On" copy. The companion LoadKisiKisiButton is a small `ClipboardPaste` icon button with tooltip, visible only when KK is on. The LoadKisiKisiSheet drops the "Generate dari Soal" card — that flow is replaced by the natural inline authoring path. Most importantly, the canvas no longer forces a "must load template" empty state: when KK is on without a blueprint, sections render normally and the user can write questions immediately. Backend `handleCreateQuestion` gains an auto-blueprint path: when `uses_kisi_kisi=true` AND no `blueprintSlotId` is supplied, helper `ensureExamBlueprint` creates an ad-hoc blueprint (curriculum=k13, type=reguler, no template_id) and `appendBlueprintSlot` mints a slot carrying the inline pedagogical fields. The clone path that loads templates remains unchanged; users who want a template-first flow get it.

**Change 3 — Kisi-Kisi fields inline in question accordion.** When `exam.usesKisiKisi=true`, every question accordion renders a `KisiKisiInline` subsection alongside content/options/stimulus. Two layouts:

- Reguler: KD / Materi / Indikator + Cognitive Level (C1–C6) + Difficulty (mudah/sedang/sulit)
- AKM: KD / Materi / Indikator + Konten / Konteks / Proses Kognitif + Level (1–5)

When the bound slot was cloned from a template (`slot.fromTemplate=true`, derived from the new `sourceTemplateId` field on `slots-with-questions`), fields render read-only with a Lock icon and tooltip "Terkunci dari template". When the slot is from an auto-blueprint, fields stay editable and `handleUpdateQuestion` writes them through to the slot in one transaction with a dedicated `exam_blueprints.slot_updated` audit event.

### What ships

- **Backend**: `migrations/000018_section_mandatory_and_inline_kisi_kisi.sql`, `exams.go` (default-section in create tx + `defaultSectionId` in response), `exam_sections.go` (last-section delete guard + member reassignment), `questions.go` (inline KK fields on create/update + `ensureExamBlueprint` + `appendBlueprintSlot` helpers + `slotPayloadHasMeta` + AKM/template_id surfaced on `Question.slot`), `blueprint_slots.go` (`sourceTemplateId` on slots-with-questions response).
- **Frontend**: `lib/modules-api.ts` (extended `CreateQuestionPayload`, `QuestionSlotRef`, `SlotsWithQuestionsResponse`), `components/exams/section-card.tsx` (pencil edit + isOnlySection prop + new footer), `components/exams/exam-canvas.tsx` (full rewrite — sections always rendered, dashed +Tambah Section CTA, no slot-first list, no empty-state gating), `components/exams/kisi-kisi-toggle.tsx` (single pill), `components/exams/load-kisi-kisi-button.tsx` (icon-only), `components/exams/load-kisi-kisi-sheet.tsx` (Generate card removed), `components/exams/question-accordion.tsx` (KisiKisiInline subsection + slot writeback in payload).
- **Tests**: `backend/internal/app/section_mandatory_test.go` (5 helper tests covering CreateExam_AutoCreatesDefaultSection, DeleteSection_BlocksWhenLastSection, CreateQuestion_AutoCreatesBlueprintWhenKisiKisiOnAndNoBlueprintYet, CreateQuestion_AppendsSlotWhenKisiKisiOnAndNoTemplate, SlotPayloadHasMeta_EmptyShortCircuits).
- **Verification**: `go vet ./...` clean, `go build ./...` clean, `go test ./...` clean (PASS), `npx tsc --noEmit` clean, migration 000018 applied (28ms), `/readyz` 200, `exam_questions.section_id` NOT NULL confirmed via `information_schema.columns`.

### Audit/safety notes

- Auto-blueprint creation is fully transactional: the question insert and the blueprint+slot inserts share the same `tx`. A failure rolls back both, so there are no orphan blueprints.
- Section-delete attempts that hit the last-section guard emit an `exam_sections.delete_blocked` audit event so admin trails are preserved even when the action is rejected.
- Slot writeback on update emits `exam_blueprints.slot_updated`, separate from `questions.update`, so the audit log distinguishes pedagogical metadata edits from content edits.
- Inline KK fields are only forwarded by the frontend when `usesKisiKisi=true` and `slotLockedFromTemplate=false`; template-cloned slots stay immutable from this surface.
