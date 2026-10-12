-- +goose Up
-- Этап 7.5: Manager и планы, скиллы, MCP-профили, исполнители (docs/stage7-5-manager-skills-mcp.md).

-- Что хост умеет подключать: проверка плана Manager'а идёт по этим спискам (ТЗ п. 10.1).
ALTER TABLE exec.hosts
    ADD COLUMN skills       jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN mcp_profiles jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN executors    jsonb NOT NULL DEFAULT '["claude"]';

-- Принудительная привязка и запрет скиллов для репозитория (ТЗ п. 10.2).
ALTER TABLE exec.repositories ADD COLUMN skill_policy jsonb NOT NULL DEFAULT '{}';

CREATE TABLE exec.plans (
    id               uuid PRIMARY KEY,
    project_id       uuid NOT NULL REFERENCES tree.projects (id),
    repository_id    uuid NOT NULL REFERENCES exec.repositories (id),
    node_id          uuid REFERENCES tree.nodes (id),
    created_by       uuid NOT NULL REFERENCES core.users (id),
    request          text NOT NULL,
    execution_mode   text NOT NULL DEFAULT 'manual' CHECK (execution_mode IN ('manual', 'semi_auto', 'night')),
    status           text NOT NULL CHECK (status IN ('QUEUED', 'PLANNING', 'AWAITING_APPROVAL', 'APPROVED', 'REJECTED', 'FAILED')),
    worker_id        uuid REFERENCES exec.workers (id),
    lease_expires_at timestamptz,
    attempt          int NOT NULL DEFAULT 0,
    proposal         jsonb,               -- предложенный Manager'ом план (после проверки)
    error            text NOT NULL DEFAULT '',
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX exec_plans_queue_idx ON exec.plans (status, created_at);

ALTER TABLE exec.tasks
    ADD COLUMN plan_id         uuid REFERENCES exec.plans (id),
    ADD COLUMN required_skills jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN mcp_profile     text NOT NULL DEFAULT 'default',
    ADD COLUMN touches_ui      boolean NOT NULL DEFAULT false,
    ADD COLUMN executor        text NOT NULL DEFAULT 'claude' CHECK (executor IN ('claude', 'junior'));

-- +goose Down
ALTER TABLE exec.tasks DROP COLUMN executor, DROP COLUMN touches_ui, DROP COLUMN mcp_profile,
    DROP COLUMN required_skills, DROP COLUMN plan_id;
DROP TABLE exec.plans;
ALTER TABLE exec.repositories DROP COLUMN skill_policy;
ALTER TABLE exec.hosts DROP COLUMN executors, DROP COLUMN mcp_profiles, DROP COLUMN skills;
