# Project Memory — Morfoschools

## Mission

Production-grade LMS SaaS untuk sekolah Indonesia. Multi-tenant, role-based, exam-reliable, AI-ready.

## Stack

| Layer | Technology | Notes |
|-------|-----------|-------|
| Backend | Go modular monolith | Single binary, stdlib net/http + pgx |
| Frontend | Next.js 15 App Router | React 19, SSR capable |
| Styling | Tailwind v4 + OKLCH tokens | Morfosis Design System |
| Forms | react-hook-form + Zod | Typed validation |
| Data fetching | TanStack Query + typed API client | Cache, optimistic, retry |
| DB | PostgreSQL 16 via PgBouncer | Shared-schema multi-tenancy |
| Cache | Valkey | Theme cache, sessions, locks |
| Queue | NATS JetStream | Exam shock absorber |
| Analytics | ClickHouse (optional) | Docker Compose profile, not required for boot |
| Runtime | Docker Compose | VPS-friendly deployment |
| Tests (BE) | Go test + real Postgres (testcontainers or compose) | No SQLite-only strategy |
| Tests (FE) | Vitest + Playwright | Unit + E2E |
| Storage | R2 / local filesystem | Tenant logos, resources |

## Architecture Decisions

### Program as Enrollment Unit (ADR-0001)
- Program = container for courses + exams
- Siswa enroll ke Program, bukan individual course/exam
- Program-only access v1
- Sections (grouped, ordered) → Items (sequential, orderable)
- Section unlock: configurable (sequential | always_open)
- Completion: semua required items done, per-item config (passing_grade, max_attempts)

### Score Decoupling (ADR-0002)
- Score immutable, snapshot at grade time (max_score, passing_score captured)
- Exam edit does not affect existing scores
- Recalculation only via explicit admin action + audit
- Exam version incremented on grading-critical field changes

### Enrollment Persistence on Class Transfer (ADR-0003)
- Enrollment attach ke student + program (permanent)
- ClassAssignment = temporal delivery context
- Pindah kelas → new ClassAssignment, enrollment untouched
- Progress, scores tetap di enrollment yang sama

### Status + Result Separation (ADR-0004)
- Enrollment Status (admin lifecycle): draft, active, suspended, completed, withdrawn, cancelled
- Enrollment Result (academic outcome): not_started, in_progress, pending_review, passed, failed, incomplete, void
- Coordination invariants enforced in domain service

### No Structure Locking (ADR-0005)
- Guru bebas restructure Program (reorder, add, remove items/sections)
- Progress resolve by item_id, bukan posisi
- Hapus item → progress orphan, tidak dihitung ke completion
- Auto-complete jika item sudah pernah passed di posisi baru
- Reset progress = explicit guru action

## Conventions

### Backend
- Handler tests with real Postgres (not SQLite)
- Standard error envelope: `{error: {code, message, details, requestId}}`
- Structured validation: return `fields` object, not global message
- CSRF on all writes
- Tenant-scoped SQL with `tenant_id` in all domain tables (denormalized)
- Audit events for all writes and sensitive actions
- RBAC centralized, testable
- UUID v7 for primary keys

### Frontend
- `page.tsx` = React Query wiring + mutations + toast
- `*-page.tsx` = pure UI component
- Morfosis Design System (OKLCH tokens, dark/light, tenant palette)
- All buttons: idle/hover/focus/loading/disabled states
- FormDrawer (pulled-right) + ConfirmDialog (centered)
- Skeleton/empty/error states, no dummy rows
- Floating labels on inputs — never placeholder-as-label

### Git
- Clean commits per logical unit
- Feature branches, PR workflow
- No dirty state accumulation

### Docs
- OpenAPI per module
- AI Tool Manifest per module
- Module review notes after completion

## Constraints

- VPS-friendly (low-spec infrastructure)
- Exam critical path: no external API dependency
- Indonesian school context (KKM/KKTP, remedial, rapor, semester ganjil/genap)
- Multi-tenant isolation mandatory
- httpOnly secure cookies for browser sessions
- No secrets in AI memory or logs

## Reference

- Old codebase (read-only reference): `/home/bayw/Documents/Morfosis/morfschools_old/morfoschools`
- Prototype (visual reference): `/home/bayw/Documents/Morfosis/morfschools_old/morfoschools_prototype`
