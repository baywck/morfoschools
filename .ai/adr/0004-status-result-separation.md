# ADR-0004: Enrollment Status and Result Separation

**Date:** 2026-05-18
**Status:** Accepted
**Context:** Enrollment state needs to track both administrative lifecycle (active, suspended, withdrawn) and academic outcome (passed, failed). Combining them creates ambiguity.

## Decision

Two independent state machines with coordination invariants.

## Enrollment Status (Administrative Lifecycle)

```
States: draft, active, suspended, completed, withdrawn, cancelled

Transitions:
  draft      → active, cancelled
  active     → suspended, completed, withdrawn
  suspended  → active, withdrawn
  completed  → active (admin reopen, audited)
  withdrawn  → active (admin reactivate, audited)
  cancelled  → [terminal]
```

## Enrollment Result (Academic Outcome)

```
States: not_started, in_progress, pending_review, passed, failed, incomplete, void

Transitions:
  not_started    → in_progress, void
  in_progress    → pending_review, passed, failed, incomplete, void
  pending_review → passed, failed, incomplete, in_progress (returned)
  passed         → in_progress (admin reopen)
  failed         → in_progress (admin reopen)
  incomplete     → in_progress (admin reopen/extension)
  void           → [terminal]
```

## Coordination Invariants

| Status | Result Constraint |
|--------|------------------|
| draft | must be not_started |
| active | any non-terminal |
| suspended | unchanged from before suspension |
| completed | must be passed or failed |
| withdrawn | incomplete (had progress) or void (no progress) |
| cancelled | must be void |

## Item-Level Progress (same pattern, simpler)

- **status:** locked, available, in_progress, completed
- **result:** none, passed, failed, exempted, overridden

## Consequences

- Clear semantics: "failed but can retake" = status:active, result:in_progress
- Reporting: filter by result for academic reports, filter by status for admin ops
- State machine validation enforced in domain service
- Slightly more fields than single-enum approach

## Alternatives Considered

1. **Single status enum** — combinatorial explosion (active_passed, active_failed, suspended_in_progress...)
2. **Numeric progress percentage** — doesn't capture pass/fail semantics
