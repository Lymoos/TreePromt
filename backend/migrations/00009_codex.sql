-- +goose Up
-- Этап 7.8: исполнитель Codex (docs/stage7-8-codex.md). Работник вида codex берёт только задачи Codex.
ALTER TABLE exec.tasks DROP CONSTRAINT tasks_executor_check;
ALTER TABLE exec.tasks ADD CONSTRAINT tasks_executor_check CHECK (executor IN ('claude', 'junior', 'codex'));

-- +goose Down
ALTER TABLE exec.tasks DROP CONSTRAINT tasks_executor_check;
ALTER TABLE exec.tasks ADD CONSTRAINT tasks_executor_check CHECK (executor IN ('claude', 'junior'));
