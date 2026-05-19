package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// AI Chat endpoint — orchestrates LLM + tools + memory

const (
	maxToolLoops    = 10
	maxRecentMsgs   = 12
	maxSummaryChars = 500
)

// affirmativeRe matches short free-text confirmation replies in Indonesian and
// English. We keep it intentionally tight so phrases like "oke tapi ganti
// emailnya" do NOT auto-execute (that's a follow-up instruction, not consent).
var affirmativeWords = map[string]bool{
	"ya": true, "iya": true, "yup": true, "yep": true, "yes": true,
	"ok": true, "oke": true, "okay": true, "okeh": true,
	"lanjut": true, "lanjutkan": true, "jalankan": true, "eksekusi": true,
	"setuju": true, "konfirmasi": true, "benar": true, "betul": true,
	"sip": true, "siap": true, "go": true, "do it": true, "proceed": true, "confirm": true,
}

var negativeWords = map[string]bool{
	"tidak": true, "engga": true, "enggak": true, "nggak": true, "ga": true, "gak": true,
	"jangan": true, "batal": true, "batalkan": true, "cancel": true, "stop": true,
	"no": true, "nope": true, "abort": true,
}

// classifyShortReply returns "affirm", "deny", or "". Only fires for messages
// short enough to plausibly be a one-shot confirmation; anything longer is
// treated as a fresh instruction so the LLM can route it normally.
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
	// Multi-word: check first word only (e.g. "oke lanjutkan")
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

