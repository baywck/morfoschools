package app

import (
	"context"
	"encoding/json"
	"strings"
)

func (a *App) generateAgentActionPlanFromLLM(ctx context.Context, tenantID, userID string, req agentActionPlanRequest, message string) (agentPlannedAction, error) {
	provider, err := a.resolveAIProvider(ctx, &AuthContext{UserID: userID, EffectiveTenantID: &tenantID}, tenantID)
	if err != nil {
		return agentPlannedAction{}, err
	}
	system := `Kamu adalah planner eksekusi AI Agent Morfoschools. Balas JSON valid saja, tanpa markdown.
Tugasmu menghasilkan rencana eksekusi bertahap yang bisa dijalankan backend secara batch.
Gunakan workflow yang sudah didukung sistem bila memungkinkan: create_exam, edit_exam, create_exam_section, create_blueprint_slots, edit_blueprint_slot, edit_blueprint_slots.
Jika pekerjaan besar tidak punya workflow khusus, gunakan workflow generik yang aman dan konsisten: misalnya action_plan_batch.
Jangan mengarang field yang tidak diperlukan.
Setiap batch harus realistis untuk dieksekusi.
Gunakan progressUnits untuk merepresentasikan bobot kerja relatif.
Output JSON:
{
  "scopeType": "exam|blueprint|question_set|section|generic",
  "source": "chat|audit|reverse_planning|create|update|repair|bulk",
  "goal": "",
  "intentSummary": "",
  "planJson": {},
  "batches": [
    {"batchIndex":1,"actionType":"analyze|create|update|repair|merge|link|generate|finalize","workflow":"","targetType":"blueprint_slot|question|exam|section|question_set|generic","targetIds":[],"argsJson":{},"preview":"","progressUnits":1}
  ]
}
Rules:
- Jika user meminta audit + repair besar, pecah menjadi batch bertahap.
- Jika user meminta reverse planning dari banyak soal, pecah menjadi batch analisis + merge.
- Jika permintaan hanya satu langkah, tetap kembalikan 1 batch.
- Jangan takut membuat 3-6 batch untuk pekerjaan besar.
- Pastikan batchIndex mulai dari 1 dan naik berurutan.`
	user := message
	if user == "" {
		user = req.Goal
	}
	extra := map[string]any{"response_format": map[string]string{"type": "json_object"}}
	resp, err := a.callLLMWithProviderOptions(ctx, provider, []llmMessage{{Role: "system", Content: system}, {Role: "user", Content: user}}, 0.2, 2400, extra)
	if err != nil {
		return agentPlannedAction{}, err
	}
	if len(resp.Choices) == 0 {
		return agentPlannedAction{}, err
	}
	raw := extractJSONObject(resp.Choices[0].Message.Content)
	var out agentPlannedAction
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return agentPlannedAction{}, err
	}
	if len(out.Batches) == 0 {
		return agentPlannedAction{}, err
	}
	for i := range out.Batches {
		if out.Batches[i].BatchIndex == 0 {
			out.Batches[i].BatchIndex = i + 1
		}
		if out.Batches[i].Workflow == "" {
			out.Batches[i].Workflow = "action_plan_batch"
		}
		if out.Batches[i].ActionType == "" {
			out.Batches[i].ActionType = "update"
		}
		if out.Batches[i].TargetType == "" {
			out.Batches[i].TargetType = "generic"
		}
	}
	if out.ScopeType == "" {
		out.ScopeType = req.ScopeType
	}
	if out.Source == "" {
		out.Source = req.Source
	}
	if out.Goal == "" {
		out.Goal = req.Goal
	}
	if out.IntentSummary == "" {
		out.IntentSummary = strings.TrimSpace(user)
	}
	return out, nil
}
