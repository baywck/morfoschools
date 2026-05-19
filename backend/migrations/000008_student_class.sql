-- Student class enrollment (which class a student belongs to)
ALTER TABLE students ADD COLUMN IF NOT EXISTS class_section_id UUID REFERENCES class_sections(id);
CREATE INDEX IF NOT EXISTS idx_students_class_section ON students(tenant_id, class_section_id);
