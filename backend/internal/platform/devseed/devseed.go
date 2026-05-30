package devseed

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
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

	// Seed curricula (master, idempotent)
	if err := seedCurricula(ctx, db); err != nil {
		return fmt.Errorf("seed curricula: %w", err)
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

	// Seed master subjects (idempotent, tenant-scoped)
	if err := seedSubjects(ctx, db, tenantID); err != nil {
		return fmt.Errorf("seed subjects: %w", err)
	}

	// Seed example blueprint templates so first-run users can experience
	// the kisi-kisi flow without manual data entry. Idempotent on title.
	if err := seedBlueprintTemplates(ctx, db, tenantID); err != nil {
		return fmt.Errorf("seed blueprint templates: %w", err)
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
	{"blueprints:read", "View assessment blueprints (kisi-kisi)"},
	{"blueprints:write", "Manage assessment blueprints (kisi-kisi)"},
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
		"blueprints:read", "blueprints:write",
		"reports:read", "theme:write", "audit:read",
	}},
	{"school_admin", "School Admin", []string{
		"users:read", "users:write", "academic:read", "academic:write",
		"programs:read", "programs:write", "courses:read", "courses:write",
		"exams:read", "exams:write", "exams:grade",
		"blueprints:read", "blueprints:write",
		"reports:read", "theme:write", "audit:read",
	}},
	{"academic_admin", "Academic Admin", []string{
		"users:read", "academic:read", "academic:write",
		"programs:read", "programs:write", "courses:read", "exams:read",
		"blueprints:read", "blueprints:write",
		"reports:read",
	}},
	{"teacher", "Teacher", []string{
		"courses:read", "courses:write", "exams:read", "exams:write", "exams:grade",
		"blueprints:read", "blueprints:write",
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
		// Upsert user. We can't use ON CONFLICT (email) anymore because the
		// unique index on users.email is partial (WHERE status != 'archived')
		// per ADR-0007. Postgres rejects ON CONFLICT inference against
		// partial indexes, so we do an explicit lookup-then-insert/update.
		var userID string
		err := db.QueryRowContext(ctx,
			`SELECT id FROM users WHERE email = $1 AND status != 'archived'`,
			u.Email,
		).Scan(&userID)
		if errors.Is(err, sql.ErrNoRows) {
			err = db.QueryRowContext(ctx,
				`INSERT INTO users (email, display_name, status, is_platform_admin)
				 VALUES ($1, $2, 'active', $3) RETURNING id`,
				u.Email, u.DisplayName, u.IsPlatform,
			).Scan(&userID)
		} else if err == nil {
			_, err = db.ExecContext(ctx,
				`UPDATE users SET display_name = $2, is_platform_admin = $3, status = 'active' WHERE id = $1`,
				userID, u.DisplayName, u.IsPlatform,
			)
		}
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

// curricula seeds the curricula master table. Idempotent.
//
// The competency_label is what the frontend renders in dropdowns and
// table headers when editing a slot whose blueprint uses this curriculum.
// K13 uses "KD" (Kompetensi Dasar), Merdeka uses "CP" (Capaian
// Pembelajaran), AKM templates use the AKM-specific "Konten" axis as
// the primary label.
var curricula = []struct {
	Code            string
	Name            string
	Description     string
	CompetencyLabel string
}{
	{"k13", "Kurikulum 2013", "Kurikulum 2013 (revisi 2018) — terstruktur per Kompetensi Dasar (KD).", "KD"},
	{"merdeka", "Kurikulum Merdeka", "Kurikulum Merdeka — terstruktur per Capaian Pembelajaran (CP) per fase.", "CP"},
	{"akm_numerasi", "AKM Numerasi", "Asesmen Kompetensi Minimum — kompetensi numerasi (Bilangan, Aljabar, Geometri, Data & Ketidakpastian).", "Konten"},
	{"akm_literasi", "AKM Literasi", "Asesmen Kompetensi Minimum — kompetensi literasi membaca (Teks Informasi, Teks Sastra).", "Konten"},
}

func seedCurricula(ctx context.Context, db *sql.DB) error {
	for _, c := range curricula {
		_, err := db.ExecContext(ctx, `
			INSERT INTO curricula (code, name, description, competency_label)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (code) DO UPDATE SET
				name = EXCLUDED.name,
				description = EXCLUDED.description,
				competency_label = EXCLUDED.competency_label`,
			c.Code, c.Name, c.Description, c.CompetencyLabel,
		)
		if err != nil {
			return fmt.Errorf("seed curriculum %s: %w", c.Code, err)
		}
	}
	return nil
}

// ============================================================================
// Subjects (master, per tenant)
// ============================================================================

var masterSubjects = []struct {
	Code        string
	Name        string
	Description string
}{
	{"matematika", "Matematika", "Mata pelajaran Matematika."},
	{"pendidikan-pancasila", "Pendidikan Pancasila", "Mata pelajaran Pendidikan Pancasila."},
	{"bahasa-indonesia", "Bahasa Indonesia", "Mata pelajaran Bahasa Indonesia."},
	{"ilmu-pengetahuan-alam", "Ilmu Pengetahuan Alam", "Mata pelajaran IPA."},
	{"ilmu-pengetahuan-sosial", "Ilmu Pengetahuan Sosial", "Mata pelajaran IPS."},
}

func seedSubjects(ctx context.Context, db *sql.DB, tenantID string) error {
	for _, s := range masterSubjects {
		_, err := db.ExecContext(ctx, `
			INSERT INTO subjects (tenant_id, code, name, description, status)
			VALUES ($1, $2, $3, $4, 'active')
			ON CONFLICT (tenant_id, code) DO UPDATE SET
				name = EXCLUDED.name,
				description = EXCLUDED.description,
				status = CASE WHEN subjects.status = 'archived' THEN 'active' ELSE subjects.status END`,
			tenantID, s.Code, s.Name, s.Description,
		)
		if err != nil {
			return fmt.Errorf("seed subject %s: %w", s.Code, err)
		}
	}
	return nil
}

// ============================================================================
// Blueprint templates (real-data examples for first-run UX)
// ============================================================================

// blueprintSeedSlot mirrors the column shape of blueprint_template_slots
// minus the FK / timestamp columns. Used as a literal table here so the
// seed reads top-down without juggling slice indices.
type blueprintSeedSlot struct {
	CompetencyCode        string
	CompetencyDescription string
	Materi                string
	Indikator             string
	CognitiveLevel        string // C1..C6
	Difficulty            string // mudah/sedang/sulit
	QuestionType          string // multiple_choice / short_answer / essay / true_false
	Points                float64
}

type blueprintSeed struct {
	Title          string
	Description    string
	CurriculumCode string
	SubjectCode    string
	GradeOrPhase   string
	BlueprintType  string
	Status         string // draft / published
	StrictCoverage bool
	Slots          []blueprintSeedSlot
}

// blueprintSeeds: two realistic K13 examples covering MTK + PKN kelas 10.
// Hand-curated from the published Permendikbud KD list — the codes,
// descriptions, and indikator wording reflect the standard so teachers
// can use them as a working starting point. Cognitive levels follow
// Bloom revisi (C1–C6); difficulty is the editor's call per item.
var blueprintSeeds = []blueprintSeed{
	{
		Title:          "Contoh: UH Matematika Kelas 10 — Eksponen & Logaritma",
		Description:    "Kisi-kisi ulangan harian materi Eksponen dan Logaritma (Bab 1) untuk kelas X K13. 10 butir, campuran C1–C4.",
		CurriculumCode: "k13",
		SubjectCode:    "matematika",
		GradeOrPhase:   "10",
		BlueprintType:  "reguler",
		Status:         "published",
		StrictCoverage: false,
		Slots: []blueprintSeedSlot{
			{
				CompetencyCode:        "3.1",
				CompetencyDescription: "Menggeneralisasi sifat-sifat operasi bilangan berpangkat.",
				Materi:                "Sifat-sifat eksponen",
				Indikator:             "Disajikan ekspresi a^m · a^n, peserta didik dapat menentukan hasilnya.",
				CognitiveLevel:        "C2", Difficulty: "mudah",
				QuestionType: "multiple_choice", Points: 1,
			},
			{
				CompetencyCode:        "3.1",
				CompetencyDescription: "Menggeneralisasi sifat-sifat operasi bilangan berpangkat.",
				Materi:                "Pangkat negatif dan nol",
				Indikator:             "Peserta didik dapat menyederhanakan bentuk a^(-n) menjadi 1/a^n untuk a ≠ 0.",
				CognitiveLevel:        "C3", Difficulty: "sedang",
				QuestionType: "multiple_choice", Points: 1,
			},
			{
				CompetencyCode:        "3.1",
				CompetencyDescription: "Menggeneralisasi sifat-sifat operasi bilangan berpangkat.",
				Materi:                "Persamaan eksponen sederhana",
				Indikator:             "Diberikan persamaan 2^x = 16, peserta didik dapat menentukan nilai x.",
				CognitiveLevel:        "C3", Difficulty: "sedang",
				QuestionType: "short_answer", Points: 2,
			},
			{
				CompetencyCode:        "3.1",
				CompetencyDescription: "Menggeneralisasi sifat-sifat operasi bilangan berpangkat.",
				Materi:                "Pertidaksamaan eksponen",
				Indikator:             "Peserta didik dapat menentukan himpunan penyelesaian dari 3^x ≥ 27.",
				CognitiveLevel:        "C4", Difficulty: "sulit",
				QuestionType: "short_answer", Points: 3,
			},
			{
				CompetencyCode:        "3.1",
				CompetencyDescription: "Menggeneralisasi sifat-sifat operasi bilangan berpangkat.",
				Materi:                "Aplikasi eksponen — pertumbuhan",
				Indikator:             "Diberikan kasus pertumbuhan bakteri, peserta didik dapat memodelkan dengan fungsi eksponen.",
				CognitiveLevel:        "C4", Difficulty: "sulit",
				QuestionType: "essay", Points: 5,
			},
			{
				CompetencyCode:        "3.2",
				CompetencyDescription: "Menjelaskan dan menentukan penyelesaian persamaan/pertidaksamaan logaritma.",
				Materi:                "Definisi logaritma",
				Indikator:             "Peserta didik dapat menyatakan hubungan a^c = b sebagai ²log b = c.",
				CognitiveLevel:        "C2", Difficulty: "mudah",
				QuestionType: "multiple_choice", Points: 1,
			},
			{
				CompetencyCode:        "3.2",
				CompetencyDescription: "Menjelaskan dan menentukan penyelesaian persamaan/pertidaksamaan logaritma.",
				Materi:                "Sifat-sifat logaritma",
				Indikator:             "Diberikan ekspresi log a + log b, peserta didik dapat menyederhanakan menjadi log(ab).",
				CognitiveLevel:        "C3", Difficulty: "sedang",
				QuestionType: "multiple_choice", Points: 1,
			},
			{
				CompetencyCode:        "3.2",
				CompetencyDescription: "Menjelaskan dan menentukan penyelesaian persamaan/pertidaksamaan logaritma.",
				Materi:                "Persamaan logaritma",
				Indikator:             "Peserta didik dapat menyelesaikan persamaan ²log(x+1) = 3.",
				CognitiveLevel:        "C3", Difficulty: "sedang",
				QuestionType: "short_answer", Points: 2,
			},
			{
				CompetencyCode:        "3.2",
				CompetencyDescription: "Menjelaskan dan menentukan penyelesaian persamaan/pertidaksamaan logaritma.",
				Materi:                "Pertidaksamaan logaritma",
				Indikator:             "Peserta didik dapat menentukan himpunan penyelesaian ³log(x−1) < 2.",
				CognitiveLevel:        "C4", Difficulty: "sulit",
				QuestionType: "short_answer", Points: 3,
			},
			{
				CompetencyCode:        "3.2",
				CompetencyDescription: "Menjelaskan dan menentukan penyelesaian persamaan/pertidaksamaan logaritma.",
				Materi:                "Aplikasi logaritma — pH / decibel",
				Indikator:             "Diberikan rumus pH = -log[H+], peserta didik dapat menghitung pH dari konsentrasi ion hidrogen yang diberikan.",
				CognitiveLevel:        "C4", Difficulty: "sulit",
				QuestionType: "essay", Points: 5,
			},
		},
	},
	{
		Title:          "Contoh: UTS PKN Kelas 10 — Pancasila & UUD NRI 1945",
		Description:    "Kisi-kisi UTS PPKn semester ganjil kelas X K13. 8 butir, fokus Bab 1 (Hakikat Bangsa & Negara) dan Bab 2 (Pembagian Kekuasaan).",
		CurriculumCode: "k13",
		SubjectCode:    "pkn",
		GradeOrPhase:   "10",
		BlueprintType:  "reguler",
		Status:         "published",
		StrictCoverage: false,
		Slots: []blueprintSeedSlot{
			{
				CompetencyCode:        "3.1",
				CompetencyDescription: "Menganalisis nilai-nilai Pancasila dalam kerangka praktik penyelenggaraan pemerintahan negara.",
				Materi:                "Hakikat bangsa dan negara",
				Indikator:             "Peserta didik dapat menjelaskan pengertian bangsa menurut Ernest Renan.",
				CognitiveLevel:        "C2", Difficulty: "mudah",
				QuestionType: "multiple_choice", Points: 1,
			},
			{
				CompetencyCode:        "3.1",
				CompetencyDescription: "Menganalisis nilai-nilai Pancasila dalam kerangka praktik penyelenggaraan pemerintahan negara.",
				Materi:                "Unsur-unsur pembentuk negara",
				Indikator:             "Disajikan deskripsi suatu wilayah, peserta didik dapat menentukan unsur konstitutif yang terpenuhi.",
				CognitiveLevel:        "C3", Difficulty: "sedang",
				QuestionType: "multiple_choice", Points: 1,
			},
			{
				CompetencyCode:        "3.1",
				CompetencyDescription: "Menganalisis nilai-nilai Pancasila dalam kerangka praktik penyelenggaraan pemerintahan negara.",
				Materi:                "Nilai-nilai Pancasila sebagai dasar negara",
				Indikator:             "Peserta didik dapat menganalisis penerapan sila ke-3 Pancasila dalam kebijakan publik.",
				CognitiveLevel:        "C4", Difficulty: "sulit",
				QuestionType: "essay", Points: 5,
			},
			{
				CompetencyCode:        "3.2",
				CompetencyDescription: "Menelaah ketentuan UUD NRI Tahun 1945 yang mengatur tentang wilayah, warga negara, agama dan kepercayaan, serta pertahanan dan keamanan.",
				Materi:                "Wilayah NKRI",
				Indikator:             "Peserta didik dapat menyebutkan batas-batas wilayah NKRI sesuai pasal 25A UUD NRI 1945.",
				CognitiveLevel:        "C1", Difficulty: "mudah",
				QuestionType: "short_answer", Points: 2,
			},
			{
				CompetencyCode:        "3.2",
				CompetencyDescription: "Menelaah ketentuan UUD NRI Tahun 1945 yang mengatur tentang wilayah, warga negara, agama dan kepercayaan, serta pertahanan dan keamanan.",
				Materi:                "Kewarganegaraan Indonesia",
				Indikator:             "Diberikan kasus, peserta didik dapat menentukan asas kewarganegaraan yang dipakai (ius soli/ius sanguinis).",
				CognitiveLevel:        "C3", Difficulty: "sedang",
				QuestionType: "multiple_choice", Points: 1,
			},
			{
				CompetencyCode:        "3.3",
				CompetencyDescription: "Menganalisis fungsi dan kewenangan lembaga-lembaga negara menurut UUD NRI Tahun 1945.",
				Materi:                "Pembagian kekuasaan secara horizontal",
				Indikator:             "Peserta didik dapat membedakan kekuasaan eksekutif, legislatif, dan yudikatif beserta lembaganya.",
				CognitiveLevel:        "C2", Difficulty: "mudah",
				QuestionType: "multiple_choice", Points: 1,
			},
			{
				CompetencyCode:        "3.3",
				CompetencyDescription: "Menganalisis fungsi dan kewenangan lembaga-lembaga negara menurut UUD NRI Tahun 1945.",
				Materi:                "Lembaga negara: MPR, DPR, DPD",
				Indikator:             "Peserta didik dapat menjelaskan kewenangan MPR pasca amandemen UUD 1945.",
				CognitiveLevel:        "C3", Difficulty: "sedang",
				QuestionType: "short_answer", Points: 2,
			},
			{
				CompetencyCode:        "3.3",
				CompetencyDescription: "Menganalisis fungsi dan kewenangan lembaga-lembaga negara menurut UUD NRI Tahun 1945.",
				Materi:                "Hubungan antar lembaga negara",
				Indikator:             "Disajikan suatu konflik kewenangan, peserta didik dapat menelaah lembaga mana yang berwenang menyelesaikan.",
				CognitiveLevel:        "C4", Difficulty: "sulit",
				QuestionType: "essay", Points: 5,
			},
		},
	},
}

// seedBlueprintTemplates inserts the example templates above. Idempotent
// on (tenant_id, title) so re-running devseed updates the row but does
// not duplicate. Slots are wiped + reinserted on each run so the
// authoritative copy is always the seed file.
func seedBlueprintTemplates(ctx context.Context, db *sql.DB, tenantID string) error {
	// Resolve owner — prefer the school admin so the template is shared
	// across teachers via the existing collaborator model.
	var ownerID string
	if err := db.QueryRowContext(ctx,
		`SELECT id FROM users WHERE email = $1 AND status != 'archived'`,
		"admin@morfoschools.local",
	).Scan(&ownerID); err != nil {
		return fmt.Errorf("resolve admin user: %w", err)
	}

	for _, b := range blueprintSeeds {
		var curriculumID string
		if err := db.QueryRowContext(ctx,
			`SELECT id FROM curricula WHERE code = $1`, b.CurriculumCode,
		).Scan(&curriculumID); err != nil {
			return fmt.Errorf("resolve curriculum %s: %w", b.CurriculumCode, err)
		}

		// Upsert blueprint header. UNIQUE constraint on (tenant_id, title)
		// is not present in schema, so we lookup-then-insert/update.
		var templateID string
		err := db.QueryRowContext(ctx,
			`SELECT id FROM blueprint_templates WHERE tenant_id = $1 AND title = $2`,
			tenantID, b.Title,
		).Scan(&templateID)
		if errors.Is(err, sql.ErrNoRows) {
			err = db.QueryRowContext(ctx, `
				INSERT INTO blueprint_templates (
				    tenant_id, owner_user_id, title, description,
				    curriculum_id, subject_code, grade_or_phase,
				    blueprint_type, strict_coverage, status, version
				) VALUES ($1, $2, $3, NULLIF($4,''),
				          $5, NULLIF($6,''), NULLIF($7,''),
				          $8, $9, $10, 1)
				RETURNING id`,
				tenantID, ownerID, b.Title, b.Description,
				curriculumID, b.SubjectCode, b.GradeOrPhase,
				b.BlueprintType, b.StrictCoverage, b.Status,
			).Scan(&templateID)
		} else if err == nil {
			_, err = db.ExecContext(ctx, `
				UPDATE blueprint_templates SET
				    description = NULLIF($2,''),
				    curriculum_id = $3,
				    subject_code = NULLIF($4,''),
				    grade_or_phase = NULLIF($5,''),
				    blueprint_type = $6,
				    strict_coverage = $7,
				    status = $8,
				    updated_at = now()
				 WHERE id = $1`,
				templateID, b.Description, curriculumID, b.SubjectCode,
				b.GradeOrPhase, b.BlueprintType, b.StrictCoverage, b.Status,
			)
		}
		if err != nil {
			return fmt.Errorf("seed blueprint %q: %w", b.Title, err)
		}

		// Wipe + reinsert slots so the seed file is authoritative.
		if _, err := db.ExecContext(ctx,
			`DELETE FROM blueprint_template_slots WHERE template_id = $1`,
			templateID,
		); err != nil {
			return fmt.Errorf("clear slots %q: %w", b.Title, err)
		}
		totalPoints := 0.0
		for i, s := range b.Slots {
			if _, err := db.ExecContext(ctx, `
				INSERT INTO blueprint_template_slots (
				    template_id, position,
				    competency_code, competency_description, materi, indikator,
				    cognitive_level, difficulty, question_type, points
				) VALUES ($1, $2,
				          NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), NULLIF($6,''),
				          NULLIF($7,''), NULLIF($8,''), NULLIF($9,''), $10)`,
				templateID, i,
				s.CompetencyCode, s.CompetencyDescription, s.Materi, s.Indikator,
				s.CognitiveLevel, s.Difficulty, s.QuestionType, s.Points,
			); err != nil {
				return fmt.Errorf("insert slot %d for %q: %w", i, b.Title, err)
			}
			totalPoints += s.Points
		}

		// Refresh totals on the header so the list view shows the right
		// slot count + total points without a roundtrip.
		if _, err := db.ExecContext(ctx, `
			UPDATE blueprint_templates SET
			    total_slots = $2, total_points = $3, updated_at = now()
			 WHERE id = $1`,
			templateID, len(b.Slots), totalPoints,
		); err != nil {
			return fmt.Errorf("update totals %q: %w", b.Title, err)
		}
	}
	return nil
}
