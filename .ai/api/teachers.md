# Teachers API

> Permission: `users:read` / `users:write` | Tenant-scoped

## GET /api/v1/teachers
**Query:** `?search=&status=&page=1&pageSize=20`

**200:**
```json
{
  "data": [{ "id", "userId", "email", "displayName", "employeeId", "specialization", "status", "createdAt" }],
  "pagination": { "page", "pageSize", "total", "totalPages" }
}
```

## POST /api/v1/teachers
Create teacher record (user must exist).

**Body:** `{ "userId": "uuid", "employeeId?", "specialization?" }`

## POST /api/v1/teachers/create-full
Create user + teacher + optional subject assignments.

**Body:**
```json
{ "displayName": "required", "email": "required", "password": "required, min 6", "employeeId?", "specialization?", "subjectIds?": ["uuid"] }
```

**201:** `{ "id": "teacher_uuid", "userId": "user_uuid" }`

## PATCH /api/v1/teachers/{id}
**Body:** `{ "employeeId?", "specialization?", "status?": "active|inactive|archived" }`

## PATCH /api/v1/teachers/{id}/archive
Archive teacher. **Cascades**: when this is the user's last active profile, the user is also archived and their email is freed for reuse (replaced with a synthetic `archived+<uuid>@archived.morfoschools.local`; original preserved in `users.original_email`).

**200:** `{ "status": "archived", "userArchived": bool }`

## PATCH /api/v1/teachers/{id}/restore
Restore an archived teacher and the parent user account. Restores the user's original email if still available.

**409 (validation):** `{ "fields": { "email": "..." } }` when the original email is now held by another active user.

**200:** `{ "id", "status": "active" }`

---

## Teacher Subjects

> Permission: `academic:read` / `academic:write`

### GET /api/v1/teachers/{id}/subjects
**200:** `{ "data": [{ "id", "code", "name" }] }`

### POST /api/v1/teachers/{id}/subjects
**Body:** `{ "subjectId": "uuid required" }`

**201:** `{ "teacherId", "subjectId" }`

### DELETE /api/v1/teachers/{id}/subjects/{subjectId}
**200:** `{ "status": "unassigned" }`
