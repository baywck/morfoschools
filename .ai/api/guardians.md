# Guardians API

> Permission: `users:read` / `users:write` | Tenant-scoped

## GET /api/v1/guardians
**Query:** `?studentId=uuid`

**200:** `{ "data": [{ "id", "name", "phone", "email", "relationship", "status" }] }`

## POST /api/v1/guardians
**Body:** `{ "name": "required", "phone": "required", "email": "required", "relationship": "required", "userId?": "uuid" }`

## PATCH /api/v1/guardians/{id}
**Body:** `{ "name?", "phone?", "email?", "relationship?", "status?" }`

## PATCH /api/v1/guardians/{id}/archive
Archive guardian. **Cascades**: when the guardian has a linked user account and this is their last active profile, the user is also archived and the email is freed for reuse.

**200:** `{ "status": "archived", "userArchived": bool }`

## PATCH /api/v1/guardians/{id}/restore
Restore an archived guardian and the parent user (if linked).

**409 (validation):** `{ "fields": { "email": "..." } }` when the original email is now held by another active user.

**200:** `{ "id", "status": "active" }`

## POST /api/v1/guardians/{id}/link-student
Link guardian to a student.

**Body:** `{ "studentId": "uuid required", "isPrimary?": "bool" }`
