package app

import (
	"context"
	"encoding/json"
	"strings"
)

// AI duplicate guards
//
// Why these exist:
//   When the bot creates entities in batches, each call goes through the
//   propose → confirm → execute flow. Without a pre-propose duplicate check
//   the bot would happily propose 5 students with emails that already exist
//   (or worse: propose the same email twice in one batch by reusing the same
//   plausible-sounding value). Users would only learn at execute time, after
//   confirming, when the unique constraint trips.
//
//   These guards check two layers:
//     1. Committed rows in the database (with status != 'archived' so freed
//        emails don't false-positive)
//     2. Pending proposals in ai_pending_actions for the SAME session — this
//        catches the in-flight collision where the bot just proposed
//        siti.aminah@example.com a turn ago and now wants to propose it again.

// pendingArgsForSession returns tool_args of all pending (non-expired) proposals
// for the active session matching the given tool name.
func (a *App) pendingArgsForSession(ctx context.Context, toolName string) []json.RawMessage {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	if sessionID == "" {
		return nil
	}
	rows, err := a.db.QueryContext(ctx, `
		SELECT tool_args FROM ai_pending_actions
		 WHERE session_id = $1 AND tool_name = $2
		   AND status = 'pending' AND expires_at > now()`,
		sessionID, toolName)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var a json.RawMessage
		if err := rows.Scan(&a); err == nil {
			out = append(out, a)
		}
	}
	return out
}

// pendingHasField returns true when any pending proposal (same tool, same
// session) has a string field equal to the given value, case-insensitive.
func (a *App) pendingHasField(ctx context.Context, toolName, field, value string) bool {
	if value == "" {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(value))
	for _, raw := range a.pendingArgsForSession(ctx, toolName) {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		v, ok := m[field].(string)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(v)) == target {
			return true
		}
	}
	return false
}

// --- Student ---

func (a *App) checkStudentDuplicate(ctx context.Context, tenantID, email, sidNumber, displayName string) string {
	// In-flight: another pending create_student in this session already used this email
	if a.pendingHasField(ctx, "create_student", "email", email) {
		return errDuplicateEntryWithRecovery(
			"student", "email", email,
			"You already proposed creating a student with this email in the current batch. Pick a different email.",
		)
	}
	if sidNumber != "" && a.pendingHasField(ctx, "create_student", "studentIdNumber", sidNumber) {
		return errDuplicateEntryWithRecovery(
			"student", "studentIdNumber", sidNumber,
			"You already proposed creating a student with this NIS in the current batch. Pick a different NIS.",
		)
	}

	// Committed: email already taken by an active user
	var exists bool
	_ = a.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND status != 'archived')`, email,
	).Scan(&exists)
	if exists {
		return errDuplicateEntryWithRecovery(
			"student", "email", email,
			"A user with this email already exists. Call list_students with a different filter or pick a unique email.",
		)
	}

	// Committed: NIS already taken in this tenant
	if sidNumber != "" {
		_ = a.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM students WHERE tenant_id = $1 AND student_id_number = $2 AND status != 'archived')`,
			tenantID, sidNumber,
		).Scan(&exists)
		if exists {
			return errDuplicateEntryWithRecovery(
				"student", "studentIdNumber", sidNumber,
				"A student with this NIS already exists. Pick a different NIS or call list_students to see existing ones.",
			)
		}
	}

	// Committed: an active student with the exact same display name in this tenant.
	// Two students may legitimately share a name, so this is a soft warning,
	// not a hard fail. Skip — let the bot propose with the user's typed name
	// and let the human catch the dupe at confirm time.
	_ = displayName

	return ""
}

// --- Teacher ---

func (a *App) checkTeacherDuplicate(ctx context.Context, tenantID, email, employeeID, displayName string) string {
	if a.pendingHasField(ctx, "create_teacher", "email", email) {
		return errDuplicateEntryWithRecovery(
			"teacher", "email", email,
			"You already proposed creating a teacher with this email in the current batch. Pick a different email.",
		)
	}
	if employeeID != "" && a.pendingHasField(ctx, "create_teacher", "employeeId", employeeID) {
		return errDuplicateEntryWithRecovery(
			"teacher", "employeeId", employeeID,
			"You already proposed creating a teacher with this employee ID in the current batch.",
		)
	}

	var exists bool
	_ = a.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND status != 'archived')`, email,
	).Scan(&exists)
	if exists {
		return errDuplicateEntryWithRecovery(
			"teacher", "email", email,
			"A user with this email already exists. Pick a unique email.",
		)
	}

	if employeeID != "" {
		_ = a.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM teachers WHERE tenant_id = $1 AND employee_id = $2 AND status != 'archived')`,
			tenantID, employeeID,
		).Scan(&exists)
		if exists {
			return errDuplicateEntryWithRecovery(
				"teacher", "employeeId", employeeID,
				"A teacher with this employee ID already exists.",
			)
		}
	}

	_ = displayName
	return ""
}

// --- Class ---

func (a *App) checkClassDuplicate(ctx context.Context, tenantID, academicYearID, name string) string {
	// In-flight collision check uses both academicYearId+name composite.
	for _, raw := range a.pendingArgsForSession(ctx, "create_class") {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		ay, _ := m["academicYearId"].(string)
		nm, _ := m["name"].(string)
		if ay == academicYearID && strings.EqualFold(strings.TrimSpace(nm), strings.TrimSpace(name)) {
			return errDuplicateEntryWithRecovery(
				"class", "name", name,
				"You already proposed a class with this name in the current academic year this turn.",
			)
		}
	}

	if academicYearID == "" {
		return ""
	}
	var exists bool
	_ = a.db.QueryRowContext(ctx, `
		SELECT EXISTS(
		    SELECT 1 FROM class_sections
		     WHERE tenant_id = $1 AND academic_year_id = $2 AND name = $3 AND status != 'archived'
		)`, tenantID, academicYearID, name,
	).Scan(&exists)
	if exists {
		return errDuplicateEntryWithRecovery(
			"class", "name", name,
			"A class with this name already exists in the active academic year. Pick a different name.",
		)
	}
	return ""
}

// --- Subject ---

func (a *App) checkSubjectDuplicate(ctx context.Context, tenantID, code, name string) string {
	if code != "" && a.pendingHasField(ctx, "create_subject", "code", code) {
		return errDuplicateEntryWithRecovery(
			"subject", "code", code,
			"You already proposed a subject with this code in the current batch.",
		)
	}
	if a.pendingHasField(ctx, "create_subject", "name", name) {
		return errDuplicateEntryWithRecovery(
			"subject", "name", name,
			"You already proposed a subject with this name in the current batch.",
		)
	}

	var exists bool
	if code != "" {
		_ = a.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM subjects WHERE tenant_id = $1 AND code = $2 AND status != 'archived')`,
			tenantID, code,
		).Scan(&exists)
		if exists {
			return errDuplicateEntryWithRecovery(
				"subject", "code", code,
				"A subject with this code already exists. Call list_subjects to see existing codes.",
			)
		}
	}
	_ = a.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM subjects WHERE tenant_id = $1 AND lower(name) = lower($2) AND status != 'archived')`,
		tenantID, name,
	).Scan(&exists)
	if exists {
		return errDuplicateEntryWithRecovery(
			"subject", "name", name,
			"A subject with this name already exists.",
		)
	}
	return ""
}
