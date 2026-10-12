-- +goose Up
-- Этап 7.6: инциденты (Kill Switch), аудит, ApprovalRequest (docs/stage7-6-security-audit.md).

CREATE TABLE exec.incidents (
    id            uuid PRIMARY KEY,
    user_id       uuid NOT NULL REFERENCES core.users (id),     -- владелец: его хосты и репозитории на паузе
    repository_id uuid REFERENCES exec.repositories (id),       -- NULL — «Остановить всё»
    task_id       uuid REFERENCES exec.tasks (id),
    kind          text NOT NULL CHECK (kind IN ('integration_red', 'secret_leak', 'qa_spike', 'manual')),
    status        text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved')),
    reason        text NOT NULL DEFAULT '',
    opened_at     timestamptz NOT NULL DEFAULT now(),
    resolved_at   timestamptz,
    resolved_by   uuid REFERENCES core.users (id),
    comment       text NOT NULL DEFAULT ''
);
CREATE INDEX exec_incidents_open_idx ON exec.incidents (user_id, status);

-- Слияния подряд, не прошедшие проверку после слияния (integration_red на третьем).
ALTER TABLE exec.repositories ADD COLUMN integration_failures int NOT NULL DEFAULT 0;

-- Аудит (ТЗ п. 14): только добавление.
CREATE TABLE exec.audit (
    id      bigserial PRIMARY KEY,
    at      timestamptz NOT NULL DEFAULT now(),
    user_id uuid NOT NULL REFERENCES core.users (id),
    host_id uuid REFERENCES exec.hosts (id),
    task_id uuid,
    attempt int,
    kind    text NOT NULL CHECK (kind IN ('command', 'model', 'attach', 'incident', 'approval')),
    data    jsonb NOT NULL DEFAULT '{}'
);
CREATE INDEX exec_audit_task_idx ON exec.audit (task_id, id);

-- +goose StatementBegin
CREATE FUNCTION exec.audit_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'exec.audit is append-only';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER audit_no_update BEFORE UPDATE OR DELETE ON exec.audit FOR EACH ROW EXECUTE FUNCTION exec.audit_append_only();
CREATE TRIGGER audit_no_truncate BEFORE TRUNCATE ON exec.audit FOR EACH STATEMENT EXECUTE FUNCTION exec.audit_append_only();

-- Снимки промптов без секретов, по хэшу из аудита.
CREATE TABLE exec.prompt_snapshots (
    hash       text NOT NULL,
    user_id    uuid NOT NULL REFERENCES core.users (id),
    text       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, hash)
);

-- Запросы на решение человека (ТЗ п. 13).
CREATE TABLE exec.approval_requests (
    id          uuid PRIMARY KEY,
    project_id  uuid NOT NULL REFERENCES tree.projects (id),
    task_id     uuid REFERENCES exec.tasks (id),
    plan_id     uuid REFERENCES exec.plans (id),
    incident_id uuid REFERENCES exec.incidents (id),
    type        text NOT NULL CHECK (type IN ('merge', 'plan', 'incident')),
    payload     jsonb NOT NULL DEFAULT '{}',
    status      text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'partial', 'expired')),
    reviewer    uuid REFERENCES core.users (id),
    comment     text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    expires_at  timestamptz
);
CREATE INDEX exec_approvals_pending_idx ON exec.approval_requests (project_id, status);

-- +goose Down
DROP TABLE exec.approval_requests;
DROP TABLE exec.prompt_snapshots;
DROP TRIGGER audit_no_truncate ON exec.audit;
DROP TRIGGER audit_no_update ON exec.audit;
DROP FUNCTION exec.audit_append_only();
DROP TABLE exec.audit;
ALTER TABLE exec.repositories DROP COLUMN integration_failures;
DROP TABLE exec.incidents;
