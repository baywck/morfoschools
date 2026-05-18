-- 000004_programs.sql
-- Programs: enrollment unit combining courses + exams

-- Programs
CREATE TABLE IF NOT EXISTS programs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    kind TEXT NOT NULL DEFAULT 'regular' CHECK (kind IN ('regular', 'remedial', 'enrichment', 'tryout')),
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    academic_year_id UUID REFERENCES academic_years(id),
    semester_id UUID REFERENCES semesters(id),
    subject_id UUID REFERENCES subjects(id),
    grade_level TEXT,
    published_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_programs_tenant_status ON programs(tenant_id, status);

-- Program sections
CREATE TABLE IF NOT EXISTS program_sections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    program_id UUID NOT NULL REFERENCES programs(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    unlock_mode TEXT NOT NULL DEFAULT 'sequential' CHECK (unlock_mode IN ('sequential', 'always_open')),
    is_required BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_program_sections_program ON program_sections(program_id, sort_order);

-- Program items
CREATE TABLE IF NOT EXISTS program_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    program_id UUID NOT NULL REFERENCES programs(id) ON DELETE CASCADE,
    section_id UUID NOT NULL REFERENCES program_sections(id) ON DELETE CASCADE,
    item_type TEXT NOT NULL CHECK (item_type IN ('course', 'exam')),
    item_id UUID NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_required BOOLEAN NOT NULL DEFAULT true,
    passing_grade INTEGER CHECK (passing_grade BETWEEN 0 AND 100),
    max_attempts INTEGER NOT NULL DEFAULT 1,
    due_at TIMESTAMPTZ,
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_program_items_section ON program_items(section_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_program_items_item ON program_items(item_type, item_id);

-- Program assignments (to class_section or individual student)
CREATE TABLE IF NOT EXISTS program_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    program_id UUID NOT NULL REFERENCES programs(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL CHECK (target_type IN ('class_section', 'student')),
    target_id UUID NOT NULL,
    assigned_by UUID REFERENCES users(id),
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    auto_enroll BOOLEAN NOT NULL DEFAULT true,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'ended')),
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_program_assignments_program ON program_assignments(tenant_id, program_id);
CREATE INDEX IF NOT EXISTS idx_program_assignments_target ON program_assignments(target_type, target_id);

-- Program enrollments
CREATE TABLE IF NOT EXISTS program_enrollments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    program_id UUID NOT NULL REFERENCES programs(id) ON DELETE CASCADE,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    assignment_id UUID REFERENCES program_assignments(id),
    assigned_via TEXT NOT NULL DEFAULT 'direct' CHECK (assigned_via IN ('class_assignment', 'direct', 'admin')),
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'suspended', 'completed', 'withdrawn', 'cancelled')),
    result TEXT NOT NULL DEFAULT 'not_started' CHECK (result IN ('not_started', 'in_progress', 'pending_review', 'passed', 'failed', 'incomplete', 'void')),
    enrolled_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    withdrawn_at TIMESTAMPTZ,
    UNIQUE(tenant_id, program_id, student_id)
);
CREATE INDEX IF NOT EXISTS idx_program_enrollments_student ON program_enrollments(tenant_id, student_id);
CREATE INDEX IF NOT EXISTS idx_program_enrollments_program ON program_enrollments(tenant_id, program_id, status);

-- Class assignments (temporal delivery context)
CREATE TABLE IF NOT EXISTS class_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    enrollment_id UUID NOT NULL REFERENCES program_enrollments(id) ON DELETE CASCADE,
    class_section_id UUID NOT NULL REFERENCES class_sections(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'transferred_out', 'completed')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at TIMESTAMPTZ,
    transfer_reason TEXT
);
CREATE INDEX IF NOT EXISTS idx_class_assignments_enrollment ON class_assignments(enrollment_id);

-- Program item progress
CREATE TABLE IF NOT EXISTS program_item_progress (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    enrollment_id UUID NOT NULL REFERENCES program_enrollments(id) ON DELETE CASCADE,
    program_item_id UUID NOT NULL REFERENCES program_items(id) ON DELETE CASCADE,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    section_id UUID NOT NULL REFERENCES program_sections(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'locked' CHECK (status IN ('locked', 'available', 'in_progress', 'completed')),
    result TEXT NOT NULL DEFAULT 'none' CHECK (result IN ('none', 'passed', 'failed', 'exempted', 'overridden')),
    best_score NUMERIC,
    latest_score NUMERIC,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    manual_override_by UUID REFERENCES users(id),
    manual_override_at TIMESTAMPTZ,
    UNIQUE(tenant_id, enrollment_id, program_item_id)
);
CREATE INDEX IF NOT EXISTS idx_program_item_progress_enrollment ON program_item_progress(enrollment_id);
CREATE INDEX IF NOT EXISTS idx_program_item_progress_student ON program_item_progress(tenant_id, student_id);

-- Program exam attempts
CREATE TABLE IF NOT EXISTS program_exam_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    enrollment_id UUID NOT NULL REFERENCES program_enrollments(id) ON DELETE CASCADE,
    program_item_id UUID NOT NULL REFERENCES program_items(id) ON DELETE CASCADE,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    attempt_number INTEGER NOT NULL,
    score NUMERIC,
    max_score_snapshot NUMERIC NOT NULL,
    passing_score_snapshot NUMERIC NOT NULL,
    passed BOOLEAN,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    UNIQUE(tenant_id, enrollment_id, program_item_id, attempt_number)
);
