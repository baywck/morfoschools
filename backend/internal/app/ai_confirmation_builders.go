package app

import (
	"encoding/json"
	"fmt"
	"strings"
)

// confirmation_builder — shared utilities for building rich, content-
// aware confirmation cards for AI write proposals. The principle: a
// user about to click "ya" must see EXACTLY what's about to land in
// the database. Vague messages like "Edit section." cause confusion
// and erode trust.
//
// Each builder takes the raw json.RawMessage args from the model's
// tool call and produces markdown that surfaces:
//   - identity of the target entity
//   - every field that will change, with the new value (truncated
//     for token economy when bodies are long)
//   - destructive markers when relevant
//
// Long fields (passages, descriptions) render as blockquotes; lists
// (questions, options, slots) render as bullet items; identifiers
// stay inline.
//
// Truncation policy:
//   title-like:  100 chars
//   body-like:   400 chars
//   excerpts:    160 chars

// truncate cuts s to n chars + ellipsis when over.
func truncateConfirm(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// blockquote prefixes every line with "> " for markdown blockquote
// rendering. Used for long-form fields like stimulus body, exam
// description, indikator.
func blockquote(s string) string {
	if s == "" {
		return ""
	}
	return "> " + strings.ReplaceAll(strings.TrimSpace(s), "\n", "\n> ")
}

// confirmDeleteQuestion.
func confirmDeleteQuestion(args json.RawMessage) string {
	var p struct{ QuestionID string `json:"questionId"` }
	_ = json.Unmarshal(args, &p)
	return fmt.Sprintf("**Hapus soal**\n\n⚠ Soal `%s` akan dihapus permanen.\n\n- Opsi + jawaban benar ikut hilang\n- Submission siswa yang merujuk soal ini tetap ada (snapshot di score history)\n- Aksi tidak bisa di-undo\n", p.QuestionID)
}

// confirmCreateExamSection.
func confirmCreateExamSection(args json.RawMessage) string {
	var p struct {
		ExamID      string  `json:"examId"`
		Title       string  `json:"title"`
		Description *string `json:"description"`
		SortOrder   *int    `json:"sortOrder"`
	}
	_ = json.Unmarshal(args, &p)
	var sb strings.Builder
	sb.WriteString("**Buat section baru**\n")
	fmt.Fprintf(&sb, "\n**Judul:** %s\n", truncateConfirm(p.Title, 100))
	if p.Description != nil && *p.Description != "" {
		sb.WriteString("\n**Deskripsi:**\n")
		sb.WriteString(blockquote(truncateConfirm(*p.Description, 300)))
		sb.WriteString("\n")
	}
	if p.SortOrder != nil {
		fmt.Fprintf(&sb, "\n**Urutan:** %d\n", *p.SortOrder+1)
	}
	return sb.String()
}

// confirmCreateQuestionGroup.
func confirmCreateQuestionGroup(args json.RawMessage) string {
	var p struct {
		ExamID        string  `json:"examId"`
		SectionID     *string `json:"sectionId"`
		StimulusID    *string `json:"stimulusId"`
		TitleSnapshot *string `json:"titleSnapshot"`
		BodySnapshot  *string `json:"bodySnapshot"`
		Name          *string `json:"name"`
	}
	_ = json.Unmarshal(args, &p)
	var sb strings.Builder
	sb.WriteString("**Buat group soal baru**\n")
	if p.Name != nil {
		fmt.Fprintf(&sb, "\n**Nama group:** %s\n", truncateConfirm(*p.Name, 100))
	}
	if p.SectionID != nil && *p.SectionID != "" {
		fmt.Fprintf(&sb, "\n**Section:** %s\n", *p.SectionID)
	}
	if p.StimulusID != nil && *p.StimulusID != "" {
		fmt.Fprintf(&sb, "\n**Link stimulus library:** %s\n", *p.StimulusID)
	}
	if p.TitleSnapshot != nil && *p.TitleSnapshot != "" {
		fmt.Fprintf(&sb, "\n**\U0001F4C4 Judul stimulus:** %s\n", truncateConfirm(*p.TitleSnapshot, 100))
	}
	if p.BodySnapshot != nil && *p.BodySnapshot != "" {
		sb.WriteString("\n**\U0001F4D6 Isi stimulus:**\n")
		sb.WriteString(blockquote(truncateConfirm(*p.BodySnapshot, 400)))
		sb.WriteString("\n")
	}
	return sb.String()
}

// confirmCreateStimulus.
func confirmCreateStimulus(args json.RawMessage) string {
	var p struct {
		Title     string  `json:"title"`
		Content   string  `json:"content"`
		Source    *string `json:"source"`
		Type      *string `json:"type"`
		Lifecycle *string `json:"lifecycle"`
	}
	_ = json.Unmarshal(args, &p)
	var sb strings.Builder
	sb.WriteString("**Buat stimulus baru di library**\n")
	if p.Title != "" {
		fmt.Fprintf(&sb, "\n**Judul:** %s\n", truncateConfirm(p.Title, 100))
	}
	if p.Content != "" {
		sb.WriteString("\n**Isi:**\n")
		sb.WriteString(blockquote(truncateConfirm(p.Content, 400)))
		sb.WriteString("\n")
	}
	if p.Source != nil && *p.Source != "" {
		fmt.Fprintf(&sb, "\n**Sumber:** %s\n", *p.Source)
	}
	if p.Type != nil {
		fmt.Fprintf(&sb, "\n**Tipe:** %s\n", *p.Type)
	}
	if p.Lifecycle != nil {
		fmt.Fprintf(&sb, "\n**Lifecycle:** %s\n", *p.Lifecycle)
	}
	return sb.String()
}

// confirmMoveQuestion.
func confirmMoveQuestion(args json.RawMessage) string {
	var p struct {
		QuestionID string  `json:"questionId"`
		SectionID  *string `json:"sectionId"`
		GroupID    *string `json:"groupId"`
		SortOrder  *int    `json:"sortOrder"`
	}
	_ = json.Unmarshal(args, &p)
	var sb strings.Builder
	fmt.Fprintf(&sb, "**Pindah / reorder soal** (`%s`)\n", p.QuestionID)
	if p.SectionID != nil {
		if *p.SectionID == "" {
			sb.WriteString("\n**Pindah:** lepas dari section\n")
		} else {
			fmt.Fprintf(&sb, "\n**Pindah ke section:** %s\n", *p.SectionID)
		}
	}
	if p.GroupID != nil {
		if *p.GroupID == "" {
			sb.WriteString("\n**Pindah:** lepas dari group (jadi standalone)\n")
		} else {
			fmt.Fprintf(&sb, "\n**Pindah ke group:** %s\n", *p.GroupID)
		}
	}
	if p.SortOrder != nil {
		fmt.Fprintf(&sb, "\n**Urutan baru:** %d\n", *p.SortOrder+1)
	}
	return sb.String()
}

// confirmUpdateExam — preview for update_exam (title, description,
// type, score, schedule).
func confirmUpdateExam(args json.RawMessage) string {
	var p struct {
		ExamID            string   `json:"examId"`
		Title             *string  `json:"title"`
		Description       *string  `json:"description"`
		ExamType          *string  `json:"examType"`
		DurationMinutes   *int     `json:"durationMinutes"`
		MaxScore          *float64 `json:"maxScore"`
		PassingScore      *float64 `json:"passingScore"`
		ShuffleQuestions  *bool    `json:"shuffleQuestions"`
		ShuffleOptions    *bool    `json:"shuffleOptions"`
		Status            *string  `json:"status"`
	}
	_ = json.Unmarshal(args, &p)
	var sb strings.Builder
	sb.WriteString("**Update exam**\n")
	if p.Title != nil {
		fmt.Fprintf(&sb, "\n**Judul baru:** %s\n", truncateConfirm(*p.Title, 100))
	}
	if p.Description != nil {
		sb.WriteString("\n**Deskripsi baru:**\n")
		sb.WriteString(blockquote(truncateConfirm(*p.Description, 400)))
		sb.WriteString("\n")
	}
	if p.ExamType != nil {
		fmt.Fprintf(&sb, "\n**Tipe:** %s\n", *p.ExamType)
	}
	if p.DurationMinutes != nil {
		fmt.Fprintf(&sb, "\n**Durasi:** %d menit\n", *p.DurationMinutes)
	}
	if p.MaxScore != nil {
		fmt.Fprintf(&sb, "\n**Nilai maksimum:** %.0f\n", *p.MaxScore)
	}
	if p.PassingScore != nil {
		fmt.Fprintf(&sb, "\n**Nilai lulus:** %.0f\n", *p.PassingScore)
	}
	if p.ShuffleQuestions != nil {
		fmt.Fprintf(&sb, "\n**Acak soal:** %v\n", *p.ShuffleQuestions)
	}
	if p.ShuffleOptions != nil {
		fmt.Fprintf(&sb, "\n**Acak opsi:** %v\n", *p.ShuffleOptions)
	}
	if p.Status != nil {
		fmt.Fprintf(&sb, "\n**Status:** %s\n", *p.Status)
	}
	return sb.String()
}

// confirmPublishExam — surfaces target id since publishing is irreversible.
func confirmPublishExam(args json.RawMessage) string {
	var p struct{ ExamID string `json:"examId"` }
	_ = json.Unmarshal(args, &p)
	return fmt.Sprintf("**Publish exam**\n\n⚠ Exam %s akan berubah dari draft → published.\n\nSetelah published:\n- Soal tidak bisa diedit lagi tanpa unpublish dulu\n- Siswa bisa mengikuti exam (kalau gate window aktif)\n- Reset progress + reseed status berlaku\n", p.ExamID)
}

// confirmUpdateExamSection — preview update_exam_section.
func confirmUpdateExamSection(args json.RawMessage) string {
	var p struct {
		SectionID   string  `json:"sectionId"`
		Title       *string `json:"title"`
		Description *string `json:"description"`
		SortOrder   *int    `json:"sortOrder"`
	}
	_ = json.Unmarshal(args, &p)
	var sb strings.Builder
	sb.WriteString("**Update section**\n")
	if p.Title != nil {
		fmt.Fprintf(&sb, "\n**Judul baru:** %s\n", truncateConfirm(*p.Title, 100))
	}
	if p.Description != nil {
		sb.WriteString("\n**Deskripsi baru:**\n")
		sb.WriteString(blockquote(truncateConfirm(*p.Description, 400)))
		sb.WriteString("\n")
	}
	if p.SortOrder != nil {
		fmt.Fprintf(&sb, "\n**Urutan:** %d\n", *p.SortOrder+1)
	}
	return sb.String()
}

// confirmDeleteExamSection — destructive, surface ID.
func confirmDeleteExamSection(args json.RawMessage) string {
	var p struct{ SectionID string `json:"sectionId"` }
	_ = json.Unmarshal(args, &p)
	return fmt.Sprintf("**Hapus section**\n\n⚠ Section `%s` akan dihapus.\n\n- Soal di dalam section TIDAK ikut dihapus\n- Mereka jadi unsectioned (akan masuk ke section default)\n- Aksi tidak bisa di-undo\n", p.SectionID)
}

// confirmDeleteQuestionGroup — destructive, surface ID.
func confirmDeleteQuestionGroup(args json.RawMessage) string {
	var p struct{ GroupID string `json:"groupId"` }
	_ = json.Unmarshal(args, &p)
	return fmt.Sprintf("**Hapus group**\n\n⚠ Group `%s` akan dihapus.\n\n- Soal di dalam group TIDAK ikut dihapus\n- Mereka jadi ungrouped (lepas dari stimulus group)\n- Stimulus snapshot ikut hilang\n- Aksi tidak bisa di-undo\n", p.GroupID)
}

// confirmUpdateStimulus — preview update_stimulus (master row, NOT
// snapshot). Mutates shared library.
func confirmUpdateStimulus(args json.RawMessage) string {
	var p struct {
		StimulusID string  `json:"stimulusId"`
		Title      *string `json:"title"`
		Content    *string `json:"content"`
		Source     *string `json:"source"`
		Type       *string `json:"type"`
	}
	_ = json.Unmarshal(args, &p)
	var sb strings.Builder
	sb.WriteString("**Update stimulus (master library)**\n")
	if p.Title != nil {
		fmt.Fprintf(&sb, "\n**Judul baru:** %s\n", truncateConfirm(*p.Title, 100))
	}
	if p.Content != nil {
		sb.WriteString("\n**Isi baru:**\n")
		sb.WriteString(blockquote(truncateConfirm(*p.Content, 400)))
		sb.WriteString("\n")
	}
	if p.Source != nil {
		fmt.Fprintf(&sb, "\n**Sumber:** %s\n", *p.Source)
	}
	if p.Type != nil {
		fmt.Fprintf(&sb, "\n**Tipe:** %s\n", *p.Type)
	}
	sb.WriteString("\n⚠ Update master row ini hanya mempengaruhi snapshot baru. Group existing yang sudah snapshot tetap pakai versi lama (snapshot-on-use).\n")
	return sb.String()
}

// confirmArchiveStimulus — destructive-ish (lifecycle change).
func confirmArchiveStimulus(args json.RawMessage) string {
	var p struct{ StimulusID string `json:"stimulusId"` }
	_ = json.Unmarshal(args, &p)
	return fmt.Sprintf("**Archive stimulus**\n\nStimulus `%s` akan dilepas dari library aktif (lifecycle → archived).\n\n- Snapshot existing di group tetap aman\n- Tidak bisa dipilih dari library picker lagi\n- Bisa di-restore via promote_stimulus\n", p.StimulusID)
}

// confirmPromoteStimulus.
func confirmPromoteStimulus(args json.RawMessage) string {
	var p struct{ StimulusID string `json:"stimulusId"` }
	_ = json.Unmarshal(args, &p)
	return fmt.Sprintf("**Promote stimulus ke library shared**\n\nStimulus `%s` akan diubah dari exam_scoped → shared.\n\n- Bisa dipakai di exam lain\n- Mutasi master row akan terlihat di group yang re-snapshot\n", p.StimulusID)
}

// confirmExamGate — buat / edit gate window.
func confirmCreateExamGate(args json.RawMessage) string {
	var p struct {
		ExamID      string  `json:"examId"`
		Mode        *string `json:"mode"`
		StartsAt    *string `json:"startsAt"`
		EndsAt      *string `json:"endsAt"`
		MaxAttempts *int    `json:"maxAttempts"`
		Description *string `json:"description"`
	}
	_ = json.Unmarshal(args, &p)
	var sb strings.Builder
	sb.WriteString("**Buat gate window baru**\n")
	if p.Mode != nil {
		fmt.Fprintf(&sb, "\n**Mode:** %s\n", *p.Mode)
	}
	if p.StartsAt != nil {
		fmt.Fprintf(&sb, "\n**Mulai:** %s\n", *p.StartsAt)
	}
	if p.EndsAt != nil {
		fmt.Fprintf(&sb, "\n**Berakhir:** %s\n", *p.EndsAt)
	}
	if p.MaxAttempts != nil {
		fmt.Fprintf(&sb, "\n**Maks percobaan:** %d\n", *p.MaxAttempts)
	}
	if p.Description != nil {
		fmt.Fprintf(&sb, "\n**Catatan:** %s\n", truncateConfirm(*p.Description, 200))
	}
	return sb.String()
}

func confirmUpdateExamGate(args json.RawMessage) string {
	var p struct {
		GateID      string  `json:"gateId"`
		Mode        *string `json:"mode"`
		StartsAt    *string `json:"startsAt"`
		EndsAt      *string `json:"endsAt"`
		MaxAttempts *int    `json:"maxAttempts"`
		Description *string `json:"description"`
	}
	_ = json.Unmarshal(args, &p)
	var sb strings.Builder
	fmt.Fprintf(&sb, "**Update gate window** (`%s`)\n", p.GateID)
	if p.Mode != nil {
		fmt.Fprintf(&sb, "\n**Mode baru:** %s\n", *p.Mode)
	}
	if p.StartsAt != nil {
		fmt.Fprintf(&sb, "\n**Mulai baru:** %s\n", *p.StartsAt)
	}
	if p.EndsAt != nil {
		fmt.Fprintf(&sb, "\n**Berakhir baru:** %s\n", *p.EndsAt)
	}
	if p.MaxAttempts != nil {
		fmt.Fprintf(&sb, "\n**Maks percobaan:** %d\n", *p.MaxAttempts)
	}
	if p.Description != nil {
		fmt.Fprintf(&sb, "\n**Catatan:** %s\n", truncateConfirm(*p.Description, 200))
	}
	return sb.String()
}

func confirmDeleteExamGate(args json.RawMessage) string {
	var p struct{ GateID string `json:"gateId"` }
	_ = json.Unmarshal(args, &p)
	return fmt.Sprintf("**Hapus gate window**\n\n⚠ Gate `%s` akan dihapus.\n\n- Window akses siswa hilang\n- Aksi tidak bisa di-undo\n", p.GateID)
}

// confirmAssignQuestionToSlot.
func confirmAssignQuestionToSlot(args json.RawMessage) string {
	var p struct {
		QuestionID string `json:"questionId"`
		SlotID     string `json:"slotId"`
	}
	_ = json.Unmarshal(args, &p)
	return fmt.Sprintf("**Bind soal ke blueprint slot**\n\n- **Soal:** `%s`\n- **Slot kisi-kisi:** `%s`\n\nSoal akan inherit kisi-kisi metadata dari slot. Jika sebelumnya soal sudah punya kisi-kisi sendiri, akan ditimpa.\n", p.QuestionID, p.SlotID)
}

// confirmExportExamToTemplate.
func confirmExportExamToTemplate(args json.RawMessage) string {
	var p struct {
		ExamID string `json:"examId"`
		Title  string `json:"title"`
	}
	_ = json.Unmarshal(args, &p)
	if p.Title == "" {
		p.Title = "(auto-derive)"
	}
	return fmt.Sprintf("**Export ke blueprint template baru**\n\nDari exam `%s`\nJudul template baru: **%s**\n\n- Template baru dibuat dengan status `draft`\n- Setiap soal di exam jadi 1 slot di template\n- KD/Materi/Indikator/Cog/Difficulty disalin dari soal\n- Bisa direvisi sebelum publish\n", p.ExamID, p.Title)
}
