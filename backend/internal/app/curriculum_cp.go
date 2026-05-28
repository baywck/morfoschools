package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"morfoschools/backend/internal/platform/httpx"
)

const cpSourceName = "Kemendikdasmen Capaian Pembelajaran"

type cpElementRow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	SortOrder int    `json:"sortOrder"`
}

type cpReferenceRow struct {
	ID             string         `json:"id"`
	CurriculumCode string         `json:"curriculumCode"`
	LevelCode      string         `json:"levelCode"`
	LevelName      *string        `json:"levelName"`
	SubjectCode    string         `json:"subjectCode"`
	SubjectName    string         `json:"subjectName"`
	Phase          string         `json:"phase"`
	GeneralCP      string         `json:"generalCp"`
	SourceName     string         `json:"sourceName"`
	SourceURL      *string        `json:"sourceUrl"`
	Status         string         `json:"status"`
	ElementsCount  int            `json:"elementsCount"`
	Elements       []cpElementRow `json:"elements,omitempty"`
	CreatedAt      string         `json:"createdAt"`
	UpdatedAt      string         `json:"updatedAt"`
}

type kemendikCPResponse struct {
	Compiled struct {
		Params struct {
			Level   string `json:"level"`
			Subject string `json:"subject"`
			Phase   string `json:"phase"`
		} `json:"params"`
		Phase       string `json:"phase"`
		PDFList     []any  `json:"pdfList"`
		GeneralInfo string `json:"generalInfo"`
		Elements    []struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		} `json:"elements"`
	} `json:"compiled"`
	Subject string `json:"subject"`
}

func (a *App) registerCurriculumCPRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/curriculum/cp-references", a.handleListCPReferences)
	mux.HandleFunc("GET /api/v1/curriculum/cp-references/{id}", a.handleGetCPReference)
	mux.HandleFunc("POST /api/v1/curriculum/cp-references", a.handleCreateCPReference)
	mux.HandleFunc("PATCH /api/v1/curriculum/cp-references/{id}", a.handleUpdateCPReference)
	mux.HandleFunc("POST /api/v1/curriculum/cp-references/{id}/elements", a.handleCreateCPElement)
	mux.HandleFunc("PATCH /api/v1/curriculum/cp-elements/{id}", a.handleUpdateCPElement)
	mux.HandleFunc("DELETE /api/v1/curriculum/cp-elements/{id}", a.handleDeleteCPElement)
	mux.HandleFunc("POST /api/v1/curriculum/cp-references/seed", a.handleSeedCPReference)
}

