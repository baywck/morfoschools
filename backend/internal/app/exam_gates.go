package app

import (
	"net/http"
	"strconv"
	"time"
)

// Exam Gate Windows — schedule when an exam is takeable.
//
// Multiple windows per exam are supported (e.g. retake window after the
// main exam). An exam is "open" right now iff there exists a window with
// opens_at <= now() <= closes_at.
//
// Take-flow code (Phase 10) will check isExamOpen before serving questions
// and use the access_code (if set) for password-gated rooms.

func (a *App) registerExamGateRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/exams/{id}/gates", a.handleListExamGates)
	mux.HandleFunc("POST /api/v1/exams/{id}/gates", a.handleCreateExamGate)
	mux.HandleFunc("PATCH /api/v1/exam-gates/{gateId}", a.handleUpdateExamGate)
	mux.HandleFunc("DELETE /api/v1/exam-gates/{gateId}", a.handleDeleteExamGate)
}

type examGateRow struct {
	ID         string  `json:"id"`
	ExamID     string  `json:"examId"`
	OpensAt    string  `json:"opensAt"`
	ClosesAt   string  `json:"closesAt"`
	AccessCode *string `json:"accessCode,omitempty"`
	IsOpen     bool    `json:"isOpen"`
	CreatedAt  string  `json:"createdAt"`
}

