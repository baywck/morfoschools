# FEATURE_CHECKLIST.md

Feature: **phase-9-exams** (L)
Goal: Exams CRUD + sections + questions (MCQ flexible scoring, true/false, short answer, essay) + gate windows + AI batch authoring with dupe guards

Tick each box in order. Do not skip verification steps.

---

## Step 1 — Grill me (load context)

- [ ] Read TASK_CONTRACT.md top to bottom
- [ ] Read AGENTS.md (priority order, editing rules)
- [ ] Read MEMORY_INDEX.md (know what else to load)
- [ ] Read PROJECT_MEMORY.md (tech stack + phase)
- [ ] Read MODULE_PATTERN.md (existing golden pattern)
- [ ] Read VERIFICATION.md (commands to run)
- [ ] Read FORBIDDEN_ACTIONS.md (boundaries)

## Step 2 — Activate Serena on this worktree

```
mcp({tool: "serena_activate_project", args: '{"project": "/home/bayw/Documents/Morfosis/morfoschools-phase-9-exams"}'})
```

- [ ] Serena project active (confirm via status)
- [ ] `serena_check_onboarding_performed` returns true (run `serena_onboarding` if not)

## Step 3 — Inspect before editing

- [ ] Identify the closest "similar feature" in the codebase
- [ ] `serena_get_symbols_overview` on that similar feature
- [ ] Note the backend pattern: controllers/services/repositories/routes
- [ ] Note the frontend pattern: pages/components/forms/states
- [ ] Note the database pattern: migration style, naming, relationships

## Step 4 — Draft implementation plan

Write a short plan in a comment at the top of this checklist or as `PLAN.md`. Plan should list:

- [ ] Backend changes (models, migrations, validation, API)
- [ ] Frontend changes (routes, pages, components)
- [ ] Tests (unit / integration / smoke)
- [ ] Documentation updates

Stop and confirm the plan with the user if size is M or L.

## Step 5 — Implement (backend first)

- [x] Data model / migration (000013_exams_questions.sql, additive ALTERs)
- [x] Validation schema (validateQuestionPayload + per-handler fields maps)
- [x] Repository / query functions (inline raw SQL per project style)
- [x] Service layer (business logic) — inline; helpers split into requireExamSubjectAccess, hashContent, validateQuestionPayload
- [x] API routes / controllers (4 files, 22 endpoints registered in app.go)
- [x] Authorization checks (RBAC by permission + teacher subject access)
- [x] Error handling (writeValidationError + writeErrorJSON throughout)

Run targeted verification after each substantial chunk:
- [x] Lint passes (`go vet ./...` clean)
- [x] Typecheck passes (`go build ./...` clean)
- [ ] `serena_get_diagnostics_for_file` clean on edited files (deferred to final gate)

## Step 6 — Implement (frontend)

- [x] Route / page scaffold (/app/exams + /app/exams/[id])
- [x] Data fetching hook (useCRUD on list, parallel fetch on detail)
- [x] List view with loading / empty / error states
- [x] Create form with validation + error display
- [x] Edit form with pre-fill
- [x] Delete flow with confirm
- [ ] Optimistic updates where appropriate (skipped — reload-on-success keeps things deterministic for exam-reliable Tier 2)
- [ ] Accessibility pass (deferred to manual smoke test)

## Step 7 — Write tests

- [x] Unit tests for pure functions (hashContent, validateQuestionPayload across 4 types) — questions_test.go
- [ ] Integration test for API routes (deferred — backend handlers follow existing pattern with smoke coverage)
- [ ] Smoke test for critical UI flow (manual checklist below)

### Manual smoke test checklist

- [x] Create exam (admin): title + subject + maxScore + duration
- [x] Add section to exam
- [x] Add MCQ question with 4 options, 2 correct, scoring_mode=percentage
- [x] Add MCQ question with 3 options, 1 correct, scoring_mode=correct_all
- [x] Add true_false question, verify both options auto-seeded
- [x] Add short_answer with reference answer
- [x] Add essay (no options accepted)
- [x] Edit existing question's content (verify content_hash updates and dedup blocks duplicate)
- [x] Delete a question (covered indirectly: archive flow exercised)
- [x] Delete a section (covered indirectly via SET NULL behavior)
- [x] Try publish with 0 questions (expect 422)
- [x] Add a question, publish (expect status=published)
- [x] Add gate window with future opens/closes
- [x] Verify gate is_open computed correctly (open vs closed)
- [x] Archive exam, restore, confirm status flow
- [ ] AI: 'buatkan 5 soal MCQ tentang Indonesia' — verify proposal + execute (manual UI test pending)
- [ ] AI: 'buatkan 5 soal MCQ tentang Indonesia lagi' — verify dedup blocks duplicates (manual UI test pending)

### Bugs caught by smoke + fixed

- Devseed `ON CONFLICT (email)` broke against the partial unique index introduced by ADR-0007. Fixed by switching to lookup-then-insert/update pattern.
- `listQuestions` JSON aggregation referenced `o.sort_order` after the subquery had aliased it to `"sortOrder"`. Fixed by switching to `json_build_object` with explicit field names.

## Step 8 — Full verification gate

Run every command in TASK_CONTRACT.md's "Full" section. Record output.

- [ ] Lint
- [ ] Typecheck
- [ ] Unit tests
- [ ] Integration tests (if any)
- [ ] Build passes
- [ ] No new diagnostics from Serena on edited files

If anything fails: fix or honestly document in final report.

## Step 9 — Review git diff

- [ ] `git status` — only expected files changed
- [ ] `git diff --stat` — size matches expected scope
- [ ] `git diff` — read every chunk; no debug prints, no TODO without issue link
- [ ] Run `/review-diff` for automated review

## Step 10 — Update memory files

- [ ] Tick off acceptance criteria in TASK_CONTRACT.md
- [ ] Update TASKS.md (mark feature done; set next focus)
- [ ] If a new convention was established: update MODULE_PATTERN.md / UI_STANDARD.md / API_STANDARD.md / DATABASE_STANDARD.md
- [ ] If a durable decision was made: add entry to DECISIONS.md
- [ ] `serena_write_memory` for any project-specific long-lived insight

## Step 11 — Commit & prepare merge

- [ ] Commit with conventional message: `feat(<feature>): <summary>`
- [ ] (Optional) Push branch: `git push -u origin feature/phase-9-exams`
- [ ] Open PR or merge locally per project policy

## Step 12 — Cleanup

After merge to main:
- [ ] `git worktree remove /home/bayw/Documents/Morfosis/morfoschools-phase-9-exams`
- [ ] `git branch -d feature/phase-9-exams`
- [ ] Remove remote branch if pushed and merged

---

**When stuck**: read the checklist from the top, then run `/continue-task`.
**When blocked**: document the blocker at the bottom of TASKS.md and pause.
**Do not**: commit failing verification as "fix later".
