package app

import "testing"

// Phase 9.9 — group/section binding unit tests.
//
// Mirrors the gates inside handleCreateQuestionGroup and
// handleUpdateQuestionGroup. Pure helpers / structural assertions only;
// DB-backed integration is exercised by the migration smoke + manual
// browser smoke documented in AUDIT_REPORT.md.

// resolveGroupSection mirrors the predicate inside
// handleCreateQuestionGroup: when the caller supplies a non-empty
// sectionId it must reference a section in the same exam; when omitted
// the handler falls back to the exam's first section. The "default"
// outcome is encoded as the empty string so the caller pattern is
// (ok, sectionID).
func resolveGroupSection(reqSection *string, sectionExam, examID, firstSection string) (string, bool) {
	if reqSection != nil && *reqSection != "" {
		if sectionExam != examID {
			return "", false
		}
		return *reqSection, true
	}
	if firstSection == "" {
		return "", false
	}
	return firstSection, true
}

func TestCreateGroup_DefaultsToFirstSectionWhenSectionIdMissing(t *testing.T) {
	got, ok := resolveGroupSection(nil, "", "exam-1", "section-1")
	if !ok || got != "section-1" {
		t.Errorf("missing sectionId must default to first section, got (%q, %v)", got, ok)
	}
	empty := ""
	got, ok = resolveGroupSection(&empty, "", "exam-1", "section-1")
	if !ok || got != "section-1" {
		t.Errorf("empty sectionId must default to first section, got (%q, %v)", got, ok)
	}
}

func TestCreateGroup_RejectsForeignSection(t *testing.T) {
	foreign := "section-from-other-exam"
	got, ok := resolveGroupSection(&foreign, "other-exam", "exam-1", "section-1")
	if ok || got != "" {
		t.Errorf("foreign section must be rejected, got (%q, %v)", got, ok)
	}
}

func TestCreateGroup_AcceptsValidSection(t *testing.T) {
	target := "section-2"
	got, ok := resolveGroupSection(&target, "exam-1", "exam-1", "section-1")
	if !ok || got != "section-2" {
		t.Errorf("valid same-exam section must be accepted, got (%q, %v)", got, ok)
	}
}

func TestCreateGroup_ErrorsWhenExamHasNoSection(t *testing.T) {
	// Phase 9.8 makes sections mandatory; this should never happen at
	// runtime, but the helper should still return !ok rather than
	// silently falling through.
	got, ok := resolveGroupSection(nil, "", "exam-1", "")
	if ok || got != "" {
		t.Errorf("exam without sections must error, got (%q, %v)", got, ok)
	}
}

// updateGroupAcceptsSectionMove mirrors the predicate inside
// handleUpdateQuestionGroup: a non-empty sectionId in the PATCH body
// must reference a section in the same exam. Empty string clears the
// binding (group becomes section-less). nil leaves it untouched.
func updateGroupAcceptsSectionMove(reqSection *string, sectionExam, examID string) (mutates, ok bool) {
	if reqSection == nil {
		return false, true
	}
	if *reqSection == "" {
		return true, true
	}
	if sectionExam != examID {
		return false, false
	}
	return true, true
}

func TestUpdateGroup_MovesBetweenSections(t *testing.T) {
	target := "section-2"
	mutates, ok := updateGroupAcceptsSectionMove(&target, "exam-1", "exam-1")
	if !ok || !mutates {
		t.Errorf("valid section move must be accepted with mutation, got (mutates=%v ok=%v)", mutates, ok)
	}
}

func TestUpdateGroup_RejectsForeignSection(t *testing.T) {
	target := "section-from-other-exam"
	mutates, ok := updateGroupAcceptsSectionMove(&target, "other-exam", "exam-1")
	if ok || mutates {
		t.Errorf("foreign section move must be rejected, got (mutates=%v ok=%v)", mutates, ok)
	}
}

func TestUpdateGroup_OmittedSectionLeavesBindingUntouched(t *testing.T) {
	mutates, ok := updateGroupAcceptsSectionMove(nil, "", "exam-1")
	if !ok || mutates {
		t.Errorf("omitted sectionId must be a no-op, got (mutates=%v ok=%v)", mutates, ok)
	}
}

func TestUpdateGroup_EmptySectionClearsBinding(t *testing.T) {
	empty := ""
	mutates, ok := updateGroupAcceptsSectionMove(&empty, "", "exam-1")
	if !ok || !mutates {
		t.Errorf("empty sectionId must clear binding, got (mutates=%v ok=%v)", mutates, ok)
	}
}
