# Users API

> Permission: `users:read` / `users:write` | Tenant-scoped

## GET /api/v1/users
**Query:** `?search=&status=&role=school_admin&page=1&pageSize=20`

**200:**
```json
{
  "data": [{ "id", "email", "displayName", "status", "isPlatformAdmin", "createdAt" }],
  "pagination": { "page", "pageSize", "total", "totalPages" }
}
```

## POST /api/v1/users
**Body:**
```json
{ "email": "required", "password": "required, min 6", "displayName": "required", "roleSlug?": "school_admin|teacher|student|staff" }
```

## PATCH /api/v1/users/{id}
**Body:**
```json
{ "displayName?", "email?": "unique", "password?": "min 6", "status?": "active|inactive|archived" }
```

## PATCH /api/v1/users/{id}/archive
Archive user. Cascades to **all** profile rows (teachers/students/staff/guardians) for that user. The email is replaced with a synthetic `archived+<uuid>@archived.morfoschools.local` value and the original is preserved in `users.original_email` for restore.

**200:** `{ "status": "archived" }`

## PATCH /api/v1/users/{id}/restore
Restore an archived user. Restores `email` from `original_email` when available.

**Body (optional):** `{ "email": "override@example.com" }` — used when the original email is taken or the admin wants a new address.

**409 (validation):** `{ "fields": { "email": "Email <addr> is already in use. Provide a different email..." } }` when the resolved email collides with an active user.

**200:** `{ "id", "status": "active", "email": "resolved@example.com" }`

## GET /api/v1/roles
List roles for current tenant.

**200:** `{ "data": [{ "id", "slug", "name" }] }`
