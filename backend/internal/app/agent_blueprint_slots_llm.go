package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type blueprintSlotsLLMOutput struct {
	Topic string                    `json:"topic"`
	Slots []agentBlueprintSlotDraft `json:"slots"`
}

func (a *App) generateBlueprintSlotsDraft(ctx context.Context, tenantID, userID string, req aiChatRequest, lower string) (agentCreateBlueprintSlotsArgs, error) {
	examID := strings.TrimSpace(req.Shadow.ActiveEntities["examId"])
	count := blueprintSlotCountOrDefault(lower)
	ctxResp, _ := a.ensureExamCurriculumContext(ctx, tenantID, examID)
	warnings := append([]string{}, ctxResp.Warnings...)
	if ctxResp.Status != "ready" {
		warnings = append(warnings, "CP resmi belum siap; kisi-kisi wajib diverifikasi manual sebelum dipakai.")
	}
	contextPack := a.agentContextPackJSONForTurn(ctx, tenantID, strings.TrimSpace(req.SessionID), req.Shadow.ActiveEntities, req.Message)
	prompt := a.blueprintSlotPrompt(req.Message, count, ctxResp)
	prompt += " AgentContextPack JSON berikut adalah memori kerja wajib. Gunakan untuk memahami target, draft sebelumnya, distribusi existing, dan menghindari duplikasi. Jangan membuat slot yang bertentangan dengan draft yang sudah disetujui user. " + contextPack
	prompt += " Jika user meminta 'lanjut' / 'lanjut 5 slot lainnya', lanjutkan rencana distribusi yang sudah disepakati di percakapan dan perhatikan existing blueprint stats. Jangan mengulang materi/indikator yang sudah ada. Jaga kualitas pedagogis lintas mapel: materi harus sesuai CP, fase/kelas, dan tingkat asesmen; indikator harus menguji pemahaman/aplikasi/penalaran sesuai level kognitif yang dipilih, bukan sekadar trivia/hafalan dangkal kecuali distribusi memang membutuhkan C1. Setiap batch harus menjaga keseimbangan elemen CP, level kognitif, dan questionType menuju target akhir. "
	if strings.Contains(strings.ToLower(req.Message), "simpan") {
		prompt += " User meminta menyimpan draft dari percakapan sebelumnya. Ekstrak/rekonstruksi slot dari AgentContextPack/recentMessages jika tersedia; jangan membuat topik baru yang tidak sesuai draft sebelumnya."
	}
	provider, err := a.resolveAIProvider(ctx, &AuthContext{UserID: userID, EffectiveTenantID: &tenantID}, tenantID)
	if err != nil {
		return agentCreateBlueprintSlotsArgs{}, err
	}
	content, err := a.callBlueprintSlotsLLMJSON(ctx, provider, prompt, req.Message)
	if err != nil {
		return agentCreateBlueprintSlotsArgs{}, err
	}
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	if start := strings.Index(content, "{"); start >= 0 {
		if end := strings.LastIndex(content, "}"); end > start {
			content = content[start : end+1]
		}
	}
	var out blueprintSlotsLLMOutput
	if err := json.Unmarshal([]byte(content), &out); err != nil || len(out.Slots) == 0 {
		repairPrompt := prompt + " Output sebelumnya tidak valid. Perbaiki menjadi JSON valid saja, tepat sesuai shape, tanpa markdown."
		repaired, repairErr := a.callBlueprintSlotsLLMJSON(ctx, provider, repairPrompt, "Perbaiki draft kisi-kisi ini menjadi JSON valid: "+content)
		if repairErr == nil {
			content = repaired
			err = json.Unmarshal([]byte(content), &out)
		}
		if err != nil || len(out.Slots) == 0 {
			return agentCreateBlueprintSlotsArgs{}, fmt.Errorf("invalid blueprint JSON: %w", err)
		}
		warnings = append(warnings, "LLM membutuhkan repair pass agar JSON proposal valid; review manual tetap wajib.")
	}
	for i := range out.Slots {
		if out.Slots[i].Position <= 0 {
			out.Slots[i].Position = i + 1
		}
		out.Slots[i].CognitiveLevel = normalizeCognitiveLevel(out.Slots[i].CognitiveLevel)
		out.Slots[i].QuestionType = normalizeQuestionType(out.Slots[i].QuestionType)
		if out.Slots[i].Points <= 0 {
			out.Slots[i].Points = 1
		}
		if out.Slots[i].SourceConfidence == "" {
			out.Slots[i].SourceConfidence = ctxResp.Status
		}
	}
	out.Slots = a.repairInvalidBlueprintSlots(ctx, provider, prompt, out.Slots)
	return agentCreateBlueprintSlotsArgs{ExamID: examID, Topic: out.Topic, Slots: out.Slots, Warnings: warnings, CPStatus: ctxResp.Status, CPSource: ctxResp.Source, Confirmable: true}, nil
}

