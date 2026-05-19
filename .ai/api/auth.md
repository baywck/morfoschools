# Auth API

## POST /api/v1/auth/login
Login. Sets `morfoschools_session` cookie.

**Body:** `{ "email": "string", "password": "string" }`

**200:** `{ "user": { "id", "email", "displayName", "isPlatformAdmin" }, "csrfToken": "string" }`

---

## POST /api/v1/auth/logout
Destroy session. Requires CSRF.

**200:** `{ "status": "logged_out" }`

---

## GET /api/v1/auth/me
Current session info.

**200:**
```json
{
  "user": { "id", "email", "displayName", "isPlatformAdmin" },
  "effectiveTenantId": "uuid|null",
  "roles": ["school_admin"],
  "permissions": ["users:read", "users:write", ...]
}
```