func (a *App) handleListExamGates(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	examID := r.PathValue("id")

	rows, err := a.db.QueryContext(r.Context(), `
		SELECT id, exam_id, opens_at, closes_at, access_code, created_at,
		       (now() BETWEEN opens_at AND closes_at) AS is_open
		  FROM exam_gate_windows
		 WHERE exam_id = $1 AND tenant_id = $2
		 ORDER BY opens_at ASC`,
		examID, tenantID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "gates_lookup_failed", "Could not load gates", r)
		return
	}
	defer rows.Close()

	out := make([]examGateRow, 0)
	for rows.Next() {
		var g examGateRow
		if err := rows.Scan(&g.ID, &g.ExamID, &g.OpensAt, &g.ClosesAt, &g.AccessCode, &g.CreatedAt, &g.IsOpen); err == nil {
			out = append(out, g)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *App) handleCreateExamGate(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	examID := r.PathValue("id")

	auth := AuthFromContext(r.Context())
	if _, ok := a.requireExamWriteAccess(w, r, tenantID, auth, examID); !ok {
		return
	}

	var req struct {
		OpensAt    string  `json:"opensAt"`
		ClosesAt   string  `json:"closesAt"`
		AccessCode *string `json:"accessCode"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}

	opens, err1 := time.Parse(time.RFC3339, req.OpensAt)
	closes, err2 := time.Parse(time.RFC3339, req.ClosesAt)
	errs := map[string]string{}
	if err1 != nil {
		errs["opensAt"] = "Must be ISO 8601 (e.g. 2026-05-20T08:00:00Z)"
	}
	if err2 != nil {
		errs["closesAt"] = "Must be ISO 8601 (e.g. 2026-05-20T10:00:00Z)"
	}
	if err1 == nil && err2 == nil && !closes.After(opens) {
		errs["closesAt"] = "closesAt must be after opensAt"
	}
	if len(errs) > 0 {
		writeValidationError(w, errs, r)
		return
	}

	var id string
	err := a.db.QueryRowContext(r.Context(), `
		INSERT INTO exam_gate_windows (tenant_id, exam_id, opens_at, closes_at, access_code, password)
		VALUES ($1, $2, $3, $4, NULLIF($5,''), NULLIF($5,''))
		RETURNING id`,
		tenantID, examID, opens, closes, ptrToString(req.AccessCode),
	).Scan(&id)
	// Note: we mirror access_code into the legacy `password` column too so a
	// rollback to migration 000012 still has the data.
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create gate", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "exam_gates.create", "exam_gate", id, r)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *App) handleUpdateExamGate(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	gid := r.PathValue("gateId")

	auth := AuthFromContext(r.Context())
	// Resolve parent exam for layered access check.
	var examID string
	if err := a.db.QueryRowContext(r.Context(),
		`SELECT exam_id::text FROM exam_gate_windows WHERE id = $1 AND tenant_id = $2`,
		gid, tenantID,
	).Scan(&examID); err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Gate not found", r)
		return
	}
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}

	var req struct {
		OpensAt    *string `json:"opensAt"`
		ClosesAt   *string `json:"closesAt"`
		AccessCode *string `json:"accessCode"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}

	// We need the final (post-patch) opens_at / closes_at to enforce the
	// invariant `closes_at > opens_at`. Load the existing row first so a
	// partial patch can be compared against the persisted value.
	var curOpens, curCloses time.Time
	if err := a.db.QueryRowContext(r.Context(),
		`SELECT opens_at, closes_at FROM exam_gate_windows WHERE id = $1 AND tenant_id = $2`,
		gid, tenantID,
	).Scan(&curOpens, &curCloses); err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Gate not found", r)
		return
	}

	newOpens := curOpens
	newCloses := curCloses
	parseErrs := map[string]string{}
	if req.OpensAt != nil {
		t, err := time.Parse(time.RFC3339, *req.OpensAt)
		if err != nil {
			parseErrs["opensAt"] = "Must be ISO 8601"
		} else {
			newOpens = t
		}
	}
	if req.ClosesAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ClosesAt)
		if err != nil {
			parseErrs["closesAt"] = "Must be ISO 8601"
		} else {
			newCloses = t
		}
	}
	if len(parseErrs) > 0 {
		writeValidationError(w, parseErrs, r)
		return
	}
	if (req.OpensAt != nil || req.ClosesAt != nil) && !newCloses.After(newOpens) {
		writeValidationError(w, map[string]string{
			"closesAt": "closesAt must be after opensAt",
		}, r)
		return
	}

	parts := []string{}
	args := []any{}
	idx := 1
	add := func(col string, val any) {
		parts = append(parts, col+" = $"+strconv.Itoa(idx))
		args = append(args, val)
		idx++
	}
	if req.OpensAt != nil {
		add("opens_at", newOpens)
	}
	if req.ClosesAt != nil {
		add("closes_at", newCloses)
	}
	if req.AccessCode != nil {
		if *req.AccessCode == "" {
			parts = append(parts, "access_code = NULL", "password = NULL")
		} else {
			add("access_code", *req.AccessCode)
			add("password", *req.AccessCode)
		}
	}
	if len(parts) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"id": gid, "status": "no_change"})
		return
	}

	q := "UPDATE exam_gate_windows SET " + joinComma(parts) +
		" WHERE id = $" + strconv.Itoa(idx) +
		" AND tenant_id = $" + strconv.Itoa(idx+1)
	args = append(args, gid, tenantID)

	if _, err := a.db.ExecContext(r.Context(), q, args...); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not update gate", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "exam_gates.update", "exam_gate", gid, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": gid, "status": "updated"})
}

func (a *App) handleDeleteExamGate(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	gid := r.PathValue("gateId")

	auth := AuthFromContext(r.Context())
	// Resolve parent exam for layered access check.
	var examID string
	if err := a.db.QueryRowContext(r.Context(),
		`SELECT exam_id::text FROM exam_gate_windows WHERE id = $1 AND tenant_id = $2`,
		gid, tenantID,
	).Scan(&examID); err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Gate not found", r)
		return
	}
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}

	res, err := a.db.ExecContext(r.Context(),
		`DELETE FROM exam_gate_windows WHERE id = $1 AND tenant_id = $2`,
		gid, tenantID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "delete_failed", "Could not delete gate", r)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Gate not found", r)
		return
	}
	a.audit(r.Context(), &tenantID, auth.UserID, "exam_gates.delete", "exam_gate", gid, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": gid, "status": "deleted"})
}
