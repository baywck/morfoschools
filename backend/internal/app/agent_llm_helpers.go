package app

import (
	"context"
	"fmt"
	"strings"
)

// askLLMForMessage is a lightweight LLM call for generating short contextual messages.
// Used when backend needs to communicate with user but must NOT author the message itself.
// Returns the LLM-generated message or a minimal fallback if LLM fails.
func (a *App) askLLMForMessage(ctx context.Context, tenantID, userID, prompt, userContext string) string {
	provider, err := a.resolveAIProvider(ctx, &AuthContext{UserID: userID, EffectiveTenantID: &tenantID}, tenantID)
	if err != nil {
		return ""
	}
	messages := []llmMessage{
		{Role: "system", Content: prompt},
		{Role: "user", Content: userContext},
	}
	resp, err := a.callLLMWithProviderOptions(ctx, provider, messages, 0.3, 500, nil)
	if err != nil || len(resp.Choices) == 0 {
		return ""
	}
	out := strings.TrimSpace(resp.Choices[0].Message.Content)
	if out == "" {
		return ""
	}
	return out
}

// askLLMForActionPlanMessage generates a user-facing message about an action plan creation/execution.
func (a *App) askLLMForActionPlanMessage(ctx context.Context, tenantID, userID, sessionID, eventType string, detail agentActionPlanDetail, extra string) string {
	planSummary := planPreview(detail, 5)
	prompt := "Kamu adalah AI assistant untuk sistem LMS. Buat pesan singkat dalam Bahasa Indonesia untuk user. " +
		"Gunakan format yang jelas dan ringkas. Jangan gunakan placeholder atau template yang kosong. " +
		"Jangan gunakan emoji berlebihan. Maksimal 3-4 kalimat."
	userCtx := fmt.Sprintf("Event: %s\nPlan goal: %s\nPlan summary:\n%s\n%s", eventType, detail.Goal, planSummary, extra)
	msg := a.askLLMForMessage(ctx, tenantID, userID, prompt, userCtx)
	if msg == "" {
		return planSummary // fallback to structural summary if LLM fails
	}
	return msg
}

// askLLMForErrorMessage generates a user-facing error/helpful message when an operation fails.
func (a *App) askLLMForErrorMessage(ctx context.Context, tenantID, userID, situation, detail string) string {
	prompt := "Kamu adalah AI assistant untuk sistem LMS. User mengalami masalah. Buat pesan singkat yang helpful dalam Bahasa Indonesia. " +
		"Jelaskan masalahnya dan sarankan langkah selanjutnya. Maksimal 2-3 kalimat. Jangan gunakan emoji berlebihan."
	userCtx := fmt.Sprintf("Situasi: %s\nDetail: %s", situation, detail)
	msg := a.askLLMForMessage(ctx, tenantID, userID, prompt, userCtx)
	if msg == "" {
		return fmt.Sprintf("Terjadi masalah: %s. Silakan coba lagi.", situation)
	}
	return msg
}
