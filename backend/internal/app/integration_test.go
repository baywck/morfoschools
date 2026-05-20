//go:build integration

package app

// Integration suite codifying the live verification documented in
// .ai/collab-security-verification-2026-05-20.md (T1–T28). DB-backed,
// gated behind the `integration` build tag so the default `go test`
// run stays fast and DB-free.
//
// Run with:
//   INTEGRATION_DATABASE_URL=postgres://morfoschools:dev-secret-not-for-prod-use-only@127.0.0.1:6432/morfoschools?sslmode=disable \
//   go test -tags integration ./backend/internal/app/...
//
// The harness creates an httptest server backed by a real Postgres
// database. Migrations + devseed run once per binary in TestMain.
// Each test case is independent (creates its own resources, asserts,
// cleans up via parent transaction rollback or explicit DELETE).

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"morfoschools/backend/internal/platform/devseed"
	"morfoschools/backend/internal/platform/migrate"
	"morfoschools/backend/migrations"
)

// ---- harness ------------------------------------------------------

type harness struct {
	server *httptest.Server
	db     *sql.DB
	logger *slog.Logger
}

var globalHarness *harness

func TestMain(m *testing.M) {
	dbURL := os.Getenv("INTEGRATION_DATABASE_URL")
	if dbURL == "" {
		fmt.Println("INTEGRATION_DATABASE_URL not set; skipping integration suite")
		os.Exit(0)
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ping db: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := migrate.Run(ctx, db, logger, migrations.FS); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
	if err := devseed.Run(ctx, db, logger); err != nil {
		fmt.Fprintf(os.Stderr, "devseed: %v\n", err)
		os.Exit(1)
	}

	a, err := New(Config{
		Port:           "0",
		AppEnv:         "test",
		DBUrl:          dbURL,
		AllowedOrigins: []string{"http://test"},
	}, logger, db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "app new: %v\n", err)
		os.Exit(1)
	}
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	globalHarness = &harness{server: srv, db: db, logger: logger}
	os.Exit(m.Run())
}

// ---- HTTP client helpers -----------------------------------------

type session struct {
	t      *testing.T
	jar    *cookiejar.Jar
	client *http.Client
	csrf   string
	userID string
}

