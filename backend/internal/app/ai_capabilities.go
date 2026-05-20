package app

import (
	"context"
	"encoding/json"
)

// AI Capabilities — auto-generated from API surface, grouped by domain
// Only relevant capabilities are injected per request based on intent + permissions

type Capability struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Permission  string          `json:"-"`
	Risk        string          `json:"-"` // read, write, destructive
	Domain      string          `json:"-"` // students, teachers, classes, subjects, academic, courses, programs, tenants
	Parameters  json.RawMessage `json:"parameters"`
}

// CapabilityRegistry holds all capabilities grouped by domain
type CapabilityRegistry struct {
	byDomain map[string][]Capability
	handlers map[string]ToolHandler
}

func NewCapabilityRegistry() *CapabilityRegistry {
	return &CapabilityRegistry{
		byDomain: make(map[string][]Capability),
		handlers: make(map[string]ToolHandler),
	}
}

func (r *CapabilityRegistry) Register(cap Capability, handler ToolHandler) {
	r.byDomain[cap.Domain] = append(r.byDomain[cap.Domain], cap)
	r.handlers[cap.Name] = handler
}

// GetToolsForIntent returns only tools relevant to detected domains + user permissions
func (r *CapabilityRegistry) GetToolsForIntent(domains []string, permissions []string) []map[string]any {
	permSet := make(map[string]bool, len(permissions))
	for _, p := range permissions {
		permSet[p] = true
	}

	var tools []map[string]any
	// Always include general tools
	domains = append(domains, "general")

	seen := make(map[string]bool)
	for _, domain := range domains {
		for _, cap := range r.byDomain[domain] {
			if seen[cap.Name] {
				continue
			}
			if cap.Permission != "" && !permSet[cap.Permission] {
				continue
			}
			seen[cap.Name] = true
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        cap.Name,
					"description": cap.Description,
					"parameters":  json.RawMessage(cap.Parameters),
				},
			})
		}
	}
	return tools
}

// RequiredPermissionFor returns the permission slug that gates a
// capability/tool by name, plus whether the capability is registered
// at all. Used by the AI confirm + short-reply paths to enforce that
// the user still holds the right permission at execution time — not
// just at proposal time. Empty permission with ok=true means the tool
// is freely callable (no perm gate).
func (r *CapabilityRegistry) RequiredPermissionFor(name string) (perm string, ok bool) {
	for _, caps := range r.byDomain {
		for _, c := range caps {
			if c.Name == name {
				return c.Permission, true
			}
		}
	}
	return "", false
}

// Execute runs a capability handler
func (r *CapabilityRegistry) Execute(ctx context.Context, tenantID, userID, name, args string) (string, error) {
	handler, ok := r.handlers[name]
	if !ok {
		return `{"error":"Unknown capability: ` + name + `"}`, nil
	}
	result, err := handler(ctx, tenantID, userID, json.RawMessage(args))
	if err != nil {
		return `{"error":"` + err.Error() + `"}`, nil
	}
	return result, nil
}

// DetectDomains infers which domains are relevant from the user message
// This is a fast keyword-based router — no LLM call needed
func DetectDomains(message string) []string {
	keywords := map[string][]string{
		"students": {"siswa", "student", "murid", "peserta didik", "enroll", "daftar siswa"},
		"teachers": {"guru", "teacher", "pengajar", "wali kelas", "homeroom"},
		"classes":  {"kelas", "class", "section", "ruang"},
		"subjects": {"mapel", "mata pelajaran", "subject", "pelajaran"},
		"academic": {"tahun ajaran", "academic year", "semester", "kurikulum"},
		"courses":  {"course", "kursus", "materi", "konten"},
		"programs": {"program", "extracurricular", "ekskul"},
		"tenants":  {"tenant", "sekolah", "school", "institusi"},
		"staff":    {"staff", "staf", "karyawan", "pegawai"},
		"admin":    {"admin", "administrator", "pengguna", "user", "akun"},
		"exams":      {"ujian", "exam", "tes", "test", "soal", "question", "kuis", "quiz", "nilai pasing", "essay", "pilihan ganda", "multiple choice", "true false", "benar salah"},
		"blueprints": {"blueprint", "kisi", "kisi-kisi", "kisi kisi", "slot", "kompetensi", "competency", "akm", "literasi", "numerasi", "kurikulum k13", "kurikulum merdeka", "reverse", "analisis soal", "stimulus", "stimuli"},
		"stats":    {"statistik", "jumlah", "berapa", "total", "count", "data", "info", "laporan"},
	}

	msgLower := toLower(message)
	var domains []string
	seen := make(map[string]bool)

	for domain, words := range keywords {
		for _, w := range words {
			if containsWord(msgLower, w) && !seen[domain] {
				domains = append(domains, domain)
				seen[domain] = true
				break
			}
		}
	}

	// Default to stats if no domain detected
	if len(domains) == 0 {
		domains = []string{"stats"}
	}

	return domains
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

func containsWord(text, word string) bool {
	return len(text) >= len(word) && (text == word || contains(text, word))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
