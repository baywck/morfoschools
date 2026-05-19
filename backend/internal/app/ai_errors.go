package app

import (
	"context"
	"encoding/json"
	"fmt"
)

// ToolError is a structured error envelope returned by AI tool handlers.
// It gives the LLM enough context to self-correct without user intervention.
type ToolError struct {
	Code        string            `json:"code"`                  // Machine-readable: ENTITY_NOT_FOUND, INVALID_UUID, VALIDATION_FAILED, DUPLICATE_ENTRY, PERMISSION_DENIED
	Message     string            `json:"message"`               // Human-readable explanation
	Field       string            `json:"field,omitempty"`       // Which parameter caused the error
	Entity      string            `json:"entity,omitempty"`      // Which entity type (student, class, teacher, subject)
	Recoverable bool             `json:"recoverable"`           // Can the bot fix this by calling another tool?
	Recovery    *RecoveryHint     `json:"recovery,omitempty"`    // What tool to call to fix this
	Suggestions []string          `json:"suggestions,omitempty"` // Possible correct values (e.g. similar names found)
}

// RecoveryHint tells the LLM exactly what to do next to resolve the error.
type RecoveryHint struct {
	Tool string         `json:"tool"`           // Tool name to call (e.g. "search_students")
	Args map[string]any `json:"args,omitempty"` // Suggested arguments
	Hint string         `json:"hint,omitempty"` // Short instruction for the LLM
}

// JSON serializes the error for tool result output.
func (e *ToolError) JSON() string {
	b, _ := json.Marshal(map[string]any{"error": e})
	return string(b)
}

// --- Factory functions for common error patterns ---

func errEntityNotFound(entity, field, searchValue string) string {
	te := &ToolError{
		Code:        "ENTITY_NOT_FOUND",
		Message:     fmt.Sprintf("%s not found for %s='%s'", entity, field, searchValue),
		Field:       field,
		Entity:      entity,
		Recoverable: true,
		Recovery: &RecoveryHint{
			Tool: "search_" + pluralize(entity),
			Args: map[string]any{"search": searchValue},
			Hint: fmt.Sprintf("Search for the correct %s first, then retry with the UUID from results", entity),
		},
	}
	return te.JSON()
}

func errInvalidUUID(field, value, entity string) string {
	te := &ToolError{
		Code:        "INVALID_UUID",
		Message:     fmt.Sprintf("'%s' is not a valid UUID for %s", value, field),
		Field:       field,
		Entity:      entity,
		Recoverable: true,
		Recovery: &RecoveryHint{
			Tool: "search_" + pluralize(entity),
			Args: map[string]any{"search": value},
			Hint: fmt.Sprintf("The value looks like a name, not a UUID. Search for the %s to get its UUID", entity),
		},
	}
	return te.JSON()
}

func errValidationFailed(field, message string) string {
	te := &ToolError{
		Code:        "VALIDATION_FAILED",
		Message:     message,
		Field:       field,
		Recoverable: false,
	}
	return te.JSON()
}

func errDuplicateEntry(entity, field, value string) string {
	te := &ToolError{
		Code:        "DUPLICATE_ENTRY",
		Message:     fmt.Sprintf("%s with %s='%s' already exists", entity, field, value),
		Field:       field,
		Entity:      entity,
		Recoverable: false,
	}
	return te.JSON()
}

func errPermissionDenied(action string) string {
	te := &ToolError{
		Code:        "PERMISSION_DENIED",
		Message:     fmt.Sprintf("You don't have permission to %s", action),
		Recoverable: false,
	}
	return te.JSON()
}

func errWithSuggestions(entity, field, searchValue string, suggestions []string) string {
	te := &ToolError{
		Code:        "ENTITY_NOT_FOUND",
		Message:     fmt.Sprintf("Exact match for %s='%s' not found, but similar %s exist", field, searchValue, pluralize(entity)),
		Field:       field,
		Entity:      entity,
		Recoverable: true,
		Suggestions: suggestions,
		Recovery: &RecoveryHint{
			Tool: "search_" + pluralize(entity),
			Args: map[string]any{"search": searchValue},
			Hint: "Use one of the suggestions or search with a different term",
		},
	}
	return te.JSON()
}

func pluralize(entity string) string {
	switch entity {
	case "class":
		return "classes"
	case "staff":
		return "staff"
	case "academic_year":
		return "academic_years"
	default:
		return entity + "s"
	}
}

// findSimilarNames searches for similar display_name entries in a given profile table.
func (a *App) findSimilarNames(ctx context.Context, tenantID, table, search string) []string {
	var query string
	switch table {
	case "students":
		query = `SELECT u.display_name FROM students s JOIN users u ON u.id = s.user_id WHERE s.tenant_id = $1 AND s.status = 'active' AND u.display_name ILIKE $2 ORDER BY u.display_name LIMIT 5`
	case "teachers":
		query = `SELECT u.display_name FROM teachers t JOIN users u ON u.id = t.user_id WHERE t.tenant_id = $1 AND t.status = 'active' AND u.display_name ILIKE $2 ORDER BY u.display_name LIMIT 5`
	case "staff":
		query = `SELECT u.display_name FROM staff_profiles sp JOIN users u ON u.id = sp.user_id WHERE sp.tenant_id = $1 AND sp.status = 'active' AND u.display_name ILIKE $2 ORDER BY u.display_name LIMIT 5`
	default:
		return nil
	}

	rows, err := a.db.QueryContext(ctx, query, tenantID, "%"+search+"%")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil {
			names = append(names, name)
		}
	}
	return names
}

// findSimilarSubjects searches for subjects with similar names.
func (a *App) findSimilarSubjects(ctx context.Context, tenantID, search string) []string {
	rows, err := a.db.QueryContext(ctx,
		`SELECT name FROM subjects WHERE tenant_id = $1 AND status = 'active' AND (name ILIKE $2 OR code ILIKE $2) ORDER BY name LIMIT 5`,
		tenantID, "%"+search+"%",
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil {
			names = append(names, name)
		}
	}
	return names
}