// pendingProposalForSession returns up to N pending proposals on the session,
// oldest first. Used to resolve free-text confirmation replies.
func (a *App) pendingProposalsForSession(ctx context.Context, sessionID, userID string) []struct {
	ID       string
	ToolName string
	ToolArgs json.RawMessage
	TenantID string
	Confirm  string
} {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, tool_name, tool_args, COALESCE(tenant_id::text,''), confirmation_text
		  FROM ai_pending_actions
		 WHERE session_id = $1 AND user_id = $2
		   AND status = 'pending' AND expires_at > now()
		 ORDER BY created_at ASC
		 LIMIT 10`,
		sessionID, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []struct {
		ID       string
		ToolName string
		ToolArgs json.RawMessage
		TenantID string
		Confirm  string
	}
	for rows.Next() {
		var p struct {
			ID       string
			ToolName string
			ToolArgs json.RawMessage
			TenantID string
			Confirm  string
		}
		if err := rows.Scan(&p.ID, &p.ToolName, &p.ToolArgs, &p.TenantID, &p.Confirm); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func (a *App) registerAIChatRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/ai/chat", a.handleAIChat)
	mux.HandleFunc("POST /api/v1/ai/confirm", a.handleAIConfirm)
	mux.HandleFunc("POST /api/v1/ai/cancel", a.handleAICancel)
	mux.HandleFunc("GET /api/v1/ai/sessions", a.handleListAISessions)
	mux.HandleFunc("GET /api/v1/ai/sessions/{id}/messages", a.handleGetAISessionMessages)
}

type aiChatRequest struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
	// Shadow state from frontend
	Shadow struct {
		Route          string `json:"route"`
		ActiveEntities map[string]string `json:"activeEntities"`
	} `json:"shadow"`
}

type llmMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content,omitempty"`
	ToolCalls  json.RawMessage `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type llmToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type llmResponse struct {
	Choices []struct {
		Message struct {
			Role      string          `json:"role"`
			Content   string          `json:"content"`
			ToolCalls json.RawMessage `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// handleShortReplyForProposals executes (affirm) or cancels (deny) all
// matching pending proposals and writes a normal chat-style response. Returns
// true when the request was fully handled and the caller should return.
func (a *App) handleShortReplyForProposals(
	w http.ResponseWriter, r *http.Request, sessionID, userID, intent string,
	pendings []struct {
		ID       string
		ToolName string
		ToolArgs json.RawMessage
		TenantID string
		Confirm  string
	},
) bool {
	ctx := r.Context()

	if intent == "deny" {
		for _, p := range pendings {
			_, _ = a.db.ExecContext(ctx,
				`UPDATE ai_pending_actions SET status = 'cancelled' WHERE id = $1 AND status = 'pending'`,
				p.ID)
		}
		msg := fmt.Sprintf("Baik, %d aksi dibatalkan. Apa yang ingin Anda lakukan selanjutnya?", len(pendings))
		_, _ = a.db.ExecContext(ctx,
			`INSERT INTO ai_messages (session_id, role, content) VALUES ($1, 'assistant', $2)`,
			sessionID, msg)
		writeJSON(w, http.StatusOK, map[string]any{
			"message":   map[string]string{"role": "assistant", "content": msg},
			"sessionId": sessionID,
			"tokens":    0,
			"cancelled": true,
		})
		return true
	}

	// affirm: execute each proposal, collect results
	var successes, failures []string
	var lastErr string
	for _, p := range pendings {
		result, err := a.executeConfirmedAction(ctx, p.TenantID, userID, p.ToolName, p.ToolArgs)
		if err != nil {
			a.logger.Error("short-reply execute failed", "tool", p.ToolName, "error", err)
			failures = append(failures, p.Confirm)
			lastErr = err.Error()
			_, _ = a.db.ExecContext(ctx,
				`UPDATE ai_pending_actions SET status = 'cancelled' WHERE id = $1`, p.ID)
			continue
		}
		_, _ = a.db.ExecContext(ctx,
			`UPDATE ai_pending_actions SET status = 'confirmed' WHERE id = $1`, p.ID)
		successes = append(successes, summarizeToolResult(p.ToolName, result))
	}

	// Build a friendly summary in Indonesian
	var sb strings.Builder
	switch {
	case len(failures) == 0:
		sb.WriteString(fmt.Sprintf("✅ Berhasil! %d aksi dijalankan:\n", len(successes)))
		for _, s := range successes {
			sb.WriteString("• " + s + "\n")
		}
	case len(successes) == 0:
		sb.WriteString(fmt.Sprintf("❌ Gagal menjalankan %d aksi.", len(failures)))
		if lastErr != "" {
			sb.WriteString(" Penyebab: " + lastErr)
		}
	default:
		sb.WriteString(fmt.Sprintf("⚠️ %d aksi berhasil, %d gagal.\n", len(successes), len(failures)))
		for _, s := range successes {
			sb.WriteString("• " + s + "\n")
		}
	}

	content := sb.String()
	_, _ = a.db.ExecContext(ctx,
		`INSERT INTO ai_messages (session_id, role, content) VALUES ($1, 'assistant', $2)`,
		sessionID, content)
	_, _ = a.db.ExecContext(ctx,
		`UPDATE ai_sessions SET message_count = message_count + 1, last_active_at = now() WHERE id = $1`,
		sessionID)

	writeJSON(w, http.StatusOK, map[string]any{
		"message":   map[string]string{"role": "assistant", "content": content},
		"sessionId": sessionID,
		"tokens":    0,
		"mutated":   len(successes) > 0,
	})
	return true
}

// summarizeToolResult extracts a short success message from a tool's JSON
// result. Falls back to the raw result if the structure is unexpected.
func summarizeToolResult(toolName, result string) string {
	var parsed struct {
		Message string `json:"message"`
		Success bool   `json:"success"`
	}
	if err := json.Unmarshal([]byte(result), &parsed); err == nil && parsed.Message != "" {
		return parsed.Message
	}
	return toolName
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
	if strings.TrimSpace(req.Message) == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Message is required", r)
		return
	}

	tenantID := ""
	if auth.EffectiveTenantID != nil {
		tenantID = *auth.EffectiveTenantID
	}

	// Get or create session
	sessionID := req.SessionID
	if sessionID == "" {
		err := a.db.QueryRowContext(r.Context(),
			`INSERT INTO ai_sessions (tenant_id, user_id) VALUES (NULLIF($1,'')::uuid, $2) RETURNING id`,
			tenantID, auth.UserID,
		).Scan(&sessionID)
		if err != nil {
			a.logger.Error("create ai session failed", "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "ai_error", "Could not create session", r)
			return
		}
	}

	// Store user message
	_, _ = a.db.ExecContext(r.Context(),
		`INSERT INTO ai_messages (session_id, role, content) VALUES ($1, 'user', $2)`,
		sessionID, req.Message,
	)
	_, _ = a.db.ExecContext(r.Context(),
		`UPDATE ai_sessions SET message_count = message_count + 1, last_active_at = now() WHERE id = $1`,
		sessionID,
	)

	// Short-circuit: if there are pending proposals AND the user typed a short
	// affirmative/deny reply, resolve them directly without burning an LLM
	// round-trip. Free-text confirmation is what users actually do; the
	// inline buttons only catch the click path.
	if intent := classifyShortReply(req.Message); intent != "" {
		pendings := a.pendingProposalsForSession(r.Context(), sessionID, auth.UserID)
		if len(pendings) > 0 {
			if a.handleShortReplyForProposals(w, r, sessionID, auth.UserID, intent, pendings) {
				return
			}
		}
	}

	// Assemble context
	messages := a.assembleContext(r.Context(), sessionID, tenantID, auth, req)

	// Detect relevant domains from user message
	domains := DetectDomains(req.Message)

	// Get capabilities for detected domains + user permissions
	tools := a.capRegistry.GetToolsForIntent(domains, auth.Permissions)

	// LLM call loop (tool calls may require multiple rounds)
	var finalContent string
	var totalTokens int

	// Pass session ID via context for write tools
	ctxWithSession := context.WithValue(r.Context(), ctxKeySessionID{}, sessionID)

	for i := 0; i < maxToolLoops; i++ {
		llmResp, err := a.callLLM(ctxWithSession, messages, tools)
		if err != nil {
			a.logger.Error("llm call failed", "error", err)
			writeErrorJSON(w, http.StatusBadGateway, "ai_error", "AI service unavailable", r)
			return
		}
		totalTokens += llmResp.Usage.TotalTokens

		if len(llmResp.Choices) == 0 {
			writeErrorJSON(w, http.StatusBadGateway, "ai_error", "Empty AI response", r)
			return
		}

		choice := llmResp.Choices[0]

		// If no tool calls, we have the final response
		if choice.FinishReason != "tool_calls" || len(choice.Message.ToolCalls) == 0 {
			finalContent = choice.Message.Content
			break
		}

		// Parse and execute tool calls
		var toolCalls []llmToolCall
		if err := json.Unmarshal(choice.Message.ToolCalls, &toolCalls); err != nil {
			finalContent = choice.Message.Content
			break
		}

		// Add assistant message with tool_calls to context
		messages = append(messages, llmMessage{
			Role:      "assistant",
			ToolCalls: choice.Message.ToolCalls,
		})

		// Execute each tool and add results
		var hasProposal bool
		for _, tc := range toolCalls {
			var resultContent string
			if handler, ok := a.capRegistry.handlers[tc.Function.Name]; ok {
				resultContent, _ = handler(ctxWithSession, tenantID, auth.UserID, json.RawMessage(tc.Function.Arguments))
			} else {
				oldResult, _ := a.toolRegistry.Execute(ctxWithSession, tenantID, auth.UserID, tc.Function.Name, tc.Function.Arguments)
				resultContent = oldResult.Content
			}
			messages = append(messages, llmMessage{
				Role:       "tool",
				Content:    resultContent,
				ToolCallID: tc.ID,
			})
			if strings.Contains(resultContent, "confirmation_required") {
				hasProposal = true
			}
		}

		// If a write tool returned a proposal, stop the loop and let LLM explain
		if hasProposal {
			continue
		}
	}

	// Check if any tool results contained proposals
	var pendingProposals []ActionProposal
	for _, msg := range messages {
		if msg.Role == "tool" && strings.Contains(msg.Content, "confirmation_required") {
			var parsed struct {
				Type     string         `json:"type"`
				Proposal ActionProposal `json:"proposal"`
			}
			if json.Unmarshal([]byte(msg.Content), &parsed) == nil && parsed.Type == "confirmation_required" {
				pendingProposals = append(pendingProposals, parsed.Proposal)
			}
		}
	}

	if finalContent == "" {
		// If we have proposals, generate a summary
		if len(pendingProposals) > 0 {
			var parts []string
			for _, p := range pendingProposals {
				parts = append(parts, p.ConfirmationText)
			}
			finalContent = "Saya telah menyiapkan aksi berikut untuk dikonfirmasi:\n\n" + strings.Join(parts, "\n")
		} else {
			// Last-resort fallback: surface any pre-existing pending proposals
			// so the user is reminded of what's awaiting their decision rather
			// than seeing a dead-end "cannot process" message.
			existing := a.pendingProposalsForSession(r.Context(), sessionID, auth.UserID)
			if len(existing) > 0 {
				var parts []string
				for _, p := range existing {
					parts = append(parts, "• "+p.Confirm)
				}
				finalContent = "Masih ada aksi menunggu konfirmasi:\n\n" + strings.Join(parts, "\n") +
					"\n\nKetik **\"ya\"** untuk lanjutkan atau **\"batal\"** untuk membatalkan."
			} else {
				finalContent = "Maaf, saya tidak bisa memproses permintaan ini saat ini."
			}
		}
	}

	// Store assistant response
	_, _ = a.db.ExecContext(r.Context(),
		`INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, $3)`,
		sessionID, finalContent, totalTokens,
	)
	_, _ = a.db.ExecContext(r.Context(),
		`UPDATE ai_sessions SET message_count = message_count + 1, last_active_at = now() WHERE id = $1`,
		sessionID,
	)

	// Trigger summarization if needed (async, non-blocking)
	go a.maybeSummarize(sessionID)

	response := map[string]any{
		"message":   map[string]string{"role": "assistant", "content": finalContent},
		"sessionId": sessionID,
		"tokens":    totalTokens,
	}
	if len(pendingProposals) == 1 {
		response["proposal"] = pendingProposals[0]
	} else if len(pendingProposals) > 1 {
		response["proposals"] = pendingProposals
	}

	// Detect if any tool call resulted in a successful mutation (for frontend refresh)
	if len(pendingProposals) == 0 {
		for _, msg := range messages {
			if msg.Role == "tool" && strings.Contains(msg.Content, `"success":true`) {
				response["mutated"] = true
				break
			}
		}
	}

	writeJSON(w, http.StatusOK, response)
}

// assembleContext builds the message array for the LLM
func (a *App) assembleContext(ctx context.Context, sessionID, tenantID string, auth *AuthContext, req aiChatRequest) []llmMessage {
	var messages []llmMessage

	// 1. System prompt with shadow state
	systemPrompt := a.buildSystemPrompt(tenantID, auth, req)
	messages = append(messages, llmMessage{Role: "system", Content: systemPrompt})

	// 2. Session summary (if exists)
	var summary string
	_ = a.db.QueryRowContext(ctx, `SELECT COALESCE(summary, '') FROM ai_sessions WHERE id = $1`, sessionID).Scan(&summary)
	if summary != "" {
		messages = append(messages, llmMessage{Role: "system", Content: "Ringkasan percakapan sebelumnya: " + summary})
	}

	// 3. Recent messages (user + assistant only, from DB, chronological)
	rows, err := a.db.QueryContext(ctx,
		`SELECT role, content FROM ai_messages WHERE session_id = $1 AND role IN ('user', 'assistant') ORDER BY created_at DESC LIMIT $2`,
		sessionID, maxRecentMsgs,
	)
	if err == nil {
		defer rows.Close()
		var recent []llmMessage
		for rows.Next() {
			var m llmMessage
			if err := rows.Scan(&m.Role, &m.Content); err == nil && m.Content != "" {
				recent = append(recent, m)
			}
		}
		// Reverse to chronological order
		for i, j := 0, len(recent)-1; i < j; i, j = i+1, j-1 {
			recent[i], recent[j] = recent[j], recent[i]
		}
		messages = append(messages, recent...)
	}

	return messages
}

func (a *App) buildSystemPrompt(tenantID string, auth *AuthContext, req aiChatRequest) string {
	var sb strings.Builder
	sb.WriteString("Kamu asisten AI Morfoschools. Jawab ringkas dalam Bahasa Indonesia.\n")
	sb.WriteString("Gunakan tools untuk data aktual — jangan mengarang.\n")
	sb.WriteString("Jika info kurang, tanya user — jangan menyerah.\n")
	sb.WriteString("Untuk batch (buat banyak item), eksekusi satu per satu.\n")
	sb.WriteString("Selalu lookup data terkait sebelum create (cari UUID via search tool).\n")
	sb.WriteString("SEBELUM batch-create (>1 item dari jenis sama), WAJIB panggil list_* atau search_* dulu untuk lihat data existing. JANGAN mengusulkan nama/email/kode yang sudah ada.\n")
	sb.WriteString("Jika user bilang \"lanjutkan\", teruskan task sebelumnya dari context.\n")
	sb.WriteString("JANGAN PERNAH klaim aksi berhasil tanpa tool result yang mengkonfirmasi.\n")
	sb.WriteString("Setelah propose action, jika user balas \"ya/oke/lanjut/setuju\" → sistem auto-eksekusi proposal. Jika balas \"tidak/batal\" → auto-cancel. Kamu tidak perlu memanggil tool lagi untuk konfirmasi tersebut.\n")
	// Self-correction protocol
	sb.WriteString("\nON TOOL ERROR:\n")
	sb.WriteString("1. Baca error.recovery — panggil tool yang disarankan — retry aksi awal (DIAM, jangan beritahu user)\n")
	sb.WriteString("2. Jika gagal lagi pada aksi sama — tanya user pertanyaan spesifik\n")
	sb.WriteString("3. Jika gagal 3x — minta maaf + jelaskan kenapa\n")
	sb.WriteString("JANGAN: tampilkan error code ke user, retry >2x error sama, tebak tanpa data\n")

	// Compact user context
	sb.WriteString(fmt.Sprintf("\nUser: %s | Role: %s", auth.DisplayName, strings.Join(auth.Roles, ",")))
	if tenantID != "" {
		sb.WriteString(" | Tenant: aktif")
	}
	if req.Shadow.Route != "" {
		sb.WriteString(fmt.Sprintf(" | Page: %s", req.Shadow.Route))
	}
	sb.WriteString("\n")

	// User facts (compact)
	rows, _ := a.db.QueryContext(context.Background(),
		`SELECT fact_key, fact_value FROM ai_user_facts WHERE user_id = $1 LIMIT 5`,
		auth.UserID,
	)
	if rows != nil {
		defer rows.Close()
		var facts []string
		for rows.Next() {
			var k, v string
			if rows.Scan(&k, &v) == nil {
				facts = append(facts, k+":"+v)
			}
		}
		if len(facts) > 0 {
			sb.WriteString("Facts: " + strings.Join(facts, "; ") + "\n")
		}
	}

	return sb.String()
}

func (a *App) callLLM(ctx context.Context, messages []llmMessage, tools []map[string]any) (*llmResponse, error) {
	baseURL := os.Getenv("AI_BASE_URL")
	apiKey := os.Getenv("AI_API_KEY")
	model := os.Getenv("AI_MODEL")
	if model == "" {
		model = "MORFOSCHOOLS"
	}

	body := map[string]any{
		"model":       model,
		"messages":    messages,
		"temperature": 0.3,
		"max_tokens":  1200,
	}
	if len(tools) > 0 {
		body["tools"] = tools
		body["tool_choice"] = "auto"
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

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse SSE streaming response — accumulate chunks into final message
	if bytes.Contains(respBody, []byte("data: ")) {
		var finalContent strings.Builder
		var toolCalls json.RawMessage
		var finishReason string
		var usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}

		lines := bytes.Split(respBody, []byte("\n"))
		for _, line := range lines {
			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("data: ")) || bytes.Equal(line, []byte("data: [DONE]")) {
				continue
			}
			chunk := bytes.TrimPrefix(line, []byte("data: "))

			var parsed struct {
				Choices []struct {
					Delta struct {
						Content   string          `json:"content"`
						ToolCalls json.RawMessage `json:"tool_calls"`
					} `json:"delta"`
					Message struct {
						Content   string          `json:"content"`
						ToolCalls json.RawMessage `json:"tool_calls"`
					} `json:"message"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
				Usage struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
					TotalTokens      int `json:"total_tokens"`
				} `json:"usage"`
			}
			if json.Unmarshal(chunk, &parsed) != nil {
				continue
			}

			if len(parsed.Choices) > 0 {
				c := parsed.Choices[0]
				if c.Delta.Content != "" {
					finalContent.WriteString(c.Delta.Content)
				}
				if c.Message.Content != "" {
					finalContent.WriteString(c.Message.Content)
				}
				if len(c.Delta.ToolCalls) > 0 && string(c.Delta.ToolCalls) != "null" {
					toolCalls = c.Delta.ToolCalls
				}
				if len(c.Message.ToolCalls) > 0 && string(c.Message.ToolCalls) != "null" {
					toolCalls = c.Message.ToolCalls
				}
				if c.FinishReason != "" {
					finishReason = c.FinishReason
				}
			}
			if parsed.Usage.TotalTokens > 0 {
				usage = parsed.Usage
			}
		}

		return &llmResponse{
			Choices: []struct {
				Message struct {
					Role      string          `json:"role"`
					Content   string          `json:"content"`
					ToolCalls json.RawMessage `json:"tool_calls"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Message: struct {
						Role      string          `json:"role"`
						Content   string          `json:"content"`
						ToolCalls json.RawMessage `json:"tool_calls"`
					}{
						Role:      "assistant",
						Content:   finalContent.String(),
						ToolCalls: toolCalls,
					},
					FinishReason: finishReason,
				},
			},
			Usage: usage,
		}, nil
	}

	// Non-streaming fallback
	var llmResp llmResponse
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w (body: %s)", err, string(respBody[:min(len(respBody), 200)]))
	}

	return &llmResp, nil
}

