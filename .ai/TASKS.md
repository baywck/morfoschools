# Tasks — Morfoschools

## Current Phase: Phase 9 + 9.5 — IN AUDIT (worktrees, not merged)

Both phases are implemented in separate worktrees and pass go vet/build/test + tsc.
Reviewer audit (2026-05-19) flagged BLOCKERS in both. Neither is merge-ready.

### Worktrees
- `morfoschools-phase-9-exams` @ `8d55dc8` — **✅ ALL BLOCKERS FIXED** (committed); audit at `AUDIT_REPORT.md`
- `morfoschools-phase-9-5-blueprints-collab` @ `3f20707` — rebased on Phase 9 fix; **8 Tier 3 blockers remain**; audit at `AUDIT_REPORT.md`

### Phase 9 — BLOCKERS (must fix before merge)
- [x] **Gate window not enforced** on `GET /api/v1/exams/{id}/questions` — fixed in `questions.go:103` (callers without `exams:write` need exam published + open gate window)
- [x] **Update-exam can reassign to unassigned subject** without RBAC recheck — fixed in `exams.go:380`
- [x] **RBAC bypass on multiple writes**: publish/archive/restore, section update/delete, gate create/update/delete, all AI write tools — fixed via new `requireExamWriteAccess` / `requireExamSectionWriteAccess` / `requireExamGateWriteAccess` helpers in `exams.go`; AI tools use `checkExamWriteAccess` predicate at propose + execute layers
- [x] **Editing question options is silently dropped** — fixed in `questions.go:280`; PATCH now accepts `options[]` and atomically replaces inside transaction; adds duplicate-on-update check excluding self
- [x] **Option CRUD breaks MCQ/TF invariants** — fixed via new `validateOptionsAfterMutation` simulating post-state; wired into Create/Update/Delete handlers
- [x] **AI executors lack execute-layer dupe/RBAC/audit** — fixed in `ai_exam_tools.go`; new `checkExamWriteAccess` + `auditAI` (actor_type='ai_agent') wired into all three executors
- [x] **Gate update missing final window-ordering check** — fixed in `exam_gates.go:154`; loads existing row, validates merged `closes_at > opens_at`

### Phase 9.5 — BLOCKERS (Tier 3, security review required)
- [x] **AI authz bypass**: `capAnalyzeQuestionsToBlueprint`, `capAddBlueprintSlot`, `capGenerateQuestionForSlot` — fixed via new `ai_blueprint_access.go` (`checkAIExamAccess`, `checkAIBlueprintAccess`)
- [x] **Exam-scoped stimulus privacy broken** — fixed in `stimuli.go` list/get with parent-exam access gating
- [x] **Publish gate doesn't enforce strict-coverage blueprint** — fixed in `exams.go` publish handler; locks `exam_blueprints.status='locked'` in same tx
- [x] **Reverse-flow analyze tool returns wrong shape** — rewritten `capAnalyzeQuestionsToBlueprint` to ADR-0010 shape with `minConfidence`/`batchSize`/50-cap; `apply_blueprint_analysis` extended schema
- [x] **Stimulus lifecycle locks unimplemented** — added `stimulusContentLocked` predicate + `POST /api/v1/stimuli/{id}/sync-snapshot` endpoint
- [x] **Phase 9 retrofit incomplete** — `handleCreateExam` no longer subject-gated; only validates tenant membership
- [x] **Slot dupe guards insufficient** — added `checkBlueprintSlotDuplicate` (committed + in-flight + pending-batch)
- [x] **ShareDialog uses native `<select>`** — replaced with `SelectField`; visible label added

### Phase 9.5 — Continuity (after blockers fixed)
- [x] Fix useCRUD infinite loop (callbacks latched in refs); commit `2780cfa`
- [x] Normalize blueprint sheet button styling (h-9+text-white → h-8+token); commit `2780cfa`

### Phase 9.6/9.7/9.8 — Authoring UX evolution (in worktree, not merged)

9.6 (commit `c5d62d7`, ADR-0011): three-mode authoring (`quick`/`structured`/`akm`) — SUPERSEDED.

9.7 (commit `0a63d8e`, ADR-0012): replaced 3-mode with three orthogonal axes:
- `exams.uses_kisi_kisi` boolean (replaces authoring_mode)
- AKM detection from `exam_blueprints.blueprint_type`
- Universal stimulus axis on `exam_questions.stimulus_id` (XOR with group_id)
- @dnd-kit drag-and-drop reorder + group/section moves; slot binding preserved across DnD
- Migration 000017

