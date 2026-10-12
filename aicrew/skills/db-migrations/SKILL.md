---
name: db-migrations
description: Safe database schema changes — PostgreSQL migrations with goose on the server and Drift schema versions in the Flutter apps. Use whenever a task adds or changes tables, columns, indexes or stored data.
when: tasks that touch backend/migrations, the Drift database, or saved data formats
roles: [tech_lead]
projects: [go, flutter, prompttree]
version: "1"
---

# Database migrations

Schema changes need the owner's approval: files under `**/migrations/**` are protected paths, so the task stops at «Слить» whatever its mode. Make the change easy to review.

## Server: PostgreSQL + goose (`backend/migrations/NNNNN_name.sql`)
- New file with the next number; never edit a migration that is already in `main`.
- Always write both `-- +goose Up` and `-- +goose Down`. Functions and multi-statement bodies go between `-- +goose StatementBegin` / `StatementEnd`.
- Prefer additive changes: new nullable column or column with a default, new table, new index. Rename = add new + copy + switch code + drop old in a later migration.
- `NOT NULL` on an existing table needs a `DEFAULT` (or a backfill in the same migration).
- Big tables: create indexes in a separate migration; keep each migration fast.
- Constraints carry meaning: `CHECK` for enums, foreign keys for references. Name constraints you may need to change later.
- Test against a real database: the `testdb` helper creates a fresh database and applies every migration; add a test that exercises the new schema.

## Apps: Drift (`shared/lib/src/data/database.dart`)
- Bump `schemaVersion` and add a step in `MigrationStrategy.onUpgrade` for every change (`m.addColumn`, `m.createTable`, …). Old app versions on users' devices must upgrade cleanly.
- Regenerate code (`dart run build_runner build`) and update the migration test with a schema snapshot of the new version.
- Local data is the user's data (Local First): never drop or rewrite it without a migration that keeps it.

## Data formats
- Saved JSON (structured content, op-log payloads) gets a version field; readers accept the previous version.
- Sync operations already in queues on devices must stay readable after the change.

## Do not
- Delete columns or tables that still hold data without an explicit task and approval.
- Mix a schema change with unrelated refactoring.
