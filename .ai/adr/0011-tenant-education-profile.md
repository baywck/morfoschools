# 0011 — Tenant Education Profile Drives Curriculum Scope

## Status
Accepted

## Context
Morfoschools previously inferred relevant Kurikulum Merdeka phases from active `class_sections.grade_level`. That is useful as a fallback, but it is not a stable source of truth: new tenants may have no classes yet, historical classes can skew phase inference, and SMA/SMK subject visibility differs even when both use phases E-F.

Curriculum CP, blueprint creation, exam kisi-kisi, and subject selection need a shared tenant-scoped curriculum boundary.

## Decision
Introduce a Tenant Education Profile concept: tenant has a school type plus explicit enabled phases and a vocational-subject flag.

Normal tenant CP views are strict tenant-scoped: users see only CP phases enabled for their tenant. SMK tenants can use general subjects plus vocational/SMK subjects; SD/SMP/SMA tenants see general subjects only. Custom tenant subjects remain allowed.

## Alternatives Considered
- Infer phases from class sections only — rejected because it breaks for new tenants and is too implicit for production curriculum scope.
- Use only a single school type (`sd|smp|sma|smk`) — rejected because mixed/override cases need explicit phases.
- Show all CP phases with filters — rejected for normal tenant UX because it invites wrong CP/phase selection.

## Consequences
- CP, blueprints, exams, and subject selectors share one curriculum-scope source of truth.
- Tenant management needs UI/API for school type, enabled phases, and vocational flag.
- Existing class-section inference can remain as fallback/migration aid, but not the final source of truth.
