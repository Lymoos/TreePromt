-- +goose Up
-- Этап 7.2: домен Execution (AiCrew), docs/stage7-2-state-machine.md.
-- В таблицы PromptTree не пишет: связь только по id узла и проекта (ТЗ п. 5.4).

CREATE TABLE exec.hosts (
    id           uuid PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES core.users (id),
    device_id    uuid NOT NULL UNIQUE REFERENCES core.devices (id),
    name         text NOT NULL,
    os           text NOT NULL DEFAULT '',
    capabilities jsonb NOT NULL DEFAULT '[]',  -- что умеет хост: flutter, go, ollama, roblox_studio…
    environments jsonb NOT NULL DEFAULT '[]',  -- доступные окружения: container, windows_user
    status       text NOT NULL DEFAULT 'online' CHECK (status IN ('online', 'offline')),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE exec.workers (
    id           uuid PRIMARY KEY,
    host_id      uuid NOT NULL REFERENCES exec.hosts (id),
    kind         text NOT NULL,             -- claude_code, ollama, gemini…
    model        text NOT NULL DEFAULT '',
    status       text NOT NULL DEFAULT 'idle' CHECK (status IN ('idle', 'busy', 'offline')),
    heartbeat_at timestamptz NOT NULL DEFAULT now(),
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (host_id, kind, model)
);

CREATE TABLE exec.repositories (
    id                   uuid PRIMARY KEY,
    project_id           uuid NOT NULL REFERENCES tree.projects (id),
    host_id              uuid NOT NULL REFERENCES exec.hosts (id),
    name                 text NOT NULL,
    local_path           text NOT NULL,       -- путь относится к конкретному хосту (ТЗ п. 8.1)
    default_branch       text NOT NULL DEFAULT 'main',
    remote_url           text NOT NULL DEFAULT '',
    verification_profile jsonb NOT NULL DEFAULT '{}',
    created_at           timestamptz NOT NULL DEFAULT now(),
    UNIQUE (host_id, local_path)
);

CREATE TABLE exec.tasks (
    id                    uuid PRIMARY KEY,
    project_id            uuid NOT NULL REFERENCES tree.projects (id),
    node_id               uuid REFERENCES tree.nodes (id),
    repository_id         uuid NOT NULL REFERENCES exec.repositories (id),
    created_by            uuid NOT NULL REFERENCES core.users (id),
    title                 text NOT NULL,
    prompt                text NOT NULL,      -- снимок итоговой задачи на момент отправки
    status                text NOT NULL CHECK (status IN (
        'QUEUED', 'CLAIMED', 'IN_PROGRESS', 'VERIFYING', 'LLM_REVIEW', 'MERGEABLE', 'MERGED',
        'POST_MERGE_VERIFY', 'DONE', 'FAILED', 'BLOCKED', 'REPLAN', 'AWAITING_HUMAN', 'CANCELLING', 'CANCELLED')),
    execution_mode        text NOT NULL DEFAULT 'manual' CHECK (execution_mode IN ('manual', 'semi_auto', 'night')),
    execution_env         text NOT NULL DEFAULT 'container' CHECK (execution_env IN ('container', 'windows_user')),
    required_capabilities jsonb NOT NULL DEFAULT '[]',
    worker_id             uuid REFERENCES exec.workers (id),
    lease_expires_at      timestamptz,
    heartbeat_at          timestamptz,
    attempt               int NOT NULL DEFAULT 0,
    max_attempts          int NOT NULL DEFAULT 3,
    failure_code          text,
    failure_class         text,
    retryable             boolean,
    retry_after           timestamptz,
    cancel_requested      boolean NOT NULL DEFAULT false,
    base_commit           text,
    result_commit         text,
    max_tokens            bigint,
    max_model_calls       int,
    agent_runtime_sec     int NOT NULL DEFAULT 3600,
    verification_runtime_sec int NOT NULL DEFAULT 1200,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX exec_tasks_queue_idx ON exec.tasks (status, created_at);
CREATE INDEX exec_tasks_lease_idx ON exec.tasks (lease_expires_at) WHERE lease_expires_at IS NOT NULL;
CREATE INDEX exec_tasks_project_idx ON exec.tasks (project_id, updated_at);

CREATE TABLE exec.task_dependencies (
    task_id            uuid NOT NULL REFERENCES exec.tasks (id),
    depends_on_task_id uuid NOT NULL REFERENCES exec.tasks (id),
    type               text NOT NULL DEFAULT 'finish_to_start',
    PRIMARY KEY (task_id, depends_on_task_id),
    CHECK (task_id <> depends_on_task_id)
);

CREATE TABLE exec.attempts (
    id            uuid PRIMARY KEY,
    task_id       uuid NOT NULL REFERENCES exec.tasks (id),
    attempt       int NOT NULL,
    worker_id     uuid NOT NULL REFERENCES exec.workers (id),
    status        text NOT NULL,              -- статус, на котором попытка закончилась
    base_commit   text,
    result_commit text,
    failure_code  text,
    failure_class text,
    facts         jsonb NOT NULL DEFAULT '{}', -- exit-коды, изменённые файлы, diff --stat, расход, итог агента
    started_at    timestamptz NOT NULL DEFAULT now(),
    finished_at   timestamptz,
    UNIQUE (task_id, attempt)
);

CREATE TABLE exec.task_events (
    id          bigserial PRIMARY KEY,
    task_id     uuid NOT NULL REFERENCES exec.tasks (id),
    from_status text,
    to_status   text NOT NULL,
    actor       text NOT NULL,   -- user:<id>, worker:<id>, system
    reason      text NOT NULL DEFAULT '',
    attempt     int,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX exec_task_events_task_idx ON exec.task_events (task_id, id);

-- +goose Down
DROP TABLE exec.task_events;
DROP TABLE exec.attempts;
DROP TABLE exec.task_dependencies;
DROP TABLE exec.tasks;
DROP TABLE exec.repositories;
DROP TABLE exec.workers;
DROP TABLE exec.hosts;
