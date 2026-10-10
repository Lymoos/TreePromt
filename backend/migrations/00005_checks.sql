-- +goose Up
-- Этап 7.4: контур проверок (docs/stage7-4-checks.md).

-- Уровень ревью: logic — Logic QA; tech_lead — ещё и финальное ревью Tech Lead.
ALTER TABLE exec.tasks ADD COLUMN review_level text NOT NULL DEFAULT 'logic' CHECK (review_level IN ('logic', 'tech_lead'));
-- Файлы из protected_paths контракта: задача ждёт «Слить» в любом режиме.
ALTER TABLE exec.tasks ADD COLUMN approval_required jsonb NOT NULL DEFAULT '[]';
-- Вердикт ревьюера, с которым задача стала MERGEABLE.
ALTER TABLE exec.tasks ADD COLUMN review jsonb;

-- +goose Down
ALTER TABLE exec.tasks DROP COLUMN review, DROP COLUMN approval_required, DROP COLUMN review_level;
