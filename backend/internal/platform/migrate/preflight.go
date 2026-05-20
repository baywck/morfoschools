package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// preFlights maps a migration filename to a function that must succeed
// before that migration is applied. If the function returns an error,
// the migration runner aborts loud, prints actionable guidance, and
// the operator must resolve the underlying data issue before re-running.
//
// Add new entries here as migrations gain pre-flight requirements.
var preFlights = map[string]func(context.Context, *sql.DB) error{
	"000014_collaboration.sql": preflightOwnerBackfill,
}

// preflightOwnerBackfill is the audit gate for migration 000014. The
// migration adds owner_user_id NOT NULL columns on exams and courses,
// backfilled from created_by. If any row has NULL created_by or points
// to a missing/archived user, the migration cannot deterministically
// pick an owner. We refuse rather than silently assign a random admin.
//
// On failure, the operator must resolve via the orphaned-resources
// admin endpoint (added in this phase) which exposes the offending
// rows and lets a tenant admin claim them or bulk-assign.
func preflightOwnerBackfill(ctx context.Context, db *sql.DB) error {
	type orphan struct {
		Resource string
		ID       string
		TenantID string
		Title    string
		Reason   string
	}
	var found []orphan

	check := func(table, titleCol string) error {
		// NULL created_by
		rows, err := db.QueryContext(ctx, fmt.Sprintf(`
			SELECT id::text, tenant_id::text, %s
			  FROM %s
			 WHERE created_by IS NULL
			 LIMIT 100`, titleCol, table))
		if err != nil {
			return fmt.Errorf("preflight scan %s for null created_by: %w", table, err)
		}
		for rows.Next() {
			var o orphan
			o.Resource = table
			o.Reason = "created_by is NULL"
			if err := rows.Scan(&o.ID, &o.TenantID, &o.Title); err == nil {
				found = append(found, o)
			}
		}
		rows.Close()

		// created_by points to missing or archived user
		rows, err = db.QueryContext(ctx, fmt.Sprintf(`
			SELECT t.id::text, t.tenant_id::text, t.%s
			  FROM %s t
			  LEFT JOIN users u ON u.id = t.created_by
			 WHERE t.created_by IS NOT NULL
			   AND (u.id IS NULL OR u.status = 'archived')
			 LIMIT 100`, titleCol, table))
		if err != nil {
			return fmt.Errorf("preflight scan %s for orphan created_by: %w", table, err)
		}
		for rows.Next() {
			var o orphan
			o.Resource = table
			o.Reason = "created_by points to missing or archived user"
			if err := rows.Scan(&o.ID, &o.TenantID, &o.Title); err == nil {
				found = append(found, o)
			}
		}
		rows.Close()
		return nil
	}

	if err := check("exams", "title"); err != nil {
		return err
	}
	if err := check("courses", "title"); err != nil {
		return err
	}

	if len(found) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("=========================================================\n")
	sb.WriteString("  MIGRATION 000014 BLOCKED: orphan ownership rows found\n")
	sb.WriteString("=========================================================\n")
	sb.WriteString(fmt.Sprintf("Found %d row(s) without a deterministic owner:\n\n", len(found)))
	for _, o := range found {
		sb.WriteString(fmt.Sprintf("  [%s] id=%s tenant=%s title=%q\n      reason: %s\n",
			o.Resource, o.ID, o.TenantID, o.Title, o.Reason))
	}
	sb.WriteString("\nResolve via the admin endpoint:\n")
	sb.WriteString("    GET  /api/v1/admin/orphaned-resources\n")
	sb.WriteString("    POST /api/v1/admin/orphaned-resources/claim\n")
	sb.WriteString("Each orphan row must be claimed by a specific user.\n")
	sb.WriteString("Once cleared, restart the backend to re-run migrations.\n")
	sb.WriteString("=========================================================\n")
	return fmt.Errorf("%s", sb.String())
}
