package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type agentSessionMemory struct {
	ActiveGoal      string                    `json:"activeGoal,omitempty"`
	UserPreferences []string                  `json:"userPreferences,omitempty"`
	Drafts          []agentBlueprintDraftMemo `json:"blueprintDrafts,omitempty"`
	UpdatedAt       string                    `json:"updatedAt,omitempty"`
}

type agentBlueprintDraftMemo struct {
	DraftID   string                    `json:"draftId"`
	Range     string                    `json:"range,omitempty"`
	Status    string                    `json:"status"`
	Source    string                    `json:"source,omitempty"`
	Slots     []agentBlueprintSlotDraft `json:"slots,omitempty"`
	CreatedAt string                    `json:"createdAt,omitempty"`
}

func (a *App) loadAgentSessionMemory(ctx context.Context, tenantID, sessionID, scopeKey string) agentSessionMemory {
	if a.db == nil || sessionID == "" {
		return agentSessionMemory{}
	}
	var raw []byte
	err := a.db.QueryRowContext(ctx, `SELECT memory_json FROM agent_session_memory WHERE session_id=$1`, sessionID).Scan(&raw)
	if err == sql.ErrNoRows {
		_, _ = a.db.ExecContext(ctx, `INSERT INTO agent_session_memory (session_id, tenant_id, scope_key, memory_json) VALUES ($1,$2,$3,'{}'::jsonb) ON CONFLICT (session_id) DO NOTHING`, sessionID, tenantID, scopeKey)
		return agentSessionMemory{}
	}
	if err != nil || len(raw) == 0 {
		return agentSessionMemory{}
	}
	var mem agentSessionMemory
	_ = json.Unmarshal(raw, &mem)
	return mem
}

func (a *App) saveAgentSessionMemory(ctx context.Context, tenantID, sessionID, scopeKey string, mem agentSessionMemory) {
	if a.db == nil || sessionID == "" {
		return
	}
	mem.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	raw, _ := json.Marshal(mem)
	_, _ = a.db.ExecContext(ctx, `
		INSERT INTO agent_session_memory (session_id, tenant_id, scope_key, memory_json, updated_at)
		VALUES ($1,$2,$3,$4::jsonb,now())
		ON CONFLICT (session_id) DO UPDATE SET memory_json=EXCLUDED.memory_json, updated_at=now(), scope_key=EXCLUDED.scope_key
	`, sessionID, tenantID, scopeKey, string(raw))
}

