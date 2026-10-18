-- +goose Up
-- Этап 7.9: окно проекта, скиллы задачи, привязка кода из приложения (docs/stage7-9-projects.md).

-- Описание проекта для ИИ: уходит агентам в каждую задачу проекта.
CREATE TABLE exec.project_context (
    project_id  uuid PRIMARY KEY REFERENCES tree.projects (id),
    description text NOT NULL DEFAULT '',
    updated_by  uuid REFERENCES core.users (id),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- Файлы проекта (ТЗ, макеты): в базе, чтобы их захватывал ежедневный бэкап; до 20 МБ на файл, 200 МБ на проект.
CREATE TABLE exec.project_files (
    id           uuid PRIMARY KEY,
    project_id   uuid NOT NULL REFERENCES tree.projects (id),
    name         text NOT NULL,
    content_type text NOT NULL DEFAULT 'application/octet-stream',
    size         bigint NOT NULL,
    sha256       text NOT NULL,
    data         bytea NOT NULL,
    created_by   uuid NOT NULL REFERENCES core.users (id),
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX exec_project_files_project_idx ON exec.project_files (project_id, created_at);

-- Скиллы, выбранные в самой AI Task: при отправке добавляются к обязательным.
CREATE TABLE exec.node_skills (
    node_id    uuid PRIMARY KEY REFERENCES tree.nodes (id),
    skills     jsonb NOT NULL DEFAULT '[]',
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Описания скиллов хоста — для окна выбора скиллов.
ALTER TABLE exec.hosts ADD COLUMN skill_info jsonb NOT NULL DEFAULT '{}';

-- Привязка кода из окна проекта: папка и/или git-адрес; служба ПК выполняет заявку и сообщает итог.
ALTER TABLE exec.repositories
    ADD COLUMN git_url      text NOT NULL DEFAULT '',
    ADD COLUMN status       text NOT NULL DEFAULT 'ready' CHECK (status IN ('pending', 'ready', 'failed')),
    ADD COLUMN status_error text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE exec.repositories DROP COLUMN status_error, DROP COLUMN status, DROP COLUMN git_url;
ALTER TABLE exec.hosts DROP COLUMN skill_info;
DROP TABLE exec.node_skills;
DROP TABLE exec.project_files;
DROP TABLE exec.project_context;
