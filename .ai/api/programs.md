# Programs API

> Permission: `courses:read` / `courses:write` | Tenant-scoped

## GET /api/v1/programs
**Query:** `?search=&status=&page=1&pageSize=20`

**200:**
```json
{
  "data": [{ "id", "title", "description", "kind", "gradeLevel", "subjectId", "subjectName", "status", "createdAt" }],
  "pagination": { "page", "pageSize", "total", "totalPages" }
}
```

## POST /api/v1/programs
**Body:**
```json
{ "title": "required", "description?", "kind": "curriculum|extracurricular|remedial", "gradeLevel": "required", "subjectId": "uuid required" }
```

## PATCH /api/v1/programs/{id}
**Body:** `{ "title?", "description?", "kind?", "gradeLevel?" }`

## PATCH /api/v1/programs/{id}/archive
Archive program.

## PATCH /api/v1/programs/{id}/publish
Publish program.

---

## Sections

### GET /api/v1/programs/{id}/sections
**200:** `{ "data": [{ "id", "title", "sortOrder", "unlockMode", "isRequired", "items": [...] }] }`

### POST /api/v1/programs/{id}/sections
**Body:** `{ "title": "required", "unlockMode?": "sequential|free", "isRequired?": "bool" }`

### PATCH /api/v1/program-sections/{sectionId}
**Body:** `{ "title?", "sortOrder?": "int", "unlockMode?", "isRequired?": "bool" }`

### DELETE /api/v1/program-sections/{sectionId}
Delete section.

---

## Section Items

### POST /api/v1/program-sections/{sectionId}/items
**Body:**
```json
{ "itemType": "course|exam|assignment", "itemId": "uuid required", "isRequired?": "bool", "passingGrade?": "int", "maxAttempts?": "int" }
```

### PATCH /api/v1/program-items/{itemId}
**Body:** `{ "sortOrder?": "int", "isRequired?": "bool", "passingGrade?": "int", "maxAttempts?": "int" }`

### DELETE /api/v1/program-items/{itemId}
Delete item.