func (a *App) rememberAssistantBlueprintDraft(ctx context.Context, tenantID, sessionID, scopeKey, content string) {
	slots := extractBlueprintDraftSlotsFromText(content)
	if len(slots) == 0 {
		return
	}
	mem := a.loadAgentSessionMemory(ctx, tenantID, sessionID, scopeKey)
	draft := agentBlueprintDraftMemo{DraftID: "draft_" + strconv.FormatInt(time.Now().UnixNano(), 36), Range: inferDraftRange(slots), Status: "discussed", Source: "assistant_message", Slots: slots, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	mem.Drafts = append(mem.Drafts, draft)
	if len(mem.Drafts) > 12 {
		mem.Drafts = mem.Drafts[len(mem.Drafts)-12:]
	}
	a.saveAgentSessionMemory(ctx, tenantID, sessionID, scopeKey, mem)
}

func (a *App) latestBlueprintDraft(ctx context.Context, tenantID, sessionID, scopeKey string) (agentBlueprintDraftMemo, bool) {
	mem := a.loadAgentSessionMemory(ctx, tenantID, sessionID, scopeKey)
	for i := len(mem.Drafts) - 1; i >= 0; i-- {
		if len(mem.Drafts[i].Slots) > 0 && mem.Drafts[i].Status != "saved" && mem.Drafts[i].Status != "rejected" {
			return mem.Drafts[i], true
		}
	}
	return agentBlueprintDraftMemo{}, false
}

func (a *App) markLatestBlueprintDraftStatus(ctx context.Context, tenantID, sessionID, scopeKey, status string) {
	mem := a.loadAgentSessionMemory(ctx, tenantID, sessionID, scopeKey)
	for i := len(mem.Drafts) - 1; i >= 0; i-- {
		if len(mem.Drafts[i].Slots) > 0 {
			mem.Drafts[i].Status = status
			break
		}
	}
	a.saveAgentSessionMemory(ctx, tenantID, sessionID, scopeKey, mem)
}

var blueprintSlotHeaderRe = regexp.MustCompile(`(?i)^\s*(?:slot\s*)?(\d+)\s*[\).:-]?\s*$`)
var blueprintSlotInlineHeaderRe = regexp.MustCompile(`(?i)^\s*slot\s*(\d+)\s*[·\-–—:]\s*(.+)$`)

func extractBlueprintDraftSlotsFromText(content string) []agentBlueprintSlotDraft {
	lines := strings.Split(content, "\n")
	var slots []agentBlueprintSlotDraft
	current := agentBlueprintSlotDraft{}
	flush := func() {
		if strings.TrimSpace(current.ElemenCP) != "" && strings.TrimSpace(current.TujuanPembelajaran) != "" && strings.TrimSpace(current.IndikatorSoal) != "" {
			if current.QuestionType == "" {
				current.QuestionType = "multiple_choice"
			}
			if current.Points <= 0 {
				current.Points = 1
			}
			slots = append(slots, current)
		}
		current = agentBlueprintSlotDraft{}
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.Trim(line, "-*• "))
		if trimmed == "" {
			continue
		}
		if m := blueprintSlotInlineHeaderRe.FindStringSubmatch(trimmed); len(m) == 3 {
			flush()
			current.Position, _ = strconv.Atoi(m[1])
			parseBlueprintInlineHeader(m[2], &current)
			continue
		}
		if m := blueprintSlotHeaderRe.FindStringSubmatch(trimmed); len(m) == 2 {
			flush()
			current.Position, _ = strconv.Atoi(m[1])
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "elemen:") || strings.Contains(lower, " elemen:") {
			if current.ElemenCP != "" && current.TujuanPembelajaran != "" && current.IndikatorSoal != "" {
				flush()
			}
			parseBlueprintInlineHeader(trimmed, &current)
			continue
		}
		if strings.HasPrefix(lower, "materi:") {
			current.MateriPokok = strings.TrimSpace(trimmed[len("Materi:"):])
			continue
		}
		if strings.HasPrefix(lower, "tp:") {
			current.TujuanPembelajaran = cleanDraftValue(trimmed[len("TP:"):])
			continue
		}
		if strings.HasPrefix(lower, "indikator:") {
			current.IndikatorSoal = cleanDraftValue(trimmed[len("Indikator:"):])
			continue
		}
	}
	flush()
	for i := range slots {
		if slots[i].Position <= 0 {
			slots[i].Position = i + 1
		}
		slots[i].CognitiveLevel = normalizeCognitiveLevel(slots[i].CognitiveLevel)
		slots[i].QuestionType = normalizeQuestionType(slots[i].QuestionType)
	}
	return slots
}

func parseBlueprintInlineHeader(line string, slot *agentBlueprintSlotDraft) {
	value := line
	if idx := strings.Index(strings.ToLower(value), "elemen:"); idx >= 0 {
		value = value[idx+len("elemen:"):]
	}
	parts := splitBlueprintHeaderParts(value)
	if len(parts) > 0 {
		slot.ElemenCP = strings.TrimSpace(parts[0])
	}
	for _, p := range parts[1:] {
		p = strings.TrimSpace(p)
		upper := strings.ToUpper(p)
		if strings.HasPrefix(upper, "C") && len(upper) >= 2 {
			slot.CognitiveLevel = normalizeCognitiveLevel(upper)
			continue
		}
		lower := strings.ToLower(p)
		if strings.Contains(lower, "pilihan ganda") || strings.EqualFold(p, "pg") || strings.EqualFold(p, "mc") {
			slot.QuestionType = "multiple_choice"
		} else if strings.Contains(lower, "uraian") || strings.Contains(lower, "esai") || strings.Contains(lower, "essay") {
			slot.QuestionType = "essay"
		} else if strings.Contains(lower, "benar") || strings.Contains(lower, "salah") || strings.Contains(lower, "true") || strings.Contains(lower, "false") {
			slot.QuestionType = "true_false"
		} else if strings.Contains(lower, "singkat") || strings.Contains(lower, "short") {
			slot.QuestionType = "short_answer"
		}
	}
}

func splitBlueprintHeaderParts(value string) []string {
	parts := strings.Split(value, "·")
	if len(parts) > 1 {
		return parts
	}
	return strings.Split(value, "-")
}

func cleanDraftValue(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "("); idx > 0 && strings.Contains(strings.ToLower(s[idx:]), "a:") {
		s = strings.TrimSpace(s[:idx])
	}
	return s
}

func inferDraftRange(slots []agentBlueprintSlotDraft) string {
	if len(slots) == 0 {
		return ""
	}
	start := slots[0].Position
	end := slots[len(slots)-1].Position
	return strconv.Itoa(start) + "-" + strconv.Itoa(end)
}
