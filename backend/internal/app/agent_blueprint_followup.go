package app

import (
	"context"
	"strings"
)

func (a *App) lastAssistantBlueprintDraft(ctx context.Context, sessionID string) (string, bool) {
	var content string
	err := a.db.QueryRowContext(ctx, `
		SELECT content
		FROM ai_messages
		WHERE session_id=$1 AND role='assistant'
		ORDER BY created_at DESC
		LIMIT 1
	`, sessionID).Scan(&content)
	if err != nil {
		return "", false
	}
	lower := strings.ToLower(content)
	hasBlueprintTerms := strings.Contains(lower, "kisi-kisi") || strings.Contains(lower, "kisi kisi") || strings.Contains(lower, "blueprint") || strings.Contains(lower, "elemen:")
	hasSlotShape := strings.Contains(lower, "elemen:") && strings.Contains(lower, "tp:") && strings.Contains(lower, "indikator:")
	return content, hasBlueprintTerms && hasSlotShape
}
