package devseed

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"

	"golang.org/x/crypto/argon2"
)

// Run seeds development data. Safe to call multiple times (idempotent).
func Run(ctx context.Context, db *sql.DB, logger *slog.Logger) error {
	logger.Info("seeding development data")

	// Seed permissions
	if err := seedPermissions(ctx, db); err != nil {
		return fmt.Errorf("seed permissions: %w", err)
	}

	// Seed tenant
	tenantID, err := seedTenant(ctx, db)
	if err != nil {
		return fmt.Errorf("seed tenant: %w", err)
	}

	// Seed roles
	if err := seedRoles(ctx, db, tenantID); err != nil {
		return fmt.Errorf("seed roles: %w", err)
	}

	// Seed users
	if err := seedUsers(ctx, db, tenantID); err != nil {
		return fmt.Errorf("seed users: %w", err)
	}

	logger.Info("development data seeded")
	return nil
}

var permissions = []struct {
	Slug string
	Name string
}{
	{"users:read", "View users"},
	{"users:write", "Manage users"},
	{"tenants:read", "View tenants"},
	{"tenants:write", "Manage tenants"},
	{"academic:read", "View academic structure"},
	{"academic:write", "Manage academic structure"},
	{"programs:read", "View programs"},
	{"programs:write", "Manage programs"},
	{"courses:read", "View courses"},
	{"courses:write", "Manage courses"},
	{"exams:read", "View exams"},
	{"exams:write", "Manage exams"},
	{"exams:take", "Take exams"},
	{"exams:grade", "Grade exams"},
	{"reports:read", "View reports"},
	{"theme:write", "Manage tenant theme"},
	{"audit:read", "View audit logs"},
}

