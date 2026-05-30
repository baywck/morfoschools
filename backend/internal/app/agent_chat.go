package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
)

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
	// If the client does not supply a session id, always start a fresh
	// conversation. Do not resurrect the most recent scoped session here:
	// after the user clears chat history, frontend intentionally removes
	// its local session id, and reusing an older scoped session makes old
	// messages appear again after reload.
	var id string
	title := "Chat"
	if strings.HasPrefix(scopeKey, "exam:") {
		title = "Exam chat"
	}
	err := a.db.QueryRowContext(ctx,
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
	req.SessionID = sessionID
	_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content) VALUES ($1, 'user', $2)`, sessionID, req.Message)
	if a.tryConfirmLatestAgentProposalFromChat(w, r, tenantID, auth.UserID, sessionID, req.Message) {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE ai_sessions SET message_count = message_count + 2, last_active_at = now() WHERE id=$1`, sessionID)
		return
	}
	if a.tryCreateAgentProposalFromIntent(w, r, tenantID, auth.UserID, sessionID, req) {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE ai_sessions SET message_count = message_count + 2, last_active_at = now() WHERE id=$1`, sessionID)
		return
	}
	content, totalTokens, err := a.callDiscussionLLM(r.Context(), sessionID, tenantID, auth.UserID, auth.Roles, req.Shadow.ActiveEntities, req.Message)
	if err != nil {
		a.logger.Error("discussion llm failed", "error", err)
		writeErrorJSON(w, http.StatusBadGateway, "ai_error", "AI service unavailable", r)
		return
	}
	// Reaching discussion means this turn was neither a confirm nor a new
	// proposal. Any still-pending proposal is now stale relative to the
	// ongoing conversation, so expire it. This prevents a later "simpan"
	// from silently confirming an unrelated proposal.
	_, _ = a.db.ExecContext(r.Context(), `UPDATE agent_proposals SET status='expired' WHERE session_id=$1 AND status='pending'`, sessionID)
	_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
	a.rememberAssistantBlueprintDraft(r.Context(), tenantID, sessionID, scopeKey, content)
	_, _ = a.db.ExecContext(r.Context(), `UPDATE ai_sessions SET message_count = message_count + 2, last_active_at = now() WHERE id=$1`, sessionID)
	writeJSON(w, http.StatusOK, map[string]any{
		"message":   map[string]string{"role": "assistant", "content": content},
		"sessionId": sessionID,
		"tokens":    totalTokens,
	})
}

func (a *App) callDiscussionLLM(ctx context.Context, sessionID, tenantID, userID string, roles []string, active map[string]string, userMessage string) (string, int, error) {
	pack := a.buildAgentContextPack(ctx, tenantID, sessionID, active, userMessage)
	messages := []llmMessage{
		{Role: "system", Content: a.discussionSystemPrompt(ctx, tenantID, active)},
		{Role: "system", Content: "AgentContextPack JSON (gunakan sebagai memori kerja dan konteks nyata; jangan abaikan draft/keputusan sebelumnya): " + mustJSON(pack)},
	}
	for _, m := range pack.Recent {
		if len(pack.Blueprint.Slots) > 0 && m.Role == "assistant" && staleBlueprintContextClaim(m.Content) {
			continue
		}
		messages = append(messages, llmMessage{Role: m.Role, Content: m.Content})
	}
	messages = append(messages, llmMessage{Role: "system", Content: "Current AgentContextPack JSON di atas adalah sumber kebenaran terbaru dan mengalahkan recentMessages. Jika recentMessages berisi klaim lama seperti existingSlotCount=0, tidak punya akses langsung, data tidak termuat, atau meminta user menyalin slot, abaikan sebagai stale/invalid. Untuk slot kisi-kisi aktif, gunakan current AgentContextPack.blueprint.slots."})
	if len(pack.Recent) == 0 || pack.Recent[len(pack.Recent)-1].Role != "user" || strings.TrimSpace(pack.Recent[len(pack.Recent)-1].Content) != strings.TrimSpace(userMessage) {
		messages = append(messages, llmMessage{Role: "user", Content: userMessage})
	}
	provider, err := a.resolveAIProvider(ctx, &AuthContext{UserID: userID, Roles: roles, EffectiveTenantID: &tenantID}, tenantID)
	if err != nil {
		return "", 0, err
	}
	resp, err := a.callLLMWithProviderOptions(ctx, provider, messages, 0.35, 5000, nil)
	if err != nil {
		return "", 0, err
	}
	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		return "", resp.Usage.TotalTokens, fmt.Errorf("empty LLM response")
	}
	out := strings.TrimSpace(resp.Choices[0].Message.Content)
	return out, resp.Usage.TotalTokens, nil
}

func staleBlueprintContextClaim(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "existingslotcount: 0") ||
		strings.Contains(lower, "tidak memiliki akses langsung") ||
		strings.Contains(lower, "tidak punya akses langsung") ||
		strings.Contains(lower, "data tidak termuat") ||
		strings.Contains(lower, "tampilkan dulu slot") ||
		strings.Contains(lower, "mengetikkan data slot") ||
		strings.Contains(lower, "menyalin data slot")
}

func (a *App) discussionSystemPrompt(ctx context.Context, tenantID string, active map[string]string) string {
	var b strings.Builder
	b.WriteString("Kamu adalah asisten Morfoschools. Jawab natural dalam Bahasa Indonesia. ")
	b.WriteString("Jika permintaan user sudah didukung workflow sistem, backend akan membuat proposal sebelum pesan ini dipakai. Jika pesan sampai ke mode diskusi ini, berarti permintaan dianggap diskusi/unsupported. ")
	b.WriteString("Jangan mengklaim sudah membuat, menyimpan, mengubah, atau menghapus data. Untuk aksi penyimpanan, sistem mendukung proposal workflow: user bisa mengetik 'simpan', 'buatkan proposal', atau konfirmasi lainnya untuk membuat proposal yang bisa dikonfirmasi. Jangan pernah mengklaim fitur tidak tersedia jika fitur tersebut memang didukung oleh sistem. ")
	b.WriteString("Untuk semua diskusi kisi-kisi/blueprint/soal sekolah Indonesia, WAJIB gunakan Kurikulum Merdeka: CP, Elemen CP, TP, materi, indikator soal berbasis stimulus, level kognitif C1-C6. DILARANG menyarankan KD, SK, Kompetensi Dasar, Standar Kompetensi, atau K13 sebagai default. Jika user menyebut KD/K13, jelaskan bahwa workspace ini memakai standar Kurikulum Merdeka kecuali user eksplisit memilih kurikulum lama. TP wajib mengikuti prinsip A-B-C-D: Audience 'Peserta didik', Behavior berupa satu KKO terukur, Condition/konteks pembelajaran, dan Degree/kriteria eksplisit seperti 'minimal dua', 'dengan tepat', atau 'berdasarkan prinsip/kriteria'. KKO TP harus selaras dengan level kognitif dan indikator. Indikator wajib berbasis stimulus dan satu indikator hanya untuk satu soal. ")
	b.WriteString("Chat panel sempit: JANGAN gunakan markdown table kecuali user eksplisit meminta tabel. Untuk kisi-kisi/blueprint/soal, gunakan format daftar bernomor atau compact cards. Setiap slot maksimal 4 baris: baris 1 Elemen · Level · Bentuk, baris 2 Materi, baris 3 TP ringkas, baris 4 Indikator berbasis stimulus. Batasi contoh awal 3-5 item kecuali user meminta lebih. Jangan pernah menyebut draft diskusi sebagai 'proposal' atau mengklaim backend/sistem akan memproses sesuatu. Untuk aksi, minta user mengetik perintah eksplisit: 'buatkan proposal 5 slot pertama'. ")
	b.WriteString("Gunakan AgentContextPack sebagai working memory nyata. Jika AgentContextPack.blueprint.requestedSlots berisi data, itu adalah hasil fetch DB khusus untuk nomor/range yang user sebut pada turn ini dan WAJIB diprioritaskan. Jika AgentContextPack.blueprint.slots berisi slot, kamu BOLEH dan WAJIB membaca slot existing dari sana untuk audit, analisis, dan menjawab pertanyaan tentang nomor/nomer/no slot. Jika AgentContextPack.memory.blueprintDrafts berisi draft, itu adalah draft slot yang sudah kamu hasilkan sebelumnya dan WAJIB digunakan saat user meminta review/audit/perbaikan draft. Jangan pernah berkata 'saya tidak memiliki akses langsung', 'data tidak termuat dalam konteks', 'tampilkan dulu slot', atau meminta user menyalin data slot yang sudah ada di context. Dalam halaman kisi-kisi, kata 'nomor', 'nomer', 'no', atau angka range seperti '16-20' berarti posisi slot kisi-kisi aktif jika konteksnya membahas kisi-kisi. Jika user meminta memperbaiki/mengubah slot existing, jelaskan singkat bahwa perubahan harus lewat proposal edit slot; jangan membuat draft seolah akan disimpan otomatis dan jangan arahkan ke create slot baru. ")
	if examID := strings.TrimSpace(active["examId"]); examID != "" && a.db != nil {
		ctxResp, err := a.ensureExamCurriculumContext(ctx, tenantID, examID)
		if err == nil {
			b.WriteString("Konteks halaman aktif: ")
			b.WriteString("examId=\"")
			b.WriteString(examID)
			b.WriteString("\". ")
			if ctxResp.SubjectName != "" {
				b.WriteString("mapel=\"")
				b.WriteString(ctxResp.SubjectName)
				b.WriteString("\". ")
			}
			if ctxResp.GradeLevel != "" {
				b.WriteString("kelas=\"")
				b.WriteString(ctxResp.GradeLevel)
				b.WriteString("\". ")
			}
			if ctxResp.Phase != "" {
				b.WriteString("fase=\"")
				b.WriteString(ctxResp.Phase)
				b.WriteString("\". ")
			}
			b.WriteString("CP context status=")
			b.WriteString(ctxResp.Status)
			b.WriteString(" source=")
			b.WriteString(ctxResp.Source)
			b.WriteString(". ")
			for _, warning := range ctxResp.Warnings {
				b.WriteString("Peringatan CP: ")
				b.WriteString(warning)
				b.WriteString(" ")
			}
			if ctxResp.Reference != nil {
				b.WriteString("CP umum ringkas: ")
				b.WriteString(truncateForPrompt(ctxResp.Reference.GeneralCP, 2200))
				b.WriteString(" Elemen CP tersedia: ")
				for _, el := range ctxResp.Elements {
					b.WriteString(el.Name)
					b.WriteString(" — ")
					b.WriteString(truncateForPrompt(el.Content, 700))
					b.WriteString(" | ")
				}
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
	provider, err := a.resolveAIProvider(ctx, nil, "")
	if err != nil {
		return nil, err
	}
	return a.callLLMWithProvider(ctx, provider, messages)
}

var affirmativeWords = map[string]bool{
	"ya": true, "iya": true, "yup": true, "yep": true, "yes": true,
	"ok": true, "oke": true, "okay": true, "lanjut": true, "lanjutkan": true,
	"jalankan": true, "eksekusi": true, "lakukan": true, "setuju": true, "konfirmasi": true,
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
	if normalized == "" {
		return ""
	}
	if isNaturalApprovalReply(normalized) {
		return "affirm"
	}
	if len(normalized) > 24 {
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

func isNaturalApprovalReply(normalized string) bool {
	phrases := []string{
		"tidak ada yang perlu dirubah", "tidak ada yang perlu diubah", "nggak ada yang perlu diubah", "ga ada yang perlu diubah",
		"sudah sesuai", "sudah oke", "sudah ok", "sudah benar", "boleh simpan", "oke simpan", "ok simpan",
	}
	for _, phrase := range phrases {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}
	return false
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