func newSession(t *testing.T, email, password string) *session {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	s := &session{t: t, jar: jar, client: c}

	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req, _ := http.NewRequest(http.MethodPost,
		globalHarness.server.URL+"/api/v1/auth/login",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("login %s: %v", email, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		buf, _ := io.ReadAll(resp.Body)
		t.Fatalf("login %s: %d %s", email, resp.StatusCode, buf)
	}
	var lr struct {
		CSRFToken string `json:"csrfToken"`
		User      struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		t.Fatalf("decode login %s: %v", email, err)
	}
	s.csrf = lr.CSRFToken
	s.userID = lr.User.ID
	return s
}

// do issues a request with cookies + CSRF token already set.
func (s *session) do(method, path string, body any) (*http.Response, []byte) {
	s.t.Helper()
	var rdr io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		rdr = bytes.NewReader(buf)
	}
	u := globalHarness.server.URL + path
	req, _ := http.NewRequest(method, u, rdr)
	if rdr != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet && method != http.MethodHead {
		req.Header.Set("X-CSRF-Token", s.csrf)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		s.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	buf, _ := io.ReadAll(resp.Body)
	return resp, buf
}

func (s *session) status(method, path string, body any) int {
	resp, _ := s.do(method, path, body)
	return resp.StatusCode
}

// dec decodes the JSON body into v (must be pointer); fails the test
// if status mismatches expected.
func (s *session) dec(method, path string, body any, expectStatus int, v any) {
	s.t.Helper()
	resp, buf := s.do(method, path, body)
	if resp.StatusCode != expectStatus {
		s.t.Fatalf("%s %s: got %d expected %d, body=%s",
			method, path, resp.StatusCode, expectStatus, string(buf))
	}
	if v != nil {
		if err := json.Unmarshal(buf, v); err != nil {
			s.t.Fatalf("decode %s %s: %v body=%s", method, path, err, string(buf))
		}
	}
}

// ---- helpers ------------------------------------------------------

// cleanupExam deletes the exam at the end of a test so the suite can
// run multiple times without UNIQUE-violating on collab joins.
func cleanupExam(t *testing.T, examID string) {
	t.Helper()
	if examID == "" {
		return
	}
	_, _ = globalHarness.db.ExecContext(context.Background(),
		`DELETE FROM exams WHERE id = $1`, examID)
}

func cleanupBlueprint(t *testing.T, templateID string) {
	t.Helper()
	if templateID == "" {
		return
	}
	_, _ = globalHarness.db.ExecContext(context.Background(),
		`DELETE FROM blueprint_templates WHERE id = $1`, templateID)
}

func ensureUser(t *testing.T, tenantSlug, email, password, role string) string {
	t.Helper()
	ctx := context.Background()
	var userID, tenantID string
	if err := globalHarness.db.QueryRowContext(ctx,
		`SELECT id FROM tenants WHERE code = $1`, tenantSlug,
	).Scan(&tenantID); err != nil {
		t.Fatalf("lookup tenant %s: %v", tenantSlug, err)
	}
	err := globalHarness.db.QueryRowContext(ctx,
		`SELECT id FROM users WHERE email = $1 AND status != 'archived'`, email,
	).Scan(&userID)
	if err == sql.ErrNoRows {
		// admin login + composite create handles all the side tables;
		// reuse it instead of re-implementing.
		admin := newSession(t, "admin@morfoschools.local", "admin123")
		var resp struct {
			UserID string `json:"userId"`
		}
		admin.dec(http.MethodPost,
			"/api/v1/"+pluralOf(role)+"/create-full",
			map[string]any{
				"displayName": email,
				"email":       email,
				"password":    password,
			},
			http.StatusCreated, &resp)
		return resp.UserID
	}
	if err != nil {
		t.Fatalf("lookup user %s: %v", email, err)
	}
	return userID
}

func pluralOf(role string) string {
	switch role {
	case "teacher":
		return "teachers"
	case "student":
		return "students"
	case "staff":
		return "staff"
	}
	return role + "s"
}

// ---- T1-T16: Exam collaboration flow ------------------------------

func TestCollab_ExamFlow(t *testing.T) {
	teacher := newSession(t, "teacher@morfoschools.local", "teacher123")
	academic := newSession(t, "academic@morfoschools.local", "academic123")
	student := newSession(t, "student@morfoschools.local", "student123")

	// Create teacher2 lazily on first run; reuse on subsequent runs.
	teacher2ID := ensureUser(t, "demo-school", "teacher2@morfoschools.local",
		"teacher2-strong-pw", "teacher")

	// Owner creates the exam
	var created struct {
		ID string `json:"id"`
	}
	teacher.dec(http.MethodPost, "/api/v1/exams",
		map[string]any{
			"title":           "Collab Test " + randSuffix(),
			"examType":        "quiz",
			"durationMinutes": 30,
			"maxScore":        100,
			"passingScore":    70,
		},
		http.StatusCreated, &created)
	examID := created.ID
	t.Cleanup(func() { cleanupExam(t, examID) })

	t.Run("T1_owner_reads", func(t *testing.T) {
		got := teacher.status(http.MethodGet, "/api/v1/exams/"+examID, nil)
		if got != http.StatusOK {
			t.Errorf("owner GET: got %d want 200", got)
		}
	})

	t.Run("T2_owner_updates", func(t *testing.T) {
		got := teacher.status(http.MethodPatch, "/api/v1/exams/"+examID,
			map[string]any{"description": "Owner edit"})
		if got != http.StatusOK {
			t.Errorf("owner PATCH: got %d want 200", got)
		}
	})

	t.Run("T3_tenant_admin_reads", func(t *testing.T) {
		got := academic.status(http.MethodGet, "/api/v1/exams/"+examID, nil)
		if got != http.StatusOK {
			t.Errorf("academic GET: got %d want 200", got)
		}
	})

	t.Run("T4_tenant_admin_lacking_perm_blocked", func(t *testing.T) {
		// academic role lacks exams:write per devseed
		got := academic.status(http.MethodPatch, "/api/v1/exams/"+examID,
			map[string]any{"description": "Should fail"})
		if got != http.StatusForbidden {
			t.Errorf("academic PATCH: got %d want 403", got)
		}
	})

	t.Run("T5_student_GET_404_no_leak", func(t *testing.T) {
		got := student.status(http.MethodGet, "/api/v1/exams/"+examID, nil)
		if got != http.StatusNotFound {
			t.Errorf("student GET: got %d want 404", got)
		}
	})

	teacher2 := newSession(t, "teacher2@morfoschools.local", "teacher2-strong-pw")

	t.Run("T6_pre_invite_404", func(t *testing.T) {
		got := teacher2.status(http.MethodGet, "/api/v1/exams/"+examID, nil)
		if got != http.StatusNotFound {
			t.Errorf("teacher2 pre-invite: got %d want 404", got)
		}
	})

	var invite struct {
		ID   string `json:"id"`
		Role string `json:"role"`
	}
	t.Run("T7_owner_invites_editor", func(t *testing.T) {
		teacher.dec(http.MethodPost,
			"/api/v1/exams/"+examID+"/collaborators",
			map[string]any{"userId": teacher2ID, "role": "editor"},
			http.StatusCreated, &invite)
		if invite.Role != "editor" {
			t.Errorf("expected editor role, got %s", invite.Role)
		}
	})

	t.Run("T8_editor_PATCH_ok", func(t *testing.T) {
		got := teacher2.status(http.MethodPatch, "/api/v1/exams/"+examID,
			map[string]any{"description": "Editor edit"})
		if got != http.StatusOK {
			t.Errorf("editor PATCH: got %d want 200", got)
		}
	})

	t.Run("T9_editor_invite_blocked_403", func(t *testing.T) {
		got := teacher2.status(http.MethodPost,
			"/api/v1/exams/"+examID+"/collaborators",
			map[string]any{
				"userId": "00000000-0000-0000-0000-000000000000",
				"role":   "viewer",
			})
		if got != http.StatusForbidden {
			t.Errorf("editor manage: got %d want 403", got)
		}
	})

	t.Run("T10_owner_downgrades_to_viewer", func(t *testing.T) {
		got := teacher.status(http.MethodPatch,
			"/api/v1/exam-collaborators/"+invite.ID,
			map[string]any{"role": "viewer"})
		if got != http.StatusOK {
			t.Errorf("downgrade: got %d want 200", got)
		}
	})

	t.Run("T11_viewer_PATCH_403", func(t *testing.T) {
		got := teacher2.status(http.MethodPatch, "/api/v1/exams/"+examID,
			map[string]any{"description": "Hijack attempt"})
		if got != http.StatusForbidden {
			t.Errorf("viewer PATCH: got %d want 403", got)
		}
	})

	t.Run("T12_viewer_GET_ok", func(t *testing.T) {
		got := teacher2.status(http.MethodGet, "/api/v1/exams/"+examID, nil)
		if got != http.StatusOK {
			t.Errorf("viewer GET: got %d want 200", got)
		}
	})

	// Need a section for question creation
	var sectionsResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	teacher.dec(http.MethodGet, "/api/v1/exams/"+examID+"/sections", nil,
		http.StatusOK, &sectionsResp)
	if len(sectionsResp.Data) == 0 {
		t.Fatal("no sections on exam")
	}
	sectionID := sectionsResp.Data[0].ID

	t.Run("T13_viewer_create_question_403", func(t *testing.T) {
		got := teacher2.status(http.MethodPost,
			"/api/v1/exams/"+examID+"/questions",
			map[string]any{
				"sectionId":    sectionID,
				"questionType": "short_answer",
				"content":      "viewer attempt",
				"points":       1,
			})
		if got != http.StatusForbidden {
			t.Errorf("viewer question create: got %d want 403", got)
		}
	})

	t.Run("T14_editor_create_question_ok", func(t *testing.T) {
		// promote back to editor first
		teacher.dec(http.MethodPatch,
			"/api/v1/exam-collaborators/"+invite.ID,
			map[string]any{"role": "editor"},
			http.StatusOK, nil)
		got := teacher2.status(http.MethodPost,
			"/api/v1/exams/"+examID+"/questions",
			map[string]any{
				"sectionId":    sectionID,
				"questionType": "short_answer",
				"content":      "editor wrote this",
				"points":       1,
			})
		if got != http.StatusOK && got != http.StatusCreated {
			t.Errorf("editor question create: got %d want 200/201", got)
		}
	})

	t.Run("T15_transfer_ownership", func(t *testing.T) {
		got := teacher.status(http.MethodPatch,
			"/api/v1/exams/"+examID+"/transfer-ownership",
			map[string]any{"newOwnerId": teacher2ID})
		if got != http.StatusOK {
			t.Errorf("transfer: got %d want 200", got)
		}
	})

	t.Run("T16_old_owner_cannot_transfer", func(t *testing.T) {
		got := teacher.status(http.MethodPatch,
			"/api/v1/exams/"+examID+"/transfer-ownership",
			map[string]any{
				"newOwnerId": "00000000-0000-0000-0000-000000000000",
			})
		if got != http.StatusForbidden {
			t.Errorf("old owner transfer: got %d want 403", got)
		}
	})
}

// ---- T17-T21: Blueprint template collaboration --------------------

func TestCollab_BlueprintFlow(t *testing.T) {
	teacher := newSession(t, "teacher@morfoschools.local", "teacher123")
	teacher2ID := ensureUser(t, "demo-school", "teacher2@morfoschools.local",
		"teacher2-strong-pw", "teacher")
	teacher2 := newSession(t, "teacher2@morfoschools.local", "teacher2-strong-pw")

	var created struct {
		ID string `json:"id"`
	}
	teacher.dec(http.MethodPost, "/api/v1/blueprint-templates",
		map[string]any{
			"title":          "Collab BP " + randSuffix(),
			"curriculumCode": "k13",
			"blueprintType":  "reguler",
		},
		http.StatusCreated, &created)
	templateID := created.ID
	t.Cleanup(func() { cleanupBlueprint(t, templateID) })

	t.Run("T17_owner_creates_bp", func(t *testing.T) {
		if templateID == "" {
			t.Fatal("templateID empty")
		}
	})

	t.Run("T18_non_collab_GET_404", func(t *testing.T) {
		got := teacher2.status(http.MethodGet,
			"/api/v1/blueprint-templates/"+templateID, nil)
		if got != http.StatusNotFound {
			t.Errorf("non-collab GET: got %d want 404", got)
		}
	})

	var invite struct{ ID string }
	t.Run("T19_invite_viewer", func(t *testing.T) {
		teacher.dec(http.MethodPost,
			"/api/v1/blueprint-templates/"+templateID+"/collaborators",
			map[string]any{"userId": teacher2ID, "role": "viewer"},
			http.StatusCreated, &invite)
	})

	t.Run("T20_viewer_GET_ok", func(t *testing.T) {
		got := teacher2.status(http.MethodGet,
			"/api/v1/blueprint-templates/"+templateID, nil)
		if got != http.StatusOK {
			t.Errorf("viewer GET: got %d want 200", got)
		}
	})

	t.Run("T21_viewer_create_slot_403", func(t *testing.T) {
		got := teacher2.status(http.MethodPost,
			"/api/v1/blueprint-templates/"+templateID+"/slots",
			map[string]any{
				"competencyCode": "X",
				"cognitiveLevel": "C2",
				"difficulty":     "sedang",
				"points":         1,
			})
		if got != http.StatusForbidden {
			t.Errorf("viewer slot create: got %d want 403", got)
		}
	})
}

// ---- T22-T28: Security gates --------------------------------------

func TestSecurity_AIConfirmAndCSRF(t *testing.T) {
	teacher := newSession(t, "teacher@morfoschools.local", "teacher123")

	t.Run("T22_bogus_proposalId_404", func(t *testing.T) {
		got := teacher.status(http.MethodPost, "/api/v1/ai/confirm",
			map[string]any{
				"proposalId": "00000000-0000-0000-0000-000000000000",
			})
		if got != http.StatusNotFound {
			t.Errorf("bogus confirm: got %d want 404", got)
		}
	})

	t.Run("T23_no_csrf_403", func(t *testing.T) {
		// Bypass session.do CSRF injection: build the request manually.
		body, _ := json.Marshal(map[string]string{
			"proposalId": "00000000-0000-0000-0000-000000000000",
		})
		req, _ := http.NewRequest(http.MethodPost,
			globalHarness.server.URL+"/api/v1/ai/confirm",
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// Carry session cookie but DROP X-CSRF-Token header
		resp, err := teacher.client.Do(req)
		if err != nil {
			t.Fatalf("no-csrf: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("no-csrf: got %d want 403", resp.StatusCode)
		}
	})

	t.Run("T24_csrf_mismatch_403", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"proposalId": "00000000-0000-0000-0000-000000000000",
		})
		req, _ := http.NewRequest(http.MethodPost,
			globalHarness.server.URL+"/api/v1/ai/confirm",
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", "WRONG")
		resp, err := teacher.client.Do(req)
		if err != nil {
			t.Fatalf("csrf-mismatch: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("csrf-mismatch: got %d want 403", resp.StatusCode)
		}
	})
}

func TestSecurity_BodyCap(t *testing.T) {
	teacher := newSession(t, "teacher@morfoschools.local", "teacher123")

	t.Run("T25_body_above_1MiB_rejected", func(t *testing.T) {
		// 1.5 MiB title to bust through the 1 MiB MaxBytesReader cap
		big := bytes.Repeat([]byte("x"), 1500_000)
		body := append([]byte(`{"title":"`), big...)
		body = append(body, []byte(`"}`)...)
		req, _ := http.NewRequest(http.MethodPost,
			globalHarness.server.URL+"/api/v1/exams",
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", teacher.csrf)
		resp, err := teacher.client.Do(req)
		if err != nil {
			t.Fatalf("big body: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest &&
			resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Errorf("big body: got %d want 400 or 413", resp.StatusCode)
		}
	})
}

func TestSecurity_HeadersAndCORS(t *testing.T) {
	t.Run("T26_csp_header_present", func(t *testing.T) {
		resp, err := http.Get(globalHarness.server.URL + "/healthz")
		if err != nil {
			t.Fatalf("healthz: %v", err)
		}
		defer resp.Body.Close()
		csp := resp.Header.Get("Content-Security-Policy")
		if csp == "" {
			t.Errorf("expected CSP header, got empty")
		}
		if !strContains(csp, "default-src 'self'") ||
			!strContains(csp, "frame-ancestors 'none'") {
			t.Errorf("CSP missing expected directives: %s", csp)
		}
	})

	t.Run("T27_cors_unknown_origin_no_reflect", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet,
			globalHarness.server.URL+"/healthz", nil)
		req.Header.Set("Origin", "https://evil.example.com")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("cors evil: %v", err)
		}
		defer resp.Body.Close()
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("evil origin reflected: %q", got)
		}
	})

	t.Run("T28_cors_allowed_origin_reflect", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet,
			globalHarness.server.URL+"/healthz", nil)
		req.Header.Set("Origin", "http://test")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("cors allowed: %v", err)
		}
		defer resp.Body.Close()
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://test" {
			t.Errorf("allowed origin not reflected: got %q", got)
		}
	})
}

// ---- tiny utils ---------------------------------------------------

func strContains(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func randSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// silence unused imports under build tag combinations
var _ = url.Parse