9.7 follow-up (commit `54d083c`): inline accordion authoring rewrite
- Modal/sheet question form gone; click question row → inline accordion
- Contextual stimulus picker (location tells you scope)
- Three states for kisi-kisi toggle (off / on-no-template / on-with-template)
- New components in `frontend/src/components/exams/`
- Net delta: +247 / –1987 LOC (page file 1379 → 386)

9.8 (commit `2cc4334`): mandatory sections + reverse flow as default + inline kisi-kisi fields
- Migration 000018: section_id NOT NULL, backfill default "Section 1" for orphan exams/questions
- Auto-blueprint when kisi-kisi=ON without template (per-question slot creation)
- Section toolbar moved into each section card; "+ Tambah Section" full-width dashed button at bottom
- Last-section delete blocked (422 with structured fields)
- Kisi-Kisi subsection rendered inline in question accordion when toggle on
- AKM-aware field labels (Konten/Konteks/Proses Kognitif/Level 1-5)
- Template-bound slots: locked fields with tooltip; auto-blueprint slots: editable
- LoadKisiKisi simplified to small icon button next to toggle

### Next Session Decision Tree
1. **✅ Phase 9 blockers DONE** (commit `8d55dc8`)
2. **✅ Phase 9.5 rebased + Tier 3 blockers DONE** (commit `98dd4ec`)
3. **✅ Phase 9.6 → 9.7 → 9.8 UX evolution DONE** (commits `c5d62d7` → `0a63d8e` → `54d083c` → `2cc4334`)
4. **Browser smoke test** by user across the kisi-kisi flow at http://127.0.0.1:1666
5. **Address remaining UX feedback** if any
6. **Merge sequence**: Phase 9 first (`feature/phase-9-exams`), then Phase 9.5/9.6/9.7/9.8 (`feature/phase-9-5-blueprints-collab`)

