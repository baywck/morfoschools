-- 000015_blueprints_stimuli.sql
-- Phase 9.5 \u2014 assessment blueprints (kisi-kisi) for K13/Merdeka/AKM,
-- stimulus library, exam_question_groups for atomic AKM rendering.
-- Per ADR-0010.

-- =========================================================================
-- 1. Curricula master (small lookup, preloaded by devseed)
-- =========================================================================

CREATE TABLE IF NOT EXISTS curricula (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
        -- 'k13' | 'merdeka' | 'akm_numerasi' | 'akm_literasi'
    name TEXT NOT NULL,
    description TEXT,
    competency_label TEXT NOT NULL,
        -- 'KD' for k13, 'CP' for merdeka, 'Konten' for akm_*
        -- Used by frontend to render curriculum-aware terminology.
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- =========================================================================
-- 2. Competencies (master, populated over time; nullable FK from slots)
-- =========================================================================

CREATE TABLE IF NOT EXISTS competencies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    curriculum_id UUID NOT NULL REFERENCES curricula(id),
    subject_code TEXT NOT NULL,
        -- e.g. 'matematika', 'bahasa-indonesia', 'ipa'. Free-text now;
        -- master subjects table to be added later as needed.
    grade_or_phase TEXT NOT NULL,
        -- '10', '11', '12' for K13 (per kelas); 'A' through 'F' for
        -- Merdeka (per fase); '5', '8', '11' for AKM (per kelas survey)
    code TEXT NOT NULL,
        -- raw code as it appears in the curriculum doc, e.g. '3.1'
    normalized_code TEXT NOT NULL,
        -- lowercase, dot-stripped, whitespace-collapsed for dedup
        -- lookups. Set by app code, not a generated column, because
        -- the normalization logic may evolve.
    description TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (curriculum_id, subject_code, grade_or_phase, normalized_code)
);
CREATE INDEX IF NOT EXISTS idx_competencies_lookup
    ON competencies(curriculum_id, subject_code, grade_or_phase);

-- =========================================================================
-- 3. Stimuli (tenant-scoped library, text-only for now)
-- =========================================================================

