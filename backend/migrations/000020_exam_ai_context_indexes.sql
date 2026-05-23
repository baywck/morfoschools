CREATE TABLE IF NOT EXISTS exam_ai_context_indexes (
    tenant_id uuid NOT NULL,
    exam_id uuid NOT NULL,
    content_hash text NOT NULL,
    summary jsonb NOT NULL,
    stale boolean NOT NULL DEFAULT false,
    indexed_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, exam_id),
    CONSTRAINT exam_ai_context_indexes_exam_fk
        FOREIGN KEY (exam_id) REFERENCES exams(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_exam_ai_context_indexes_stale
    ON exam_ai_context_indexes (tenant_id, stale);
