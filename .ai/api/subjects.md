# Subjects API

> Permission: `academic:read` / `academic:write` | Tenant-scoped

## GET /api/v1/subjects
**Query:** `?search=&status=&page=1&pageSize=50`

**200:**
```json
{
  "data": [{ "id", "code", "name", "description", "status", "createdAt" }],
  "pagination": { "page", "pageSize", "total", "totalPages" }
}
```

## POST /api/v1/subjects
**Body:** `{ "code": "required", "name": "required", "description?" }`

## PATCH /api/v1/subjects/{id}
**Body:** `{ "name?", "description?", "status?": "active|inactive|archived" }`

## PATCH /api/v1/subjects/{id}/archive
Archive subject.
