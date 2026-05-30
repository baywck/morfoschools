package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func (a *App) validateAgentProposalBeforeConfirm(ctx context.Context, tenantID, userID string, workflow agentWorkflow, raw json.RawMessage) map[string]string {
	switch workflow {
	case agentWorkflowCreateBlueprintSlots:
		var args agentCreateBlueprintSlotsArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return map[string]string{"proposal": "Proposal tidak bisa dibaca"}
		}
		return a.validateAgentCreateBlueprintSlotsArgs(ctx, tenantID, userID, args)
	}
	return nil
}

// buildAgentProposalValidationMessage creates validation feedback message.
// Uses LLM if available, falls back to structural format.
func (a *App) buildAgentProposalValidationMessageWithLLM(ctx context.Context, tenantID, userID string, fields map[string]string) string {
	if len(fields) == 0 {
		return "Proposal belum bisa disimpan karena validasi gagal."
	}

	// Build structural context for LLM
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var detail strings.Builder
	for _, key := range keys {
		detail.WriteString(fmt.Sprintf("- %s: %s\n", key, fields[key]))
	}

	prompt := "Kamu adalah AI assistant untuk sistem LMS. Proposal kisi-kisi user tidak lolos validasi. " +
		"Buat pesan singkat dalam Bahasa Indonesia yang menjelaskan masalahnya dan cara memperbaiki. " +
		"Maksimal 4-5 baris. Jangan emoji berlebihan."
	userCtx := fmt.Sprintf("Validation errors:\n%s", detail.String())
	msg := a.askLLMForMessage(ctx, tenantID, userID, prompt, userCtx)
	if msg != "" {
		return msg
	}

	// Fallback to structural message
	return buildAgentProposalValidationMessageFallback(fields)
}

// buildAgentProposalValidationMessageFallback is the structural fallback.
func buildAgentProposalValidationMessageFallback(fields map[string]string) string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("Proposal belum bisa disimpan karena masih ada bagian yang belum memenuhi aturan.\n\n")
	b.WriteString("Perlu diperbaiki:\n")
	for _, key := range keys {
		b.WriteString(fmt.Sprintf("- %s: %s\n", key, fields[key]))
	}
	return b.String()
}
