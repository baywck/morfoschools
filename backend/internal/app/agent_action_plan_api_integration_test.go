//go:build integration
// +build integration

package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func testEnvOr(t *testing.T, key string) string {
	t.Helper()
	return strings.TrimSpace(os.Getenv(key))
}

func TestActionPlanSmokeEndpoints(t *testing.T) {
	dbURL := testEnvOr(t, "DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}
	a, err := New(Config{Port: "0", AppEnv: "test"}, nil, db)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	handler := a.Handler()

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"admin@morfoschools.local","password":"admin123"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", loginRec.Code, loginRec.Body.String())
	}
	var loginBody struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginBody); err != nil || loginBody.CSRFToken == "" {
		t.Fatalf("login parse failed: %v", err)
	}
	cookies := loginRec.Result().Cookies()

	postWithCSRF := func(path string, payload string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", loginBody.CSRFToken)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	getWithCookies := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	summary := getWithCookies("/api/v1/ai/action-plans/current/summary?examId=148ba6ec-a7ef-4c41-8a31-a4652e36b506")
	if summary.Code == http.StatusOK {
		t.Logf("summary before plan: %s", summary.Body.String())
	}

	payload := `{
		"sessionId": "",
		"message": "integration kisi-kisi audit",
		"scopeType": "exam",
		"source": "audit",
		"goal": "integration kisi-kisi audit",
		"examId": "148ba6ec-a7ef-4c41-8a31-a4652e36b506",
		"planned": {
			"scopeType": "exam",
			"source": "audit",
			"goal": "integration kisi-kisi audit",
			"intentSummary": "audit kisi-kisi",
			"batches": [
				{
					"batchIndex": 1,
					"actionType": "audit",
					"workflow": "audit_blueprint_slots",
					"targetType": "blueprint_slot",
					"targetIds": ["148ba6ec-a7ef-4c41-8a31-a4652e36b506"],
					"argsJson": {"examId": "148ba6ec-a7ef-4c41-8a31-a4652e36b506"},
					"preview": "Audit kisi-kisi",
					"progressUnits": 1
				}
			]
		}
	}`
	created := postWithCSRF("/api/v1/ai/action-plans", payload)
	if created.Code != http.StatusOK {
		t.Fatalf("create plan: expected 200, got %d: %s", created.Code, created.Body.String())
	}
	var createdBody struct {
		PlanID string `json:"planId"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createdBody); err != nil || createdBody.PlanID == "" {
		t.Fatalf("create plan parse failed: %v", err)
	}

	runNext := postWithCSRF("/api/v1/ai/action-plans/"+createdBody.PlanID+"/run-next", "{}")
	if runNext.Code != http.StatusOK {
		t.Fatalf("run-next: expected 200, got %d: %s", runNext.Code, runNext.Body.String())
	}

	final := getWithCookies("/api/v1/ai/action-plans/current/summary?examId=148ba6ec-a7ef-4c41-8a31-a4652e36b506")
	if final.Code != http.StatusOK {
		t.Fatalf("final summary: expected 200, got %d: %s", final.Code, final.Body.String())
	}
	if !strings.Contains(final.Body.String(), `"status":"completed"`) {
		t.Fatalf("final summary missing completed status: %s", final.Body.String())
	}
}