func (a *App) handleListCPReferences(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "academic:read") {
		return
	}
	p := httpx.ParsePagination(r)
	search := strings.TrimSpace(httpx.QueryString(r, "search", ""))
	level := strings.TrimSpace(httpx.QueryString(r, "level", ""))
	phase := strings.ToLower(strings.TrimSpace(httpx.QueryString(r, "phase", "")))

	where := []string{"1=1"}
	args := []any{}
	idx := 1
	if search != "" {
		where = append(where, `(subject_name ILIKE $`+strconv.Itoa(idx)+` OR subject_code ILIKE $`+strconv.Itoa(idx)+` OR general_cp ILIKE $`+strconv.Itoa(idx)+`)`)
		args = append(args, "%"+search+"%")
		idx++
	}
	if level != "" {
		where = append(where, `level_code = $`+strconv.Itoa(idx))
		args = append(args, level)
		idx++
	}
	if phase != "" {
		parts := strings.Split(phase, ",")
		phases := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.ToLower(strings.TrimSpace(p))
			if p != "" {
				phases = append(phases, p)
			}
		}
		if len(phases) == 1 {
			where = append(where, `phase = $`+strconv.Itoa(idx))
			args = append(args, phases[0])
			idx++
		} else if len(phases) > 1 {
			where = append(where, `phase = ANY($`+strconv.Itoa(idx)+`)`)
			args = append(args, phases)
			idx++
		}
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM curriculum_cp_references WHERE `+whereSQL, args...).Scan(&total); err != nil {
		a.logger.Error("count cp references failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "cp_lookup_failed", "Could not load CP references", r)
		return
	}
	query := `SELECT r.id::text, r.curriculum_code, r.level_code, r.level_name, r.subject_code, r.subject_name, r.phase, r.general_cp, r.source_name, r.source_url, r.status, COUNT(e.id)::int, r.created_at::text, r.updated_at::text
		FROM curriculum_cp_references r LEFT JOIN curriculum_cp_elements e ON e.reference_id = r.id
		WHERE ` + whereSQL + ` GROUP BY r.id ORDER BY r.level_code, r.subject_name, r.phase LIMIT $` + strconv.Itoa(idx) + ` OFFSET $` + strconv.Itoa(idx+1)
	args = append(args, p.PageSize, p.Offset)
	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		a.logger.Error("list cp references failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "cp_lookup_failed", "Could not load CP references", r)
		return
	}
	defer rows.Close()
	items := []cpReferenceRow{}
	for rows.Next() {
		var row cpReferenceRow
		if err := rows.Scan(&row.ID, &row.CurriculumCode, &row.LevelCode, &row.LevelName, &row.SubjectCode, &row.SubjectName, &row.Phase, &row.GeneralCP, &row.SourceName, &row.SourceURL, &row.Status, &row.ElementsCount, &row.CreatedAt, &row.UpdatedAt); err == nil {
			items = append(items, row)
		}
	}
	writeJSON(w, http.StatusOK, httpx.NewPaginatedResponse(items, p, total))
}

func (a *App) handleGetCPReference(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "academic:read") {
		return
	}
	ref, ok := a.loadCPReference(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, ref)
}

func (a *App) requirePlatformCPWrite(w http.ResponseWriter, r *http.Request) bool {
	if !a.RequireCSRF(w, r) {
		return false
	}
	auth := AuthFromContext(r.Context())
	if auth == nil {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthenticated", "Not authenticated", r)
		return false
	}
	if auth.IsPlatformAdmin || hasAnyRole(auth, "master_admin", "platform_admin") {
		return true
	}
	writeErrorJSON(w, http.StatusForbidden, "forbidden", "Only platform admins can modify official CP master data", r)
	return false
}

func hasAnyRole(auth *AuthContext, roles ...string) bool {
	if auth == nil {
		return false
	}
	for _, have := range auth.Roles {
		for _, want := range roles {
			if have == want {
				return true
			}
		}
	}
	return false
}

func (a *App) handleCreateCPReference(w http.ResponseWriter, r *http.Request) {
	if !a.requirePlatformCPWrite(w, r) {
		return
	}
	var req struct {
		LevelCode, LevelName, SubjectCode, SubjectName, Phase, GeneralCP, SourceURL string
		Elements                                                                    []struct {
			Name, Content string `json:",omitempty"`
		}
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}
	refID, fields, err := a.upsertCPReference(r.Context(), req.LevelCode, req.LevelName, req.SubjectCode, req.SubjectName, req.Phase, req.GeneralCP, req.SourceURL, nil, false)
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}
	if err != nil {
		a.logger.Error("create cp reference failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create CP reference", r)
		return
	}
	for i, el := range req.Elements {
		_, _ = a.upsertCPElement(r.Context(), refID, el.Name, el.Content, i+1)
	}
	a.audit(r.Context(), nil, AuthFromContext(r.Context()).UserID, "curriculum_cp.create", "curriculum_cp_reference", refID, r)
	ref, _ := a.loadCPReference(w, r, refID)
	writeJSON(w, http.StatusCreated, ref)
}

func (a *App) handleUpdateCPReference(w http.ResponseWriter, r *http.Request) {
	if !a.requirePlatformCPWrite(w, r) {
		return
	}
	id := r.PathValue("id")
	var req struct {
		SubjectName, GeneralCP, Status *string `json:",omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}
	if req.Status != nil && *req.Status != "active" && *req.Status != "archived" {
		writeValidationError(w, map[string]string{"status": "Must be active or archived"}, r)
		return
	}
	res, err := a.db.ExecContext(r.Context(), `UPDATE curriculum_cp_references SET subject_name=COALESCE(NULLIF($1,''),subject_name), general_cp=COALESCE(NULLIF($2,''),general_cp), status=COALESCE(NULLIF($3,''),status), updated_at=now() WHERE id=$4`, ptrString(req.SubjectName), ptrString(req.GeneralCP), ptrString(req.Status), id)
	if err != nil {
		a.logger.Error("update cp reference failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not update CP reference", r)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "CP reference not found", r)
		return
	}
	a.audit(r.Context(), nil, AuthFromContext(r.Context()).UserID, "curriculum_cp.update", "curriculum_cp_reference", id, r)
	ref, _ := a.loadCPReference(w, r, id)
	writeJSON(w, http.StatusOK, ref)
}

