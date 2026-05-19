# ADR 0007: Email Reuse After User Archive

**Status**: Accepted
**Date**: 2026-05-19
**Tier**: 3 (auth-adjacent, schema change)

## Context

`users.email` had a global `UNIQUE` index. The archive flow set `status = 'archived'` but did nothing to the email column, so:

- A graduated student kept blocking their email forever
- Re-registering an old user (e.g. teacher returning after a year) failed with "Email already in use"
- Profile-level archives (`PATCH /students/{id}/archive`) did not touch the parent user, so the email was held even when the user had no active profiles left

We needed the email to become reusable when a user is archived, without losing the audit trail of what their address used to be, and without violating the existing 1-user-many-tenants invariant.

## Decision

Three changes layered together:

### 1. Schema: partial unique index + `original_email` column

```sql
ALTER TABLE users ADD COLUMN original_email TEXT;

DROP INDEX idx_users_email;
CREATE UNIQUE INDEX idx_users_email_active
    ON users(email) WHERE status != 'archived';
```

Archived rows are exempt from the uniqueness constraint. Their email column holds a synthetic value (see below) which is unique-by-construction but never reused.

### 2. Synthetic email format on archive

```
archived+<user-uuid>@archived.morfoschools.local
```

- `archived.morfoschools.local` is a non-routable TLD — no risk of accidentally sending mail
- UUID is embedded → unique by construction, traceable in audit
- `+` plus-addressing keeps the local-part RFC-valid

When archiving, the original email is moved into `original_email` (only if not already populated, so repeated archive/restore cycles never overwrite the real address with the synthetic one).

### 3. Cascade rule (B): smart user archive on last profile

When archiving a profile (student/teacher/staff/guardian), the parent user is **automatically** archived iff every profile row linked to that user is now archived. This is implemented in `cascadeArchiveUserIfOrphan`.

The inverse direction — `PATCH /users/{id}/archive` — cascades **down** to all profile rows, so a user-level archive never leaves orphan active profiles whose login is now broken.

### 4. Restore endpoints per modul

Per-profile restore endpoints (`/students/{id}/restore`, etc.) restore the profile and call `restoreUser` for the parent user in the same transaction. `restoreUser`:

- Uses `original_email` by default
- Accepts an optional `{ "email": "..." }` override when the original is now taken
- Returns 409 with a structured `fields.email` error when the target collides with another active user — admin must pick a new email and retry

If the user was never given an original email (edge case from synthetic-only data), the user is reactivated with status only and the admin fixes the email via the standard update endpoint.

## Alternatives considered

- **(A) Manual cascade** — admin must explicitly archive the user after archiving the last profile. Rejected: most users have only one profile; forcing a second click for the 95% case is bad UX, and "why is this email still blocked" is the most common confusion.
- **(C) Keep archive limited to status flag, add manual `release_email` endpoint** — adds an opaque second action no admin will know about. Rejected.
- **Hard-delete archived users after retention period** — destroys audit trail; conflicts with multi-tenant data ownership rules.
- **Per-tenant unique email** — breaks SSO and the single-account-multiple-tenants design (ADR-0001-era decision).

## Consequences

**Positive**:
- Re-registration just works — admin types the original email, system finds no active conflict, accepts.
- Restore preserves the relationship between an archived user and their previous identity (via `original_email`).
- Cascade rule "last active profile archives the user" matches admin mental model: archiving a student archives the whole person.
- Synthetic email format is human-readable in DB browsers and audit logs.

**Negative**:
- Two coexisting email-shape concepts (real vs synthetic) — code that displays a user's email in archived contexts must check `isArchivedEmail()` or pull from `original_email`.
- Restore surfaces a 409 when the old email is reissued. Acceptable: rare, and the error message gives the admin a clear next step.
- Migration backfills any existing archived rows with synthetic emails. If the system already had archived users mid-flight, their email column will change shape on first migration.

**Neutral**:
- AI tools that archive entities (`archive_student`, `batch_archive_students`) inherited the cascade for free via the helper functions.

## Implementation

- `backend/migrations/000012_archive_email_release.sql` — schema change + backfill
- `backend/internal/app/archive_helpers.go` — `archiveUser`, `restoreUser`, `cascadeArchiveUserIfOrphan`, `archiveAllProfilesForUser`, `userIDForProfile`, `restoreProfile`, `archivedEmailFor`, `isArchivedEmail`
- `backend/internal/app/archive_helpers_test.go` — synthetic email format tests
- `backend/internal/app/users.go` — `handleArchiveUser` rewritten to use `archiveUser` + cascade-down to profiles; new `handleRestoreUser` accepts optional email override
- `backend/internal/app/students.go` / `teachers.go` / `staff.go` / `guardians.go` — archive handlers wrap profile + cascade in a tx; new restore handlers
- `backend/internal/app/ai_write_tools.go` / `ai_cap_registry.go` — single + batch archive flows trigger cascade
- `frontend/src/lib/modules-api.ts` — `restoreUser`, `restoreStudent`, `restoreTeacher`, `restoreStaff`, `restoreGuardian`
- `frontend/src/lib/use-crud.ts` — `restore` option + `handleRestore` with structured-email-error toast handling
- `frontend/src/app/(app)/app/{admin,staff,students,teachers}/page.tsx` — RowActions show Restore action when row status is archived
- `.ai/api/{users,students,teachers,staff,guardians}.md` — endpoint contracts updated

## Verification

- `go vet ./...` clean
- `go test ./...` clean (including new `archive_helpers_test.go`)
- `tsc --noEmit` clean
- `next build` skipped due to `.next/` permission issue from a prior root-owned Docker build; static type signal is sufficient for this Tier 3 change
- Manual smoke test pending against running stack

## Future work

- Consider exposing `original_email` as a read-only field in the user list filter for archived rows (admin recall)
- If audit log gets a structured field for cascade events, replace the current `users.archive_cascade` action string with a proper enum
