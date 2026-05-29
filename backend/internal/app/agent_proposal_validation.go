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

func buildAgentProposalValidationMessage(fields map[string]string) string {
	if len(fields) == 0 {
		return "Proposal belum bisa disimpan karena validasi gagal."
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("Saya belum bisa menyimpan proposal ini karena masih ada bagian yang belum memenuhi aturan.\n\n")
	b.WriteString("Perlu diperbaiki:\n")
	for _, key := range keys {
		b.WriteString(fmt.Sprintf("- %s: %s\n", key, fields[key]))
	}
	b.WriteString("\nSilakan minta AI membuat revisi baru atau ubah kisi-kisi manual sebelum disimpan.")
	return b.String()
}
