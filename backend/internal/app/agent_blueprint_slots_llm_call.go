package app

import (
	"context"
	"fmt"
	"strings"
)

func (a *App) callBlueprintSlotsLLMJSON(ctx context.Context, provider resolvedAIProvider, prompt, userMessage string) (string, error) {
	extra := map[string]any{"response_format": map[string]string{"type": "json_object"}}
	// 8000 tokens: enough for ~15-20 slots with full CP/TP/materi/indikator content
	resp, err := a.callLLMWithProviderOptions(ctx, provider, []llmMessage{{Role: "system", Content: prompt}, {Role: "user", Content: userMessage}}, 0.2, 12000, extra)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty blueprint LLM response")
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	if start := strings.Index(content, "{"); start >= 0 {
		if end := strings.LastIndex(content, "}"); end > start {
			content = content[start : end+1]
		}
	}
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("empty blueprint LLM content")
	}
	return content, nil
}
