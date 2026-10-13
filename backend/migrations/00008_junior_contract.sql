-- +goose Up
-- Junior только по контракту внутри плана Manager'а (решение владельца 13.10.2026, docs/stage7-7-junior-contract.md):
-- какие файлы Junior может менять и какие тесты (написанные Claude и пропущенные с меткой aicrew-contract) он должен оживить.
ALTER TABLE exec.tasks ADD COLUMN contract jsonb;

-- +goose Down
ALTER TABLE exec.tasks DROP COLUMN contract;