func seedPermissions(ctx context.Context, db *sql.DB) error {
	for _, p := range permissions {
		_, err := db.ExecContext(ctx,
			`INSERT INTO permissions (slug, name) VALUES ($1, $2) ON CONFLICT (slug) DO NOTHING`,
			p.Slug, p.Name,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func seedTenant(ctx context.Context, db *sql.DB) (string, error) {
	var id string
	err := db.QueryRowContext(ctx,
		`INSERT INTO tenants (id, name, code, status)
		 VALUES ('d0000000-0000-0000-0000-000000000001', 'SMA Morfoschools Demo', 'demo-school', 'active')
		 ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id`,
	).Scan(&id)
	if err != nil {
		return "", err
	}

	// Seed theme
	_, err = db.ExecContext(ctx,
		`INSERT INTO tenant_theme_settings (tenant_id, preset, primary_color, accent_color, brand_name)
		 VALUES ($1, 'default', 'oklch(0.55 0.15 250)', 'oklch(0.6 0.18 145)', 'SMA Morfoschools Demo')
		 ON CONFLICT (tenant_id) DO NOTHING`,
		id,
	)
	return id, err
}

type roleSpec struct {
	Slug        string
	Name        string
	Permissions []string
}

var roles = []roleSpec{
	{"master_admin", "Master Admin", []string{
		"users:read", "users:write", "tenants:read", "tenants:write",
		"academic:read", "academic:write", "programs:read", "programs:write",
		"courses:read", "courses:write", "exams:read", "exams:write", "exams:grade",
		"reports:read", "theme:write", "audit:read",
	}},
	{"school_admin", "School Admin", []string{
		"users:read", "users:write", "academic:read", "academic:write",
		"programs:read", "programs:write", "courses:read", "courses:write",
		"exams:read", "exams:write", "exams:grade", "reports:read", "theme:write", "audit:read",
	}},
	{"academic_admin", "Academic Admin", []string{
		"users:read", "academic:read", "academic:write",
		"programs:read", "programs:write", "courses:read", "exams:read", "reports:read",
	}},
	{"teacher", "Teacher", []string{
		"courses:read", "courses:write", "exams:read", "exams:write", "exams:grade",
		"programs:read", "reports:read", "academic:read",
	}},
	{"student", "Student", []string{
		"courses:read", "exams:read", "exams:take", "programs:read",
	}},
	{"staff", "Staff", []string{
		"users:read", "academic:read",
	}},
	{"guardian", "Guardian", []string{
		"reports:read",
	}},
}

func seedRoles(ctx context.Context, db *sql.DB, tenantID string) error {
	for _, r := range roles {
		var roleID string
		err := db.QueryRowContext(ctx,
			`INSERT INTO roles (tenant_id, name, slug, is_system)
			 VALUES ($1, $2, $3, true)
			 ON CONFLICT (tenant_id, slug) DO UPDATE SET name = EXCLUDED.name
			 RETURNING id`,
			tenantID, r.Name, r.Slug,
		).Scan(&roleID)
		if err != nil {
			return fmt.Errorf("seed role %s: %w", r.Slug, err)
		}

		// Assign permissions
		for _, permSlug := range r.Permissions {
			_, err := db.ExecContext(ctx,
				`INSERT INTO role_permissions (role_id, permission_id)
				 SELECT $1, id FROM permissions WHERE slug = $2
				 ON CONFLICT DO NOTHING`,
				roleID, permSlug,
			)
			if err != nil {
				return fmt.Errorf("assign permission %s to role %s: %w", permSlug, r.Slug, err)
			}
		}
	}

	// Also seed platform-level master_admin role (tenant_id = NULL)
	var platformRoleID string
	err := db.QueryRowContext(ctx,
		`INSERT INTO roles (tenant_id, name, slug, is_system)
		 VALUES (NULL, 'Platform Master Admin', 'master_admin', true)
		 ON CONFLICT (tenant_id, slug) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id`,
	).Scan(&platformRoleID)
	if err != nil {
		return fmt.Errorf("seed platform master_admin role: %w", err)
	}

	// Give platform master_admin all permissions
	for _, p := range permissions {
		_, err := db.ExecContext(ctx,
			`INSERT INTO role_permissions (role_id, permission_id)
			 SELECT $1, id FROM permissions WHERE slug = $2
			 ON CONFLICT DO NOTHING`,
			platformRoleID, p.Slug,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

type userSpec struct {
	Email       string
	DisplayName string
	Password    string
	IsPlatform  bool
	RoleSlug    string
}

var users = []userSpec{
	{"master@morfoschools.local", "Master Admin", "master123", true, "master_admin"},
	{"admin@morfoschools.local", "School Admin", "admin123", false, "school_admin"},
	{"academic@morfoschools.local", "Academic Admin", "academic123", false, "academic_admin"},
	{"teacher@morfoschools.local", "Guru Demo", "teacher123", false, "teacher"},
	{"student@morfoschools.local", "Siswa Demo", "student123", false, "student"},
	{"staff@morfoschools.local", "Staff Demo", "staff123", false, "staff"},
	{"guardian@morfoschools.local", "Wali Demo", "guardian123", false, "guardian"},
}

func seedUsers(ctx context.Context, db *sql.DB, tenantID string) error {
	for _, u := range users {
		// Upsert user
		var userID string
		err := db.QueryRowContext(ctx,
			`INSERT INTO users (email, display_name, status, is_platform_admin)
			 VALUES ($1, $2, 'active', $3)
			 ON CONFLICT (email) DO UPDATE SET display_name = EXCLUDED.display_name
			 RETURNING id`,
			u.Email, u.DisplayName, u.IsPlatform,
		).Scan(&userID)
		if err != nil {
			return fmt.Errorf("seed user %s: %w", u.Email, err)
		}

		// Upsert password
		hash := hashPassword(u.Password)
		_, err = db.ExecContext(ctx,
			`INSERT INTO password_credentials (user_id, password_hash)
			 VALUES ($1, $2)
			 ON CONFLICT (user_id) DO UPDATE SET password_hash = EXCLUDED.password_hash`,
			userID, hash,
		)
		if err != nil {
			return fmt.Errorf("seed password for %s: %w", u.Email, err)
		}

		// Tenant membership (non-platform users)
		if !u.IsPlatform {
			_, err = db.ExecContext(ctx,
				`INSERT INTO tenant_memberships (tenant_id, user_id, status, is_primary)
				 VALUES ($1, $2, 'active', true)
				 ON CONFLICT (tenant_id, user_id) DO NOTHING`,
				tenantID, userID,
			)
			if err != nil {
				return fmt.Errorf("seed membership for %s: %w", u.Email, err)
			}

			// Assign tenant-scoped role
			_, err = db.ExecContext(ctx,
				`INSERT INTO user_roles (tenant_id, user_id, role_id)
				 SELECT $1, $2, id FROM roles WHERE tenant_id = $1 AND slug = $3
				 ON CONFLICT (tenant_id, user_id, role_id) DO NOTHING`,
				tenantID, userID, u.RoleSlug,
			)
			if err != nil {
				return fmt.Errorf("seed role for %s: %w", u.Email, err)
			}
		} else {
			// Platform master admin gets platform-level role
			_, err = db.ExecContext(ctx,
				`INSERT INTO user_roles (tenant_id, user_id, role_id)
				 SELECT $1, $2, id FROM roles WHERE tenant_id IS NULL AND slug = 'master_admin'
				 ON CONFLICT (tenant_id, user_id, role_id) DO NOTHING`,
				tenantID, userID,
			)
			if err != nil {
				return fmt.Errorf("seed platform role for %s: %w", u.Email, err)
			}
		}
	}
	return nil
}

func hashPassword(password string) string {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%s$%s",
		hex.EncodeToString(salt),
		hex.EncodeToString(hash),
	)
}
