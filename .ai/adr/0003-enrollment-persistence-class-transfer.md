# ADR-0003: Enrollment Persistence on Class Transfer

**Date:** 2026-05-18
**Status:** Accepted
**Context:** Indonesian schools have students transferring between classes mid-semester. Progress must not be lost.

## Decision

Enrollment attaches to student + program (permanent). Class is a temporal delivery context tracked separately via ClassAssignment.

## Model

```
ProgramEnrollment → student + program (permanent, progress lives here)
ClassAssignment → enrollment + class_section (temporal, can change)
```

## Rules

1. Same-program class transfer: end old ClassAssignment, create new one. Enrollment untouched.
2. All progress, scores, completion remain on the same enrollment record
3. Class context at time of scoring derivable from ClassAssignment dates
4. Different-program transfer: withdraw old enrollment, create new enrollment
5. Admin validation: warn if new class doesn't offer the enrolled program

## Consequences

- Pindah kelas = zero data loss
- Single enrollment record per student per program (simple aggregation)
- GPA/progress calculation doesn't need to merge multiple records
- Class-centric reporting requires JOIN through ClassAssignment (acceptable with proper indexing)

## Alternatives Considered

1. **New enrollment per class transfer** — creates multiple records, complex aggregation, confusing for admins
2. **Enrollment attached to class** — progress lost on transfer, unacceptable