// maybeSummarize checks if session needs summarization
func (a *App) maybeSummarize(sessionID string) {
	var msgCount int
	_ = a.db.QueryRowContext(context.Background(),
		`SELECT message_count FROM ai_sessions WHERE id = $1`, sessionID,
	).Scan(&msgCount)

	// Summarize every 10 messages
	if msgCount > 0 && msgCount%10 == 0 {
		a.summarizeSession(sessionID)
	}
}

func (a *App) summarizeSession(sessionID string) {
	rows, err := a.db.QueryContext(context.Background(),
		`SELECT role, content FROM ai_messages WHERE session_id = $1 ORDER BY created_at LIMIT 20`,
		sessionID,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	var conversation strings.Builder
	for rows.Next() {
		var role, content string
		if rows.Scan(&role, &content) == nil {
			conversation.WriteString(role + ": " + content + "\n")
		}
	}

	if conversation.Len() == 0 {
		return
	}

	// Call LLM to summarize
	messages := []llmMessage{
		{Role: "system", Content: "Ringkas percakapan berikut dalam 2-3 kalimat Bahasa Indonesia. Fokus pada topik utama dan keputusan yang dibuat."},
		{Role: "user", Content: conversation.String()},
	}

	resp, err := a.callLLM(context.Background(), messages, nil)
	if err != nil || len(resp.Choices) == 0 {
		return
	}

	summary := resp.Choices[0].Message.Content
	if len(summary) > maxSummaryChars {
		summary = summary[:maxSummaryChars]
	}

	_, _ = a.db.ExecContext(context.Background(),
		`UPDATE ai_sessions SET summary = $1 WHERE id = $2`,
		summary, sessionID,
	)
}

// handleListAISessions returns recent sessions for the user
func (a *App) handleListAISessions(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}

	rows, err := a.db.QueryContext(r.Context(),
		`SELECT id, COALESCE(title, ''), COALESCE(summary, ''), message_count, last_active_at FROM ai_sessions WHERE user_id = $1 ORDER BY last_active_at DESC LIMIT 20`,
		auth.UserID,
	)
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
	}
	var sessions []SessionRow
	for rows.Next() {
		var s SessionRow
		if err := rows.Scan(&s.ID, &s.Title, &s.Summary, &s.MessageCount, &s.LastActiveAt); err == nil {
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

// handleAIConfirm executes a previously proposed action
func (a *App) handleAIConfirm(w http.ResponseWriter, r *http.Request) {
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

	// Fetch and validate proposal
	var toolName, tenantID string
	var toolArgs json.RawMessage
	var status string
	var expiresAt time.Time
	err := a.db.QueryRowContext(r.Context(),
		`SELECT tool_name, tool_args, COALESCE(tenant_id::text, ''), status, expires_at FROM ai_pending_actions WHERE id = $1 AND user_id = $2`,
		req.ProposalID, auth.UserID,
	).Scan(&toolName, &toolArgs, &tenantID, &status, &expiresAt)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Proposal not found", r)
		return
	}

	if status != "pending" {
		writeErrorJSON(w, http.StatusConflict, "already_processed", "This action has already been processed", r)
		return
	}
	if time.Now().After(expiresAt) {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE ai_pending_actions SET status = 'expired' WHERE id = $1`, req.ProposalID)
		writeErrorJSON(w, http.StatusGone, "expired", "This action has expired. Please try again.", r)
		return
	}

	// Execute the action
	result, err := a.executeConfirmedAction(r.Context(), tenantID, auth.UserID, toolName, toolArgs)
	if err != nil {
		a.logger.Error("execute confirmed action failed", "error", err, "tool", toolName)
		writeErrorJSON(w, http.StatusInternalServerError, "execution_failed", "Could not execute action", r)
		return
	}

	// Mark as confirmed
	_, _ = a.db.ExecContext(r.Context(), `UPDATE ai_pending_actions SET status = 'confirmed' WHERE id = $1`, req.ProposalID)

	// Store result as assistant message in session so LLM has context for follow-ups
	var sessionID string
	_ = a.db.QueryRowContext(r.Context(), `SELECT session_id FROM ai_pending_actions WHERE id = $1`, req.ProposalID).Scan(&sessionID)
	if sessionID != "" {
		_, _ = a.db.ExecContext(r.Context(),
			`INSERT INTO ai_messages (session_id, role, content) VALUES ($1, 'assistant', $2)`,
			sessionID, result,
		)
	}

	writeJSON(w, http.StatusOK, map[string]any{"result": json.RawMessage(result)})
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
