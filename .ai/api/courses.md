# Courses API

> Permission: `courses:read` / `courses:write` | Tenant-scoped

## GET /api/v1/courses
**Query:** `?search=&status=&page=1&pageSize=20`

**200:**
```json
{
  "data": [{ "id", "title", "description", "subjectId", "subjectName", "status", "createdAt" }],
  "pagination": { "page", "pageSize", "total", "totalPages" }
}
```

## POST /api/v1/courses
**Body:** `{ "title": "required", "description?", "subjectId": "uuid required" }`

## PATCH /api/v1/courses/{id}
**Body:** `{ "title?", "description?" }`

## PATCH /api/v1/courses/{id}/archive
Archive course.

## PATCH /api/v1/courses/{id}/publish
Publish course (status → `published`).