func (a *App) handleCreateCPElement(w http.ResponseWriter, r *http.Request) {
	if !a.requirePlatformCPWrite(w, r) {
		return
	}
	var req struct {
		Name, Content string
		SortOrder     int
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}
	id, err := a.upsertCPElement(r.Context(), r.PathValue("id"), req.Name, req.Content, req.SortOrder)
	if err != nil {
		writeValidationError(w, map[string]string{"element": "Name and content are required"}, r)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *App) handleUpdateCPElement(w http.ResponseWriter, r *http.Request) {
	if !a.requirePlatformCPWrite(w, r) {
		return
	}
	var req struct {
		Name, Content *string
		SortOrder     *int
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}
	res, err := a.db.ExecContext(r.Context(), `UPDATE curriculum_cp_elements SET name=COALESCE(NULLIF($1,''),name), content=COALESCE(NULLIF($2,''),content), sort_order=COALESCE($3,sort_order), updated_at=now() WHERE id=$4`, ptrString(req.Name), ptrString(req.Content), req.SortOrder, r.PathValue("id"))
	if err != nil {
		a.logger.Error("update cp element failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not update CP element", r)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "CP element not found", r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": r.PathValue("id"), "status": "updated"})
}

func (a *App) handleDeleteCPElement(w http.ResponseWriter, r *http.Request) {
	if !a.requirePlatformCPWrite(w, r) {
		return
	}
	res, err := a.db.ExecContext(r.Context(), `DELETE FROM curriculum_cp_elements WHERE id=$1`, r.PathValue("id"))
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "delete_failed", "Could not delete CP element", r)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "CP element not found", r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

func (a *App) handleSeedCPReference(w http.ResponseWriter, r *http.Request) {
	if !a.requirePlatformCPWrite(w, r) {
		return
	}
	var req struct{ LevelCode, SubjectCode, Phase string }
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}
	ref, fields, err := a.seedCPReferenceFromKemendikdasmen(r.Context(), req.LevelCode, req.SubjectCode, req.Phase)
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}
	if err != nil {
		a.logger.Error("seed cp reference failed", "error", err)
		writeErrorJSON(w, http.StatusBadGateway, "seed_failed", "Could not seed CP reference from Kemendikdasmen", r)
		return
	}
	a.audit(r.Context(), nil, AuthFromContext(r.Context()).UserID, "curriculum_cp.seed", "curriculum_cp_reference", ref.ID, r)
	writeJSON(w, http.StatusOK, ref)
}

