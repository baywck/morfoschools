# Collaboration & Security Verification — 2026-05-20

Live verification of the collaboration model and post-audit security gates after PR #2 (`fix/security-audit-2026-05-20`) was applied. All tests run against a fresh DB on this branch with the new migrations applied.

## Stack

- Branch: `fix/security-audit-2026-05-20` (9 fix commits + audit doc)
- Backend: 200 at `:8080`
- Migrations 1–18 applied clean
- Test users from devseed (admin / academic / teacher / student) plus `teacher2` created on the fly via `POST /api/v1/teachers/create-full`

## Results

### Collaboration flow (exams)

| # | Scenario | Expected | Actual |
|---|---|---|---|
| T1 | Owner reads own exam | 200 | ✅ 200 |
| T2 | Owner updates exam | 200 | ✅ 200 |
| T3 | Tenant admin (academic) reads any exam | 200 | ✅ 200 |
| T4 | Academic admin lacks `exams:write` → blocked | 403 forbidden | ✅ 403 |
| T5 | Student GET on non-owned exam → no leak | 404 | ✅ 404 |
| T6 | Newly created teacher2 GET pre-invite | 404 | ✅ 404 |
| T7 | Owner invites teacher2 as `editor` | 201 | ✅ 201 |
| T8 | Editor PATCH metadata | 200 | ✅ 200 |
| T9 | Editor invites another collab | 403 manage-only | ✅ 403 |
| T10 | Owner downgrades teacher2 → `viewer` | 200 | ✅ 200 |
| T11 | Viewer PATCH attempt | 403 write | ✅ 403 |
| T12 | Viewer GET still works | 200 | ✅ 200 |
| T13 | Viewer creates question | 403 write | ✅ 403 |
| T14 | Editor creates question | 200 | ✅ 200 |
| T15 | Owner transfers ownership | 200 transferred | ✅ 200 |
| T16 | Old owner attempts transfer (now editor) | 403 manage | ✅ 403 |

### Collaboration flow (blueprint templates)

| # | Scenario | Expected | Actual |
|---|---|---|---|
| T17 | Teacher creates blueprint template | 201 | ✅ 201 |
| T18 | Non-collab on different teacher's blueprint | 404 | ✅ 404 |
| T19 | Owner invites viewer | 201 | ✅ 201 |
| T20 | Viewer GET ok | 200 | ✅ 200 |
| T21 | Viewer creates slot | 403 write | ✅ 403 |

### Security gates (post-audit fixes)

| # | Fix | Scenario | Expected | Actual |
|---|---|---|---|---|
| T22 | H-1 | Confirm with bogus proposalId | 404 | ✅ 404 |
| T23 | CSRF | Confirm without CSRF header | 403 csrf_missing | ✅ 403 |
| T24 | CSRF | CSRF mismatch | 403 csrf_invalid | ✅ 403 |
| T25 | M-5 | 1.5 MB body on `POST /exams` | 400 invalid_request | ✅ 400 |
| T26 | L-1 | CSP header present | `default-src 'self'…` | ✅ matches policy |
| T27 | H-2 | Unknown origin → no CORS reflection | empty | ✅ empty |
| T28 | H-2 | Whitelisted origin reflected | `localhost:1666` | ✅ matches |

### 404 vs 403 split (no info leakage)

The collab helper distinguishes the two cases per ADR-0009:
- **No role at all** (cross-tenant or pure outsider) → 404. The resource may exist somewhere else; the response must not let the caller discover it.
- **Has read but lacks write/manage** → 403 `forbidden`. The caller knows it exists, surface the access shortfall.

T5 (student → 404), T11 (viewer write → 403), T18 (non-collab template → 404) all confirm the split is implemented correctly across both resources.

### H-1 (AI confirm permission re-check)

The new `App.authorizeConfirmedAction()` runs before every executor. T22-T24 prove the confirm path is end-to-end gated by:
1. Auth context present
2. CSRF token valid (T23, T24)
3. Proposal exists + owned by user (T22)
4. Permission re-check at exec time (per code review of `ai_chat.go:914`)
5. Tenant equality check (per code review of `ai_chat.go:914`)

Live exercise of items 4-5 requires a role-revoke roundtrip which doesn't have a fast endpoint; covered by static review + test fixture in `ai_chat.go`. Build clean (`go test ./... → ok`).

## Findings during this round

None. All collab and security gates respond as documented in the audit report.

## Recommendations

- **Add integration tests** for T1-T16 to catch regressions. The flow above is currently manual; codifying as `*_test.go` suites prevents future PRs from breaking the 404/403 split silently.
- **L-4 SameSite Strict** still deferred — verify SPA redirect flow first.
- **M-3 follow-up** — Valkey-backed limiter when we add a second API replica.
