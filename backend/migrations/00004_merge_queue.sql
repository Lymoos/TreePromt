-- +goose Up
-- Этап 7.3: очередь слияния, integration-ветка, откат (docs/stage7-3-merge-queue.md).

ALTER TABLE exec.tasks DROP CONSTRAINT tasks_status_check;
ALTER TABLE exec.tasks ADD CONSTRAINT tasks_status_check CHECK (status IN (
    'QUEUED', 'CLAIMED', 'IN_PROGRESS', 'VERIFYING', 'LLM_REVIEW', 'MERGEABLE',
    'MERGE_QUEUED', 'MERGING', 'POST_MERGE_VERIFY', 'MERGED', 'DONE',
    'ROLLBACK_QUEUED', 'ROLLING_BACK', 'ROLLED_BACK',
    'FAILED', 'BLOCKED', 'REPLAN', 'AWAITING_HUMAN', 'CANCELLING', 'CANCELLED'));

ALTER TABLE exec.tasks
    ADD COLUMN merge_commit  text,   -- коммит слияния задачи в integration-ветку
    ADD COLUMN revert_commit text;   -- коммит отката, если задачу откатили

-- Ветка, в которую AiCrew сливает проверенные результаты; main переносит только человек.
ALTER TABLE exec.repositories ADD COLUMN integration_branch text NOT NULL DEFAULT 'aicrew/integration';

-- +goose Down
ALTER TABLE exec.repositories DROP COLUMN integration_branch;
ALTER TABLE exec.tasks DROP COLUMN revert_commit, DROP COLUMN merge_commit;
ALTER TABLE exec.tasks DROP CONSTRAINT tasks_status_check;
ALTER TABLE exec.tasks ADD CONSTRAINT tasks_status_check CHECK (status IN (
    'QUEUED', 'CLAIMED', 'IN_PROGRESS', 'VERIFYING', 'LLM_REVIEW', 'MERGEABLE', 'MERGED',
    'POST_MERGE_VERIFY', 'DONE', 'FAILED', 'BLOCKED', 'REPLAN', 'AWAITING_HUMAN', 'CANCELLING', 'CANCELLED'));
