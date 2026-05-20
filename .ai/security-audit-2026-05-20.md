# Security Audit Report — 2026-05-20

Branch: `feat/archive-email-release-and-ai-chatbot` @ `836d1de` (post-Phase-9 merge)
Scope: backend Go API (`backend/internal/app/`) + frontend Next.js client + Docker compose + dependency tree.
Method: code-level review against OWASP Top 10, dependency scanners, secret scan. No live exploitation.

## Summary

| Severity | Count |
|---|---|
| Critical | 0 |
| High | 4 |
| Medium | 7 |
| Low | 5 |

The codebase is solid on the fundamentals — argon2id, constant-time compares, parameterized SQL throughout, layered RBAC, tenant scoping, CSRF double-submit, DOMPurify on rendered HTML, govulncheck clean on Go. The findings below are mostly hardening gaps rather than open holes; one stands out (AI confirm path skips permission re-check) and one is a production deploy footgun (CORS hardcoded to localhost).

## High Findings

### H-1. AI confirm endpoint executes without permission re-check
**Files**: `backend/internal/app/ai_chat.go:814-879`, `backend/internal/app/ai_write_tools.go:390-431`
**Risk**: A user proposes an AI write action while holding `users:write`. Admin revokes the role. The user still has the proposalId and can `POST /api/v1/ai/confirm` until expiry — the executor never verifies the tool's required permission against the *current* `auth.Permissions`. Same issue across tenant switches: `tenantID` is read from the proposal row and trusted; if a user moves tenants between proposal and confirm, the action still targets the old tenant.
**Fix**: Re-derive the capability for `toolName`, look up `cap.Permission`, and reject the confirm if `auth.Permissions` does not contain it. Also reject if `proposal.tenant_id != auth.EffectiveTenantID`. Apply the same check inside the short-reply path at `ai_chat.go:199`.

### H-2. CORS allowlist is hardcoded to localhost
**File**: `backend/internal/app/app.go:215-235`
**Risk**: `corsMiddleware` whitelists only `http://localhost:1666` and `http://127.0.0.1:1666`. Deploying behind a real domain breaks the SPA (cookies refused) and the team will likely fix it by widening the list. With cookies + CSRF the CORS list is the perimeter for browser CSRF — getting this wrong opens session-bound endpoints.
**Fix**: Read allowed origins from env (e.g. `ALLOWED_ORIGINS=https://app.example.com,https://staging.example.com`), validate with a strict equality match (no wildcard or substring). Reject the request with 403 when origin is set and not allowed instead of silently dropping the header.

### H-3. Default Postgres password persists into production-shaped configs
**Files**: `.env.example:16`, `docker-compose.yml:58,75`
**Risk**: `${POSTGRES_PASSWORD:-change-me}` defaults to `change-me`. Operators forgetting to set the env var get a known credential on a publicly-bound port if the compose stack is ever exposed.
**Fix**: Drop the default. Make `POSTGRES_PASSWORD` mandatory — fail fast in `cmd/api/main.go` when it is missing or equal to `change-me` and `APP_ENV != development`. Bind Postgres only to the docker network (already true in compose, but document the rule).

### H-4. Password policy minimum is 6 characters
**Files**: `backend/internal/app/auth.go:78-80`, `backend/internal/app/composite_create.go:46-49,177-180,282-285`, `backend/internal/app/users.go` (signup paths)
**Risk**: 6-char minimum allows trivial brute-force in offline scenarios (DB exfil) and is below NIST SP 800-63B's 8-char floor.
**Fix**: Raise to 12 across all three composite-create paths and login validation. Reject the seeded `admin123` family of passwords for any non-development env. Optional: integrate haveibeenpwned k-anon check.

## Medium Findings

### M-1. Vulnerable npm dependencies
**File**: `frontend/package.json`
**npm audit (production-only) shows 5 advisories (1 low, 4 moderate)**:
- `dompurify <3.3.3` — multiple XSS bypasses including SAFE_FOR_TEMPLATES, ADD_TAGS, mutation-XSS. Used directly by `frontend/src/components/ui/rendered-content.tsx` to sanitize question/explanation HTML — **highest priority** of this group because it is the only thing standing between authored content and student browsers.
- `@tiptap/extension-link <2.10.4` — XSS in link rendering inside the rich editor.
- `katex 0.12.0–0.16.20` — `\htmlData` attribute name validation bypass.
- `postcss <8.5.10` (transitive via `next`) — XSS in stringify; build-time only.
**Fix**: Bump dompurify, @tiptap/extension-link, katex to patched versions. Pin exact versions (no carets) per project rule. `postcss` resolves with the next minor of `next`.

