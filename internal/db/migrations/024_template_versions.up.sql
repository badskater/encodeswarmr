-- 024_template_versions.up.sql
-- Adds template versioning: each update to a template archives the previous
-- content as a numbered version row.

CREATE TABLE IF NOT EXISTS template_versions (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID        NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    version     INT         NOT NULL,
    content     TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT  uq_template_versions UNIQUE (template_id, version)
);

CREATE INDEX IF NOT EXISTS idx_template_versions_template_id ON template_versions (template_id);
