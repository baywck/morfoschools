package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// AI chat is intentionally reset to discussion-only mode.
//
// The legacy generic agent/tool orchestration has been removed from the hot
// path because it became unsafe and difficult to reason about after the
// Kurikulum Merdeka/kisi-kisi upgrade. This endpoint now preserves the chat UX
// and session history, but it does not expose tools or mutate domain data.

func (a *App) registerAIChatRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/ai/chat", a.handleAIChat)
	mux.HandleFunc("POST /api/v1/ai/confirm", a.handleAgentConfirm)
	mux.HandleFunc("POST /api/v1/ai/cancel", a.handleAgentCancel)
	mux.HandleFunc("GET /api/v1/ai/sessions", a.handleListAISessions)
	mux.HandleFunc("GET /api/v1/ai/sessions/{id}/messages", a.handleGetAISessionMessages)
	mux.HandleFunc("DELETE /api/v1/ai/sessions/{id}", a.handleDeleteAISession)
}

type aiChatRequest struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
	Shadow    struct {
		Route          string            `json:"route"`
		ActiveEntities map[string]string `json:"activeEntities"`
	} `json:"shadow"`
}

type llmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content,omitempty"`
}

// deriveScopeKey returns a stable scope identifier for the active page.
func deriveScopeKey(active map[string]string) string {
	if len(active) == 0 {
		return "global"
	}
	if examID := strings.TrimSpace(active["examId"]); examID != "" {
		return "exam:" + examID
	}
	if templateID := strings.TrimSpace(active["templateId"]); templateID != "" {
		return "blueprint:" + templateID
	}
	return "global"
}

func (a *App) resolveOrCreateSession(ctx context.Context, suppliedID, tenantID, userID, scopeKey string) (string, error) {
	if scopeKey == "" {
		scopeKey = "global"
	}
	if suppliedID != "" {
		var id string
		err := a.db.QueryRowContext(ctx,
			`SELECT id FROM ai_sessions WHERE id=$1 AND user_id=$2 AND scope_key=$3`,
			suppliedID, userID, scopeKey,
		).Scan(&id)
		if err == nil && id != "" {
			return id, nil
		}
	}
	var id string
	err := a.db.QueryRowContext(ctx,
		`SELECT id FROM ai_sessions WHERE tenant_id=NULLIF($1,'')::uuid AND user_id=$2 AND scope_key=$3 ORDER BY last_active_at DESC LIMIT 1`,
		tenantID, userID, scopeKey,
	).Scan(&id)
	if err == nil && id != "" {
		return id, nil
	}
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	title := "Chat"
	if strings.HasPrefix(scopeKey, "exam:") {
		title = "Exam chat"
	}
	err = a.db.QueryRowContext(ctx,
		`INSERT INTO ai_sessions (tenant_id, user_id, title, scope_key) VALUES (NULLIF($1,'')::uuid, $2, $3, $4) RETURNING id`,
		tenantID, userID, title, scopeKey,
	).Scan(&id)
	return id, err
}

