# Golden CRUD Pattern — Morfoschools

Template untuk setiap domain module. Semua module harus mengikuti pattern ini.

## Backend Pattern

### File Structure
```
backend/internal/app/<module>.go       — handlers + routes
backend/internal/app/<module>_test.go  — handler tests
```

### Route Registration
```go
func (a *App) register<Module>Routes(mux *http.ServeMux) {
    mux.HandleFunc("GET /api/v1/<resources>", a.handleList<Resources>)
    mux.HandleFunc("POST /api/v1/<resources>", a.handleCreate<Resource>)
    mux.HandleFunc("PATCH /api/v1/<resources>/{id}", a.handleUpdate<Resource>)
    mux.HandleFunc("PATCH /api/v1/<resources>/{id}/archive", a.handleArchive<Resource>)
}
```

### Handler Template (List)
```go
func (a *App) handleList<Resources>(w http.ResponseWriter, r *http.Request) {
    // 1. Permission check
    if !a.RequirePermission(w, r, "<module>:read") { return }

    // 2. Tenant context
    tenantID := a.RequireEffectiveTenant(w, r)
    if tenantID == "" { return }

    // 3. Parse pagination + filters
    p := httpx.ParsePagination(r)
    search := httpx.QueryString(r, "search", "")
    status := httpx.QueryString(r, "status", "")

    // 4. Count query (tenant-scoped)
    // 5. Data query (tenant-scoped, paginated)
    // 6. Return httpx.NewPaginatedResponse(data, p, total)
}
```

### Handler Template (Create)
```go
func (a *App) handleCreate<Resource>(w http.ResponseWriter, r *http.Request) {
    // 1. Permission check
    if !a.RequirePermission(w, r, "<module>:write") { return }

    // 2. Tenant context
    tenantID := a.RequireEffectiveTenant(w, r)
    if tenantID == "" { return }

    // 3. CSRF check
    if !a.RequireCSRF(w, r) { return }

    // 4. Parse + validate body (structured field errors)
    // 5. Business logic validation (uniqueness, relationships)
    // 6. Insert (transaction if multi-table)
    // 7. Audit event
    // 8. Return 201 with created resource
}
```

### Handler Template (Update)
```go
func (a *App) handleUpdate<Resource>(w http.ResponseWriter, r *http.Request) {
    // 1. Permission check
    // 2. Tenant context
    // 3. CSRF check
    // 4. Path param: id
    // 5. Verify resource belongs to tenant
    // 6. Parse + validate body
    // 7. Update fields
    // 8. Audit event
    // 9. Return 200 with updated resource
}
```

### Handler Template (Archive)
```go
func (a *App) handleArchive<Resource>(w http.ResponseWriter, r *http.Request) {
    // 1. Permission check
    // 2. Tenant context
    // 3. CSRF check
    // 4. Path param: id
    // 5. Verify resource belongs to tenant
    // 6. Set status = 'archived'
    // 7. Audit event
    // 8. Return 200 {status: "archived"}
}
```

### Validation Rules
- Always return structured field errors: `writeValidationError(w, map[string]string{"field": "message"}, r)`
- Validate required fields first
- Then validate format/length
- Then validate business rules (uniqueness, relationships)

### Audit Events
- Format: `<module>.<action>` (e.g. `users.create`, `users.update`, `users.archive`)
- Always include: tenantID, actorID, resourceType, resourceID

### SQL Rules
- ALL queries must include `tenant_id` filter
- Use `$N` parameterized queries (never string interpolation)
- Use transactions for multi-table writes
- Scan nullable fields into `sql.NullString` then map to Go types

---

## Frontend Pattern

### File Structure
```
frontend/src/lib/<module>-api.ts           — API client functions
frontend/src/app/(app)/app/<module>/page.tsx — page component
```

### API Client
```typescript
import { get, post, patch } from "./api-client";
import type { ApiResponse } from "./api-client";

export interface <Resource> {
  id: string;
  // ... fields
  status: string;
  createdAt: string;
}

export interface <Resource>ListResponse {
  data: <Resource>[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

export function list<Resources>(params?: { page?: number; search?: string; status?: string }) {
  const query = new URLSearchParams();
  if (params?.page) query.set("page", String(params.page));
  if (params?.search) query.set("search", params.search);
  if (params?.status) query.set("status", params.status);
  return get<Resource>ListResponse>(`/api/v1/<resources>?${query}`);
}

export function create<Resource>(data: Create<Resource>Request) {
  return post<Resource>("/api/v1/<resources>", data);
}

export function update<Resource>(id: string, data: Update<Resource>Request) {
  return patch<Resource>(`/api/v1/<resources>/${id}`, data);
}

export function archive<Resource>(id: string) {
  return patch<{ status: string }>(`/api/v1/<resources>/${id}/archive`);
}
```

### Page Component (inside AppShell)
```
Page must include:
1. Page header (title + subtitle + create action button)
2. Search/filter toolbar
3. Data states: loading (skeleton), empty (EmptyState), error, data (table/cards)
4. Create/Edit via RightPullSheet or Modal
5. Archive via ConfirmDialog
6. Toast feedback on mutations
```

### Data States (mandatory)
- **Loading**: skeleton rows matching real content structure
- **Empty**: EmptyState component with icon + message + create action
- **Error**: error message + retry button
- **No results** (with active filter): different message from empty

---

## Module Definition of Done

```
[ ] Backend handlers implemented (list, create, update, archive)
[ ] All handlers: permission check + tenant scope + CSRF on writes
[ ] Structured validation errors (fields object)
[ ] Audit events emitted for all writes
[ ] SQL is tenant-scoped (no cross-tenant leaks)
[ ] Frontend API client with typed responses
[ ] Frontend page with all data states (loading, empty, error, data)
[ ] Create/Edit form with validation feedback
[ ] Archive with confirmation
[ ] Toast feedback on success/error
[ ] `go build ./...` passes
[ ] `go test ./...` passes
[ ] `npx tsc --noEmit` passes
```
