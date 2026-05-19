# Academic Years API

> Permission: `academic:read` / `academic:write` | Tenant-scoped

## GET /api/v1/academic-years
**200:**
```json
{
  "data": [{ "id", "code", "name", "startsOn", "endsOn", "status", "createdAt" }],
  "pagination": { "page", "pageSize", "total", "totalPages" }
}
```

## POST /api/v1/academic-years
**Body:** `{ "code": "required", "name": "required", "startsOn?": "YYYY-MM-DD", "endsOn?": "YYYY-MM-DD" }`

## PATCH /api/v1/academic-years/{id}
**Body:** `{ "name?", "startsOn?": "YYYY-MM-DD", "endsOn?": "YYYY-MM-DD", "status?": "active|inactive|archived" }`

## PATCH /api/v1/academic-years/{id}/archive
Archive academic year.
