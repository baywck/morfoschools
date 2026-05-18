# Tasks — Morfoschools

## Current Phase: Phase 0 — Project Scaffold

### Active
- [x] Create .ai/ directory structure
- [x] Write CONTEXT.md (domain glossary)
- [x] Write PROJECT_MEMORY.md (stack, conventions, decisions)
- [ ] Write TASKS.md (this file)
- [ ] Write ADRs for critical decisions
- [ ] Create AGENTS.md at project root
- [ ] Scaffold Docker Compose
- [ ] Scaffold Go backend skeleton
- [ ] Scaffold Next.js frontend skeleton
- [ ] Verify `docker compose up` boots all services
- [ ] Verify backend `/healthz` and `/readyz`
- [ ] Verify frontend dev server boots

## Backlog

### Phase 1 — Infra + DB Foundation
- [ ] Docker Compose: frontend, backend, PostgreSQL, PgBouncer, Valkey, NATS, optional ClickHouse
- [ ] Backend boots with health endpoints
- [ ] Migration runner (go:embed)
- [ ] Base schema: auth/RBAC/tenant/theme/audit
- [ ] Dev seed system
- [ ] PgBouncer used in app runtime
- [ ] Valkey and NATS reachable

### Phase 2 — Auth/Login/RBAC/Session/Theme
- [ ] POST /api/v1/auth/login
- [ ] POST /api/v1/auth/logout
- [ ] GET /api/v1/auth/me
- [ ] httpOnly secure cookie sessions
- [ ] CSRF protection
- [ ] Login rate limiting
- [ ] Password hashing (Argon2id)
- [ ] RBAC roles + permissions seedable
- [ ] Master admin global context + act-as audit
- [ ] Tenant theme (preset + primary + accent + logo)
- [ ] Theme cached in Valkey
- [ ] Dark/light mode (local preference)

### Phase 3 — Frontend Shell + Base Components
- [ ] App shell (sidebar + header + AI sidecar placeholder)
- [ ] Morfosis Design System tokens
- [ ] Base component library
- [ ] Login page
- [ ] Dashboard page
- [ ] Role-aware navigation
- [ ] Dark/light + tenant palette integration

### Phase 4 — Backend Architecture Patterns
- [ ] Middleware stack (requestID, logging, recovery, CORS, security headers, auth, tenant, RBAC, rate limit, CSRF, audit)
- [ ] Standard error envelope with structured validation
- [ ] Pagination/filter/sort pattern
- [ ] Test helpers (auth, tenant, RBAC, DB, handlers)
- [ ] OpenAPI documentation convention
- [ ] AI Tool Manifest convention

### Phase 5 — Domain Schema
- [ ] User profiles (teachers, students, staff, guardians)
- [ ] Academic structure (years, terms, class_sections, subjects, subject_groups)
- [ ] Programs (programs, sections, items, assignments, enrollments, progress, attempts)
- [ ] Courses (courses, modules, lessons, resources)
- [ ] Exams (exams, sections, questions, options, targets, gates, attempts, scores)

### Phase 6 — User & School Admin Modules
- [ ] Users CRUD
- [ ] Tenants/Schools management
- [ ] Teachers directory
- [ ] Students directory
- [ ] Staff directory
- [ ] Guardians directory
- [ ] Student-Guardian linking

### Phase 7 — Academic Structure Modules
- [ ] Academic Years + Semesters
- [ ] Class Sections
- [ ] Subjects
- [ ] Subject Groups
- [ ] Teaching Assignments
- [ ] Enrollments

### Phase 8 — Programs + Courses
- [ ] Program CRUD (create, publish, archive)
- [ ] Program Sections + Items management
- [ ] Program Assignment (to class/student)
- [ ] Auto-enrollment reconciler
- [ ] Course CRUD
- [ ] Course Modules + Lessons + Resources
- [ ] Student program view + progress tracking
- [ ] Section unlock logic
- [ ] Item completion evaluation

### Phase 9 — Exam Management
- [ ] Exam CRUD
- [ ] Exam Sections + Questions (MC, essay, short-answer)
- [ ] Answer key / expected answer / rubric
- [ ] Exam versioning
- [ ] Targets + Gate Windows
- [ ] Prerequisites
- [ ] Publish flow + materialized eligibility

### Phase 10 — Exam Critical Path
- [ ] Exam Gate
- [ ] Take Exam
- [ ] Autosave (cheap, resilient)
- [ ] Submit (append-only, durable, receipt)
- [ ] Integrity events
- [ ] Attempt locking
- [ ] NATS JetStream shock absorber

### Phase 11 — Teacher Operations
- [ ] Exam Monitor
- [ ] Manual Grading
- [ ] AI-assisted Grading (uses correct answer/rubric)
- [ ] Performance views
- [ ] Reports/Export

### Phase 12 — AI Agent Runtime
- [ ] Provider config (BYO + platform default)
- [ ] Conversation persistence
- [ ] Context builder
- [ ] Tool invocations (deterministic, audited)
- [ ] Question generation (jobs/drafts/batches)
- [ ] Memory (tenant-scoped, redacted)

## Completed
- [x] Phase 0: .ai/ memory files (CONTEXT.md, PROJECT_MEMORY.md, TASKS.md)
