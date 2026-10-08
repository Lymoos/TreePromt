-- +goose Up
-- Домены разделены по схемам (ТЗ п. 5.4): core — пользователи и доступ,
-- tree — PromptTree, ai — структурирование (этап 6), exec — AiCrew (этап 7).
CREATE SCHEMA core;
CREATE SCHEMA tree;
CREATE SCHEMA ai;
CREATE SCHEMA exec;

-- Сквозной порядок изменений. Выдаётся под глобальной блокировкой записи,
-- поэтому ревизии фиксируются строго по возрастанию (см. tree/apply.go).
CREATE SEQUENCE tree.server_revision_seq;

CREATE TABLE core.users (
    id            uuid PRIMARY KEY,
    email         text NOT NULL UNIQUE CHECK (email = lower(email)),
    password_hash text NOT NULL,
    name          text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE core.devices (
    id           uuid PRIMARY KEY, -- = client_id устройства
    user_id      uuid NOT NULL REFERENCES core.users (id),
    name         text NOT NULL DEFAULT '',
    platform     text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    revoked_at   timestamptz
);

CREATE TABLE core.sessions (
    id                 uuid PRIMARY KEY,
    user_id            uuid NOT NULL REFERENCES core.users (id),
    device_id          uuid NOT NULL REFERENCES core.devices (id),
    family_id          uuid NOT NULL, -- цепочка ротаций одного входа
    refresh_token_hash bytea NOT NULL UNIQUE,
    expires_at         timestamptz NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    rotated_at         timestamptz,
    revoked_at         timestamptz
);
CREATE INDEX sessions_family_idx ON core.sessions (family_id);

CREATE TABLE tree.projects (
    id          uuid PRIMARY KEY,
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    revision    bigint NOT NULL,
    created_by  uuid NOT NULL REFERENCES core.users (id),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz
);
CREATE INDEX projects_revision_idx ON tree.projects (revision);

CREATE TABLE core.project_access (
    project_id uuid NOT NULL REFERENCES tree.projects (id),
    user_id    uuid NOT NULL REFERENCES core.users (id),
    role       text NOT NULL CHECK (role IN ('owner', 'editor', 'viewer', 'executor')),
    PRIMARY KEY (project_id, user_id)
);
CREATE INDEX project_access_user_idx ON core.project_access (user_id);

CREATE TABLE tree.repositories (
    id             uuid PRIMARY KEY,
    project_id     uuid NOT NULL REFERENCES tree.projects (id),
    provider       text NOT NULL DEFAULT 'github',
    remote_url     text NOT NULL,
    default_branch text NOT NULL DEFAULT '',
    description    text NOT NULL DEFAULT '',
    revision       bigint NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz
);

CREATE TABLE tree.nodes (
    id           uuid PRIMARY KEY, -- UUIDv7, создаётся на клиенте
    project_id   uuid NOT NULL REFERENCES tree.projects (id),
    parent_id    uuid REFERENCES tree.nodes (id),
    kind         text NOT NULL CHECK (kind IN ('folder', 'raw_note', 'ai_task')),
    name         text NOT NULL,
    sort_key     text NOT NULL,
    revision     bigint NOT NULL,
    has_conflict boolean NOT NULL DEFAULT false,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz
);
CREATE INDEX nodes_project_revision_idx ON tree.nodes (project_id, revision);
CREATE INDEX nodes_revision_idx ON tree.nodes (revision);
CREATE INDEX nodes_parent_idx ON tree.nodes (parent_id);

CREATE TABLE tree.node_content (
    node_id                  uuid PRIMARY KEY REFERENCES tree.nodes (id),
    raw_content              text NOT NULL DEFAULT '',
    raw_revision             bigint NOT NULL DEFAULT 0,
    raw_updated_at           timestamptz NOT NULL DEFAULT now(),
    structured_content       jsonb,
    structured_revision      bigint NOT NULL DEFAULT 0,
    structured_from_revision bigint,
    structure_status         text NOT NULL DEFAULT 'none'
        CHECK (structure_status IN ('none', 'pending', 'done', 'stale', 'failed'))
);

CREATE TABLE tree.content_versions (
    id              uuid PRIMARY KEY,
    node_id         uuid NOT NULL REFERENCES tree.nodes (id),
    project_id      uuid NOT NULL REFERENCES tree.projects (id),
    field           text NOT NULL CHECK (field IN ('raw', 'structured')),
    content         text NOT NULL,
    at_revision     bigint NOT NULL,
    reason          text NOT NULL CHECK (reason IN
        ('before_structure', 'conflict_local', 'conflict_server', 'before_restore', 'checkpoint')),
    device_id       uuid,
    server_revision bigint NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX content_versions_revision_idx ON tree.content_versions (server_revision);
CREATE INDEX content_versions_node_idx ON tree.content_versions (node_id, created_at);

-- Журнал операций: идемпотентность (PK operation_id) и проверка конфликтов.
-- Тексты заметок здесь не хранятся: в payload вместо них хэш и длина.
CREATE TABLE tree.operations (
    operation_id    uuid PRIMARY KEY,
    user_id         uuid NOT NULL REFERENCES core.users (id),
    device_id       uuid NOT NULL,
    client_seq      bigint NOT NULL,
    type            text NOT NULL,
    entity_id       uuid NOT NULL,
    base_revision   bigint NOT NULL DEFAULT 0,
    payload         jsonb NOT NULL,
    result          text NOT NULL CHECK (result IN ('applied', 'conflict', 'rejected')),
    server_revision bigint,
    response        jsonb NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX operations_entity_idx ON tree.operations (entity_id, server_revision);

CREATE TABLE tree.audit_events (
    id              bigserial PRIMARY KEY,
    entity          text NOT NULL,
    entity_id       uuid NOT NULL,
    action          text NOT NULL,
    before          jsonb,
    after           jsonb,
    operation_id    uuid,
    device_id       uuid,
    server_revision bigint,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_events_entity_idx ON tree.audit_events (entity_id, id);

-- Резерв под этап 6 (AI Structuring Engine).
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

-- +goose Down
DROP SCHEMA exec CASCADE;
DROP SCHEMA ai CASCADE;
DROP SCHEMA tree CASCADE;
DROP SCHEMA core CASCADE;
