# ADR-0005: No Structure Locking — Progress by Item ID

**Date:** 2026-05-18
**Status:** Accepted
**Context:** Teachers need flexibility to restructure Programs (reorder sections/items, add/remove) even after students have enrolled. Traditional approach locks structure after first enrollment.

## Decision

No structural locking. Teachers can freely restructure Programs at any time. Progress resolves by item_id, not by position.

## Rules

1. Progress records reference `program_item_id` which references a specific course/exam entity
2. Reorder items/sections → progress unaffected (still linked to same item)
3. Move item to different section → progress follows (same item_id)
4. Item already passed at new position → auto-complete (no re-do required)
5. Remove item from Program → progress becomes orphan, not counted toward completion
6. Completion calculation only considers currently-active items in Program
7. Teacher can explicitly reset student progress per item (audited action)

## Consequences

- Maximum flexibility for teachers
- No "locked, can't change" frustration
- Progress never lost due to restructuring
- Completion is dynamic (recalculated based on current Program structure)
- Orphan progress records accumulate (acceptable, can be cleaned periodically)
- Edge case: teacher removes a required item that was blocking student → student may suddenly unlock next section

## Alternatives Considered

1. **Lock after first enrollment** — safe but too restrictive; teachers need to fix mistakes, adjust pacing
2. **Versioned Program snapshots** — over-engineered; each student sees their enrolled version. Complex, hard to maintain
3. **Copy-on-write per enrollment** — storage explosion, sync nightmare
