package app

import (
	"context"
	"database/sql"
	"strings"
)

// validateMerdekaKisiKisiPayload is the shared policy for Kurikulum Merdeka
// blueprint/kisi-kisi metadata. It is intentionally UI-agnostic so manual
// forms, AI tools, inline magic actions, and slot executors all converge on
// the same rule set.
func validateMerdekaKisiKisiPayload(p slotPayload) map[string]string {
	errs := map[string]string{}
	if p.CognitiveLevel != nil && strings.TrimSpace(*p.CognitiveLevel) != "" {
		switch strings.TrimSpace(*p.CognitiveLevel) {
		case "C1", "C2", "C3", "C4", "C5", "C6":
		default:
			errs["cognitiveLevel"] = "Must be C1..C6"
		}
	}
	if p.Difficulty != nil && strings.TrimSpace(*p.Difficulty) != "" {
		switch strings.TrimSpace(*p.Difficulty) {
		case "mudah", "sedang", "sulit":
		default:
			errs["difficulty"] = "Must be mudah/sedang/sulit"
		}
	}
	if p.QuestionType != nil && strings.TrimSpace(*p.QuestionType) != "" {
		switch strings.TrimSpace(*p.QuestionType) {
		case "multiple_choice", "true_false", "short_answer", "essay":
		default:
			errs["questionType"] = "Must be multiple_choice/true_false/short_answer/essay"
		}
	}
	if p.Points != nil && *p.Points < 0 {
		errs["points"] = "Must be non-negative"
	}
	if p.IndikatorSoal != nil {
		v := strings.TrimSpace(*p.IndikatorSoal)
		if v != "" && !strings.HasPrefix(strings.ToLower(v), "disajikan") {
			errs["indikatorSoal"] = "Indikator Soal harus diawali 'Disajikan ...'"
		}
	}
	if p.Kelas != nil {
		v := strings.TrimSpace(*p.Kelas)
		if v != "" && gradeLevelToPhase(v) == "" {
			errs["kelas"] = "Kelas harus 1-12"
		}
	}
	if p.Semester != nil {
		switch strings.TrimSpace(*p.Semester) {
		case "", "1", "2", "ganjil", "genap", "Ganjil", "Genap":
		default:
			errs["semester"] = "Semester harus 1/2 atau ganjil/genap"
		}
	}
	return errs
}

func mergeSlotPayload(current, patch slotPayload) slotPayload {
	merged := current
	if patch.Position != nil {
		merged.Position = patch.Position
	}
	if patch.CompetencyID != nil {
		merged.CompetencyID = patch.CompetencyID
	}
	if patch.CompetencyCode != nil {
		merged.CompetencyCode = patch.CompetencyCode
	}
	if patch.CompetencyDescription != nil {
		merged.CompetencyDescription = patch.CompetencyDescription
	}
	if patch.Materi != nil {
		merged.Materi = patch.Materi
	}
	if patch.Indikator != nil {
		merged.Indikator = patch.Indikator
	}
	if patch.CognitiveLevel != nil {
		merged.CognitiveLevel = patch.CognitiveLevel
	}
	if patch.Difficulty != nil {
		merged.Difficulty = patch.Difficulty
	}
	if patch.QuestionType != nil {
		merged.QuestionType = patch.QuestionType
	}
	if patch.Points != nil {
		merged.Points = patch.Points
	}
	if patch.StimulusID != nil {
		merged.StimulusID = patch.StimulusID
	}
	if patch.CPElementID != nil {
		merged.CPElementID = patch.CPElementID
	}
	if patch.CapaianPembelajaran != nil {
		merged.CapaianPembelajaran = patch.CapaianPembelajaran
	}
	if patch.ElemenCP != nil {
		merged.ElemenCP = patch.ElemenCP
	}
	if patch.TujuanPembelajaran != nil {
		merged.TujuanPembelajaran = patch.TujuanPembelajaran
	}
	if patch.MateriPokok != nil {
		merged.MateriPokok = patch.MateriPokok
	}
	if patch.Kelas != nil {
		merged.Kelas = patch.Kelas
	}
	if patch.Semester != nil {
		merged.Semester = patch.Semester
	}
	if patch.IndikatorSoal != nil {
		merged.IndikatorSoal = patch.IndikatorSoal
	}
	return merged
}

func nullableStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

func nullableFloatPtr(nf sql.NullFloat64) *float64 {
	if !nf.Valid {
		return nil
	}
	v := nf.Float64
	return &v
}

