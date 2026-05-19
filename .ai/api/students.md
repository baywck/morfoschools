# Students API

> Permission: `users:read` / `users:write` | Tenant-scoped

## GET /api/v1/students
**Query:** `?search=&status=&page=1&pageSize=20`

**200:**
```json
{
  "data": [{ "id", "userId", "email", "displayName", "studentIdNumber", "gradeLevel", "status", "createdAt" }],
  "pagination": { "page", "pageSize", "total", "totalPages" }
}
```

## POST /api/v1/students
Create student record (user must exist).

**Body:** `{ "userId": "uuid", "studentIdNumber?", "gradeLevel?" }`

## POST /api/v1/students/create-full
Create user + student in one call.

**Body:**
```json
{ "displayName": "required", "email": "required", "password": "required, min 6", "studentIdNumber?", "gradeLevel?", "classSectionId?": "uuid" }
```

**201:** `{ "id": "student_uuid", "userId": "user_uuid" }`

## PATCH /api/v1/students/{id}
**Body:** `{ "studentIdNumber?", "gradeLevel?", "status?": "active|inactive|archived" }`

## PATCH /api/v1/students/{id}/archive
Archive student. **Cascades**: when this is the user's last active profile, the user is also archived and their email is freed for reuse (replaced with `archived+<uuid>@archived.morfoschools.local`; original email preserved in `users.original_email`).

**200:** `{ "status": "archived", "userArchived": bool }`

## PATCH /api/v1/students/{id}/restore
Restore an archived student profile and the parent user account. Restores the user's original email if still available.

**409 (validation):** `{ "fields": { "email": "..." } }` when the original email is now held by another active user. Caller must restore the user via `PATCH /api/v1/users/{id}/restore` with a fresh email first.

**200:** `{ "id", "status": "active" }`