func (a *App) handleAIChat(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	var req aiChatRequest
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Message is required", r)
		return
	}
	tenantID := ""
	if auth.EffectiveTenantID != nil {
		tenantID = *auth.EffectiveTenantID
	}
	scopeKey := deriveScopeKey(req.Shadow.ActiveEntities)
	sessionID, err := a.resolveOrCreateSession(r.Context(), req.SessionID, tenantID, auth.UserID, scopeKey)
	if err != nil {
		a.logger.Error("resolve ai session failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "ai_error", "Could not create session", r)
		return
	}
	_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content) VALUES ($1, 'user', $2)`, sessionID, req.Message)
	if a.tryConfirmLatestAgentProposalFromChat(w, r, tenantID, auth.UserID, sessionID, req.Message) {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE ai_sessions SET message_count = message_count + 2, last_active_at = now() WHERE id=$1`, sessionID)
		return
	}
	if a.tryCreateAgentProposalFromIntent(w, r, tenantID, auth.UserID, sessionID, req) {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE ai_sessions SET message_count = message_count + 2, last_active_at = now() WHERE id=$1`, sessionID)
		return
	}
	content, totalTokens, err := a.callDiscussionLLM(r.Context(), sessionID, tenantID, req.Shadow.ActiveEntities, req.Message)
	if err != nil {
		a.logger.Error("discussion llm failed", "error", err)
		writeErrorJSON(w, http.StatusBadGateway, "ai_error", "AI service unavailable", r)
		return
	}
	_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
	_, _ = a.db.ExecContext(r.Context(), `UPDATE ai_sessions SET message_count = message_count + 2, last_active_at = now() WHERE id=$1`, sessionID)
	writeJSON(w, http.StatusOK, map[string]any{
		"message":   map[string]string{"role": "assistant", "content": content},
		"sessionId": sessionID,
		"tokens":    totalTokens,
	})
}

func (a *App) callDiscussionLLM(ctx context.Context, sessionID, tenantID string, active map[string]string, userMessage string) (string, int, error) {
	messages := []llmMessage{{Role: "system", Content: a.discussionSystemPrompt(ctx, tenantID, active)}}
	rows, err := a.db.QueryContext(ctx, `SELECT role, content FROM ai_messages WHERE session_id=$1 AND role IN ('user','assistant') ORDER BY created_at DESC LIMIT 12`, sessionID)
	if err == nil {
		defer rows.Close()
		var history []llmMessage
		for rows.Next() {
			var m llmMessage
			if scanErr := rows.Scan(&m.Role, &m.Content); scanErr == nil {
				history = append(history, m)
			}
		}
		for i := len(history) - 1; i >= 0; i-- {
			messages = append(messages, history[i])
		}
	}
	messages = append(messages, llmMessage{Role: "user", Content: userMessage})
	resp, err := a.callLLM(ctx, messages)
	if err != nil {
		return "", 0, err
	}
	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		return "", resp.Usage.TotalTokens, fmt.Errorf("empty LLM response")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), resp.Usage.TotalTokens, nil
}

func (a *App) discussionSystemPrompt(ctx context.Context, tenantID string, active map[string]string) string {
	var b strings.Builder
	b.WriteString("Kamu adalah asisten diskusi untuk Morfoschools. Jawab natural dalam Bahasa Indonesia. ")
	b.WriteString("PENTING: mode saat ini discussion-only. Kamu tidak punya tools dan tidak boleh mengklaim sudah membuat, menyimpan, mengubah, menghapus, atau mengirim proposal/action. ")
	b.WriteString("Tetap bantu diskusi, rekomendasi materi, kisi-kisi konseptual, bobot, dan penjelasan. Jangan mengulang disclaimer panjang setiap jawaban; cukup sebut singkat hanya jika user meminta aksi penyimpanan/pembuatan data. ")
	if examID := strings.TrimSpace(active["examId"]); examID != "" && a.db != nil {
		var title string
		var subject, grade sql.NullString
		_ = a.db.QueryRowContext(ctx, `SELECT e.title, s.name, e.grade_level FROM exams e LEFT JOIN subjects s ON s.id=e.subject_id AND s.tenant_id=e.tenant_id WHERE e.id=$1 AND e.tenant_id=NULLIF($2,'')::uuid`, examID, tenantID).Scan(&title, &subject, &grade)
		if title != "" || subject.Valid || grade.Valid {
			b.WriteString("Konteks halaman aktif: ")
			if title != "" {
				b.WriteString("exam=\"")
				b.WriteString(title)
				b.WriteString("\". ")
			}
			if strings.TrimSpace(subject.String) != "" {
				b.WriteString("mapel=\"")
				b.WriteString(strings.TrimSpace(subject.String))
				b.WriteString("\". ")
			}
			if strings.TrimSpace(grade.String) != "" {
				b.WriteString("kelas=\"")
				b.WriteString(strings.TrimSpace(grade.String))
				b.WriteString("\". ")
			}
		}
	}
	return b.String()
}

type llmResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func (a *App) callLLM(ctx context.Context, messages []llmMessage) (*llmResponse, error) {
	baseURL := strings.TrimRight(os.Getenv("AI_BASE_URL"), "/")
	apiKey := os.Getenv("AI_API_KEY")
	model := os.Getenv("AI_MODEL")
	if model == "" {
		model = "MORFOSCHOOLS"
	}
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("AI_BASE_URL/AI_API_KEY is not configured")
	}

	body := map[string]any{
		"model":       model,
		"messages":    messages,
		"temperature": 0.4,
		"max_tokens":  1200,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider status %d", resp.StatusCode)
	}
	jsonBody2 := respBody
	if idx := bytes.Index(jsonBody2, []byte("data: [DONE]")); idx > 0 {
		jsonBody2 = bytes.TrimSpace(jsonBody2[:idx])
	}
	if idx := bytes.Index(jsonBody2, []byte("\ndata: ")); idx > 0 {
		jsonBody2 = bytes.TrimSpace(jsonBody2[:idx])
	}
	var direct llmResponse
	if json.Unmarshal(jsonBody2, &direct) == nil && len(direct.Choices) > 0 {
		return &direct, nil
	}
	if bytes.Contains(respBody, []byte("data: ")) {
		var content strings.Builder
		var usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}
		for _, line := range bytes.Split(respBody, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("data: ")) || bytes.Equal(line, []byte("data: [DONE]")) {
				continue
			}
			var parsed struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
				Usage struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
					TotalTokens      int `json:"total_tokens"`
				} `json:"usage"`
			}
			if json.Unmarshal(bytes.TrimPrefix(line, []byte("data: ")), &parsed) != nil {
				continue
			}
			if len(parsed.Choices) > 0 {
				content.WriteString(parsed.Choices[0].Delta.Content)
				content.WriteString(parsed.Choices[0].Message.Content)
			}
			if parsed.Usage.TotalTokens > 0 {
				usage = parsed.Usage
			}
		}
		return &llmResponse{Choices: []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{{Message: struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{Role: "assistant", Content: content.String()}}}, Usage: usage}, nil
	}
	return nil, fmt.Errorf("failed to parse LLM response: %s", string(respBody[:min(len(respBody), 200)]))
}

var affirmativeWords = map[string]bool{
	"ya": true, "iya": true, "yup": true, "yep": true, "yes": true,
	"ok": true, "oke": true, "okay": true, "lanjut": true, "lanjutkan": true,
	"jalankan": true, "eksekusi": true, "setuju": true, "konfirmasi": true,
	"simpan": true, "save": true,
}

var negativeWords = map[string]bool{
	"tidak": true, "engga": true, "enggak": true, "nggak": true, "ga": true,
	"gak": true, "jangan": true, "batal": true, "batalkan": true, "cancel": true,
	"stop": true, "no": true, "nope": true, "abort": true,
}

func classifyShortReply(msg string) string {
	normalized := strings.ToLower(strings.TrimSpace(msg))
	normalized = strings.TrimRight(normalized, ".!?,")
	if normalized == "" || len(normalized) > 24 {
		return ""
	}
	if affirmativeWords[normalized] {
		return "affirm"
	}
	if negativeWords[normalized] {
		return "deny"
	}
	parts := strings.Fields(normalized)
	if len(parts) > 0 && len(parts) <= 3 {
		if affirmativeWords[parts[0]] {
			return "affirm"
		}
		if negativeWords[parts[0]] {
			return "deny"
		}
	}
	return ""
}

func looksLikeTopicAdvice(message string) bool {
	m := strings.ToLower(message)
	return (strings.Contains(m, "topik") || strings.Contains(m, "materi") || strings.Contains(m, "bab")) &&
		(strings.Contains(m, "tepat") || strings.Contains(m, "cocok") || strings.Contains(m, "sesuai") || strings.Contains(m, "semester") || strings.Contains(m, "kelas") || strings.Contains(m, "rekomendasi") || strings.Contains(m, "sarankan"))
}

func looksLikeFollowupRecommendation(message string) bool {
	m := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(m, "rekomendasi lain") ||
		strings.Contains(m, "ada rekomendasi") ||
		strings.Contains(m, "yang lain") ||
		strings.Contains(m, "lainnya") ||
		strings.Contains(m, "alternatif") ||
		strings.Contains(m, "opsi lain")
}

func looksLikeSequenceSeriesMaterial(message string) bool {
	m := strings.ToLower(message)
	return (strings.Contains(m, "barisan") || strings.Contains(m, "deret")) &&
		(strings.Contains(m, "materi") || strings.Contains(m, "bahas") || strings.Contains(m, "tentang") || strings.Contains(m, "cocok") || strings.Contains(m, "apa saja"))
}

func looksLikeGreeting(message string) bool {
	m := strings.ToLower(strings.TrimSpace(message))
	return m == "hai" || m == "hay" || m == "halo" || m == "hello" || m == "hi"
}

func looksLikeComplaint(message string) bool {
	m := strings.ToLower(message)
	return strings.Contains(m, "ngga nyambung") || strings.Contains(m, "nggak nyambung") || strings.Contains(m, "ga nyambung") || strings.Contains(m, "gak nyambung") || strings.Contains(m, "tidak nyambung") || strings.Contains(m, "work apanya")
}

func (a *App) handleListAISessions(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}

	// Optional ?scope=exam:<uuid>|blueprint:<uuid>|global filter so the
	// chat panel can list only sessions relevant to the page the user
	// is on. Empty scope = all of this user's sessions.
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))

	var (
		rows *sql.Rows
		err  error
	)
	if scope != "" {
		rows, err = a.db.QueryContext(r.Context(),
			`SELECT id, COALESCE(title, ''), COALESCE(summary, ''), message_count, last_active_at, COALESCE(scope_key,'')
			   FROM ai_sessions
			  WHERE user_id = $1 AND scope_key = $2
			  ORDER BY last_active_at DESC LIMIT 20`,
			auth.UserID, scope,
		)
	} else {
		rows, err = a.db.QueryContext(r.Context(),
			`SELECT id, COALESCE(title, ''), COALESCE(summary, ''), message_count, last_active_at, COALESCE(scope_key,'')
			   FROM ai_sessions WHERE user_id = $1 ORDER BY last_active_at DESC LIMIT 20`,
			auth.UserID,
		)
	}
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "lookup_failed", "Could not load sessions", r)
		return
	}
	defer rows.Close()

	type SessionRow struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		Summary      string `json:"summary"`
		MessageCount int    `json:"messageCount"`
		LastActiveAt string `json:"lastActiveAt"`
		ScopeKey     string `json:"scopeKey"`
	}
	var sessions []SessionRow
	for rows.Next() {
		var s SessionRow
		if err := rows.Scan(&s.ID, &s.Title, &s.Summary, &s.MessageCount, &s.LastActiveAt, &s.ScopeKey); err == nil {
			sessions = append(sessions, s)
		}
	}
	if sessions == nil {
		sessions = []SessionRow{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": sessions})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// handleAICancel cancels a pending proposal
func (a *App) handleAICancel(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	var req struct {
		ProposalID string `json:"proposalId"`
	}
	if err := readJSON(r, &req); err != nil || req.ProposalID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "proposalId is required", r)
		return
	}

	_, _ = a.db.ExecContext(r.Context(), `UPDATE ai_pending_actions SET status = 'cancelled' WHERE id = $1 AND user_id = $2 AND status = 'pending'`, req.ProposalID, auth.UserID)

	writeJSON(w, http.StatusOK, map[string]any{"status": "cancelled"})
}

// handleGetAISessionMessages returns messages for a session
func (a *App) handleGetAISessionMessages(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}

	sessionID := r.PathValue("id")
	if sessionID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Session ID is required", r)
		return
	}

	// Verify session belongs to user
	var ownerID string
	err := a.db.QueryRowContext(r.Context(), `SELECT user_id FROM ai_sessions WHERE id = $1`, sessionID).Scan(&ownerID)
	if err != nil || ownerID != auth.UserID {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Session not found", r)
		return
	}

	rows, err := a.db.QueryContext(r.Context(),
		`SELECT role, content, created_at FROM ai_messages WHERE session_id = $1 AND role IN ('user', 'assistant') ORDER BY created_at ASC`,
		sessionID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "lookup_failed", "Could not load messages", r)
		return
	}
	defer rows.Close()

	type MessageRow struct {
		Role      string `json:"role"`
		Content   string `json:"content"`
		CreatedAt string `json:"createdAt"`
	}
	var messages []MessageRow
	for rows.Next() {
		var m MessageRow
		if err := rows.Scan(&m.Role, &m.Content, &m.CreatedAt); err == nil {
			messages = append(messages, m)
		}
	}
	if messages == nil {
		messages = []MessageRow{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": messages, "sessionId": sessionID})
}

// handleDeleteAISession hard-deletes a chat session owned by the
// caller. ON DELETE CASCADE on ai_messages, ai_pending_actions, and
// ai_task_states cleans up the conversation tree atomically. Used by
// the 'clear chat history' affordance in the AI panel: the user
// wants to start a fresh conversation without prior context bleeding
// into model prompts.
//
// Authorization: caller must own the session (user_id match). Other
// users' sessions — including admins — are 404'd silently to avoid
// leaking session existence.
func (a *App) handleDeleteAISession(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	sessionID := r.PathValue("id")
	if sessionID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Session ID is required", r)
		return
	}

	res, err := a.db.ExecContext(r.Context(),
		`DELETE FROM ai_sessions WHERE id = $1 AND user_id = $2`,
		sessionID, auth.UserID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "delete_failed", "Could not delete session", r)
		return
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Session not found", r)
		return
	}

	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID != "" {
		a.audit(r.Context(), &tenantID, auth.UserID, "ai.session.delete", "ai_session", sessionID, r)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted", "id": sessionID})
}
