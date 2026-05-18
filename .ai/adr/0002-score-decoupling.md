# ADR-0002: Score Decoupling from Exam Content

**Date:** 2026-05-18
**Status:** Accepted
**Context:** Teachers need to edit exam questions after students have already taken the exam. Score integrity must be preserved.

## Decision

Scores are immutable historical records, decoupled from current exam state. Editing exam content does not affect existing scores or program progress.

## Rules

1. Score record stores snapshot fields at grade time: `max_score_snapshot`, `passing_score_snapshot`, `weight_snapshot`
2. These snapshot fields are never recalculated implicitly
3. Editing grading-critical fields on a published exam increments `exam.version`
4. Existing scores remain valid regardless of exam edits
5. Recalculation only via explicit admin action + audit log
6. Score records are append-only; corrections create new record, old marked `superseded`
7. Students taking exam during edit → their attempt uses questions loaded at start (per-attempt snapshot)

## Consequences

- Teachers can freely edit exams without fear of breaking history
- Progress/completion unaffected by content changes
- No need for structural locking on Programs
- Slightly more storage (snapshot fields per score)
- Dispute resolution: score record is self-contained proof

## Alternatives Considered

1. **Lock exam after first attempt** — too restrictive, teachers need to fix typos/errors
2. **Full versioning with version_id per attempt** — correct but over-engineered for v1; snapshot fields achieve same safety
3. **Recalculate on edit** — dangerous, breaks trust, causes parent complaints