func (a *App) loadCPReference(w http.ResponseWriter, r *http.Request, id string) (cpReferenceRow, bool) {
	var row cpReferenceRow
	err := a.db.QueryRowContext(r.Context(), `SELECT id::text,curriculum_code,level_code,level_name,subject_code,subject_name,phase,general_cp,source_name,source_url,status,created_at::text,updated_at::text FROM curriculum_cp_references WHERE id=$1`, id).Scan(&row.ID, &row.CurriculumCode, &row.LevelCode, &row.LevelName, &row.SubjectCode, &row.SubjectName, &row.Phase, &row.GeneralCP, &row.SourceName, &row.SourceURL, &row.Status, &row.CreatedAt, &row.UpdatedAt)
	if err == sql.ErrNoRows {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "CP reference not found", r)
		return row, false
	}
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "cp_lookup_failed", "Could not load CP reference", r)
		return row, false
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id::text,name,content,sort_order FROM curriculum_cp_elements WHERE reference_id=$1 ORDER BY sort_order,name`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var el cpElementRow
			if rows.Scan(&el.ID, &el.Name, &el.Content, &el.SortOrder) == nil {
				row.Elements = append(row.Elements, el)
			}
		}
	}
	row.ElementsCount = len(row.Elements)
	return row, true
}

func (a *App) seedCPReferenceFromKemendikdasmen(ctx context.Context, levelCode, subjectCode, phase string) (cpReferenceRow, map[string]string, error) {
	levelCode = normalizeCPToken(levelCode)
	subjectCode = normalizeCPToken(subjectCode)
	phase = strings.ToLower(strings.TrimSpace(phase))
	fields := map[string]string{}
	if levelCode == "" {
		fields["levelCode"] = "Level is required"
	}
	if subjectCode == "" {
		fields["subjectCode"] = "Subject is required"
	}
	if phase == "" {
		fields["phase"] = "Phase is required"
	}
	if len(fields) > 0 {
		return cpReferenceRow{}, fields, nil
	}
	url := fmt.Sprintf("https://guru.kemendikdasmen.go.id/kurikulum/referensi-penerapan/capaian-pembelajaran/%s/subject/%s/fase-%s.json", levelCode, subjectCode, phase)
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "Morfoschools CP importer/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return cpReferenceRow{}, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return cpReferenceRow{}, nil, fmt.Errorf("provider status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return cpReferenceRow{}, nil, err
	}
	var payload kemendikCPResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return cpReferenceRow{}, nil, err
	}
	compiled := map[string]any{}
	_ = json.Unmarshal(body, &compiled)
	refID, vfields, err := a.upsertCPReference(ctx, payload.Compiled.Params.Level, cpLevelName(payload.Compiled.Params.Level), payload.Compiled.Params.Subject, payload.Subject, payload.Compiled.Params.Phase, payload.Compiled.GeneralInfo, url, compiled, true)
	if len(vfields) > 0 || err != nil {
		return cpReferenceRow{}, vfields, err
	}
	for i, el := range payload.Compiled.Elements {
		if _, err := a.upsertCPElement(ctx, refID, el.Name, el.Content, i+1); err != nil {
			return cpReferenceRow{}, nil, err
		}
	}
	return a.loadCPReferenceNoHTTP(ctx, refID)
}

func (a *App) upsertCPReference(ctx context.Context, levelCode, levelName, subjectCode, subjectName, phase, generalCP, sourceURL string, compiled any, fromSeed bool) (string, map[string]string, error) {
	levelCode = normalizeCPToken(levelCode)
	subjectCode = normalizeCPToken(subjectCode)
	phase = strings.ToLower(strings.TrimSpace(phase))
	subjectName = strings.TrimSpace(subjectName)
	generalCP = normalizeCPText(generalCP)
	fields := map[string]string{}
	if levelCode == "" {
		fields["levelCode"] = "Level is required"
	}
	if subjectCode == "" {
		fields["subjectCode"] = "Subject is required"
	}
	if subjectName == "" {
		fields["subjectName"] = "Subject name is required"
	}
	if phase == "" {
		fields["phase"] = "Phase is required"
	}
	if generalCP == "" {
		fields["generalCp"] = "General CP is required"
	}
	if len(fields) > 0 {
		return "", fields, nil
	}
	b, _ := json.Marshal(compiled)
	if compiled == nil {
		b = []byte(`{}`)
	}
	var id string
	err := a.db.QueryRowContext(ctx, `INSERT INTO curriculum_cp_references (curriculum_code,level_code,level_name,subject_code,subject_name,phase,general_cp,source_name,source_url,compiled_json,status) VALUES ('merdeka',$1,NULLIF($2,''),$3,$4,$5,$6,$7,NULLIF($8,''),$9,'active') ON CONFLICT (curriculum_code,level_code,subject_code,phase) DO UPDATE SET level_name=EXCLUDED.level_name, subject_name=EXCLUDED.subject_name, general_cp=EXCLUDED.general_cp, source_url=EXCLUDED.source_url, compiled_json=CASE WHEN $10 THEN EXCLUDED.compiled_json ELSE curriculum_cp_references.compiled_json END, status='active', updated_at=now() RETURNING id::text`, levelCode, levelName, subjectCode, subjectName, phase, generalCP, cpSourceName, sourceURL, b, fromSeed).Scan(&id)
	return id, nil, err
}

func (a *App) upsertCPElement(ctx context.Context, refID, name, content string, sortOrder int) (string, error) {
	name = normalizeCPText(name)
	content = normalizeCPText(content)
	if sortOrder <= 0 {
		sortOrder = 1
	}
	if refID == "" || name == "" || content == "" {
		return "", fmt.Errorf("invalid element")
	}
	var id string
	err := a.db.QueryRowContext(ctx, `INSERT INTO curriculum_cp_elements (reference_id,name,content,sort_order) VALUES ($1,$2,$3,$4) ON CONFLICT (reference_id,name) DO UPDATE SET content=EXCLUDED.content, sort_order=EXCLUDED.sort_order, updated_at=now() RETURNING id::text`, refID, name, content, sortOrder).Scan(&id)
	return id, err
}

func (a *App) loadCPReferenceNoHTTP(ctx context.Context, id string) (cpReferenceRow, map[string]string, error) {
	var row cpReferenceRow
	err := a.db.QueryRowContext(ctx, `SELECT id::text,curriculum_code,level_code,level_name,subject_code,subject_name,phase,general_cp,source_name,source_url,status,created_at::text,updated_at::text FROM curriculum_cp_references WHERE id=$1`, id).Scan(&row.ID, &row.CurriculumCode, &row.LevelCode, &row.LevelName, &row.SubjectCode, &row.SubjectName, &row.Phase, &row.GeneralCP, &row.SourceName, &row.SourceURL, &row.Status, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		return row, nil, err
	}
	rows, err := a.db.QueryContext(ctx, `SELECT id::text,name,content,sort_order FROM curriculum_cp_elements WHERE reference_id=$1 ORDER BY sort_order,name`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var el cpElementRow
			if rows.Scan(&el.ID, &el.Name, &el.Content, &el.SortOrder) == nil {
				row.Elements = append(row.Elements, el)
			}
		}
	}
	row.ElementsCount = len(row.Elements)
	return row, nil, nil
}

func normalizeCPToken(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
func normalizeCPText(s string) string {
	return strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(s, " "))
}
func cpLevelName(code string) string {
	switch code {
	case "sd-sma":
		return "SD-SMA/Sederajat"
	case "smk":
		return "SMK/Sederajat"
	default:
		return code
	}
}
func ptrString(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}