func (a *App) blueprintSlotPrompt(userMessage string, count int, ctxResp examCurriculumContextResponse) string {
	var b strings.Builder
	b.WriteString("Kamu menyusun kisi-kisi Kurikulum Merdeka untuk exam aktif. Balas JSON object valid saja tanpa markdown. Root harus object, bukan array. ")
	b.WriteString("JANGAN gunakan KD/SK/Kompetensi Dasar/Standar Kompetensi. Basis wajib CP, Elemen CP, TP, materi, indikator soal. ")
	b.WriteString("Indikator soal wajib menyebut stimulus dan diawali/berisi pola 'Disajikan ... peserta didik dapat ...'. Satu indikator tepat untuk satu soal. ")
	b.WriteString("Level kognitif C1-C6; KKO TP dan indikator harus selaras. HOTS C4-C6 harus punya stimulus. TP wajib mengikuti prinsip A-B-C-D: Audience 'Peserta didik', Behavior berupa satu KKO terukur sesuai level, Condition/konteks pembelajaran, dan Degree/kriteria eksplisit seperti 'minimal dua', 'dengan tepat', atau 'berdasarkan prinsip/kriteria'. Contoh TP C4: 'Peserta didik dapat menganalisis struktur teks editorial berdasarkan kutipan teks aktual dengan tepat.' Jangan membuat TP tanpa audience, tanpa KKO, atau tanpa degree. Jangan memakai lebih dari satu kompetensi dalam satu TP. Indikator wajib berbasis stimulus dan satu indikator hanya untuk satu soal. Kualitas wajib lintas mapel: materi dan indikator harus sesuai CP, fase/kelas, dan tujuan asesmen; jangan memilih materi trivia/hafalan dangkal jika level kognitif atau konteks exam membutuhkan penerapan, analisis, evaluasi, atau kreasi. ")
	b.WriteString(fmt.Sprintf("Buat tepat %d slot. ", count))
	b.WriteString("QuestionType hanya multiple_choice, true_false, short_answer, essay. Default multiple_choice jika user tidak menentukan. Points default 1. ")
	b.WriteString("Output shape wajib persis: {\"topic\":\"...\",\"slots\":[{\"position\":1,\"capaianPembelajaran\":\"...\",\"elemenCp\":\"...\",\"tujuanPembelajaran\":\"...\",\"materiPokok\":\"...\",\"cognitiveLevel\":\"C4\",\"indikatorSoal\":\"Disajikan ... peserta didik dapat ...\",\"questionType\":\"multiple_choice\",\"points\":1}]}. Jangan tulis teks lain. ")
	b.WriteString("Konteks exam: subject=")
	b.WriteString(ctxResp.SubjectName)
	b.WriteString(", grade=")
	b.WriteString(ctxResp.GradeLevel)
	b.WriteString(", phase=")
	b.WriteString(ctxResp.Phase)
	b.WriteString(". ")
	if ctxResp.Reference != nil {
		b.WriteString("CP umum: ")
		b.WriteString(truncateForPrompt(ctxResp.Reference.GeneralCP, 1800))
		b.WriteString(" Elemen CP: ")
		for _, el := range ctxResp.Elements {
			b.WriteString(el.Name)
			b.WriteString(": ")
			b.WriteString(truncateForPrompt(el.Content, 800))
			b.WriteString(" | ")
		}
	} else {
		b.WriteString("CP resmi tidak tersedia; jangan klaim berdasarkan CP resmi. Buat draft konservatif dan umum. ")
	}
	return b.String()
}

func truncateForPrompt(s string, max int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "..."
}
