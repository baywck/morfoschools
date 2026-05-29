package app

import (
	"context"
	"fmt"
	"strings"
)

func (a *App) callBlueprintSlotsLLMJSON(ctx context.Context, provider resolvedAIProvider, prompt, userMessage string) (string, error) {
	resp, err := a.callLLMWithProvider(ctx, provider, []llmMessage{{Role: "system", Content: prompt}, {Role: "user", Content: userMessage}})
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