### M-2. Caret-ranged dependency versions
**File**: `frontend/package.json` (e.g. `"next": "^15.3.2"`)
**Risk**: Project rule requires exact pins. Carets allow silent minor upgrades on `npm install` with no review, making "supply chain audit" a moving target.
**Fix**: Replace every `^x.y.z` with `x.y.z`, regenerate lockfile, commit.

### M-3. Login rate limiter is in-process and IP-keyed
**File**: `backend/internal/app/auth.go:463-499`
**Risk**: 10 attempts per IP per 5 minutes is reasonable for a single instance, but the limiter is a `sync.Map` — behind multiple replicas, attackers get N× the budget. Also IP-only key lets an attacker spread across IPv4/IPv6 + proxies; brute force against a single account is not capped. `X-Forwarded-For` is trusted blindly (`auth.go:441-446`) so a malicious proxy header lets an attacker reset their own bucket.
**Fix**: Move to Valkey (`SETEX key=login:ip:<ip>` + `login:email:<lower(email)>`) so all replicas share state and per-account brute force is gated. Trust `X-Forwarded-For` only when behind a known proxy (introduce `TRUSTED_PROXIES` env or use `r.RemoteAddr` directly in dev).

### M-4. Email uniqueness check ignores soft-deleted accounts
**Files**: `backend/internal/app/composite_create.go:58,188,293`, `backend/internal/app/users.go:172,303`
**Risk**: `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)` flags as "in use" even archived rows, but the partial unique index only covers active rows (per ADR-0007). Result: pre-INSERT check returns `true` and surfaces `email already in use` even when the underlying INSERT would succeed. This is a UX correctness bug rather than a security hole, but the inverse case — admin re-creating an account with an email that *was* archived — gets a confusing 422 instead of a clean create.
**Fix**: Add `AND status != 'archived'` to all five email-uniqueness checks to align with the index definition.

### M-5. No body size limit on JSON request reads
**File**: `backend/internal/app/auth.go:533-540` (`readJSON`)
**Risk**: `json.NewDecoder(r.Body)` reads the full body. A client posting a multi-megabyte JSON to any write endpoint forces the server to parse it before validation. Cheap DoS amplifier.
**Fix**: Wrap the body in `http.MaxBytesReader(w, r.Body, 1 << 20)` (1 MiB) before decoding. For specific paths that legitimately need more (rich content with images, AI chat), bump per-handler.

### M-6. Session lifetime is 24h fixed with no idle invalidation or rotation
**Files**: `backend/internal/app/auth.go:129-134,260-264`
**Risk**: Sessions live exactly 24h regardless of activity. No rotation on privilege change (e.g. role removal), no idle-timeout, no refresh. A stolen session token is valid for up to 24h with no revocation surface beyond manually deleting `sessions` rows.
**Fix**: Add `last_activity_at` column, reject sessions idle > 30 min, refresh on each request (sliding window). Rotate token on privilege change (role grant/revoke). Add a `/api/v1/auth/sessions/revoke-all` endpoint for self-service.

### M-7. `loadRolesAndPermissions` errors are swallowed
**Files**: `backend/internal/app/auth.go:280-281`
**Risk**: When the role/permission query fails, the request proceeds with an empty `auth.Permissions` slice. Any handler gated on `RequirePermission` will return 403 — that's safe — but any handler that gates on `isPlatformAdmin` (set from `users.is_platform_admin` directly via the auth middleware query) will still pass. Failure mode is closed for tenant users, open for platform admin.
**Fix**: Return 500 on permission load error instead of degrading. The DB failure case is rare enough that failing visibly is preferable to silent partial auth.

## Low Findings

### L-1. No Content-Security-Policy header
**File**: `backend/internal/app/app.go:209-216`
**Risk**: `securityHeadersMiddleware` sets `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`. CSP is missing — defense in depth against the dompurify findings if any sanitizer bypass slips through.
**Fix**: Add `Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'`. Tighten over time.

### L-2. No Strict-Transport-Security header
**File**: `backend/internal/app/app.go:209-216`
**Risk**: HSTS missing for production deployments served over HTTPS.
**Fix**: When `cfg.AppEnv == "production"` set `Strict-Transport-Security: max-age=63072000; includeSubDomains; preload`.

