-- 000006_exams.sql
-- Exams: standalone assessment entities

-- Exams
CREATE TABLE IF NOT EXISTS exams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    subject_id UUID REFERENCES subjects(id),
    exam_type TEXT NOT NULL DEFAULT 'quiz' CHECK (exam_type IN ('quiz', 'midterm', 'final', 'tryout', 'daily')),
    duration_minutes INTEGER,
    max_score NUMERIC NOT NULL DEFAULT 100,
    passing_score NUMERIC NOT NULL DEFAULT 70,
    version INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    shuffle_questions BOOLEAN NOT NULL DEFAULT false,
    shuffle_options BOOLEAN NOT NULL DEFAULT false,
    show_result_immediately BOOLEAN NOT NULL DEFAULT false,
    created_by UUID REFERENCES users(id),
    published_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_exams_tenant_status ON exams(tenant_id, status);

-- Exam sections (grouping questions)
CREATE TABLE IF NOT EXISTS exam_sections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    exam_id UUID NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_exam_sections_exam ON exam_sections(exam_id, sort_order);

-- Exam questions
CREATE TABLE IF NOT EXISTS exam_questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    exam_id UUID NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    section_id UUID REFERENCES exam_sections(id) ON DELETE SET NULL,
    question_type TEXT NOT NULL CHECK (question_type IN ('multiple_choice', 'essay', 'short_answer', 'true_false')),
    content TEXT NOT NULL,
    explanation TEXT,
    correct_answer TEXT,
    rubric JSONB,
    points NUMERIC NOT NULL DEFAULT 1,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_exam_questions_exam ON exam_questions(exam_id, sort_order);

-- Exam question options (for multiple choice / true-false)
CREATE TABLE IF NOT EXISTS exam_question_options (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES exam_questions(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    is_correct BOOLEAN NOT NULL DEFAULT false,
    sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_exam_question_options_question ON exam_question_options(question_id, sort_order);

-- Exam gate windows (scheduling)
CREATE TABLE IF NOT EXISTS exam_gate_windows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    exam_id UUID NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    opens_at TIMESTAMPTZ NOT NULL,
    closes_at TIMESTAMPTZ NOT NULL,
    password TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_exam_gate_windows_exam ON exam_gate_windows(exam_id);

-- Exam attempts (actual student exam sessions)
CREATE TABLE IF NOT EXISTS exam_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    exam_id UUID NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'submitted', 'graded', 'voided')),
    score NUMERIC,
    max_score_snapshot NUMERIC NOT NULL,
    passing_score_snapshot NUMERIC NOT NULL,
    passed BOOLEAN,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    submitted_at TIMESTAMPTZ,
    graded_at TIMESTAMPTZ,
    graded_by UUID REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_exam_attempts_student ON exam_attempts(tenant_id, student_id, exam_id);
CREATE INDEX IF NOT EXISTS idx_exam_attempts_exam ON exam_attempts(tenant_id, exam_id, status);

-- Exam responses (per question per attempt)
CREATE TABLE IF NOT EXISTS exam_responses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    attempt_id UUID NOT NULL REFERENCES exam_attempts(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES exam_questions(id) ON DELETE CASCADE,
    answer TEXT,
    selected_option_id UUID REFERENCES exam_question_options(id),
    score NUMERIC,
    feedback TEXT,
    auto_graded BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(attempt_id, question_id)
);

-- Exam integrity events
CREATE TABLE IF NOT EXISTS exam_integrity_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    attempt_id UUID NOT NULL REFERENCES exam_attempts(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_exam_integrity_events_attempt ON exam_integrity_events(attempt_id);
