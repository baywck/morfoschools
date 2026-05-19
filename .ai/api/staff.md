# Staff API

> Permission: `users:read` / `users:write` | Tenant-scoped

## GET /api/v1/staff
**Query:** `?search=&status=&page=1&pageSize=20`

**200:**
```json
{
  "data": [{ "id", "userId", "email", "displayName", "employeeId", "department", "position", "status", "createdAt" }],
  "pagination": { "page", "pageSize", "total", "totalPages" }
}
```

## POST /api/v1/staff
Create staff record (user must exist).

**Body:** `{ "userId": "uuid", "employeeId?", "department?", "position?" }`

## POST /api/v1/staff/create-full
Create user + staff in one call.

**Body:**
```json
{ "displayName": "required", "email": "required", "password": "required, min 6", "employeeId?", "department?", "position?" }
```

**201:** `{ "id": "staff_uuid", "userId": "user_uuid" }`

## PATCH /api/v1/staff/{id}
**Body:** `{ "employeeId?", "department?", "position?", "status?": "active|inactive|archived" }`

## PATCH /api/v1/staff/{id}/archive
Archive staff. **Cascades**: when this is the user's last active profile, the user is also archived and their email is freed for reuse (replaced with a synthetic `archived+<uuid>@archived.morfoschools.local`; original preserved in `users.original_email`).

**200:** `{ "status": "archived", "userArchived": bool }`

## PATCH /api/v1/staff/{id}/restore
Restore an archived staff profile and the parent user account.

**409 (validation):** `{ "fields": { "email": "..." } }` when the original email is now held by another active user.

**200:** `{ "id", "status": "active" }`