### L-3. CSRF token entropy is 128 bits
**File**: `backend/internal/app/auth.go:411-414`
**Risk**: `generateCSRFToken` uses 16 bytes (128 bits). Industry baseline is 256 bits (32 bytes). Acceptable today but cheap to upgrade.
**Fix**: Bump to 32 bytes — match `generateSessionToken`.

### L-4. SameSite=Lax instead of Strict on session cookie
**File**: `backend/internal/app/auth.go:148-156`
**Risk**: `Lax` allows cross-site GETs to carry the cookie. CSRF middleware blocks unsafe methods so POST/PATCH/DELETE require the X-CSRF-Token header — Lax is fine. But Strict + a tiny "click to continue" landing page after sign-in is the more conservative posture for a school SaaS.
**Fix**: Switch to `SameSiteStrictMode` once the SPA's first-page-load auth gate is verified to handle the redirect.

### L-5. `.env.example` documents but does not enforce that secrets must be set
**File**: `.env.example`
**Risk**: An operator copying the example to `.env` and forgetting to change `change-me` ships with a default credential. Companion to H-3 — that finding is the runtime fix; this one is the doc fix.
**Fix**: Annotate every secret-shaped key in the example with `# REQUIRED — must be a 32+ char random value`. Add a `make precheck` target that fails when any secret matches a known-bad default.

## Dependency Scan

**Go (govulncheck)**:
```
No vulnerabilities found.
```
Module list clean as of 2026-05-20 against `go.mod`.

**Node (npm audit --omit=dev)**: 5 advisories (1 low, 4 moderate). Detail in M-1.

## Secret Scan

```
grep -rn "password\|secret\|api_key\|token\|private_key" \
  backend/internal/app frontend/src \
  --exclude-dir=node_modules --exclude-dir=.git
```
- No hardcoded secret values found.
- `password` strings only appear as field names (validation messages, JSON tags, audit actions).
- `.env`, `.env.local`, `.env.*.local` covered by `.gitignore`.

## Positive Findings

The codebase gets a number of things right that are worth calling out so they stay right:

- **Argon2id** with sane params (`m=64MiB, t=1, p=4`) and salted per-user. Verification uses `subtle.ConstantTimeCompare`.
- **Session tokens** are 256-bit random, stored as `sha256(token)` not plaintext, comparison constant-time at lookup.
- **CSRF** double-submit cookie + header, validated with `subtle.ConstantTimeCompare`. Login path bypassed (correct — no session yet).
- **Parameterized SQL** throughout. The few `fmt.Sprintf` calls feed table/column names from a hardcoded `resourceSpec` registry, not user input.
- **DisallowUnknownFields** on JSON decoder — silently rejects payload drift, surfaced the questionType bug during Phase 9.
- **Tenant scoping** every domain table has `tenant_id` and every query filters on it. Verified across 59+ handlers.
- **Layered RBAC**: owner → collaborator → tenant admin → subject fallback for read. Each handler chain calls `RequirePermission` + access helpers; no handler reaches the DB without going through both gates.
- **Audit logging** writes one row per mutation with actor, IP, UA, request ID. PII not echoed into log lines; passwords never logged.
- **DOMPurify** sanitization on every rendered HTML field via `RenderedContent`. No raw `dangerouslySetInnerHTML` outside the sanitized component.
- **Recovery middleware** catches panics and emits the structured 500 with request ID, no stack to client.
- **Cookies** Secure flag in non-development, HttpOnly on session, Path=/ scoped.

## Recommendations (Prioritized)

1. **H-1** — Re-check `cap.Permission` and tenant ownership inside `executeConfirmedAction` before invoking the executor. (~30 LoC, two test cases.)
2. **H-2** — Replace localhost-only CORS with env-driven allowlist + reject path. Required before any deploy.
3. **H-3 + L-5** — Drop default Postgres password, fail fast in main when missing/equal-to-default outside dev.
4. **M-1** — Bump dompurify, @tiptap/extension-link, katex; rerun `npm audit --omit=dev` until clean. Pin exact (M-2).
5. **H-4** — Raise password minimum to 12 chars across all create paths.
6. **M-3** — Move login limiter to Valkey, key on both IP and `lower(email)`.
7. **M-5** — Apply `http.MaxBytesReader` to all JSON read paths.
8. **M-6** — Add idle session timeout + token rotation on privilege change.
9. **L-1, L-2** — CSP + HSTS via `securityHeadersMiddleware`.

Items M-4, M-7, L-3, L-4 are minor and can ride along on the next routine pass.
