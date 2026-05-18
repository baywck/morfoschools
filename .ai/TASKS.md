# Tasks — Morfoschools

## Current Phase: Phase 9 — Exams (next session)

### Next Steps
- [ ] Exams CRUD (BE + FE)
- [ ] Exam Sections + Questions
- [ ] Exam Gate Windows
- [ ] Phase 10: Exam Critical Path (take, autosave, submit, receipt)
- [ ] Phase 11: Teacher Operations (monitor, grading)
- [ ] Phase 12: AI Agent Runtime
- [ ] Tenant logo upload (R2/local)

## Completed
- [x] Phase 0: .ai/ memory files, ADRs, AGENTS.md, standards
- [x] Phase 1: Docker Compose (6 services), migrations (6), backend skeleton, frontend skeleton
- [x] Phase 2: Auth/login/RBAC/session/CSRF, dev seed (7 users, roles, permissions)
- [x] Phase 3: Frontend shell (morfosis-studio: dark shell, 66px sidebar, floating card, AI chat panel)
- [x] Phase 4: Backend patterns (pagination, response helpers, RBAC helpers, tenant switch)
- [x] Phase 6: All Admin Modules (BE + FE) — Users, Tenants, Teachers, Students, Staff, Guardians
  - Composite create (user + profile in one request)
  - Teachers: multi-subject assignment
  - Students: optional class + guardian management
  - Role select (enum, not text)
  - Edit flows for all modules
- [x] Phase 7: Academic Structure (BE + FE) — Academic Years, Subjects, Class Sections, Teacher-Subject assignments
  - Grade level predefined (SD1-SMA12) + custom
  - Homeroom teacher select
  - Academic year select
- [x] Phase 8: Programs + Courses (BE + FE)
  - Programs: CRUD + sections + items + publish/archive
  - Courses: CRUD + publish/archive
  - Program sections with nested items (json_agg)

## UI Components Built
- [x] InputField (floating label, h-11, prefix icon)
- [x] SelectField (floating label dropdown, disabled support)
- [x] DatePicker (custom calendar, no native)
- [x] DateRangePicker (start/end range)
- [x] SearchInput (compact h-8, plain)
- [x] Button (primary=black, loading spinner)
- [x] RightPullSheet (no overlay, rounded-r-inherit)
- [x] ConfirmDialog (centered, destructive variant)
- [x] RowActions (portal dropdown, 3-dot)
- [x] Toast (border-l-4, tones)
- [x] Skeleton
- [x] Breadcrumb (dynamic, Home icon)
- [x] PageShell (sticky header, responsive)
- [x] AppShell (dark shell + floating card + AI chat push)
- [x] Sidebar (66px icon strip)
- [x] Topbar (breadcrumb + user + AI toggle)
- [x] MobileNav (bottom h-16, horizontal scroll)
- [x] AI Chat Panel (360px, model selector, attach menu, auto-resize)

## Key Decisions Made
- No native form validation (all server-side)
- Loading state on every async action
- System font stack (no Google Fonts)
- Portal for dropdowns (escape overflow)
- RightPullSheet without overlay (user can interact outside)
- AI Chat pushes content (not overlays)
- Composite create endpoints (user + profile + assignments)
- Programs as enrollment unit (ADR-0001)
- Score decoupling (ADR-0002)
- Enrollment persistence on class transfer (ADR-0003)
- Status + Result separation (ADR-0004)
- No structure locking (ADR-0005)
