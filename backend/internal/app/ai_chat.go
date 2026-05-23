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

// AI Chat endpoint — orchestrates LLM + tools + memory

const (
	maxToolLoops    = 10
	maxRecentMsgs   = 8
	maxSummaryChars = 400
	maxHistMsgChars = 800 // skip historical messages longer than this
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
	"masukkan": true, "masukan": true, "simpan": true, "buat": true,
	"tambahkan": true, "tambah": true, "submit": true, "create": true, "save": true,
	"sikat": true, "gas": true, "ayo": true, "yoi": true, "yoy": true,
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

// pendingProposalForSession returns the latest pending proposal on the session.
// Confirmation replies like "ya" should execute the proposal the user just saw,
// not every stale pending action in the session. Older versions returned up to
// 10 oldest pendings; in long AI sessions this could execute/cancel unrelated
// old proposals or appear to hang while processing stale bulk payloads.
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
		 ORDER BY created_at DESC
		 LIMIT 1`,
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
	mux.HandleFunc("DELETE /api/v1/ai/sessions/{id}", a.handleDeleteAISession)
}

type aiChatRequest struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
	// Shadow state from frontend
	Shadow struct {
		Route          string            `json:"route"`
		ActiveEntities map[string]string `json:"activeEntities"`
	} `json:"shadow"`
}

type llmMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content,omitempty"`
	ToolCalls  json.RawMessage `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	// ReasoningContent is required by reasoning models (mimo / qwen-cot)
	// when echoing an assistant message back on subsequent turns. The
	// upstream rejects with 400 "reasoning_content must be passed back"
	// otherwise.
	ReasoningContent string `json:"reasoning_content,omitempty"`
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
			Role             string          `json:"role"`
			Content          string          `json:"content"`
			ReasoningContent string          `json:"reasoning_content"`
			ToolCalls        json.RawMessage `json:"tool_calls"`
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
	w http.ResponseWriter, r *http.Request, sessionID, tenantID string, auth *AuthContext,
	req aiChatRequest, intent string,
	pendings []struct {
		ID       string
		ToolName string
		ToolArgs json.RawMessage
		TenantID string
		Confirm  string
	},
) bool {
	userID := auth.UserID
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
	var resultBlocks []string // raw tool outputs for continuation
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
		// Compress raw tool result for continuation context: just the
		// resource IDs + message, not the full JSON envelope. Cuts the
		// continuation prompt by ~60% on multi-step plans.
		resultBlocks = append(resultBlocks, p.ToolName+" -> "+compactToolResult(result))
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

	// Auto-continuation: if confirms succeeded and the AI's previous
	// response hinted at follow-up steps (e.g. "setelah stimulus, saya
	// akan buat group + soal"), run one LLM round so the model can
	// propose those follow-up actions. Without this the conversation
	// dead-ends at the first proposal even when the model's plan had
	// 3 steps.
	if len(successes) > 0 && len(failures) == 0 && !allTerminalProposalTools(pendings) {
		if followup := a.runContinuationRound(ctx, r, sessionID, tenantID, auth, req, successes, resultBlocks); followup != "" {
			content = followup
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":   map[string]string{"role": "assistant", "content": content},
		"sessionId": sessionID,
		"tokens":    0,
		"mutated":   len(successes) > 0,
	})
	return true
}

func allTerminalProposalTools(pendings []struct {
	ID       string
	ToolName string
	ToolArgs json.RawMessage
	TenantID string
	Confirm  string
}) bool {
	if len(pendings) == 0 {
		return false
	}
	for _, p := range pendings {
		switch p.ToolName {
		case "apply_question_kisi_kisi", "bulk_apply_question_kisi_kisi", "apply_blueprint_analysis", "update_question", "update_question_group":
			continue
		default:
			return false
		}
	}
	return true
}

// summarizeToolResult extracts a short success message from a tool's JSON
// result. Falls back to the raw result if the structure is unexpected.
// IMPORTANT: detects error envelopes (ToolError shape) and returns the
// error message instead of pretending success.
func summarizeToolResult(toolName, result string) string {
	// Error envelope check first — ToolError.JSON() shape: {"error": {...}}
	var errEnv struct {
		Error *struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(result), &errEnv); err == nil && errEnv.Error != nil {
		return "❌ " + errEnv.Error.Message
	}
	var parsed struct {
		Message string `json:"message"`
		Success bool   `json:"success"`
	}
	if err := json.Unmarshal([]byte(result), &parsed); err == nil && parsed.Message != "" {
		return parsed.Message
	}
	return toolName
}

// compactToolResult extracts just the IDs + message from a tool
// result JSON envelope. The full JSON (including nested arrays /
// schema fields the model already saw at call time) is wasteful to
// echo back — the model only needs the new resource IDs and a status
// line to advance the plan.
// isChitchat returns true when the message is a greeting, thanks,
// or other short conversational filler that has zero chance of
// needing tools. Lets us skip shipping the ~5000-token tool catalog
// for these turns. Conservative — only matches very obvious patterns.
func isChitchat(msg string) bool {
	m := strings.TrimSpace(strings.ToLower(msg))
	if len(m) > 40 {
		return false
	}
	patterns := []string{
		"halo", "hai", "hi ", "hello", "hei", "selamat pagi", "selamat siang",
		"selamat sore", "selamat malam", "pagi", "siang", "sore", "malam",
		"makasih", "terima kasih", "thanks", "thx", "oke", "ok", "sip",
		"siapa kamu", "kamu siapa", "who are you", "apa kabar",
		"test", "testing", "coba", "halo bot",
	}
	for _, p := range patterns {
		if m == p || m == p+"." || m == p+"!" || m == p+"?" || strings.HasPrefix(m, p+" ") {
			return true
		}
	}
	return false
}

// dispatchToolCall is the single source of truth for executing a
// tool call from the LLM. Tries capRegistry first, falls back to
// toolRegistry. Crucially, when neither has the tool registered, it
// returns a structured UNKNOWN_TOOL error with a fuzzy-matched
// suggestion so the model can self-correct on the next iteration
// instead of giving up. This catches reasoning-model hallucinations
// like 'add_questions_to_exam' (real name: batch_create_questions).
func (a *App) dispatchToolCall(ctx context.Context, tenantID, userID, name, argsJSON string) string {
	if handler, ok := a.capRegistry.handlers[name]; ok {
		result, _ := handler(ctx, tenantID, userID, json.RawMessage(argsJSON))
		return result
	}
	if _, ok := a.toolRegistry.handlers[name]; ok {
		oldResult, _ := a.toolRegistry.Execute(ctx, tenantID, userID, name, argsJSON)
		return oldResult.Content
	}
	suggestion := a.capRegistry.findSimilarToolName(name)
	te := &ToolError{
		Code:        "UNKNOWN_TOOL",
		Message:     "Tool '" + name + "' tidak terdaftar. Pakai nama tool yang ada di tools list yang sudah disediakan.",
		Field:       "tool_name",
		Recoverable: true,
	}
	if suggestion != "" {
		te.Suggestions = []string{suggestion}
		te.Recovery = &RecoveryHint{
			Tool: suggestion,
			Hint: "Maksud kamu '" + suggestion + "'? Retry dengan nama itu.",
		}
	}
	return te.JSON()
}

// deriveScopeKey returns a stable scope identifier for the active
// page, used to key AI chat sessions per resource. The backend
// derives this from request shadow on every call — clients never
// set it. Format:
//   - 'exam:<uuid>'      when /app/exams/{id}
//   - 'blueprint:<uuid>' when /app/blueprints/{id}
//   - 'global'           everywhere else (dashboard, lists, etc.)
//
// The 'global' scope acts as the catch-all where general help and
// cross-resource queries live.
func deriveScopeKey(active map[string]string) string {
	if id := active["examId"]; id != "" {
		return "exam:" + id
	}
	if id := active["templateId"]; id != "" {
		return "blueprint:" + id
	}
	return "global"
}

// resolveOrCreateSession finds the right ai_sessions row for this
// (user, scope) tuple. Robustness rules:
//  1. If client supplied a sessionId AND it matches this user + scope,
//     reuse it. Trusts the client when scope agrees.
//  2. If client supplied a sessionId but scope doesn't match, IGNORE it
//     — the user navigated to a different resource since their last
//     turn. Fall through to scope-based lookup.
//  3. Look up the most recent session for this (user, scope). If found,
//     reuse it (auto-resume).
//  4. Otherwise create a new session with scope_key set.
//
// This is the single point where session identity is decided so the
// rest of the handler doesn't need to think about scope.
func (a *App) resolveOrCreateSession(ctx context.Context, suppliedID, tenantID, userID, scopeKey string) (string, error) {
	if suppliedID != "" {
		var existingScope sql.NullString
		var ownerID string
		err := a.db.QueryRowContext(ctx,
			`SELECT user_id, scope_key FROM ai_sessions WHERE id = $1`,
			suppliedID,
		).Scan(&ownerID, &existingScope)
		if err == nil && ownerID == userID {
			if !existingScope.Valid {
				_, _ = a.db.ExecContext(ctx,
					`UPDATE ai_sessions SET scope_key = $1 WHERE id = $2`,
					scopeKey, suppliedID)
				return suppliedID, nil
			}
			if existingScope.String == scopeKey {
				return suppliedID, nil
			}
			// Scope mismatch — fall through to scope-based lookup.
		}
	}

	var foundID string
	err := a.db.QueryRowContext(ctx,
		`SELECT id FROM ai_sessions
		  WHERE user_id = $1 AND scope_key = $2
		  ORDER BY last_active_at DESC LIMIT 1`,
		userID, scopeKey,
	).Scan(&foundID)
	if err == nil && foundID != "" {
		return foundID, nil
	}

	var newID string
	err = a.db.QueryRowContext(ctx,
		`INSERT INTO ai_sessions (tenant_id, user_id, scope_key)
		 VALUES (NULLIF($1,'')::uuid, $2, $3) RETURNING id`,
		tenantID, userID, scopeKey,
	).Scan(&newID)
	if err != nil {
		return "", err
	}
	return newID, nil
}

func compactToolResult(raw string) string {
	var parsed map[string]any
	if json.Unmarshal([]byte(raw), &parsed) != nil {
		if len(raw) > 200 {
			return raw[:200] + "…"
		}
		return raw
	}
	var parts []string
	if msg, ok := parsed["message"].(string); ok && msg != "" {
		parts = append(parts, msg)
	}
	for k, v := range parsed {
		if strings.HasSuffix(k, "Id") || k == "id" {
			if s, ok := v.(string); ok && s != "" {
				parts = append(parts, k+"="+s)
			}
		}
	}
	if len(parts) == 0 {
		return raw
	}
	return strings.Join(parts, ", ")
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

	// Derive scope key from active page (Phase 9.12). The scope locks
	// the session to a specific resource so switching from /exams/A
	// to /exams/B doesn't carry forward A's conversation state. The
	// backend is the source of truth here — we ignore client-supplied
	// sessionId if it doesn't match this scope.
	scopeKey := deriveScopeKey(req.Shadow.ActiveEntities)

	sessionID, err := a.resolveOrCreateSession(r.Context(), req.SessionID, tenantID, auth.UserID, scopeKey)
	if err != nil {
		a.logger.Error("resolve ai session failed", "error", err, "scope", scopeKey)
		writeErrorJSON(w, http.StatusInternalServerError, "ai_error", "Could not create session", r)
		return
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
			if a.handleShortReplyForProposals(w, r, sessionID, tenantID, auth, req, intent, pendings) {
				return
			}
		}
	}

	// Assemble context
	messages := a.assembleContext(r.Context(), sessionID, tenantID, auth, req)

	// Detect relevant domains from user message
	domains := DetectDomains(req.Message)

	// Active-page domain inference (Phase 9.11). When the user is on a
	// detail page, the page itself is a stronger signal than keywords.
	// Force-include the matching domain so tools like create_question /
	// batch_create_questions are exposed even when the user message
	// doesn't contain the word "soal". Smart variant: only adds adjacent
	// domains (blueprints/stimuli) when message hints at them.
	domains = appendActiveDomainsForMessage(domains, req.Shadow.ActiveEntities, req.Message)

	// Chitchat fast-path: if message is a short greeting or meta
	// question, ship NO tools at all. Saves ~5000 tokens per turn on
	// every "halo", "makasih", "siapa kamu". The model can still
	// answer normally; we just don't burn the catalog for messages
	// that have zero chance of needing tools.
	var tools []map[string]any
	if !isChitchat(req.Message) {
		tools = a.capRegistry.GetToolsForIntent(domains, auth.Permissions)
	}

	// LLM call loop (tool calls may require multiple rounds)
	var finalContent string
	var totalTokens int

	// Pass session ID via context for write tools
	ctxWithSession := context.WithValue(r.Context(), ctxKeySessionID{}, sessionID)

	for i := 0; i < maxToolLoops; i++ {
		// Token economy: only ship the full tool list on iter 0. After
		// the first round the model has either (a) emitted tool_calls
		// and now just needs to summarise the result — no tools needed,
		// or (b) said its piece in content and we're already breaking
		// out of the loop. The tool list is dead weight on every call
		// after iter 0. Multi-step plans are handled by emitting
		// multiple tool_calls in ONE assistant message (system prompt
		// instructs this) rather than chaining iterations.
		iterTools := tools
		if i >= 1 {
			// Default: drop tools to save tokens. EXCEPTION: when the
			// most recent tool result was an error, keep them so the
			// model can self-correct (retry with a different tool name
			// or fixed args). Without this, hallucinated tool names
			// like 'add_questions_to_exam' get a single-shot recovery
			// hint but no way to actually retry.
			iterTools = nil
			if len(messages) > 0 {
				last := messages[len(messages)-1]
				if last.Role == "tool" && (strings.Contains(last.Content, `"error"`) || strings.Contains(last.Content, `UNKNOWN_TOOL`) || strings.Contains(last.Content, `VALIDATION_FAILED`)) {
					iterTools = tools
				}
			}
		}
		llmResp, err := a.callLLM(ctxWithSession, messages, iterTools)
		if err != nil {
			a.logger.Error("llm call failed", "error", err)
			writeErrorJSON(w, http.StatusBadGateway, "ai_error", "AI service unavailable", r)
			return
		}
		totalTokens += llmResp.Usage.TotalTokens

		if len(llmResp.Choices) == 0 {
			a.logger.Warn("llm returned no choices",
				"loop", i, "tools", len(tools), "messages", len(messages))
			// If we already collected proposals during earlier
			// iterations, surface those instead of failing — the user
			// can still confirm them even though the chat narration is
			// broken on the upstream side.
			finalContent = ""
			break
		}

		choice := llmResp.Choices[0]

		// Reasoning models can finish_reason='length' having spent the
		// budget on hidden reasoning. We don't want to fail the request —
		// surface a graceful message and let the user retry. Same for any
		// other empty-content + no-tool-calls condition that isn't an
		// upstream transport error.
		if strings.TrimSpace(choice.Message.Content) == "" && len(choice.Message.ToolCalls) == 0 {
			a.logger.Warn("llm returned empty content + no tools",
				"loop", i, "finishReason", choice.FinishReason,
				"tools_offered", len(tools), "messages", len(messages))
			if choice.FinishReason == "length" || choice.FinishReason == "max_tokens" {
				finalContent = "Maaf, model kehabisan budget reasoning sebelum sempat respons. Coba: (1) pertanyaan lebih spesifik / ringkas, (2) batasi scope (per-section atau per-group dulu, jangan exam-wide), atau (3) buka chat panel dan minta langsung secara bertahap."
				break
			}
			writeErrorJSON(w, http.StatusBadGateway, "ai_error", "Empty AI response", r)
			return
		}

		// If no tool calls, we have the final response
		if choice.FinishReason != "tool_calls" || len(choice.Message.ToolCalls) == 0 {
			// Some reasoning models occasionally emit tool calls as XML-
			// style text inside content (<tool_call>...</tool_call>) instead
			// of the native tool_calls field. Try to parse + execute those
			// before falling back to plain content; otherwise the user sees
			// raw XML markup and the action never runs.
			if synth := parseXMLToolCalls(choice.Message.Content); len(synth) > 0 {
				// Synthesise a proper assistant message with the native
				// tool_calls JSON shape upstream expects, then echo each
				// tool result back with a matching tool_call_id. Without
				// this the upstream rejects the next call with
				// 'tool_call_id is not set' / 'content is not set'.
				synthJSON, _ := json.Marshal(synth)
				messages = append(messages, llmMessage{
					Role:             "assistant",
					Content:          " ",
					ToolCalls:        synthJSON,
					ReasoningContent: choice.Message.ReasoningContent,
				})
				hasProposal := false
				for _, tc := range synth {
					resultContent := a.dispatchToolCall(ctxWithSession, tenantID, auth.UserID, tc.Function.Name, tc.Function.Arguments)
					messages = append(messages, llmMessage{
						Role:       "tool",
						Content:    resultContent,
						ToolCallID: tc.ID,
					})
					if strings.Contains(resultContent, "confirmation_required") {
						hasProposal = true
					}
				}
				// Once any tool returned a proposal, stop the loop —
				// otherwise the model spins trying to confirm via more
				// XML calls. The proposal summary is built post-loop.
				if hasProposal {
					finalContent = ""
					break
				}
				continue
			}
			finalContent = choice.Message.Content
			break
		}

		// Parse and execute tool calls
		var toolCalls []llmToolCall
		if err := json.Unmarshal(choice.Message.ToolCalls, &toolCalls); err != nil {
			finalContent = choice.Message.Content
			break
		}

		// Add assistant message with tool_calls + reasoning to context.
		// Reasoning models reject the next call when reasoning_content
		// from the previous turn is not echoed back. Some upstreams also
		// reject empty content with 'content is not set' — default to a
		// space so the message validates.
		assistantContent := choice.Message.Content
		if assistantContent == "" {
			assistantContent = " "
		}
		messages = append(messages, llmMessage{
			Role:             "assistant",
			Content:          assistantContent,
			ToolCalls:        choice.Message.ToolCalls,
			ReasoningContent: choice.Message.ReasoningContent,
		})

		// Execute each tool and add results
		var hasProposal bool
		for _, tc := range toolCalls {
			resultContent := a.dispatchToolCall(ctxWithSession, tenantID, auth.UserID, tc.Function.Name, tc.Function.Arguments)
			messages = append(messages, llmMessage{
				Role:       "tool",
				Content:    resultContent,
				ToolCallID: tc.ID,
			})
			if strings.Contains(resultContent, "confirmation_required") {
				hasProposal = true
			}
		}

		// If a write tool returned a proposal, stop the loop — the
		// proposal summary is rendered post-loop. Continuing here lets
		// the model spin trying to call confirm_action which doesn't
		// exist, eventually exhausting maxToolLoops or hitting upstream
		// rate limits.
		if hasProposal {
			finalContent = ""
			break
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
				// Trim historical messages above maxHistMsgChars. The full
				// version stays in DB; we just stop replaying it on every
				// turn. Keep the head + a marker so model knows it was
				// truncated.
				if len(m.Content) > maxHistMsgChars {
					m.Content = m.Content[:maxHistMsgChars] + " …[trimmed]"
				}
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
	// Compact system prompt. Previous version was 517 tokens of
	// repeated "JANGAN" copy. Model behaviour is the same with the
	// concise rules below + structured ToolError recovery hints; we
	// just stop paying for the explanation on every turn.
	sb.WriteString("Asisten Morfoschools. Jawab ringkas dalam Bahasa Indonesia. Pakai tools untuk semua data; jangan mengarang.\n")
	sb.WriteString("Multi-step: emit semua tool_calls dalam satu turn (native function-calling support multiple). User konfirmasi semua dengan satu 'ya'.\n")
	sb.WriteString("PASSAGE + SOAL: default untuk permintaan umum 'buat N soal dengan bacaan/stimulus' adalah batch_create_questions dengan stimulus/konteks panjang tertanam di stem setiap soal (standalone, bukan group). Pakai create_stimulus_block HANYA jika user eksplisit minta satu bacaan bersama/group stimulus/soal berdasarkan teks yang sama. JANGAN chain create_stimulus + create_question_group + create_question terpisah.\n")
	sb.WriteString("Sebelum batch-create, panggil list_* / search_* untuk hindari duplikat.\n")
	sb.WriteString("DUPLICATE GUARD: di exam dengan >=5 soal existing, WAJIB panggil find_similar_questions(examId, content) SEBELUM tiap create_question/batch_create_questions. Kalau ada hasil sim>=0.85, ganti pendekatan (topik/level kognitif/stimulus berbeda) dan retry. Skip pre-check hanya kalau user eksplisit minta soal serupa (variant problem).\n")
	sb.WriteString("Tool error: ikuti error.recovery, retry diam-diam max 2x, lalu tanya user.\n")
	sb.WriteString("JANGAN PERNAH bilang 'berhasil', 'sudah dibuat', 'sudah ditambahkan', atau bahasa konfirmasi serupa kalau kamu TIDAK memanggil tool yang menulis ke DB di turn ini. Kalau user bilang 'masukkan'/'simpan'/'ya'/'lanjut' dan kamu lihat ada proposal pending, JANGAN narasi sukses—harness sudah handle eksekusi otomatis.\n")

	sb.WriteString(fmt.Sprintf("User: %s | %s", auth.DisplayName, strings.Join(auth.Roles, ",")))
	if req.Shadow.Route != "" {
		sb.WriteString(" | Page: " + req.Shadow.Route)
	}
	sb.WriteString("\n")

	if active := a.buildActiveContext(tenantID, auth, req.Shadow.ActiveEntities); active != "" {
		sb.WriteString("\n=== HALAMAN AKTIF ===\n")
		sb.WriteString(active)
		sb.WriteString("=== END ===\n")
		sb.WriteString("Target ke entitas di konteks tanpa minta user menyebut ulang. Jangan duplikasi item yang sudah ada di konteks. Untuk buat/edit soal: WAJIB pakai tools, jangan minta user copy-paste.\n")
	}

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
		// Reasoning models (gemini-3 / mimo / qwen-cot / deepseek-r1)
		// consume a chunk of the budget on hidden reasoning tokens before
		// emitting visible content. Phase 9.16 exam-level prompts (e.g.
		// 'Generate kisi-kisi dari semua soal') trigger long reasoning
		// passes that burned all 4096 max_tokens on reasoning alone,
		// leaving no budget for the actual tool_call payload (3933
		// reasoning_tokens, 0 content). 8192 gives reasoning + tool
		// arguments room to breathe; bump again only if we hit this on
		// even larger compound prompts.
		"max_tokens": 8192,
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
	if a.logger != nil && len(respBody) > 0 {
		preview := respBody
		if len(preview) > 600 {
			preview = preview[:600]
		}
		a.logger.Debug("llm response preview", "bytes", len(respBody), "head", string(preview))
	}

	// Try plain JSON first (Chat Completions non-streaming response).
	// Some upstreams append a trailing 'data: [DONE]' marker even on
	// non-streamed responses; that fooled an earlier strings.Contains
	// check into running the SSE parser, which then dropped the JSON
	// body entirely. Trying JSON first gives us the canonical shape
	// when present and only falls back to SSE accumulation when the
	// body is genuinely streamed.
	jsonBody2 := respBody
	// Strip trailing 'data: [DONE]' marker which some upstreams append
	// directly after the JSON body without a newline separator.
	if idx := bytes.Index(jsonBody2, []byte("data: [DONE]")); idx > 0 {
		jsonBody2 = bytes.TrimSpace(jsonBody2[:idx])
	}
	if idx := bytes.Index(jsonBody2, []byte("\ndata: ")); idx > 0 {
		jsonBody2 = bytes.TrimSpace(jsonBody2[:idx])
	}
	var llmRespDirect llmResponse
	if json.Unmarshal(jsonBody2, &llmRespDirect) == nil && len(llmRespDirect.Choices) > 0 {
		return &llmRespDirect, nil
	}

	// Parse SSE streaming response — accumulate chunks into final message
	if bytes.Contains(respBody, []byte("data: ")) {
		var finalContent strings.Builder
		var finalReasoning strings.Builder
		var toolCalls json.RawMessage
		var finishReason string
		// Streaming tool_calls are fragmented across chunks per OpenAI
		// spec: each delta has tool_calls[].index pointing to the slot,
		// and arguments arrive as substring fragments to be concatenated.
		// Without this accumulator we'd lose every tool call past iter 0
		// on Gemini / OpenAI / any well-behaved streaming upstream.
		type tcAccum struct {
			ID        string
			Type      string
			Name      string
			Arguments strings.Builder
		}
		var tcSlots []*tcAccum
		ensureSlot := func(idx int) *tcAccum {
			for len(tcSlots) <= idx {
				tcSlots = append(tcSlots, &tcAccum{Type: "function"})
			}
			return tcSlots[idx]
		}
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

			type tcDelta struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			}
			var parsed struct {
				Choices []struct {
					Delta struct {
						Content          string    `json:"content"`
						ReasoningContent string    `json:"reasoning_content"`
						ToolCalls        []tcDelta `json:"tool_calls"`
					} `json:"delta"`
					Message struct {
						Content          string          `json:"content"`
						ReasoningContent string          `json:"reasoning_content"`
						ToolCalls        json.RawMessage `json:"tool_calls"`
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
				if c.Delta.ReasoningContent != "" {
					finalReasoning.WriteString(c.Delta.ReasoningContent)
				}
				if c.Message.ReasoningContent != "" {
					finalReasoning.WriteString(c.Message.ReasoningContent)
				}
				// Per-fragment streaming accumulation. Each delta's
				// tool_calls[].arguments is a SUBSTRING to append, not a
				// full replacement.
				for _, td := range c.Delta.ToolCalls {
					slot := ensureSlot(td.Index)
					if td.ID != "" {
						slot.ID = td.ID
					}
					if td.Type != "" {
						slot.Type = td.Type
					}
					if td.Function.Name != "" {
						slot.Name = td.Function.Name
					}
					if td.Function.Arguments != "" {
						slot.Arguments.WriteString(td.Function.Arguments)
					}
				}
				// Some upstreams send the full tool_calls array on a
				// terminating message frame instead of streaming deltas.
				// Capture it as a final-state override.
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

		// If we accumulated any streamed slots, materialise them into
		// the final tool_calls JSON expected by the consumer (overriding
		// the message-frame override only if no message-frame was sent).
		if len(tcSlots) > 0 && len(toolCalls) == 0 {
			type outTC struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			}
			out := make([]outTC, 0, len(tcSlots))
			for _, s := range tcSlots {
				if s == nil || s.Name == "" {
					continue
				}
				var tc outTC
				tc.ID = s.ID
				tc.Type = s.Type
				tc.Function.Name = s.Name
				tc.Function.Arguments = s.Arguments.String()
				out = append(out, tc)
			}
			if len(out) > 0 {
				toolCalls, _ = json.Marshal(out)
				if finishReason == "" {
					finishReason = "tool_calls"
				}
			}
		}

		return &llmResponse{
			Choices: []struct {
				Message struct {
					Role             string          `json:"role"`
					Content          string          `json:"content"`
					ReasoningContent string          `json:"reasoning_content"`
					ToolCalls        json.RawMessage `json:"tool_calls"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Message: struct {
						Role             string          `json:"role"`
						Content          string          `json:"content"`
						ReasoningContent string          `json:"reasoning_content"`
						ToolCalls        json.RawMessage `json:"tool_calls"`
					}{
						Role:             "assistant",
						Content:          finalContent.String(),
						ReasoningContent: finalReasoning.String(),
						ToolCalls:        toolCalls,
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

// parseXMLToolCalls handles the XML-style tool-call format some
// reasoning models emit inside content when they get confused about
// schema. Shape:
//
//	<tool_call>
//	  <function=create_question>
//	    <parameter=examId>uuid-here</parameter>
//	    <parameter=content>Apa ibukota...</parameter>
//	    <parameter=options>[{"content":"Jakarta","isCorrect":true}]</parameter>
//	  </function>
//	</tool_call>
//
// We extract each <function=NAME> ... </function> block, collect its
// <parameter=KEY>VALUE</parameter> pairs, and synthesise a real
// llmToolCall the existing executor path can run. Returns nil when
// no XML markers found.
func parseXMLToolCalls(content string) []llmToolCall {
	if !strings.Contains(content, "<tool_call>") || !strings.Contains(content, "<function=") {
		return nil
	}
	var out []llmToolCall
	idx := 0
	for {
		open := strings.Index(content[idx:], "<function=")
		if open < 0 {
			break
		}
		open += idx
		nameEnd := strings.Index(content[open:], ">")
		if nameEnd < 0 {
			break
		}
		name := content[open+len("<function=") : open+nameEnd]
		name = strings.TrimSpace(name)
		close := strings.Index(content[open:], "</function>")
		if close < 0 {
			break
		}
		body := content[open+nameEnd+1 : open+close]
		params := map[string]any{}
		paramIdx := 0
		for {
			pOpen := strings.Index(body[paramIdx:], "<parameter=")
			if pOpen < 0 {
				break
			}
			pOpen += paramIdx
			pNameEnd := strings.Index(body[pOpen:], ">")
			if pNameEnd < 0 {
				break
			}
			pName := strings.TrimSpace(body[pOpen+len("<parameter=") : pOpen+pNameEnd])
			pClose := strings.Index(body[pOpen:], "</parameter>")
			if pClose < 0 {
				break
			}
			pVal := body[pOpen+pNameEnd+1 : pOpen+pClose]
			pVal = strings.TrimSpace(pVal)
			// Try parse as JSON (for arrays/objects/numbers/booleans),
			// fall back to string for everything else.
			var parsedVal any
			if err := json.Unmarshal([]byte(pVal), &parsedVal); err == nil {
				params[pName] = parsedVal
			} else {
				params[pName] = pVal
			}
			paramIdx = pOpen + pClose + len("</parameter>")
		}
		args, _ := json.Marshal(params)
		call := llmToolCall{
			ID:   fmt.Sprintf("xml_%d", len(out)),
			Type: "function",
		}
		call.Function.Name = name
		call.Function.Arguments = string(args)
		out = append(out, call)
		idx = open + close + len("</function>")
	}
	return out
}

// runContinuationRound is invoked after a short-reply confirm has
// successfully executed one or more proposals. It runs a single
// abbreviated LLM round so the model can propose follow-up actions
// from a multi-step plan (e.g. "setelah stimulus, saya akan bikin
// group lalu 2 soal"). Returns a friendly user-facing message that
// supersedes the basic "Berhasil!" summary when new proposals are
// generated; returns empty string when nothing follow-up was needed.
func (a *App) runContinuationRound(
	ctx context.Context, r *http.Request,
	sessionID, tenantID string, auth *AuthContext, req aiChatRequest,
	successes []string, resultBlocks []string,
) string {
	// Pull tool names that just ran for short-name reference.
	var lastTools []string
	for _, b := range resultBlocks {
		if idx := strings.Index(b, " -> "); idx > 0 {
			lastTools = append(lastTools, b[:idx])
		}
	}

	note := "AKSI BARU SAJA DIEKSEKUSI — hasil tool sebagai berikut:\n\n" +
		strings.Join(resultBlocks, "\n\n") +
		"\n\nTool yang sudah berhasil dipanggil: " + strings.Join(lastTools, ", ") + ". " +
		"JANGAN propose tool yang sama dengan args yang sama lagi — itu duplikasi. " +
		"Lihat ID baru di hasil tool di atas dan gunakan untuk langkah berikutnya. " +
		"Misal: kalau create_stimulus mengembalikan stimulusId X, langkah berikut bisa create_question_group dengan stimulusId=X. " +
		"Kalau rencana awal kamu masih punya LANGKAH LANJUTAN yang BERBEDA dari yang sudah dieksekusi, propose tool_calls berikut sekarang. " +
		"Kalau rencana sudah selesai, balas konfirmasi singkat dan tawarkan langkah berikutnya — JANGAN ulang langkah lama."
	// Persist a COMPACT version to history (just "ran: tool1, tool2")
	// so future turns know the tools fired without replaying the full
	// JSON payload. The full note only goes to the current round's
	// system message; assembleContext doesn't load it for later turns.
	persistedHistory := "[ran: " + strings.Join(lastTools, ", ") + "]"
	_, _ = a.db.ExecContext(ctx,
		`INSERT INTO ai_messages (session_id, role, content) VALUES ($1, 'assistant', $2)`,
		sessionID, persistedHistory)

	// Continuation context: build a MINIMAL message list rather than
	// pulling 8 messages of history. The synthetic note already encodes
	// what just happened (tools ran + IDs returned), and the system
	// prompt + active-page block carry current state. Skipping history
	// replay saves ~3-5k tokens per affirm.
	systemPrompt := a.buildSystemPrompt(tenantID, auth, req)
	messages := []llmMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: req.Message},
		{Role: "system", Content: note},
	}
	domains := DetectDomains(req.Message)
	domains = appendActiveDomainsForMessage(domains, req.Shadow.ActiveEntities, req.Message)
	tools := a.capRegistry.GetToolsForIntent(domains, auth.Permissions)

	ctxWithSession := context.WithValue(ctx, ctxKeySessionID{}, sessionID)
	var finalContent string
	hadProposal := false
	// One round is enough for the next-step proposal. Continuing past
	// that just burns tokens — the model has the full plan in history
	// and either emits the next tool_call(s) on round 0 or it's done.
	for i := 0; i < 1; i++ {
		llmResp, err := a.callLLM(ctxWithSession, messages, tools)
		if err != nil || len(llmResp.Choices) == 0 {
			break
		}
		choice := llmResp.Choices[0]
		if choice.FinishReason != "tool_calls" || len(choice.Message.ToolCalls) == 0 {
			if synth := parseXMLToolCalls(choice.Message.Content); len(synth) > 0 {
				synthJSON, _ := json.Marshal(synth)
				messages = append(messages, llmMessage{Role: "assistant", Content: " ", ToolCalls: synthJSON, ReasoningContent: choice.Message.ReasoningContent})
				for _, tc := range synth {
					resultContent := a.dispatchToolCall(ctxWithSession, tenantID, auth.UserID, tc.Function.Name, tc.Function.Arguments)
					messages = append(messages, llmMessage{Role: "tool", Content: resultContent, ToolCallID: tc.ID})
					if strings.Contains(resultContent, "confirmation_required") {
						hadProposal = true
					}
				}
				if hadProposal {
					break
				}
				continue
			}
			finalContent = choice.Message.Content
			break
		}
		var toolCalls []llmToolCall
		if err := json.Unmarshal(choice.Message.ToolCalls, &toolCalls); err != nil {
			finalContent = choice.Message.Content
			break
		}
		assistantContent := choice.Message.Content
		if assistantContent == "" {
			assistantContent = " "
		}
		messages = append(messages, llmMessage{Role: "assistant", Content: assistantContent, ToolCalls: choice.Message.ToolCalls, ReasoningContent: choice.Message.ReasoningContent})
		for _, tc := range toolCalls {
			resultContent := a.dispatchToolCall(ctxWithSession, tenantID, auth.UserID, tc.Function.Name, tc.Function.Arguments)
			messages = append(messages, llmMessage{Role: "tool", Content: resultContent, ToolCallID: tc.ID})
			if strings.Contains(resultContent, "confirmation_required") {
				hadProposal = true
			}
		}
		if hadProposal {
			break
		}
	}

	if !hadProposal {
		return ""
	}

	// Collect new pending proposals for the response. We re-query DB
	// rather than parsing message bodies because the proposal IDs are
	// already persisted and the executor returns the canonical struct
	// in pendingProposalsForSession.
	pending := a.pendingProposalsForSession(ctx, sessionID, auth.UserID)
	if len(pending) == 0 {
		return ""
	}
	var parts []string
	for _, p := range pending {
		parts = append(parts, "• "+p.Confirm)
	}
	if finalContent == "" {
		finalContent = "✅ Berhasil! " + strings.Join(successes, "; ") + ".\n\nLangkah lanjutan menunggu konfirmasi:\n" + strings.Join(parts, "\n") + "\n\nKetik **ya** untuk lanjutkan."
	} else {
		finalContent = finalContent + "\n\nLangkah lanjutan menunggu konfirmasi:\n" + strings.Join(parts, "\n") + "\n\nKetik **ya** untuk lanjutkan."
	}
	_, _ = a.db.ExecContext(ctx,
		`INSERT INTO ai_messages (session_id, role, content) VALUES ($1, 'assistant', $2)`,
		sessionID, finalContent)
	return finalContent
}
