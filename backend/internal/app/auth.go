package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

// AuthContext holds the authenticated user's context.
type AuthContext struct {
	UserID            string
	Email             string
	DisplayName       string
	IsPlatformAdmin   bool
	SessionID         string
	EffectiveTenantID *string
	Roles             []string
	Permissions       []string
}

const ctxKeyAuth contextKey = "auth"

func AuthFromContext(ctx context.Context) *AuthContext {
	if v, ok := ctx.Value(ctxKeyAuth).(*AuthContext); ok {
		return v
	}
	return nil
}

// --- Auth Routes ---

func (a *App) registerAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login", a.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", a.handleLogout)
	mux.HandleFunc("GET /api/v1/auth/me", a.handleMe)
}

// --- Login ---

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeErrorJSON(w, http.StatusServiceUnavailable, "service_unavailable", "Database not available", r)
		return
	}

	// Rate limiting. M-3: limit by both IP and lower(email) so a
	// single-account brute-force attack is bounded even if the attacker
	// rotates source IPs. The in-memory limiter is per-replica; scaling
	// past a single API instance requires moving this to Valkey —
	// tracked as TODO in security-audit-2026-05-20.md.
	ip := clientIP(r)
	if !a.loginLimiter.allow("ip:" + ip) {
		writeErrorJSON(w, http.StatusTooManyRequests, "rate_limited", "Too many login attempts. Try again later.", r)
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	// Validate
	fields := map[string]string{}
	if req.Email == "" {
		fields["email"] = "Email is required"
	}
	if req.Password == "" {
		fields["password"] = "Password is required"
	}
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(req.Email))
	// Per-account budget. Stricter than the IP cap so a single account
	// can't be brute-forced even from a botnet of fresh IPs.
	if !a.loginLimiter.allow("email:" + normalizedEmail) {
		writeErrorJSON(w, http.StatusTooManyRequests, "rate_limited", "Too many login attempts. Try again later.", r)
		return
	}

	// Find user
	var userID, displayName, passwordHash string
	var isPlatformAdmin bool
	err := a.db.QueryRowContext(r.Context(),
		`SELECT u.id, u.display_name, u.is_platform_admin, pc.password_hash
		 FROM users u
		 JOIN password_credentials pc ON pc.user_id = u.id
		 WHERE u.email = $1 AND u.status = 'active'`,
		normalizedEmail,
	).Scan(&userID, &displayName, &isPlatformAdmin, &passwordHash)
	if err == sql.ErrNoRows {
		writeErrorJSON(w, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password", r)
		return
	}
	if err != nil {
		a.logger.Error("login query failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "login_failed", "Login failed", r)
		return
	}

	// Verify password
	if !verifyPassword(req.Password, passwordHash) {
		writeErrorJSON(w, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password", r)
		return
	}

	// Resolve default tenant (first primary membership)
	var effectiveTenantID *string
	if !isPlatformAdmin {
		var tid string
		err := a.db.QueryRowContext(r.Context(),
			`SELECT tenant_id FROM tenant_memberships
			 WHERE user_id = $1 AND status = 'active' AND is_primary = true
			 LIMIT 1`,
			userID,
		).Scan(&tid)
		if err == nil {
			effectiveTenantID = &tid
		}
	}

	// Create session
	token := generateSessionToken()
	tokenHash := hashSessionToken(token)
	expiresAt := time.Now().Add(24 * time.Hour)

	_, err = a.db.ExecContext(r.Context(),
		`INSERT INTO sessions (user_id, token_hash, effective_tenant_id, ip_address, user_agent, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, tokenHash, effectiveTenantID, ip, r.UserAgent(), expiresAt,
	)
	if err != nil {
		a.logger.Error("create session failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "login_failed", "Login failed", r)
		return
	}

	// Generate CSRF token
	csrfToken := generateCSRFToken()

	// Set cookies
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cfg.AppEnv != "development",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    csrfToken,
		Path:     "/",
		HttpOnly: false, // JS needs to read this
		Secure:   a.cfg.AppEnv != "development",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	// Audit
	a.audit(r.Context(), effectiveTenantID, userID, "auth.login", "session", "", r)

	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":              userID,
			"email":           req.Email,
			"displayName":     displayName,
			"isPlatformAdmin": isPlatformAdmin,
		},
		"csrfToken": csrfToken,
	})
}

// --- Logout ---

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err != nil || cookie.Value == "" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
		return
	}

	tokenHash := hashSessionToken(cookie.Value)
	if a.db != nil {
		_, _ = a.db.ExecContext(r.Context(),
			`DELETE FROM sessions WHERE token_hash = $1`, tokenHash,
		)
	}

	// Clear cookies
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// --- Me ---

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthenticated", "Not authenticated", r)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":              auth.UserID,
			"email":           auth.Email,
			"displayName":     auth.DisplayName,
			"isPlatformAdmin": auth.IsPlatformAdmin,
		},
		"effectiveTenantId": auth.EffectiveTenantID,
		"roles":             auth.Roles,
		"permissions":       auth.Permissions,
	})
}

// --- Auth Middleware ---

func (a *App) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for public endpoints
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value == "" {
			writeErrorJSON(w, http.StatusUnauthorized, "unauthenticated", "Not authenticated", r)
			return
		}

		tokenHash := hashSessionToken(cookie.Value)

		var auth AuthContext
		var effectiveTenantID sql.NullString
		// M-6: idle timeout. A session is valid only when it has not
		// passed its absolute expires_at AND has been touched within the
		// idle window. The window is generous (30 min) so casual UX is
		// unaffected; a stolen token without active use is auto-expired.
		const idleWindow = 30 * time.Minute
		err = a.db.QueryRowContext(r.Context(),
			`SELECT s.id, s.user_id, s.effective_tenant_id, u.email, u.display_name, u.is_platform_admin
			 FROM sessions s
			 JOIN users u ON u.id = s.user_id
			 WHERE s.token_hash = $1
			   AND s.expires_at > now()
			   AND s.last_activity_at > now() - $2::interval
			   AND u.status = 'active'`,
			tokenHash, fmt.Sprintf("%d seconds", int(idleWindow.Seconds())),
		).Scan(&auth.SessionID, &auth.UserID, &effectiveTenantID, &auth.Email, &auth.DisplayName, &auth.IsPlatformAdmin)
		if err == sql.ErrNoRows {
			writeErrorJSON(w, http.StatusUnauthorized, "session_expired", "Session expired or invalid", r)
			return
		}
		if err != nil {
			a.logger.Error("session lookup failed", "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "internal_error", "Internal error", r)
			return
		}

		if effectiveTenantID.Valid {
			auth.EffectiveTenantID = &effectiveTenantID.String
		}

		// Bump idle timer (best-effort, non-fatal). Done before role load
		// so a slow role query still extends the session liveness window.
		_, _ = a.db.ExecContext(r.Context(),
			`UPDATE sessions SET last_activity_at = now() WHERE id = $1`,
			auth.SessionID,
		)

		// Load roles and permissions. M-7: a load error here used to
		// silently leave Permissions empty, which fails closed for
		// permission-gated handlers but fails OPEN for any handler that
		// gates on isPlatformAdmin (set above from users.is_platform_admin
		// directly). Refuse the request instead.
		auth.Roles, auth.Permissions, err = a.loadRolesAndPermissions(r.Context(), auth.UserID, auth.EffectiveTenantID)
		if err != nil {
			a.logger.Error("load roles failed", "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "internal_error", "Could not load authorization context", r)
			return
		}

		ctx := context.WithValue(r.Context(), ctxKeyAuth, &auth)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) loadRolesAndPermissions(ctx context.Context, userID string, tenantID *string) ([]string, []string, error) {
	var roles []string
	var perms []string

	// Load roles
	query := `SELECT DISTINCT r.slug FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE ur.user_id = $1`
	args := []any{userID}
	if tenantID != nil {
		query += ` AND (ur.tenant_id = $2 OR r.tenant_id IS NULL)`
		args = append(args, *tenantID)
	}

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, nil, err
		}
		roles = append(roles, slug)
	}

	// Load permissions from roles
	permQuery := `SELECT DISTINCT p.slug
		FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id = ur.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE ur.user_id = $1`
	permArgs := []any{userID}
	if tenantID != nil {
		permQuery += ` AND (ur.tenant_id = $2 OR (SELECT tenant_id FROM roles WHERE id = ur.role_id) IS NULL)`
		permArgs = append(permArgs, *tenantID)
	}

	rows2, err := a.db.QueryContext(ctx, permQuery, permArgs...)
	if err != nil {
		return roles, nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var slug string
		if err := rows2.Scan(&slug); err != nil {
			return roles, nil, err
		}
		perms = append(perms, slug)
	}

	return roles, perms, nil
}

// --- CSRF Middleware ---

func (a *App) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only check unsafe methods
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Skip for public paths
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Login doesn't need CSRF (no session yet)
		if r.URL.Path == "/api/v1/auth/login" {
			next.ServeHTTP(w, r)
			return
		}

		csrfCookie, err := r.Cookie("csrf_token")
		if err != nil || csrfCookie.Value == "" {
			writeErrorJSON(w, http.StatusForbidden, "csrf_missing", "CSRF token missing", r)
			return
		}

		csrfHeader := r.Header.Get("X-CSRF-Token")
		if csrfHeader == "" {
			writeErrorJSON(w, http.StatusForbidden, "csrf_missing", "CSRF token header missing", r)
			return
		}

		if subtle.ConstantTimeCompare([]byte(csrfCookie.Value), []byte(csrfHeader)) != 1 {
			writeErrorJSON(w, http.StatusForbidden, "csrf_invalid", "CSRF token mismatch", r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// --- Helpers ---

func isPublicPath(path string) bool {
	publicPaths := []string{"/healthz", "/readyz", "/api/v1/auth/login"}
	for _, p := range publicPaths {
		if path == p {
			return true
		}
	}
	return strings.HasPrefix(path, "/assets/")
}

func generateSessionToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func hashSessionToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func generateCSRFToken() string {
	// L-3: bump from 128 to 256 bits to match the session token. Cheap
	// upgrade; subtle.ConstantTimeCompare cost is constant in length.
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func verifyPassword(password, hash string) bool {
	// Parse argon2id hash: $argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}

	saltHex := parts[4]
	expectedHashHex := parts[5]

	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false
	}
	expectedHash, err := hex.DecodeString(expectedHashHex)
	if err != nil {
		return false
	}

	computed := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return subtle.ConstantTimeCompare(computed, expectedHash) == 1
}

// PasswordMinLength is the enforced floor for new passwords across
// every create/update path. Raised from 6 to 12 per H-4 of the
// 2026-05-20 security audit (NIST SP 800-63B baseline + offline
// brute-force margin).
const PasswordMinLength = 12

// validatePasswordPolicy returns a fields-style error map when the
// candidate password fails the minimum policy. Empty map means
// acceptable. Callers feed this into writeValidationError.
func validatePasswordPolicy(field, password string) map[string]string {
	out := map[string]string{}
	if password == "" {
		out[field] = "Password is required"
		return out
	}
	if len(password) < PasswordMinLength {
		out[field] = fmt.Sprintf("Password must be at least %d characters", PasswordMinLength)
	}
	return out
}

func hashPassword(password string) string {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%s$%s",
		hex.EncodeToString(salt),
		hex.EncodeToString(hash),
	)
}

func clientIP(r *http.Request) string {
	// M-3: X-Forwarded-For is only trusted when the request comes from
	// a known reverse proxy. Without that gate, any attacker can spoof
	// the header to evade IP-based rate limiting. We trust the header
	// only when TRUSTED_PROXIES env lists the immediate peer.
	if isTrustedProxy(r.RemoteAddr) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[0])
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return xri
		}
	}
	parts := strings.Split(r.RemoteAddr, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return r.RemoteAddr
}

// isTrustedProxy returns true when the immediate request peer is in
// the comma-separated TRUSTED_PROXIES env var. Loopback (127.0.0.1)
// is always trusted to keep dev workflows working through localhost.
func isTrustedProxy(remoteAddr string) bool {
	host := remoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	host = strings.Trim(host, "[]")
	if host == "127.0.0.1" || host == "::1" || host == "localhost" {
		return true
	}
	list := os.Getenv("TRUSTED_PROXIES")
	if list == "" {
		return false
	}
	for _, p := range strings.Split(list, ",") {
		if strings.TrimSpace(p) == host {
			return true
		}
	}
	return false
}

// --- Rate Limiter ---

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{attempts: make(map[string][]time.Time)}
}

func (l *loginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	window := 5 * time.Minute
	maxAttempts := 10

	// Clean old entries
	var recent []time.Time
	for _, t := range l.attempts[ip] {
		if now.Sub(t) < window {
			recent = append(recent, t)
		}
	}

	if len(recent) >= maxAttempts {
		l.attempts[ip] = recent
		return false
	}

	l.attempts[ip] = append(recent, now)
	return true
}

// --- Audit Helper ---

func (a *App) audit(ctx context.Context, tenantID *string, actorID, action, resourceType, resourceID string, r *http.Request) {
	if a.db == nil {
		return
	}
	_, _ = a.db.ExecContext(ctx,
		`INSERT INTO audit_events (tenant_id, actor_id, actor_type, action, resource_type, resource_id, ip_address, user_agent, request_id)
		 VALUES ($1, $2, 'user', $3, $4, $5, $6, $7, $8)`,
		tenantID, actorID, action, resourceType, resourceID, clientIP(r), r.UserAgent(), RequestID(ctx),
	)
}

// --- JSON Helpers ---

func writeErrorJSON(w http.ResponseWriter, status int, code, message string, r *http.Request) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":      code,
			"message":   message,
			"requestId": RequestID(r.Context()),
		},
	})
}

func writeValidationError(w http.ResponseWriter, fields map[string]string, r *http.Request) {
	writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
		"error": map[string]any{
			"code":      "validation_failed",
			"message":   "Validation failed",
			"fields":    fields,
			"requestId": RequestID(r.Context()),
		},
	})
}

func readJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return fmt.Errorf("empty body")
	}
	// M-5: cap inbound payloads to keep a single large POST from
	// monopolising memory and parser CPU. 1 MiB covers every legitimate
	// path (longest is the rich-editor question content with embedded
	// images base64-encoded; bumped via the per-handler override pattern
	// when we add image upload). The middleware-level cap defends every
	// JSON write endpoint without per-handler ceremony.
	defer r.Body.Close()
	limited := http.MaxBytesReader(nil, r.Body, 1<<20) // 1 MiB
	dec := jsonDecoder(limited)
	return dec.Decode(v)
}
