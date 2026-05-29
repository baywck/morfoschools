package app

import (
	"context"
	"strings"
)

func (a *App) lastAssistantBlueprintProposalPrompt(ctx context.Context, sessionID string) (string, bool) {
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
	ok := (strings.Contains(lower, "kisi-kisi") || strings.Contains(lower, "kisi kisi") || strings.Contains(lower, "blueprint")) &&
		strings.Contains(lower, "buatkan proposal")
	return content, ok
}

func (a *App) lastAssistantAskedForBlueprintProposal(ctx context.Context, sessionID string) bool {
	_, ok := a.lastAssistantBlueprintProposalPrompt(ctx, sessionID)
	return ok
}
