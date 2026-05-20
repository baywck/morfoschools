# ADR 0009: Resource Collaboration Model

**Status**: Accepted
**Date**: 2026-05-19
**Tier**: 3 (auth-adjacent: changes how write access is gated across exams,
courses, blueprint templates)

## Context

Phase 9 enforced exam write access via subject membership: a teacher
assigned to "Mathematics" via `teacher_subjects` could author exams whose
`subject_id` was Mathematics. This was passable for a single-author model
but does not match how Indonesian schools actually divide work:

- Senior teacher writes the exam blueprint, junior teacher fills questions
- Two teachers co-author a multi-disciplinary exam (e.g. PKn + Sosiologi)
- Substitute teacher needs temporary edit access for one exam without
  being added to the subject permanently
- Department head wants read-only visibility into exam drafts without
  being able to break them
- Teacher leaves the school; ownership of their exams must transfer to
  someone else

The subject-based gate also caused a subtle bug: an exam with no subject
(`subject_id IS NULL`) was open to any teacher with `exams:write`,
silently bypassing the intended restriction.

## Decision

Adopt an explicit ownership + collaborator model on **exams**, **courses**,
and **blueprint_templates**, **layered on top of the existing subject-based
RBAC**. Subject-based RBAC is **preserved** as an institutional fallback
for implicit authority (waka kurikulum, ketua MGMP, head-of-department
patterns common in Indonesian schools).

### Schema

Each governed resource gains `owner_user_id` (FK to `users`, NOT NULL,
backfilled from `created_by` on migration). Plus a per-resource
collaborator junction:

```sql
exam_collaborators (
    id UUID PK,
    tenant_id UUID,
    exam_id UUID FK exams ON DELETE CASCADE,
    user_id UUID FK users ON DELETE CASCADE,
    role TEXT CHECK (role IN ('editor', 'viewer')),
    invited_by UUID FK users,
    invited_at TIMESTAMPTZ,
    UNIQUE (exam_id, user_id)
);
-- analogous: course_collaborators, blueprint_template_collaborators
```

Tables are tenant-scoped to keep audit cleanly partitioned even though
the FK transitively enforces tenant.

### Access matrix

| Actor | Read | Edit metadata + content | Manage collaborators | Transfer ownership | Archive/delete |
|---|---|---|---|---|---|
| Tenant admin (master_admin, school_admin, academic_admin) | ✓ | ✓ | ✓ | ✓ | ✓ |
| Owner | ✓ | ✓ | ✓ | ✓ (forfeits ownership) | ✓ |
| Collaborator (editor) | ✓ | ✓ | — | — | — |
| Collaborator (viewer) | ✓ | — | — | — | — |
| Subject teacher (institutional fallback) | ✓ | — | — | — | — |
| Other teacher / staff (same tenant) | List view only | — | — | — | — |
| Cross-tenant user | — (404) | — | — | — | — |

**Institutional fallback**: when a teacher's `teacher_subjects`
assignment matches the exam's `subject_id`, they get viewer-equivalent
read access (full detail, no edit). This preserves the implicit
authority pattern where a head-of-department or waka kurikulum can
monitor exams in their domain without explicit invitation. To grant
edit access, they must still be added as `editor` collaborator OR be
promoted to `academic_admin` role.

Documentation will recommend: "Give waka kurikulum the
`academic_admin` role for full department-wide override. Use
`editor` collaborator for ad-hoc co-authoring."