CREATE TABLE IF NOT EXISTS stimuli (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    owner_user_id UUID NOT NULL REFERENCES users(id),
    type TEXT NOT NULL DEFAULT 'text' CHECK (type IN ('text')),
        -- Future: 'image', 'audio', 'video', 'table'. Constrained to
        -- 'text' in this phase so handlers don't need to branch.
    title TEXT NOT NULL,
    content TEXT NOT NULL,
        -- Markdown body. Rendered server-side or client-side; treat as
        -- untrusted user input on render.
    source TEXT,
        -- Optional citation, e.g. "Adapted from Kompas, 2024"
    lifecycle TEXT NOT NULL DEFAULT 'exam_scoped'
        CHECK (lifecycle IN ('exam_scoped', 'shared', 'archived')),
        -- exam_scoped: created inline in question form, only visible
        --   inside its parent exam. Reusable only after explicit
        --   "Save to library" promotion.
        -- shared: visible across the tenant in the library.
        -- archived: hidden from new selections but kept for audit.
    parent_exam_id UUID REFERENCES exams(id) ON DELETE SET NULL,
        -- Set when lifecycle = 'exam_scoped' so we can show it only
        -- inside that exam. Cleared when promoted to 'shared'.
    usage_count INTEGER NOT NULL DEFAULT 0,
        -- Cached: number of exam_question_groups currently referencing
        -- this stimulus. Maintained by trigger (see below).
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_stimuli_tenant_lifecycle
    ON stimuli(tenant_id, lifecycle);
CREATE INDEX IF NOT EXISTS idx_stimuli_parent_exam
    ON stimuli(parent_exam_id) WHERE parent_exam_id IS NOT NULL;

-- =========================================================================
-- 4. Exam question groups (atomic shuffle units)
-- =========================================================================
--
-- Every exam question lives inside a group. For non-AKM exams, each
-- question gets a `group_type='standalone'` row of its own (1:1). For
-- AKM, multiple questions share a `group_type='stimulus'` group with a
-- shared stimulus snapshot.
--
-- Phase 10 (take-exam) renders by iterating groups in display_order.
-- Within a group, questions never shuffle. Stimulus is rendered once
-- above the group's questions.

CREATE TABLE IF NOT EXISTS exam_question_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    exam_id UUID NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    section_id UUID REFERENCES exam_sections(id) ON DELETE SET NULL,
    stimulus_id UUID REFERENCES stimuli(id) ON DELETE SET NULL,
        -- Live link to library stimulus. ON DELETE SET NULL is safe
        -- because the snapshot below is what students see.
    stimulus_title_snapshot TEXT,
    stimulus_body_snapshot TEXT,
        -- Frozen copy taken when the group is first associated with a
        -- stimulus or when "Sync from library" is invoked on a draft
        -- exam. Locked once the parent exam is published or attempted.
    group_type TEXT NOT NULL DEFAULT 'standalone'
        CHECK (group_type IN ('standalone', 'stimulus')),
    display_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_exam_question_groups_exam_order
    ON exam_question_groups(exam_id, display_order);
CREATE INDEX IF NOT EXISTS idx_exam_question_groups_section
    ON exam_question_groups(section_id) WHERE section_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_exam_question_groups_stimulus
    ON exam_question_groups(stimulus_id) WHERE stimulus_id IS NOT NULL;

-- Stimulus usage_count maintenance
CREATE OR REPLACE FUNCTION fn_stimulus_recount_usage()
RETURNS TRIGGER AS $$
DECLARE
    sid UUID;
BEGIN
    IF TG_OP = 'INSERT' THEN
        sid := NEW.stimulus_id;
    ELSIF TG_OP = 'DELETE' THEN
        sid := OLD.stimulus_id;
    ELSE
        IF OLD.stimulus_id IS DISTINCT FROM NEW.stimulus_id THEN
            IF OLD.stimulus_id IS NOT NULL THEN
                UPDATE stimuli SET usage_count = (
                    SELECT COUNT(*) FROM exam_question_groups
                     WHERE stimulus_id = OLD.stimulus_id
                ) WHERE id = OLD.stimulus_id;
            END IF;
            sid := NEW.stimulus_id;
        ELSE
            RETURN NEW;
        END IF;
    END IF;
    IF sid IS NOT NULL THEN
        UPDATE stimuli SET usage_count = (
            SELECT COUNT(*) FROM exam_question_groups
             WHERE stimulus_id = sid
        ) WHERE id = sid;
    END IF;
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql SET search_path = pg_catalog, public;

DROP TRIGGER IF EXISTS trg_stimulus_recount ON exam_question_groups;
CREATE TRIGGER trg_stimulus_recount
    AFTER INSERT OR UPDATE OF stimulus_id OR DELETE
    ON exam_question_groups
    FOR EACH ROW
    EXECUTE FUNCTION fn_stimulus_recount_usage();

-- =========================================================================
-- 5. Blueprint templates (library, reusable per tenant)
-- =========================================================================

CREATE TABLE IF NOT EXISTS blueprint_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    owner_user_id UUID NOT NULL REFERENCES users(id),
    title TEXT NOT NULL,
    description TEXT,
    curriculum_id UUID NOT NULL REFERENCES curricula(id),
    subject_code TEXT,
    grade_or_phase TEXT,
    blueprint_type TEXT NOT NULL DEFAULT 'reguler'
        CHECK (blueprint_type IN ('reguler', 'akm_literasi', 'akm_numerasi')),
    total_slots INTEGER NOT NULL DEFAULT 0,
    total_points NUMERIC NOT NULL DEFAULT 0,
    strict_coverage BOOLEAN NOT NULL DEFAULT false,
        -- AKM templates default to strict at the application layer;
        -- column default is false here so reguler templates are not
        -- accidentally locked.
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'published', 'archived')),
    version INTEGER NOT NULL DEFAULT 1,
        -- Advisory; increments on edit. Cloned exam_blueprints record
        -- the version they snapshotted from.
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_blueprint_templates_tenant_status
    ON blueprint_templates(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_blueprint_templates_owner
    ON blueprint_templates(owner_user_id);

CREATE TABLE IF NOT EXISTS blueprint_template_collaborators (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    template_id UUID NOT NULL REFERENCES blueprint_templates(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('editor', 'viewer')),
    invited_by UUID REFERENCES users(id),
    invited_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (template_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_blueprint_template_collaborators_user
    ON blueprint_template_collaborators(user_id, template_id);

-- =========================================================================
-- 6. Blueprint template slots (the structural definition)
-- =========================================================================

CREATE TABLE IF NOT EXISTS blueprint_template_slots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID NOT NULL REFERENCES blueprint_templates(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    competency_id UUID REFERENCES competencies(id),
    competency_code TEXT,
    competency_description TEXT,
    materi TEXT,
    indikator TEXT,
    cognitive_level TEXT
        CHECK (cognitive_level IS NULL OR cognitive_level IN ('C1','C2','C3','C4','C5','C6')),
    difficulty TEXT
        CHECK (difficulty IS NULL OR difficulty IN ('mudah','sedang','sulit')),
    question_type TEXT
        CHECK (question_type IS NULL OR question_type IN
            ('multiple_choice','true_false','short_answer','essay')),
    points NUMERIC NOT NULL DEFAULT 1,
    stimulus_id UUID REFERENCES stimuli(id),
        -- Optional template-level stimulus reference. When the template
        -- is cloned to an exam, the stimulus is copied (referenced by
        -- exam_question_groups) and a snapshot is captured.
    -- AKM-specific (NULL for blueprint_type='reguler')
    akm_konten TEXT,
    akm_konteks TEXT,
    akm_proses TEXT,
    akm_level INTEGER CHECK (akm_level IS NULL OR akm_level BETWEEN 1 AND 5),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (template_id, position)
);
CREATE INDEX IF NOT EXISTS idx_blueprint_template_slots_position
    ON blueprint_template_slots(template_id, position);

-- =========================================================================
-- 7. Exam blueprints (snapshot of template, owned by exam)
-- =========================================================================

CREATE TABLE IF NOT EXISTS exam_blueprints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    exam_id UUID NOT NULL UNIQUE REFERENCES exams(id) ON DELETE CASCADE,
        -- 1:1 with exam: exam owns at most one blueprint.
    source_template_id UUID REFERENCES blueprint_templates(id) ON DELETE SET NULL,
        -- NULL when blueprint was created from scratch or generated by
        -- reverse-flow analysis. ON DELETE SET NULL preserves the exam
        -- blueprint even if the source template is deleted.
    source_template_version INTEGER,
        -- Snapshot of template.version at clone time.
    created_via TEXT NOT NULL DEFAULT 'manual'
        CHECK (created_via IN ('manual', 'template_clone', 'reverse_analysis')),
    title TEXT NOT NULL,
    description TEXT,
    curriculum_id UUID NOT NULL REFERENCES curricula(id),
    subject_code TEXT,
    grade_or_phase TEXT,
    blueprint_type TEXT NOT NULL DEFAULT 'reguler'
        CHECK (blueprint_type IN ('reguler', 'akm_literasi', 'akm_numerasi')),
    total_slots INTEGER NOT NULL DEFAULT 0,
    total_points NUMERIC NOT NULL DEFAULT 0,
    strict_coverage BOOLEAN NOT NULL DEFAULT false,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'locked')),
        -- 'locked' means the parent exam is published; blueprint
        -- becomes read-only.
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS exam_blueprint_slots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_blueprint_id UUID NOT NULL REFERENCES exam_blueprints(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    competency_id UUID REFERENCES competencies(id),
    competency_code TEXT,
    competency_description TEXT,
    materi TEXT,
    indikator TEXT,
    cognitive_level TEXT
        CHECK (cognitive_level IS NULL OR cognitive_level IN ('C1','C2','C3','C4','C5','C6')),
    difficulty TEXT
        CHECK (difficulty IS NULL OR difficulty IN ('mudah','sedang','sulit')),
    question_type TEXT
        CHECK (question_type IS NULL OR question_type IN
            ('multiple_choice','true_false','short_answer','essay')),
    points NUMERIC NOT NULL DEFAULT 1,
    stimulus_id UUID REFERENCES stimuli(id),
    akm_konten TEXT,
    akm_konteks TEXT,
    akm_proses TEXT,
    akm_level INTEGER CHECK (akm_level IS NULL OR akm_level BETWEEN 1 AND 5),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (exam_blueprint_id, position)
);
CREATE INDEX IF NOT EXISTS idx_exam_blueprint_slots_position
    ON exam_blueprint_slots(exam_blueprint_id, position);

-- =========================================================================
-- 8. Extend exam_questions with blueprint and group FKs
-- =========================================================================

ALTER TABLE exam_questions
    ADD COLUMN IF NOT EXISTS blueprint_slot_id UUID
        REFERENCES exam_blueprint_slots(id) ON DELETE SET NULL;

ALTER TABLE exam_questions
    ADD COLUMN IF NOT EXISTS group_id UUID
        REFERENCES exam_question_groups(id) ON DELETE SET NULL;

-- A slot can be filled by at most one question. Partial unique index
-- so the constraint only applies where slot is set.
CREATE UNIQUE INDEX IF NOT EXISTS idx_exam_questions_one_per_slot
    ON exam_questions(blueprint_slot_id)
 WHERE blueprint_slot_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_exam_questions_group
    ON exam_questions(group_id) WHERE group_id IS NOT NULL;

-- Backfill group_id: for every existing exam_question, create a
-- standalone group so all questions have uniform shape going forward.
-- This keeps Phase 10 rendering simple (always iterate groups).
DO $$
DECLARE
    q RECORD;
    new_group_id UUID;
    next_order INTEGER;
BEGIN
    FOR q IN
        SELECT id, tenant_id, exam_id, section_id, sort_order
          FROM exam_questions
         WHERE group_id IS NULL
         ORDER BY exam_id, sort_order, id
    LOOP
        SELECT COALESCE(MAX(display_order), -1) + 1 INTO next_order
          FROM exam_question_groups
         WHERE exam_id = q.exam_id;

        INSERT INTO exam_question_groups
            (tenant_id, exam_id, section_id, group_type, display_order)
        VALUES (q.tenant_id, q.exam_id, q.section_id, 'standalone', next_order)
        RETURNING id INTO new_group_id;

        UPDATE exam_questions SET group_id = new_group_id WHERE id = q.id;
    END LOOP;
END$$;
