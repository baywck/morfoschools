package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type agentTurnClassification struct {
	Mode               string          `json:"mode"`
	Workflow           string          `json:"workflow,omitempty"`
	Args               json.RawMessage `json:"args,omitempty"`
	Confidence         float64         `json:"confidence"`
	Reason             string          `json:"reason,omitempty"`
	NeedsClarification bool            `json:"needsClarification,omitempty"`
}

func (a *App) classifyAgentTurn(ctx context.Context, tenantID, userID string, roles []string, sessionID string, req aiChatRequest) (agentTurnClassification, error) {
	prompt := a.agentTurnClassifierPrompt(ctx, tenantID, sessionID, req)
	provider, err := a.resolveAIProvider(ctx, &AuthContext{UserID: userID, Roles: roles, EffectiveTenantID: &tenantID}, tenantID)
	if err != nil {
		return agentTurnClassification{}, err
	}
	extra := map[string]any{"response_format": map[string]string{"type": "json_object"}}
	resp, err := a.callLLMWithProviderOptions(ctx, provider, []llmMessage{{Role: "system", Content: prompt}, {Role: "user", Content: req.Message}}, 0.1, 900, extra)
	if err != nil {
		return agentTurnClassification{}, err
	}
	if len(resp.Choices) == 0 {
		return agentTurnClassification{}, fmt.Errorf("empty classifier response")
	}
	content := extractJSONObject(resp.Choices[0].Message.Content)
	var out agentTurnClassification
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return agentTurnClassification{}, err
	}
	out.Mode = strings.ToLower(strings.TrimSpace(out.Mode))
	out.Workflow = strings.TrimSpace(out.Workflow)
	if len(out.Args) == 0 || string(out.Args) == "null" {
		out.Args = json.RawMessage(`{}`)
	}
	return out, nil
}

func (a *App) agentTurnClassifierPrompt(ctx context.Context, tenantID, sessionID string, req aiChatRequest) string {
	var b strings.Builder
	b.WriteString("Kamu adalah classifier intent untuk AI Agent Morfoschools. Balas JSON object valid saja. Jangan markdown. ")
	b.WriteString("Tugasmu HANYA menentukan mode percakapan, bukan membuat konten dan bukan menjalankan aksi. ")
	b.WriteString("Mode valid: discussion, planning, clarification, proposal_request, unsupported. ")
	b.WriteString("Workflow valid saat proposal_request: create_exam, edit_exam, create_exam_section, create_blueprint_slots. ")
	b.WriteString("Aturan penting: planning/diskusi kisi-kisi bukan proposal. Kalimat seperti 'aku ingin membuat kisi-kisi', 'aku berencana membuat 50 soal', 'bantu aku membuat kisi-kisi', 'ayo diskusi kisi-kisi' => planning/clarification, workflow kosong. ")
	b.WriteString("Proposal_request hanya jika user eksplisit meminta dibuatkan/disusun/generate/disimpan/ajukan proposal sekarang, atau lanjutan konteks yang jelas seperti 'mari kita buat 5 dulu' setelah membahas kisi-kisi. Kata 'simpan', 'save', 'ok simpan', 'simpan dulu' setelah assistant memberi draft kisi-kisi/soal berarti proposal_request untuk menyimpan draft tersebut, bukan discussion. ")
	b.WriteString("PENTING khusus halaman kisi-kisi: jika route mengandung 'kisi-kisi' dan ada examId aktif, perintah eksekusi membuat slot kisi-kisi (mis. 'buatkan 10 slot', 'buat 10 kisi-kisi', 'langsung buatkan 10', 'buatkan saja 10 sekaligus', 'tidak usah preview, buat 10') WAJIB mode=proposal_request dengan workflow=create_blueprint_slots. Jangan pernah membalas daftar slot sebagai teks diskusi; pembuatan slot HARUS lewat proposal. ")
	b.WriteString("Angka jumlah soal dalam kalimat rencana tidak cukup untuk proposal. Jumlah menjadi proposal hanya jika ada perintah eksekusi sekarang. ")
	b.WriteString("Jika user minta delete/hapus exam, mode unsupported. ")
	b.WriteString("Output shape: {\"mode\":\"discussion|planning|clarification|proposal_request|unsupported\",\"workflow\":\"\",\"args\":{},\"confidence\":0.0,\"reason\":\"...\",\"needsClarification\":false}. ")
	b.WriteString("Active route: ")
	b.WriteString(req.Shadow.Route)
	b.WriteString(" Active entities: ")
	b.WriteString(activeEntitiesJSON(req.Shadow.ActiveEntities))
	b.WriteString(" AgentContextPack JSON: ")
	b.WriteString(a.agentContextPackJSON(ctx, tenantID, sessionID, req.Shadow.ActiveEntities))
	if examID := strings.TrimSpace(req.Shadow.ActiveEntities["examId"]); examID != "" {
		ctxResp, err := a.ensureExamCurriculumContext(ctx, tenantID, examID)
		if err == nil {
			b.WriteString(" Exam context: subject=")
			b.WriteString(ctxResp.SubjectName)
			b.WriteString(" grade=")
			b.WriteString(ctxResp.GradeLevel)
			b.WriteString(" phase=")
			b.WriteString(ctxResp.Phase)
			b.WriteString(" cpStatus=")
			b.WriteString(ctxResp.Status)
			b.WriteString(".")
		}
	}
	return b.String()
}

func extractJSONObject(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	if start := strings.Index(content, "{"); start >= 0 {
		if end := strings.LastIndex(content, "}"); end > start {
			return content[start : end+1]
		}
	}
	return content
}
