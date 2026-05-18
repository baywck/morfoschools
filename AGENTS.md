# AGENTS.md

You are an AI coding agent. This file is your only auto-loaded instruction set.

## Project

**Morfoschools** — Production-grade LMS SaaS untuk sekolah Indonesia.

## Quick Context

- Stack: Go backend + Next.js frontend + PostgreSQL + Valkey + NATS
- Multi-tenant, RBAC, exam-reliable
- Enrollment unit = Program (grouped courses + exams)
- Full context: `.ai/PROJECT_MEMORY.md`
- Domain glossary: `.ai/CONTEXT.md`
- Architecture decisions: `.ai/adr/`
- Task backlog: `.ai/TASKS.md`

## Mandatory First Actions

1. Read `.ai/TASKS.md` for current phase and active task
2. Read `.ai/PROJECT_MEMORY.md` for stack and conventions
3. Read `.ai/CONTEXT.md` before naming anything

## Rules

- **Before writing ANY UI code**: load Morfosis UI skill at `/home/bayw/.pi/agent/skills/morfosis-ui/SKILL.md`, read `DESIGN_SPEC.md` and `COMPONENT_REFERENCE.md`. No exceptions.
- Follow existing patterns. Do not invent new architecture.
- Prefer small, incremental changes.
- Do not refactor unrelated code.
- Do not edit generated files, vendor, node_modules, build, dist.
- Do not commit secrets.
- TDD: write test first, see it fail, then implement.
- Every domain table must have `tenant_id`.
- Backend validation must return structured `fields` object.
- Frontend: Morfosis Design System, floating labels, no placeholder-as-label.
- All buttons must have loading/disabled states.
- Score records are immutable snapshots — never recalculate implicitly.
- Progress resolves by item_id, not position.

## Verification

| Risk Tier | Trigger | Required |
|---|---|---|
| Tier 0 | CSS, copy, comments | Lint only |
| Tier 1 | Feature code, UI | Lint + typecheck + targeted tests |
| Tier 2 | Business logic, APIs | Lint + typecheck + full test suite + build |
| Tier 3 | Auth, data deletion | Tier 2 + security review |
| Tier 4 | CI, deploy, DB schema | Tier 2 + dry-run |

## Module Definition of Done

```
[ ] Backend API implemented + tests pass
[ ] RBAC enforced
[ ] Audit events emitted for writes
[ ] Structured validation (fields object)
[ ] OpenAPI updated
[ ] AI Tool Manifest updated
[ ] Frontend implemented with real API
[ ] All actions have loading states
[ ] Skeleton/empty/error states
[ ] No dummy initial rows
[ ] Typecheck passes
[ ] Browser smoke pass
```