func (a *App) loadSlotPayload(ctx context.Context, table, slotID string) (slotPayload, error) {
	var p slotPayload
	var competencyID, competencyCode, competencyDescription, materi, indikator sql.NullString
	var cognitiveLevel, difficulty, questionType, stimulusID, cpElementID sql.NullString
	var capaian, elemen, tp, materiPokok, kelas, semester, indikatorSoal sql.NullString
	var points sql.NullFloat64
	err := a.db.QueryRowContext(ctx, `SELECT competency_id::text, competency_code, competency_description, materi, indikator,
		cognitive_level, difficulty, question_type, points, stimulus_id::text, cp_element_id::text,
		capaian_pembelajaran, elemen_cp, tujuan_pembelajaran, materi_pokok, kelas, semester, indikator_soal
		FROM `+table+` WHERE id=$1`, slotID).Scan(
		&competencyID, &competencyCode, &competencyDescription, &materi, &indikator,
		&cognitiveLevel, &difficulty, &questionType, &points, &stimulusID, &cpElementID,
		&capaian, &elemen, &tp, &materiPokok, &kelas, &semester, &indikatorSoal,
	)
	if err != nil {
		return p, err
	}
	p.CompetencyID = nullableStringPtr(competencyID)
	p.CompetencyCode = nullableStringPtr(competencyCode)
	p.CompetencyDescription = nullableStringPtr(competencyDescription)
	p.Materi = nullableStringPtr(materi)
	p.Indikator = nullableStringPtr(indikator)
	p.CognitiveLevel = nullableStringPtr(cognitiveLevel)
	p.Difficulty = nullableStringPtr(difficulty)
	p.QuestionType = nullableStringPtr(questionType)
	p.Points = nullableFloatPtr(points)
	p.StimulusID = nullableStringPtr(stimulusID)
	p.CPElementID = nullableStringPtr(cpElementID)
	p.CapaianPembelajaran = nullableStringPtr(capaian)
	p.ElemenCP = nullableStringPtr(elemen)
	p.TujuanPembelajaran = nullableStringPtr(tp)
	p.MateriPokok = nullableStringPtr(materiPokok)
	p.Kelas = nullableStringPtr(kelas)
	p.Semester = nullableStringPtr(semester)
	p.IndikatorSoal = nullableStringPtr(indikatorSoal)
	return p, nil
}

func (a *App) validateTenantKisiKisiPayload(ctx context.Context, tenantID string, p slotPayload) map[string]string {
	errs := validateMerdekaKisiKisiPayload(p)
	if p.Kelas != nil {
		if phaseErrs := a.validateTenantPhaseValue(ctx, tenantID, "kelas", strings.TrimSpace(*p.Kelas)); len(phaseErrs) > 0 {
			for k, v := range phaseErrs {
				errs[k] = v
			}
		}
	}
	if p.CPElementID != nil && strings.TrimSpace(*p.CPElementID) != "" {
		phaseFromKelas := ""
		if p.Kelas != nil {
			phaseFromKelas = gradeLevelToPhase(*p.Kelas)
		}
		fields := a.validateCPElementForTenantPhase(ctx, tenantID, strings.TrimSpace(*p.CPElementID), phaseFromKelas)
		for k, v := range fields {
			errs[k] = v
		}
	}
	return errs
}

func (a *App) validateCPElementForTenantPhase(ctx context.Context, tenantID, cpElementID, expectedPhase string) map[string]string {
	var phase string
	err := a.db.QueryRowContext(ctx, `
		SELECT lower(r.phase)
		  FROM curriculum_cp_elements e
		  JOIN curriculum_cp_references r ON r.id = e.reference_id
		 WHERE e.id = $1 AND r.status = 'active'`, cpElementID).Scan(&phase)
	if err != nil || phase == "" {
		return map[string]string{"cpElementId": "CP element not found"}
	}
	profile, err := a.loadTenantEducationProfile(ctx, tenantID)
	if err != nil {
		return map[string]string{"cpElementId": "Could not validate CP element phase"}
	}
	if !profile.allowsPhase(phase) {
		return map[string]string{"cpElementId": "CP element phase is outside this tenant's enabled phases"}
	}
	if expectedPhase != "" && expectedPhase != phase {
		return map[string]string{"cpElementId": "CP element phase does not match selected kelas"}
	}
	return nil
}

func slotPayloadFromQuestionMeta(req struct {
	QuestionType        string
	Points              *float64
	CognitiveLevel      *string
	Difficulty          *string
	CPElementID         *string
	CapaianPembelajaran *string
	ElemenCP            *string
	TujuanPembelajaran  *string
	MateriPokok         *string
	Kelas               *string
	Semester            *string
	IndikatorSoal       *string
}) slotPayload {
	return slotPayload{
		CPElementID:         req.CPElementID,
		CapaianPembelajaran: req.CapaianPembelajaran,
		ElemenCP:            req.ElemenCP,
		TujuanPembelajaran:  req.TujuanPembelajaran,
		MateriPokok:         req.MateriPokok,
		Kelas:               req.Kelas,
		Semester:            req.Semester,
		IndikatorSoal:       req.IndikatorSoal,
		CognitiveLevel:      req.CognitiveLevel,
		Difficulty:          req.Difficulty,
		QuestionType:        &req.QuestionType,
		Points:              req.Points,
	}
}
