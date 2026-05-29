package app

import "strings"

type blueprintSlotsLLMOutput struct {
	Topic string                    `json:"topic"`
	Slots []agentBlueprintSlotDraft `json:"slots"`
}

func (a *App) fallbackBlueprintSlotsDraft(ctxResp examCurriculumContextResponse, message string, count int) (blueprintSlotsLLMOutput, bool) {
	if count <= 0 {
		count = 5
	}
	if count > 20 {
		count = 20
	}
	topic := inferBlueprintTopic(message)
	if topic == "" {
		topic = ctxResp.SubjectName
	}
	cp := "Draft CP perlu diverifikasi manual"
	elementName := "Elemen umum"
	elementContent := "Peserta didik menunjukkan pemahaman sesuai capaian pembelajaran fase terkait."
	if ctxResp.Reference != nil && strings.TrimSpace(ctxResp.Reference.GeneralCP) != "" {
		cp = ctxResp.Reference.GeneralCP
	}
	if len(ctxResp.Elements) > 0 {
		elementName = ctxResp.Elements[0].Name
		elementContent = ctxResp.Elements[0].Content
	}
	levels := []string{"C2", "C3", "C4", "C4", "C5"}
	slots := make([]agentBlueprintSlotDraft, 0, count)
	for i := 0; i < count; i++ {
		level := levels[i%len(levels)]
		kko := "menjelaskan"
		switch level {
		case "C3":
			kko = "menerapkan"
		case "C4":
			kko = "menganalisis"
		case "C5":
			kko = "menilai"
		case "C6":
			kko = "merancang"
		}
		materi := topic
		if count > 1 {
			materi = topic + " bagian " + string(rune('A'+i))
		}
		slots = append(slots, agentBlueprintSlotDraft{
			Position:            i + 1,
			CapaianPembelajaran: cp,
			ElemenCP:            elementName + " — " + elementContent,
			TujuanPembelajaran:  "Peserta didik dapat " + kko + " konsep " + topic + " dalam konteks kehidupan sehari-hari.",
			MateriPokok:         materi,
			CognitiveLevel:      level,
			IndikatorSoal:       "Disajikan stimulus kontekstual tentang " + topic + ", peserta didik dapat " + kko + " aspek yang relevan secara tepat.",
			QuestionType:        "multiple_choice",
			Points:              1,
			SourceConfidence:    ctxResp.Status,
		})
	}
	return blueprintSlotsLLMOutput{Topic: topic, Slots: slots}, true
}

func inferBlueprintTopic(message string) string {
	message = strings.TrimSpace(message)
	lower := strings.ToLower(message)
	markers := []string{"tentang", "materi", "topik"}
	for _, marker := range markers {
		if idx := strings.Index(lower, marker); idx >= 0 {
			part := strings.TrimSpace(message[idx+len(marker):])
			part = strings.Trim(part, " .,!?:;\n\t")
			if part != "" {
				return part
			}
		}
	}
	return "materi exam ini"
}
