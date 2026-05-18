package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Config holds application configuration.
type Config struct {
	Port    string
	AppEnv  string
	DBUrl   string
	Valkey  string
	NatsUrl string
}

// App is the main application container.
type App struct {
	cfg          Config
	logger       *slog.Logger
	db           *sql.DB
	loginLimiter *loginLimiter
}

// New creates a new App instance.
func New(cfg Config, logger *slog.Logger, db *sql.DB) (*App, error) {
	a := &App{
		cfg:          cfg,
		logger:       logger,
		db:           db,
		loginLimiter: newLoginLimiter(),
	}
	return a, nil
}

// Close releases resources.
func (a *App) Close() {}

// Handler returns the root HTTP handler with all routes and middleware.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()

	// Health endpoints (public)
	mux.HandleFunc("GET /healthz", a.handleHealthz)
	mux.HandleFunc("GET /readyz", a.handleReadyz)

	// Auth routes
	a.registerAuthRoutes(mux)

	// Tenant switch
	mux.HandleFunc("POST /api/v1/tenants/switch", a.SwitchTenant)

	// Tenants
	a.registerTenantRoutes(mux)

	// Users
	a.registerUserRoutes(mux)

	// Profiles
	a.registerTeacherRoutes(mux)
	a.registerStudentRoutes(mux)
	a.registerStaffRoutes(mux)
	a.registerGuardianRoutes(mux)

	return a.applyMiddleware(mux)
}

// --- Health Endpoints ---

func (a *App) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"service":   "morfoschools-api",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *App) handleReadyz(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{}
	ready := true

	// Database check
	if a.db != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := a.db.PingContext(ctx); err != nil {
			checks["database"] = "unavailable"
			ready = false
		} else {
			checks["database"] = "ready"
		}
	} else {
		checks["database"] = "not_configured"
	}

	status := "ready"
	code := http.StatusOK
	if !ready {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	payload := map[string]any{
		"status":    status,
		"service":   "morfoschools-api",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range checks {
		payload[k] = v
	}
	writeJSON(w, code, payload)
}

// --- Middleware ---

func (a *App) applyMiddleware(next http.Handler) http.Handler {
	// Order: requestID → recovery → securityHeaders → cors → csrf → auth → handler
	return requestIDMiddleware(
		recoveryMiddleware(a.logger)(
			securityHeadersMiddleware(
				corsMiddleware(
					a.csrfMiddleware(
						a.authMiddleware(next),
					),
				),
			),
		),
	)
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = generateRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func recoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					if logger != nil {
						logger.Error("panic recovered", "panic", rec, "requestId", RequestID(r.Context()))
					}
					writeJSON(w, http.StatusInternalServerError, map[string]any{
						"error": map[string]any{
							"code":      "internal_error",
							"message":   "Internal server error",
							"requestId": RequestID(r.Context()),
						},
					})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	allowed := map[string]bool{
		"http://localhost:1666":  true,
		"http://127.0.0.1:1666": true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID, X-Tenant-ID, X-CSRF-Token")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- Helpers ---

type contextKey string

const ctxKeyRequestID contextKey = "requestID"

func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func jsonDecoder(r io.Reader) *json.Decoder {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	return dec
}

func generateRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "req-" + time.Now().Format("20060102150405")
	}
	return "req-" + hex.EncodeToString(b[:])
}
