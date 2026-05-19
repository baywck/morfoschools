# API Index

> Base: `http://localhost:8080` | Auth: cookie session + `X-CSRF-Token` | Tenant-scoped unless noted.

| Method | Path | Module | Description |
|--------|------|--------|-------------|
| POST | /api/v1/auth/login | [auth](auth.md) | Login |
| POST | /api/v1/auth/logout | [auth](auth.md) | Logout |
| GET | /api/v1/auth/me | [auth](auth.md) | Current session |
| GET | /api/v1/tenants | [tenants](tenants.md) | List tenants |
| POST | /api/v1/tenants | [tenants](tenants.md) | Create tenant |
| PATCH | /api/v1/tenants/{id} | [tenants](tenants.md) | Update tenant |
| PATCH | /api/v1/tenants/{id}/archive | [tenants](tenants.md) | Archive tenant |
| POST | /api/v1/tenants/switch | [tenants](tenants.md) | Switch tenant context |
| GET | /api/v1/users | [users](users.md) | List users |
| POST | /api/v1/users | [users](users.md) | Create user |
| PATCH | /api/v1/users/{id} | [users](users.md) | Update user |
| PATCH | /api/v1/users/{id}/archive | [users](users.md) | Archive user |
| GET | /api/v1/roles | [users](users.md) | List roles |
| GET | /api/v1/teachers | [teachers](teachers.md) | List teachers |
| POST | /api/v1/teachers | [teachers](teachers.md) | Create teacher |
| POST | /api/v1/teachers/create-full | [teachers](teachers.md) | Create user+teacher |
| PATCH | /api/v1/teachers/{id} | [teachers](teachers.md) | Update teacher |
| PATCH | /api/v1/teachers/{id}/archive | [teachers](teachers.md) | Archive teacher |
| GET | /api/v1/teachers/{id}/subjects | [teachers](teachers.md) | List teacher subjects |
| POST | /api/v1/teachers/{id}/subjects | [teachers](teachers.md) | Assign subject |
| DELETE | /api/v1/teachers/{id}/subjects/{subjectId} | [teachers](teachers.md) | Unassign subject |
| GET | /api/v1/students | [students](students.md) | List students |
| POST | /api/v1/students | [students](students.md) | Create student |
| POST | /api/v1/students/create-full | [students](students.md) | Create user+student |
| PATCH | /api/v1/students/{id} | [students](students.md) | Update student |
| PATCH | /api/v1/students/{id}/archive | [students](students.md) | Archive student |
| GET | /api/v1/staff | [staff](staff.md) | List staff |
| POST | /api/v1/staff | [staff](staff.md) | Create staff |
| POST | /api/v1/staff/create-full | [staff](staff.md) | Create user+staff |
| PATCH | /api/v1/staff/{id} | [staff](staff.md) | Update staff |
| PATCH | /api/v1/staff/{id}/archive | [staff](staff.md) | Archive staff |
| GET | /api/v1/academic-years | [academic-years](academic-years.md) | List academic years |
| POST | /api/v1/academic-years | [academic-years](academic-years.md) | Create academic year |
| PATCH | /api/v1/academic-years/{id} | [academic-years](academic-years.md) | Update academic year |
| PATCH | /api/v1/academic-years/{id}/archive | [academic-years](academic-years.md) | Archive academic year |
| GET | /api/v1/class-sections | [class-sections](class-sections.md) | List class sections |
| POST | /api/v1/class-sections | [class-sections](class-sections.md) | Create class section |
| PATCH | /api/v1/class-sections/{id} | [class-sections](class-sections.md) | Update class section |
| PATCH | /api/v1/class-sections/{id}/archive | [class-sections](class-sections.md) | Archive class section |
| GET | /api/v1/subjects | [subjects](subjects.md) | List subjects |
| POST | /api/v1/subjects | [subjects](subjects.md) | Create subject |
| PATCH | /api/v1/subjects/{id} | [subjects](subjects.md) | Update subject |
| PATCH | /api/v1/subjects/{id}/archive | [subjects](subjects.md) | Archive subject |
| GET | /api/v1/guardians | [guardians](guardians.md) | List guardians |
| POST | /api/v1/guardians | [guardians](guardians.md) | Create guardian |
| PATCH | /api/v1/guardians/{id} | [guardians](guardians.md) | Update guardian |
| PATCH | /api/v1/guardians/{id}/archive | [guardians](guardians.md) | Archive guardian |
| POST | /api/v1/guardians/{id}/link-student | [guardians](guardians.md) | Link to student |
| GET | /api/v1/courses | [courses](courses.md) | List courses |
| POST | /api/v1/courses | [courses](courses.md) | Create course |
| PATCH | /api/v1/courses/{id} | [courses](courses.md) | Update course |
| PATCH | /api/v1/courses/{id}/archive | [courses](courses.md) | Archive course |
| PATCH | /api/v1/courses/{id}/publish | [courses](courses.md) | Publish course |
| GET | /api/v1/programs | [programs](programs.md) | List programs |
| POST | /api/v1/programs | [programs](programs.md) | Create program |
| PATCH | /api/v1/programs/{id} | [programs](programs.md) | Update program |
| PATCH | /api/v1/programs/{id}/archive | [programs](programs.md) | Archive program |
| PATCH | /api/v1/programs/{id}/publish | [programs](programs.md) | Publish program |
| GET | /api/v1/programs/{id}/sections | [programs](programs.md) | List sections |
| POST | /api/v1/programs/{id}/sections | [programs](programs.md) | Create section |
| PATCH | /api/v1/program-sections/{sectionId} | [programs](programs.md) | Update section |
| DELETE | /api/v1/program-sections/{sectionId} | [programs](programs.md) | Delete section |
| POST | /api/v1/program-sections/{sectionId}/items | [programs](programs.md) | Add item |
| PATCH | /api/v1/program-items/{itemId} | [programs](programs.md) | Update item |
| DELETE | /api/v1/program-items/{itemId} | [programs](programs.md) | Delete item |
| GET | /healthz | — | Liveness probe |
| GET | /readyz | — | Readiness probe |

## Conventions

- **Error format**: `{ "error": { "code", "message", "fields?", "requestId" } }`
- **Pagination**: `?page=1&pageSize=20` → response has `pagination: { page, pageSize, total, totalPages }`
- **PATCH**: partial update, only sent fields are changed
- **Archive**: soft-delete via `PATCH /{id}/archive`
- **CSRF**: required on POST/PATCH/DELETE — header `X-CSRF-Token`