"List view only" means the resource appears in `GET /api/v1/exams` (so
the school's exam catalog is transparent) but `GET /api/v1/exams/{id}`
returns 404. This avoids leaking structured details (questions,
blueprint, etc.) to non-collaborators.

**`canAccess` field on list responses**: every item in `GET /api/v1/exams`
(and equivalent course/blueprint listings) includes a `canAccess: bool`
flag computed server-side. Frontend uses this to disable cards or show a
lock icon for non-accessible rows, preventing the click→404 dead end.

### Permission helpers

Three helpers complement the existing subject-based access (which
remains for institutional read fallback):

```go
// requireExamAccess returns true if the actor can perform the given action
// on the exam. Action is one of: "read", "write", "manage", "delete".
//
// Layered evaluation (first match wins):
//   1. Tenant admin (master/school/academic_admin) → all actions
//   2. Owner → all actions (manage/delete only for owner)
//   3. Editor collaborator → read + write
//   4. Viewer collaborator → read
//   5. Subject institutional fallback (teacher_subjects match) → read only
//   6. Otherwise → 404 for read, 403 for write attempts
//
// 404 is used for read failures to avoid leaking existence; 403 is used
// when the user CAN see the resource (read passes) but lacks the
// requested action level — the latter helps the AI bot self-correct.
func (a *App) requireExamAccess(w, r, examID, action) bool
```

Mirror functions: `requireCourseAccess`, `requireBlueprintAccess`.
(Subject fallback is exam-specific and does not apply to courses or
blueprints — those use only the explicit collaborator model.)

Cascading: nested resources (`exam_sections`, `exam_questions`,
`exam_question_options`, `exam_question_groups`, `exam_blueprint_slots`)
inherit access from their parent exam — no per-question collaborators.
The handlers for those resources call `requireExamAccess` against the
parent.

**Take-exam path is excluded**: the collaboration model applies ONLY to
authoring endpoints (`/api/v1/exams/*` management, `/api/v1/questions/*`,
`/api/v1/exam-sections/*`, `/api/v1/exam-blueprints/*`). The student
consumption surface (Phase 10: `/api/v1/programs/{programId}/exams/{examId}/take`)
uses enrollment-based access only. ADR-0009 helpers must NOT be called
from take-exam handlers, or students will get 404 on their own assigned
exams. This separation is documented in `requireExamAccess` Go doc and
enforced by code review.

### Ownership transfer

```
PATCH /api/v1/exams/{id}/transfer-ownership
Body: { "newOwnerId": "uuid" }
```

Effect (transactional):
1. Verify caller is current owner OR tenant admin
2. Verify newOwner is in the same tenant
3. If newOwner is currently a collaborator, demote that row to owner; old
   owner becomes editor in the same tx
4. If newOwner is not a collaborator, set as owner, add old owner as
   editor
5. Audit event `exams.transfer_ownership` with both UIDs

This guarantees the previous owner does not lose access entirely (avoids
"oh no I transferred and now I'm locked out" scenarios). They can leave
voluntarily by removing themselves as collaborator afterwards.

### Collaborator management

```
GET    /api/v1/exams/{id}/collaborators       — list (read access required)
POST   /api/v1/exams/{id}/collaborators       — invite (manage required)
       Body: { userId, role }
PATCH  /api/v1/exam-collaborators/{collabId}  — change role (manage required)
DELETE /api/v1/exam-collaborators/{collabId}  — remove (manage; or self for "leave")
```

The "leave" semantic: collaborator can DELETE their own row even without
manage permission. Owner cannot leave — they must transfer first.

### AI bot integration

AI tools call the same helper functions. Bot acting as teacher A who is
not collaborator on exam X gets a `403 forbidden` mapped to the existing
`ToolError` envelope:

```json
{
  "error": {
    "code": "PERMISSION_DENIED",
    "message": "You don't have edit access to this exam",
    "recoverable": true,
    "recovery": {
      "tool": "list_exams",
      "hint": "List exams the user has access to first."
    }
  }
}
```

Bot self-corrects per ADR-0006 protocol.

### Subject-based RBAC: removed

`requireExamSubjectAccess` is deleted. The `teacher_subjects` table
remains for academic operations (e.g. who teaches what class) but no
longer gates exam edits. Admins who want to restrict by subject simply
invite the right teacher as collaborator.

This removes one of two parallel permission systems, simplifies mental
model, and closes the `subject_id IS NULL` bypass bug.

### Migration backfill — pre-flight audit required

Because Phase 9's `created_by` columns are nullable, a naive backfill
risks silent NULL-to-random-admin assignment. Migration 14 will run a
pre-flight audit query and **abort with a structured error** if any
problematic row exists, rather than silently picking an owner.

Process:

1. **Pre-migration audit script**
   `backend/internal/platform/migrate/preflight_owner_backfill.go` runs
   before migration 000014 and reports:

   ```sql
   -- Rows with NULL created_by
   SELECT id, tenant_id, title FROM exams WHERE created_by IS NULL;
   SELECT id, tenant_id, title FROM courses WHERE created_by IS NULL;

   -- Rows where created_by points to an archived/deleted user
   SELECT e.id, e.tenant_id, e.title FROM exams e
     LEFT JOIN users u ON u.id = e.created_by
    WHERE e.created_by IS NOT NULL
      AND (u.id IS NULL OR u.status = 'archived');
   ```

2. **If any rows surface**: migration aborts with a clear error message
   listing the orphaned rows. Operator must resolve via the
   `/admin/orphaned-resources` endpoint (added in this phase) which
   lets a tenant admin claim ownership row-by-row OR bulk-assign to a
   chosen admin user.

3. **Once audit is clean**:
   ```sql
   ALTER TABLE exams   ADD COLUMN owner_user_id UUID REFERENCES users(id);
   ALTER TABLE courses ADD COLUMN owner_user_id UUID REFERENCES users(id);
   UPDATE exams   SET owner_user_id = created_by;
   UPDATE courses SET owner_user_id = created_by;
   ALTER TABLE exams   ALTER COLUMN owner_user_id SET NOT NULL;
   ALTER TABLE courses ALTER COLUMN owner_user_id SET NOT NULL;
   ```

This is deterministic across environments and never silently picks an
owner.

## Alternatives considered

- **Keep subject-based RBAC, add collaborator overrides** — two parallel
  systems, hard to reason about. Rejected.
- **Roles per resource via generic `permissions` table** — over-general
  for current needs, hides the data model from queries. Rejected for
  simplicity.
- **GitHub-style organization-level base role + per-resource override**
  — too many concepts. Tenant admin already serves the "org-level" role.
- **Three roles instead of two** (owner / editor / commenter / viewer):
  no clear use for "commenter" yet. Defer.
- **Notify on invite via email** — out of scope for Phase 9.5; will
  defer to whenever email infra lands.

## Consequences

Positive:
- Clear mental model: "who can touch this exam?" is one query
- Realistic teacher collaboration patterns are first-class supported
- Ownership transfer handles teacher transitions cleanly
- AI bot inherits the same access checks for free
- Cross-tenant isolation gets stronger (404 leak protection)

Negative:
- Migration is non-trivial: backfilling `owner_user_id` requires careful
  handling of legacy rows with NULL `created_by`
- Existing UI patterns (subject filter) still work but no longer affect
  permissions; risk of admin confusion. Mitigated by sidebar copy and
  release notes.
- Three new collaborator tables. Fine for Postgres at LMS scale.

Neutral:
- `teacher_subjects` keeps its current role for academic structure.
  Decoupling exam permission from teacher_subjects also means an exam
  authored by a teacher reflects what the school *delegated*, not what
  they happened to be assigned for class instruction.

## Implementation

- `backend/migrations/000014_collaboration.sql` — owner columns + 3
  collaborator tables + backfill
- `backend/internal/app/collaborators.go` — `requireExamAccess`,
  `requireCourseAccess`, `requireBlueprintAccess` helpers, plus generic
  list/invite/change/remove handlers shared by all 3 resource types
- `backend/internal/app/exams.go` — replace
  `requireExamSubjectAccess` calls with `requireExamAccess(action)`
- `backend/internal/app/courses.go` — same treatment
- `backend/internal/app/exam_sections.go`, `questions.go`,
  `exam_gates.go` — switch parent-exam access check to new helper
- `backend/internal/app/ai_*` — same helpers used in tool handlers
- `frontend/src/components/share-dialog.tsx` — reusable invite/list UI
- `frontend/src/lib/modules-api.ts` — collaborator endpoints

## Future work

- Per-collaborator notes ("invited because X requested help with chapter 3")
- Email notifications on invite (after email infra)
- Audit-log search by user across resources
- Bulk transfer ownership (admin tool: "teacher resigning, transfer all
  their exams to head of department")
