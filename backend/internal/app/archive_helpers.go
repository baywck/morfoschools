package app

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// dbExecer is the minimal surface satisfied by both *sql.DB and *sql.Tx.
// The archive helpers use it so callers can choose to wrap operations in a
// transaction or run them on the connection directly.
type dbExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// archivedEmailFor returns the synthetic email used when archiving a user.
// The UUID is embedded so collisions are impossible by construction and the
// original email can be reused by another (or returning) user.
func archivedEmailFor(userID string) string {
	return fmt.Sprintf("archived+%s@archived.morfoschools.local", userID)
}

// isArchivedEmail reports whether the email looks like a synthetic archive
// placeholder, used to skip restoring a placeholder back into the active slot.
func isArchivedEmail(email string) bool {
	return strings.HasPrefix(email, "archived+") &&
		strings.HasSuffix(email, "@archived.morfoschools.local")
}

// archiveUser flips a user's status to 'archived', preserves the current email
// in `original_email` (if not already preserved), and swaps the live email
// with a synthetic value so the original address can be reused immediately.
// Idempotent: already-archived users return without error.
func archiveUser(ctx context.Context, e dbExecer, userID string) error {
	var status string
	var currentEmail string
	var origEmail sql.NullString
	if err := e.QueryRowContext(ctx,
		`SELECT status, email, original_email FROM users WHERE id = $1`, userID,
	).Scan(&status, &currentEmail, &origEmail); err != nil {
		return err
	}
	if status == "archived" {
		return nil
	}
	synthetic := archivedEmailFor(userID)
	// Only seed original_email if not already populated — protects against
	// repeated archive/restore cycles overwriting the real value with the
	// synthetic one.
	_, err := e.ExecContext(ctx,
		`UPDATE users
		    SET original_email = COALESCE(original_email, $3),
		        email = $2,
		        status = 'archived',
		        updated_at = now()
		  WHERE id = $1`,
		userID, synthetic, currentEmail,
	)
	return err
}

// restoreUser flips a user back to 'active' and tries to restore their original
// email. When `overrideEmail` is non-empty it takes precedence over the
// recorded original. Returns (resolvedEmail, taken, err):
//   - resolvedEmail: the email it tried to set (empty if neither override nor
//     original was available; in that case the synthetic email stays in place)
//   - taken: true when the resolved email is currently held by another active
//     user. The caller should respond 409 and ask the admin for an alternative.
func restoreUser(ctx context.Context, e dbExecer, userID, overrideEmail string) (resolvedEmail string, taken bool, err error) {
	var origEmail sql.NullString
	var currentEmail string
	if err = e.QueryRowContext(ctx,
		`SELECT email, original_email FROM users WHERE id = $1`, userID,
	).Scan(&currentEmail, &origEmail); err != nil {
		return "", false, err
	}

	target := strings.TrimSpace(overrideEmail)
	if target == "" && origEmail.Valid {
		target = strings.TrimSpace(origEmail.String)
	}

	// No usable target email. Flip status only — caller should subsequently
	// patch the email via the standard update endpoint.
	if target == "" || isArchivedEmail(target) {
		_, err = e.ExecContext(ctx,
			`UPDATE users SET status = 'active', updated_at = now() WHERE id = $1`,
			userID,
		)
		return "", false, err
	}

	var exists bool
	if err = e.QueryRowContext(ctx,
		`SELECT EXISTS(
		    SELECT 1 FROM users
		    WHERE email = $1 AND status != 'archived' AND id != $2
		)`, target, userID,
	).Scan(&exists); err != nil {
		return target, false, err
	}
	if exists {
		return target, true, nil
	}

	_, err = e.ExecContext(ctx,
		`UPDATE users
		    SET email = $2,
		        original_email = NULL,
		        status = 'active',
		        updated_at = now()
		  WHERE id = $1`,
		userID, target,
	)
	return target, false, err
}

// userHasNoActiveProfiles returns true when the user has zero non-archived
// rows across teachers, students, staff_profiles and guardians.
func userHasNoActiveProfiles(ctx context.Context, e dbExecer, userID string) (bool, error) {
	if userID == "" {
		return true, nil
	}
	var c int
	err := e.QueryRowContext(ctx, `
        SELECT
          (SELECT COUNT(*) FROM teachers        WHERE user_id = $1 AND status != 'archived') +
          (SELECT COUNT(*) FROM students        WHERE user_id = $1 AND status != 'archived') +
          (SELECT COUNT(*) FROM staff_profiles  WHERE user_id = $1 AND status != 'archived') +
          (SELECT COUNT(*) FROM guardians       WHERE user_id = $1 AND status != 'archived')
    `, userID).Scan(&c)
	if err != nil {
		return false, err
	}
	return c == 0, nil
}

// cascadeArchiveUserIfOrphan archives the user iff no active profile remains.
// Safe to call with empty userID (orphan guardian rows have no user link).
func cascadeArchiveUserIfOrphan(ctx context.Context, e dbExecer, userID string) (cascaded bool, err error) {
	if userID == "" {
		return false, nil
	}
	noActive, err := userHasNoActiveProfiles(ctx, e, userID)
	if err != nil || !noActive {
		return false, err
	}
	if err := archiveUser(ctx, e, userID); err != nil {
		return false, err
	}
	return true, nil
}

// archiveAllProfilesForUser archives every profile row linked to the given
// user across all tenants. Used when a user is archived directly so we don't
// leave dangling active profiles whose login is now broken.
func archiveAllProfilesForUser(ctx context.Context, e dbExecer, userID string) error {
	queries := []string{
		`UPDATE teachers       SET status = 'archived', updated_at = now() WHERE user_id = $1 AND status != 'archived'`,
		`UPDATE students       SET status = 'archived', updated_at = now() WHERE user_id = $1 AND status != 'archived'`,
		`UPDATE staff_profiles SET status = 'archived', updated_at = now() WHERE user_id = $1 AND status != 'archived'`,
		`UPDATE guardians      SET status = 'archived', updated_at = now() WHERE user_id = $1 AND status != 'archived'`,
	}
	for _, q := range queries {
		if _, err := e.ExecContext(ctx, q, userID); err != nil {
			return err
		}
	}
	return nil
}

// userIDForProfile returns the user_id for a profile row in the given table.
// `table` is hard-coded by callers — never user input — so direct
// interpolation is safe. Returns ("", nil) when the row doesn't exist or has
// a null user link (e.g. a guardian with no login account).
func userIDForProfile(ctx context.Context, e dbExecer, table, profileID string) (string, error) {
	switch table {
	case "teachers", "students", "staff_profiles", "guardians":
		// allowed
	default:
		return "", fmt.Errorf("userIDForProfile: invalid table %q", table)
	}
	var uid sql.NullString
	err := e.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT user_id FROM %s WHERE id = $1`, table), profileID,
	).Scan(&uid)
	if err != nil {
		return "", err
	}
	if !uid.Valid {
		return "", nil
	}
	return uid.String, nil
}

// restoreProfile flips a single profile row back to 'active' (or another
// status the caller specifies via `targetStatus`). Used by per-profile restore
// endpoints. Caller is responsible for restoring the parent user when needed.
func restoreProfile(ctx context.Context, e dbExecer, table, profileID, targetStatus string) error {
	switch table {
	case "teachers", "students", "staff_profiles", "guardians":
		// allowed
	default:
		return fmt.Errorf("restoreProfile: invalid table %q", table)
	}
	if targetStatus == "" {
		targetStatus = "active"
	}
	_, err := e.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET status = $2, updated_at = now() WHERE id = $1`, table),
		profileID, targetStatus,
	)
	return err
}
