-- +goose Up
-- Этап 6: AI Structuring Engine (docs/stage6-ai-structuring.md).

-- Результат ИИ, который не применён автоматически (исходник изменился или валидатор
-- нашёл нарушения); пользователь решает сам.
ALTER TABLE tree.node_content ADD COLUMN structure_proposal jsonb;

DROP TABLE ai.structure_requests;
CREATE TABLE ai.structure_requests (
    request_id          uuid PRIMARY KEY,
    node_id             uuid NOT NULL REFERENCES tree.nodes (id),
    user_id             uuid NOT NULL REFERENCES core.users (id),
    device_id           uuid,
    source_revision     bigint NOT NULL,
    source_content_hash text NOT NULL,
    prompt_version      text NOT NULL,
    model               text NOT NULL,
    status              text NOT NULL CHECK (status IN
        ('queued', 'running', 'applied', 'proposed', 'flagged', 'failed', 'superseded')),
    generated           jsonb,
    findings            jsonb,
    error               text,
    attempts            int NOT NULL DEFAULT 0,
    created_at          timestamptz NOT NULL DEFAULT now(),
    started_at          timestamptz,
    completed_at        timestamptz
);
CREATE INDEX structure_requests_status_idx ON ai.structure_requests (status, created_at);
CREATE INDEX structure_requests_user_idx ON ai.structure_requests (user_id, created_at);
CREATE INDEX structure_requests_node_idx ON ai.structure_requests (node_id, created_at);

-- +goose Down
DROP TABLE ai.structure_requests;
CREATE TABLE ai.structure_requests (
    request_id          uuid PRIMARY KEY,
    node_id             uuid NOT NULL REFERENCES tree.nodes (id),
    source_revision     bigint NOT NULL,
    source_content_hash text NOT NULL,
    prompt_version      text NOT NULL,
    model               text NOT NULL,
    generated           jsonb,
    status              text NOT NULL,
    error               text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    completed_at        timestamptz
);
ALTER TABLE tree.node_content DROP COLUMN structure_proposal;
