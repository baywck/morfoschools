# Verification Standards — Morfoschools

## Commands

### Backend (Go)
```bash
# From /backend directory
go test ./...                    # All tests
go test ./internal/app -run X    # Targeted test
go vet ./...                     # Static analysis
go build ./...                   # Build check
```

### Frontend (Next.js)
```bash
# From /frontend directory
npx tsc --noEmit                 # Typecheck
npm test -- --run                # All Vitest tests
npm test -- --run src/path.test.ts  # Targeted test
npm run build                    # Production build
npm run lint                     # ESLint
```

### Docker
```bash
# From project root
docker compose up -d             # Start all services
docker compose up -d --build backend   # Rebuild backend
docker compose logs backend -f   # Follow backend logs
```

## Tiered Verification

| Tier | When | Run |
|------|------|-----|
| 0 | CSS, copy, comments | lint only |
| 1 | Feature code, UI components | lint + typecheck + targeted tests |
| 2 | Business logic, APIs | lint + typecheck + full test suite + build |
| 3 | Auth, payment, data deletion | Tier 2 + security review |
| 4 | CI, deploy, DB schema | Tier 2 + dry-run + rollback plan |

## TDD Workflow

1. Write test (RED) — test must fail
2. Implement minimum code (GREEN) — test passes
3. Refactor — clean up, test still passes
4. Verify full suite — no regressions

## Runtime Smoke

After backend changes that affect API:
1. Rebuild: `docker compose up -d --build backend`
2. Check: `curl http://127.0.0.1:8080/readyz`
3. Test endpoint with authenticated session
4. For kisi-kisi execution planner smoke test: run `backend/scripts/action-plan-smoke.sh`

After frontend changes:
1. Check typecheck: `npx tsc --noEmit`
2. Check browser: `http://127.0.0.1:1666`
3. Verify no console errors, correct loading states
