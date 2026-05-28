package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func (a *App) extractAgentIntent(ctx context.Context, tenantID, userID string, roles []string, sessionID string, req aiChatRequest) (agentIntentResponse, error) {
	subjects := a.agentSubjectCatalog(ctx, tenantID)
	conversationContext := a.agentRecentConversationContext(ctx, sessionID)
	prompt := `Kamu adalah intent extractor. Balas JSON valid saja, tanpa markdown dan tanpa penjelasan.
Intent/workflow yang didukung sekarang: create_exam, edit_exam, create_exam_section, discussion, unsupported.
WAJIB pilih create_exam jika user meminta buat/create/bikin exam/ujian/tes/kuis baru.
WAJIB pilih edit_exam jika user meminta ubah/edit/update/ganti detail exam/ujian yang sudah ada.
WAJIB pilih create_exam_section jika user meminta buat/tambah section/bagian/sesi baru pada exam aktif/questions manager.
Untuk create_exam, args boleh berisi: title, description, subjectId, subjectName, gradeLevel, examType, durationMinutes, maxScore, passingScore, usesKisiKisi.
Untuk edit_exam, args boleh berisi: examId, title, description, subjectId, subjectName, gradeLevel, examType, durationMinutes, maxScore, passingScore, shuffleQuestions, shuffleOptions, showResultImmediately, usesKisiKisi.
Untuk create_exam_section, args boleh berisi: examId, title, description, sortOrder.
examType valid: quiz, midterm, final, tryout, daily. UAS/Ujian Akhir Semester => final. UTS => midterm. Kenaikan kelas/semester genap/sumatif akhir tahun => final.
Jika user meminta kisi-kisi aktif, set usesKisiKisi=true. Jika user meminta matikan/nonaktifkan kisi-kisi, set usesKisiKisi=false.
Subject valid tenant: ` + subjects + `
Active entities dari halaman: ` + activeEntitiesJSON(req.Shadow.ActiveEntities) + `
Konteks percakapan terbaru: ` + conversationContext + `
Untuk create_exam lanjutan seperti "buat ujiannya dulu", ambil subject/grade/exam type/title dari konteks percakapan terbaru. Jangan pakai judul generik "Ujian Baru" jika konteks berisi Pendidikan Pancasila/Kelas/UAS/kenaikan kelas.
Untuk edit_exam/create_exam_section, jika activeEntities berisi examId dan user tidak menyebut ID lain, pakai examId aktif itu.
Untuk edit_exam lanjutan seperti "ubah namanya/judulnya jadi ..." setelah exam baru berhasil dibuat, pakai examId dari hasil workflow create_exam terakhir di konteks percakapan terbaru.
Jika user menyebut mapel, pilih subjectName PERSIS dari daftar subject valid. Jangan gabungkan subject dengan kata tambahan judul.
Contoh: "Matematika Disposable Agent Test" => subjectName "Matematika", title boleh memuat "Disposable Agent Test".
Jika create_exam title tidak eksplisit, buat title wajar dari jenis ujian + mapel + kelas, contoh "Ujian Akhir Semester Matematika - 11".
Field wajib create_exam hanya title. Field wajib edit_exam adalah examId dan minimal satu perubahan. Field wajib create_exam_section adalah examId dan title.
missingFields hanya untuk field wajib yang benar-benar tidak bisa disimpulkan.
Output shape: {"intent":"create_exam|edit_exam|create_exam_section|discussion|unsupported","workflow":"create_exam|edit_exam|create_exam_section","args":{},"missingFields":[]}`
	messages := []llmMessage{{Role: "system", Content: prompt}, {Role: "user", Content: req.Message}}
	provider, providerErr := a.resolveAIProvider(ctx, &AuthContext{UserID: userID, Roles: roles, EffectiveTenantID: &tenantID}, tenantID)
	if providerErr != nil {
		return agentIntentResponse{}, providerErr
	}
	resp, err := a.callLLMWithProvider(ctx, provider, messages)
	if err != nil {
		return agentIntentResponse{}, err
	}
	if len(resp.Choices) == 0 {
		return agentIntentResponse{}, fmt.Errorf("empty intent response")
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
	var out agentIntentResponse
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return agentIntentResponse{}, err
	}
	if len(out.Args) == 0 || string(out.Args) == "null" {
		out.Args = json.RawMessage(`{}`)
	}
	return out, nil
}

func (a *App) agentRecentConversationContext(ctx context.Context, sessionID string) string {
	if sessionID == "" {
		return "[]"
	}
	rows, err := a.db.QueryContext(ctx, `SELECT role, content FROM ai_messages WHERE session_id=$1 ORDER BY created_at DESC LIMIT 8`, sessionID)
	if err != nil {
		return "[]"
	}
	defer rows.Close()
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	items := []msg{}
	for rows.Next() {
		var m msg
		if err := rows.Scan(&m.Role, &m.Content); err == nil {
			if len(m.Content) > 1200 {
				m.Content = m.Content[:1200]
			}
			items = append(items, m)
		}
	}
	b, _ := json.Marshal(items)
	return string(b)
}

func activeEntitiesJSON(active map[string]string) string {
	if active == nil {
		return "{}"
	}
	b, _ := json.Marshal(active)
	return string(b)
}

func (a *App) agentSubjectCatalog(ctx context.Context, tenantID string) string {
	rows, err := a.db.QueryContext(ctx, `SELECT name FROM subjects WHERE tenant_id=$1 ORDER BY name LIMIT 50`, tenantID)
	if err != nil {
		return "[]"
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil && strings.TrimSpace(name) != "" {
			names = append(names, strings.TrimSpace(name))
		}
	}
	b, _ := json.Marshal(names)
	return string(b)
}
