# Class Sections API

> Permission: `academic:read` / `academic:write` | Tenant-scoped

## GET /api/v1/class-sections
**Query:** `?search=&status=&page=1&pageSize=50`

**200:**
```json
{
  "data": [{ "id", "name", "gradeLevel", "academicYearId", "academicYearName", "homeroomTeacherId", "homeroomTeacherName", "capacity", "status", "createdAt" }],
  "pagination": { "page", "pageSize", "total", "totalPages" }
}
```

## POST /api/v1/class-sections
**Body:**
```json
{ "name": "required", "gradeLevel": "required", "academicYearId": "uuid required", "homeroomTeacherId?": "uuid", "capacity?": "int" }
```

## PATCH /api/v1/class-sections/{id}
**Body:** `{ "name?", "gradeLevel?", "homeroomTeacherId?": "uuid", "capacity?": "int", "status?": "active|inactive|archived" }`

## PATCH /api/v1/class-sections/{id}/archive
Archive class section.
