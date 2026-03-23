CREATE TABLE IF NOT EXISTS encoding_profiles (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                   TEXT NOT NULL UNIQUE,
    description            TEXT NOT NULL DEFAULT '',
    run_template_id        UUID NOT NULL REFERENCES templates(id) ON DELETE RESTRICT,
    frameserver_template_id UUID REFERENCES templates(id) ON DELETE SET NULL,
    audio_codec            TEXT NOT NULL DEFAULT '',
    audio_bitrate          TEXT NOT NULL DEFAULT '',
    output_extension       TEXT NOT NULL DEFAULT 'mkv',
    output_path_pattern    TEXT NOT NULL DEFAULT '',
    target_tags            TEXT[] NOT NULL DEFAULT '{}',
    priority               INT NOT NULL DEFAULT 5,
    enabled                BOOLEAN NOT NULL DEFAULT TRUE,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
