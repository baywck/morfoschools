package app

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
)

type tenantEducationProfile struct {
	SchoolType                string
	EnabledPhases             []string
	IncludeVocationalSubjects bool
}

func defaultPhasesForSchoolType(schoolType string) []string {
	switch schoolType {
	case "sd":
		return []string{"a", "b", "c"}
	case "smp":
		return []string{"d"}
	case "sma", "smk":
		return []string{"e", "f"}
	default:
		return []string{"e", "f"}
	}
}

func normalizeTenantEducationProfile(schoolType string, phases []string, includeVocational *bool) (tenantEducationProfile, map[string]string) {
	fields := map[string]string{}
	schoolType = strings.ToLower(strings.TrimSpace(schoolType))
	switch schoolType {
	case "sd", "smp", "sma", "smk", "mixed":
	default:
		fields["schoolType"] = "School type must be sd, smp, sma, smk, or mixed"
	}
	if len(phases) == 0 || schoolType != "mixed" {
		phases = defaultPhasesForSchoolType(schoolType)
	}
	seen := map[string]bool{}
	clean := make([]string, 0, len(phases))
	for _, phase := range phases {
		phase = strings.ToLower(strings.TrimSpace(phase))
		if phase == "" {
			continue
		}
		if phase < "a" || phase > "f" || len(phase) != 1 {
			fields["enabledPhases"] = "Phases must be A-F"
			continue
		}
		if !seen[phase] {
			seen[phase] = true
			clean = append(clean, phase)
		}
	}
	if len(clean) == 0 {
		fields["enabledPhases"] = "At least one phase is required"
	}
	vocational := schoolType == "smk"
	if schoolType == "mixed" && includeVocational != nil {
		vocational = *includeVocational
	}
	if schoolType != "smk" && schoolType != "mixed" {
		vocational = false
	}
	return tenantEducationProfile{SchoolType: schoolType, EnabledPhases: clean, IncludeVocationalSubjects: vocational}, fields
}

func parseDBTextArray(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func gradeLevelToPhase(gradeLevel string) string {
	gradeLevel = strings.TrimSpace(strings.ToLower(gradeLevel))
	gradeLevel = strings.TrimPrefix(gradeLevel, "kelas ")
	gradeLevel = strings.TrimPrefix(gradeLevel, "grade ")
	roman := map[string]int{"i": 1, "ii": 2, "iii": 3, "iv": 4, "v": 5, "vi": 6, "vii": 7, "viii": 8, "ix": 9, "x": 10, "xi": 11, "xii": 12}
	grade, err := strconv.Atoi(gradeLevel)
	if err != nil {
		grade = roman[gradeLevel]
	}
	switch {
	case grade >= 1 && grade <= 2:
		return "a"
	case grade >= 3 && grade <= 4:
		return "b"
	case grade >= 5 && grade <= 6:
		return "c"
	case grade >= 7 && grade <= 9:
		return "d"
	case grade == 10:
		return "e"
	case grade >= 11 && grade <= 12:
		return "f"
	default:
		return ""
	}
}

func (p tenantEducationProfile) allowsPhase(phase string) bool {
	phase = strings.ToLower(strings.TrimSpace(phase))
	for _, enabled := range p.EnabledPhases {
		if strings.ToLower(strings.TrimSpace(enabled)) == phase {
			return true
		}
	}
	return false
}

func gradeOrPhaseToPhase(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimPrefix(value, "fase ")
	if len(value) == 1 && value >= "a" && value <= "f" {
		return value
	}
	return gradeLevelToPhase(value)
}

func (a *App) validateTenantPhaseValue(ctx context.Context, tenantID, field, value string) map[string]string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	phase := gradeOrPhaseToPhase(value)
	if phase == "" {
		return map[string]string{field: "Must be phase A-F or grade level 1-12/I-XII"}
	}
	profile, err := a.loadTenantEducationProfile(ctx, tenantID)
	if err != nil {
		return map[string]string{field: "Could not validate tenant enabled phases"}
	}
	if !profile.allowsPhase(phase) {
		return map[string]string{field: "Value is outside this tenant's enabled phases"}
	}
	return nil
}

func (a *App) validateTenantGradeLevel(ctx context.Context, tenantID, gradeLevel string) map[string]string {
	gradeLevel = strings.TrimSpace(gradeLevel)
	if gradeLevel == "" {
		return nil
	}
	phase := gradeLevelToPhase(gradeLevel)
	if phase == "" {
		return map[string]string{"gradeLevel": "Grade level must be 1-12 or I-XII"}
	}
	profile, err := a.loadTenantEducationProfile(ctx, tenantID)
	if err != nil {
		return map[string]string{"gradeLevel": "Could not validate tenant grade range"}
	}
	if !profile.allowsPhase(phase) {
		return map[string]string{"gradeLevel": "Grade level is outside this tenant's enabled phases"}
	}
	return nil
}

func (a *App) loadTenantEducationProfile(ctx context.Context, tenantID string) (tenantEducationProfile, error) {
	var p tenantEducationProfile
	var phases string
	err := a.db.QueryRowContext(ctx, `SELECT school_type, array_to_string(enabled_phases, ','), include_vocational_subjects FROM tenants WHERE id = $1`, tenantID).Scan(&p.SchoolType, &phases, &p.IncludeVocationalSubjects)
	if err != nil {
		if err == sql.ErrNoRows {
			return tenantEducationProfile{}, err
		}
		return tenantEducationProfile{}, err
	}
	p.EnabledPhases = parseDBTextArray(phases)
	return p, nil
}
