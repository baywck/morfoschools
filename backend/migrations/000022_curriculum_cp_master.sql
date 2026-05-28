-- 000022_curriculum_cp_master.sql
-- Master Capaian Pembelajaran (CP) references for Kurikulum Merdeka.
-- Source shape mirrors Kemendikdasmen CP JSON endpoints.

CREATE TABLE IF NOT EXISTS curriculum_cp_references (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    curriculum_code TEXT NOT NULL DEFAULT 'merdeka',
    level_code TEXT NOT NULL,
        -- Endpoint level, e.g. 'sd-sma' or 'smk'.
    level_name TEXT,
    subject_code TEXT NOT NULL,
    subject_name TEXT NOT NULL,
    phase TEXT NOT NULL,
    general_cp TEXT NOT NULL,
    source_name TEXT NOT NULL DEFAULT 'Kemendikdasmen Capaian Pembelajaran',
    source_url TEXT,
    source_version TEXT,
    compiled_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (curriculum_code, level_code, subject_code, phase)
);

CREATE INDEX IF NOT EXISTS idx_curriculum_cp_references_lookup
    ON curriculum_cp_references(curriculum_code, level_code, subject_code, phase);

CREATE TABLE IF NOT EXISTS curriculum_cp_elements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reference_id UUID NOT NULL REFERENCES curriculum_cp_references(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    content TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (reference_id, name)
);

CREATE INDEX IF NOT EXISTS idx_curriculum_cp_elements_reference
    ON curriculum_cp_elements(reference_id, sort_order);

ALTER TABLE exam_blueprint_slots
    ADD COLUMN IF NOT EXISTS cp_element_id UUID REFERENCES curriculum_cp_elements(id),
    ADD COLUMN IF NOT EXISTS capaian_pembelajaran TEXT,
    ADD COLUMN IF NOT EXISTS elemen_cp TEXT,
    ADD COLUMN IF NOT EXISTS tujuan_pembelajaran TEXT,
    ADD COLUMN IF NOT EXISTS materi_pokok TEXT,
    ADD COLUMN IF NOT EXISTS kelas TEXT,
    ADD COLUMN IF NOT EXISTS semester TEXT,
    ADD COLUMN IF NOT EXISTS indikator_soal TEXT;

ALTER TABLE blueprint_template_slots
    ADD COLUMN IF NOT EXISTS cp_element_id UUID REFERENCES curriculum_cp_elements(id),
    ADD COLUMN IF NOT EXISTS capaian_pembelajaran TEXT,
    ADD COLUMN IF NOT EXISTS elemen_cp TEXT,
    ADD COLUMN IF NOT EXISTS tujuan_pembelajaran TEXT,
    ADD COLUMN IF NOT EXISTS materi_pokok TEXT,
    ADD COLUMN IF NOT EXISTS kelas TEXT,
    ADD COLUMN IF NOT EXISTS semester TEXT,
    ADD COLUMN IF NOT EXISTS indikator_soal TEXT;