### Deferred polish (not blockers)
- Group → section drag-drop end-to-end (PATCH /groups/{id} doesn't accept sectionId yet — soft no-op)
- In-app reverse-flow review screen (currently AI chat handles it)
- ESLint v9 migration (separate concern)
- In-canvas "✨ Generate via AI" button on empty slots (currently passive placeholder)

### Backlog (after Phase 9/9.5 merge)
- [ ] Phase 10: Exam Critical Path (take, autosave, submit, receipt)
- [ ] Phase 11: Teacher Operations (monitor, grading)
- [ ] Phase 12: AI Agent Runtime (continue from Phase 2 chatbot baseline)
- [ ] Tenant logo upload (R2/local)
- [ ] Consolidate `ToolRegistry` and `CapabilityRegistry`
- [ ] AI Phase 3: task states for long workflows (50 exam questions)

## In Progress
- [ ] Execution planner kisi-kisi-first: universal action plan, kisi-kisi audit/repair/completion runners, plan progress summary, retry-friendly batch runner, deeper legacy cleanup (partial)
  - [x] universal action plan tables + plan loader
  - [x] kisi-kisi audit runner
  - [x] kisi-kisi repair runner
  - [x] kisi-kisi completion runner
  - [x] plan progress summary API
  - [x] retry-friendly next-batch runner
  - [x] unused legacy agent code cleanup (partial)

## Completed
- [x] Phase 0: .ai/ memory files, ADRs, AGENTS.md, standards
- [x] Phase 1: Docker Compose (6 services), migrations (6), backend skeleton, frontend skeleton
- [x] Phase 2: Auth/login/RBAC/session/CSRF, dev seed (7 users, roles, permissions)
- [x] Phase 3: Frontend shell (morfosis-studio: dark shell, 66px sidebar, floating card, AI chat panel)
- [x] Phase 4: Backend patterns (pagination, response helpers, RBAC helpers, tenant switch)
- [x] Phase 6: All Admin Modules (BE + FE) — Users, Tenants, Teachers, Students, Staff, Guardians
  - Composite create (user + profile in one request)
  - Teachers: multi-subject assignment
  - Students: optional class + guardian management
  - Role select (enum, not text)
  - Edit flows for all modules
- [x] Phase 7: Academic Structure (BE + FE) — Academic Years, Subjects, Class Sections, Teacher-Subject assignments
  - Grade level predefined (SD1-SMA12) + custom
  - Homeroom teacher select
  - Academic year select
- [x] Phase 8: Programs + Courses (BE + FE)
  - Programs: CRUD + sections + items + publish/archive
  - Courses: CRUD + publish/archive
  - Program sections with nested items (json_agg)
- [x] Phase 9.6 — Authoring Modes (ADR-0011, worktree `phase-9-5-blueprints-collab`) — **SUPERSEDED by 9.7 / ADR-0012 within 1 week of merge**
  - Migration 16: `exams.authoring_mode` (quick / structured / akm) + backfill from existing blueprints
  - Mode-aware question handlers: structured/akm require `blueprintSlotId`; downgrade to quick allowed only on draft (or platform admin) and surfaces a warning when slot bindings remain
  - New endpoints: `GET /api/v1/exams/{id}/slots-with-questions`, `POST /api/v1/exams/{id}/questions/from-slot`
  - Auto-set authoring_mode on `clone_blueprint_to_exam` (HTTP + AI executor) and on reverse-flow apply
  - Publish gate: quick bypasses kisi-kisi gate; structured honors `strict_coverage`; akm always strict
  - Frontend: single content area in /app/exams/[id], mode pill in header, ModeSwitchModal with three radio options, SlotFirstCanvas (slot cards + Tulis Soal / unlinked questions section)
  - AI capabilities: `set_authoring_mode`, `convert_questions_to_kisi_kisi` (high-level reverse-flow wrapper)
  - Tests: 6 new unit tests (`isValidAuthoringMode`, `isAdminOverrideRole`, mode requires/allows slot, draft-only transition, mode-immutable-after-publish)
  - **Why superseded:** AKM is structurally a property of the blueprint not the exam; `structured` and `akm` enum branches collapsed into the same handler logic; stimulus needed to be orthogonal so non-AKM exams can attach passages too. See ADR-0012 for the 3-axis replacement.
- [x] Phase 9.7 — Kisi-Kisi Toggle + Stimulus Axis (ADR-0012, supersedes ADR-0011, worktree `phase-9-5-blueprints-collab`)
  - Migration 17: drops `authoring_mode`, adds `exams.uses_kisi_kisi BOOLEAN` (backfilled from blueprint presence) + `exam_questions.stimulus_id UUID` + `stimulus_id XOR group_id` CHECK constraint
  - 3-axis model: kisi-kisi boolean toggle (replaces 3-mode discriminator) + AKM detection from `exam_blueprints.blueprint_type` (no exam-level AKM column) + universal stimulus axis (direct stimulus_id OR group-mediated)
  - Drag-and-drop: `@dnd-kit/core@6.1.0` + `@dnd-kit/sortable@8.0.0` + `@dnd-kit/utilities@3.2.2` (exact pins). New `POST /api/v1/exams/{id}/questions/move` atomic endpoint; slot binding (`blueprint_slot_id`) PRESERVED across moves — visual reorder is decoupled from pedagogical anchoring
  - Group CRUD: `POST /api/v1/exams/{id}/groups`, `PATCH /api/v1/groups/{id}` (name, sortOrder, stimulus reassign + resyncSnapshot), `DELETE /api/v1/groups/{id}` (draft-only)
  - AKM auto-grouping: when applying an AKM template (blueprint_type IN akm_*), `autoCreateAkmGroups` scans cloned slots for unique `stimulus_id` refs and pre-creates `exam_question_groups`. Best-effort; templates without stimulus pre-assignments fall through silently
  - Frontend: `KisiKisiToggle` replaces 3-radio `ModeSwitchModal` (deleted); SlotFirstCanvas rewritten with @dnd-kit + AKM-aware metadata labels (Konten/Konteks/Proses Kognitif/Level vs Cog/Diff) read from `blueprintType` in `/slots-with-questions`; AKM badge in page header derived from blueprint_type; stimulus picker in question form (radio: none / direct / group, mutex matching backend constraint)
  - AI tools: `set_authoring_mode` → renamed `set_uses_kisi_kisi` (`{examId, enabled}`); `clone_blueprint_to_exam` and `convert_questions_to_kisi_kisi` flip `uses_kisi_kisi=true` instead of rotating an enum
  - Stale `exam-blueprint-section.tsx` deleted alongside `mode-switch-modal.tsx` cleanup
  - Tests: `kisi_kisi_test.go` (6 tests covering RequiresSlotWhenKisiKisiOn, AllowsNoSlotWhenKisiKisiOff, DisableKisiKisi_DraftOnly, KisiKisiToggleImmutableAfterPublish, StimulusAndGroupMutuallyExclusive, MoveQuestion_PreservesSlotBinding)
  - Verification clean: go vet/build/test + tsc --noEmit + migration 000017 applied + /readyz 200
- [x] Phase 9.8 — Section-mandatory + inline kisi-kisi UX (worktree `phase-9-5-blueprints-collab`)
  - Migration 18: backfills a default "Section 1" for every exam without one, reassigns orphan questions/groups to the first section, then locks `exam_questions.section_id NOT NULL`
  - `handleCreateExam` now creates "Section 1" inside the same transaction as the exam row; response carries `defaultSectionId` so the frontend can land users on a usable empty surface
  - `handleDeleteExamSection` blocks delete when the candidate is the only remaining section (returns structured 422 with `fields.section`); when allowed, reassigns members to the first remaining section in one tx
  - Auto-blueprint + auto-slot path on `handleCreateQuestion`: when `uses_kisi_kisi=true` AND no `blueprintSlotId` supplied, helper `ensureExamBlueprint` creates an ad-hoc blueprint (curriculum=k13, blueprint_type=reguler, no template_id) and `appendBlueprintSlot` mints a slot carrying the inline kisi-kisi metadata. Replaces the old "must load template before authoring" gate
  - `handleUpdateQuestion` accepts inline kisi-kisi fields (`competencyCode`, `materi`, `indikator`, `cognitiveLevel`, `difficulty`, AKM dimensions) and writes them through to the bound slot in the same tx with a dedicated `exam_blueprints.slot_updated` audit event
  - `slots-with-questions` response carries `sourceTemplateId` so the frontend can lock metadata fields on template-cloned slots; `Question.slot` now embeds AKM dimensions + `fromTemplate` flag
  - Frontend canvas restructure: `SectionCard` gains pencil-icon edit + disabled-trash-when-only behaviour; old global toolbar removed; new full-width dashed `+ Tambah Section` CTA below all sections; per-section footer carries `+ Tambah Soal` and `+ Tambah Group`
  - `KisiKisiToggle` simplified to single pill ("Kisi-Kisi" filled vs ghost); `LoadKisiKisiButton` becomes a small `ClipboardPaste` icon button with tooltip; `LoadKisiKisiSheet` drops the "Generate dari Soal" card (inline KK fields replace it as the natural authoring path)
  - `QuestionAccordion` adds an inline `KisiKisiInline` subsection (KD / Materi / Indikator + Cognitive/Difficulty for reguler, Konten / Konteks / Proses Kognitif / Level 1–5 for AKM) gated on `usesKisiKisi`; fields lock with a `Lock` icon when `slotLockedFromTemplate=true`
  - Tests: `section_mandatory_test.go` (5 tests — CreateExam_AutoCreatesDefaultSection, DeleteSection_BlocksWhenLastSection, CreateQuestion_AutoCreatesBlueprintWhenKisiKisiOnAndNoBlueprintYet, CreateQuestion_AppendsSlotWhenKisiKisiOnAndNoTemplate, SlotPayloadHasMeta_EmptyShortCircuits)
  - Verification clean: go vet/build/test + tsc --noEmit + migration 000018 applied + /readyz 200
- [x] AI Chatbot Phase 1-2 (ADR-0006)
  - CapabilityRegistry + domain-routed tool injection
  - Structured ToolError envelopes with recovery hints
  - Self-correction protocol in system prompt
  - Multi-proposal confirmation flow
  - `morfoschools:data-changed` event for cross-component refresh
- [x] Email Reuse After Archive (ADR-0007)
  - Migration 12: partial unique index + `original_email` column
  - Cascade rules (profile→user up, user→profiles down)
  - Restore endpoints per module with 409 conflict resolution
  - Synthetic email format `archived+<uuid>@archived.morfoschools.local`

## UI Components Built
- [x] InputField (floating label, h-11, prefix icon)
- [x] SelectField (floating label dropdown, disabled support)
- [x] DatePicker (custom calendar, no native)
- [x] DateRangePicker (start/end range)
- [x] SearchInput (compact h-8, plain)
- [x] Button (primary=black, loading spinner)
- [x] RightPullSheet (no overlay, rounded-r-inherit)
- [x] ConfirmDialog (centered, destructive variant)
- [x] RowActions (portal dropdown, 3-dot)
- [x] Toast (border-l-4, tones)
- [x] Skeleton
- [x] Breadcrumb (dynamic, Home icon)
- [x] PageShell (sticky header, responsive)
- [x] AppShell (dark shell + floating card + AI chat push)
- [x] Sidebar (66px icon strip)
- [x] Topbar (breadcrumb + user + AI toggle)
- [x] MobileNav (bottom h-16, horizontal scroll)
- [x] AI Chat Panel (360px, model selector, attach menu, auto-resize)

## Key Decisions Made
- No native form validation (all server-side)
- Loading state on every async action
- System font stack (no Google Fonts)
- Portal for dropdowns (escape overflow)
- RightPullSheet without overlay (user can interact outside)
- AI Chat pushes content (not overlays)
- Composite create endpoints (user + profile + assignments)
- Programs as enrollment unit (ADR-0001)
- Score decoupling (ADR-0002)
- Enrollment persistence on class transfer (ADR-0003)
- Status + Result separation (ADR-0004)
- No structure locking (ADR-0005)
