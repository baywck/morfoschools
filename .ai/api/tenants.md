# Tenants API

> Permission: `tenants:read` / `tenants:write` (platform admin)

## GET /api/v1/tenants
**Query:** `?search=&page=1&pageSize=20`

**200:**
```json
{
  "data": [{ "id", "name", "code", "status", "createdAt" }],
  "pagination": { "page", "pageSize", "total", "totalPages" }
}
```

## POST /api/v1/tenants
**Body:** `{ "name": "required", "code": "required, unique" }`

**201:** `{ "id", "name", "code", "status": "active", "createdAt" }`

## PATCH /api/v1/tenants/{id}
**Body:** `{ "name?", "status?": "active|inactive|archived" }`

## PATCH /api/v1/tenants/{id}/archive
**200:** `{ "status": "archived" }`

## POST /api/v1/tenants/switch
Set effective tenant context for session.

**Body:** `{ "tenantId": "uuid" }`
