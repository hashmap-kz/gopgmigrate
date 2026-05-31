# gopgmigrate

Manifest-driven PostgreSQL migrations for Go.

[![License](https://img.shields.io/github/license/hashmap-kz/gopgmigrate)](https://github.com/hashmap-kz/gopgmigrate/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/hashmap-kz/gopgmigrate/v2)](https://goreportcard.com/report/github.com/hashmap-kz/gopgmigrate/v2)
[![Go Reference](https://pkg.go.dev/badge/github.com/hashmap-kz/gopgmigrate/v2.svg)](https://pkg.go.dev/github.com/hashmap-kz/gopgmigrate/v2)
[![Workflow Status](https://img.shields.io/github/actions/workflow/status/hashmap-kz/gopgmigrate/ci.yml?branch=master)](https://github.com/hashmap-kz/gopgmigrate/actions/workflows/ci.yml?query=branch:master)

Migration order and execution mode are declared in a YAML manifest - not inferred from filenames or directories.
SQL files stay plain SQL. No DSL, no ORM coupling, no magic.

---

## Features

- **Explicit ordering** - manifest declaration order is execution order, always
- **Four execution modes** - transactional, atomic batch, non-transactional, repeatable
- **Stable migration IDs** - each entry has a required `id`; the history record is keyed by `id/filename`
- **Checksum guard** - modifying an applied migration is a hard error
- **Advisory locking** - prevents concurrent runs against the same database
- **Repeatable migrations** - re-applied automatically when content changes
- **Plan before apply** - inspect pending migrations without touching the database
- **Library + CLI** - embed in your app or run as a standalone binary

---

## Installation

```sh
go install github.com/hashmap-kz/gopgmigrate/v2/cmd/gopgmigrate@latest
```

As a library:

```sh
go get github.com/hashmap-kz/gopgmigrate/v2
```

---

## Quick start

Create a manifest file (default path: `migrations/manifest.yaml`):

```yaml
manifest:
  table: schema_migrations   # optional, default: schema_migrations

migrations:
  # default: each file runs in its own transaction, applied once
  - id: rel-1.0.users
    files:
      - sql/001_create_users.sql

  # atomic: all files share one transaction - all succeed or all roll back
  - id: rel-1.0.roles
    files:
      - sql/002_add_roles.sql
      - sql/003_seed_roles.sql
    mode: atomic
    description: "release-1.0"

  # repeatable: re-applied whenever the file checksum changes
  - id: views.vw-users
    files:
      - sql/views/vw_users.sql
    mode: repeatable

  # no-tx: runs outside any transaction (required for VACUUM, CREATE INDEX CONCURRENTLY, etc.)
  - id: rel-1.0.indexes
    files:
      - sql/004_create_index_concurrently.sql
    mode: no-tx
```

Preview what would be applied, then apply:

```sh
gopgmigrate plan  --dsn postgres://user:pass@localhost/mydb --manifest migrations/manifest.yaml
gopgmigrate apply --dsn postgres://user:pass@localhost/mydb --manifest migrations/manifest.yaml
```

See the [`examples/`](examples/) directory for a real-world layout using per-release leaf files.

---

## Manifest

A manifest is either a **root** file (the entry point) or a **leaf** file (a per-release migration list). Both are YAML.

### Root manifest

The root manifest declares the tracking table and the ordered list of leaf files to include:

```yaml
manifest:
  table: schema_migrations   # optional, default: schema_migrations

includes:
  - rel-0.0.1.yaml
  - rel-0.0.2.yaml
```

Each line in `includes` is a path to a leaf file, relative to the root manifest location. Entries from all leaf files are flattened in declaration order. Adding a new release means writing a new leaf file and appending one line here — old leaf files are never modified.

A root manifest may use either `includes` or `migrations`, not both.

### Leaf manifest

A leaf file contains only a `migrations` list. It has no `manifest` header and no `includes`:

```yaml
migrations:
  - id: rel-0.0.1.schemas
    files:
      - sql/schemas.sql

  - id: rel-0.0.1.core-tables
    files:
      - sql/users.sql
      - sql/sessions.sql
    mode: atomic
    description: |
      Core tables applied atomically so a failure does not leave
      the schema in a half-created state.
```

File paths in a leaf are resolved relative to the leaf file's own location.

### Standalone manifest

For simpler projects, the root can contain `migrations` directly — no `includes` needed:

```yaml
manifest:
  table: schema_migrations

migrations:
  - id: rel-1.0.users
    files:
      - sql/001_create_users.sql
```

### Entry fields

| Field         | Required | Description |
|---------------|----------|-------------|
| `id`          | yes      | Stable identifier, unique within the manifest. Combined with the file basename to form the history key: `id/filename.sql`. Allowed characters: `[a-zA-Z0-9._-]` |
| `files`       | yes      | One or more SQL file paths, relative to the manifest file. Must be unique across all entries. |
| `mode`        | no       | Execution mode. See [Modes](#modes). |
| `description` | no       | Free-form text stored in the history table. |

### Manifest rules

- `includes` and `migrations` cannot both be present in the same file.
- Leaf files cannot have `includes` or a `manifest` header.
- `id` must match `[a-zA-Z0-9._-]` and be unique across all loaded entries.
- File paths must be unique across all entries.
- Files within the same entry must have unique basenames (they share the same `id` prefix in `migration_id`).

---

## Modes

**Default** (no `mode` key) - each file runs in its own transaction and is applied exactly once.
The file checksum is stored on apply; if the file is later modified, the next run fails with a checksum mismatch error.
Use this for ordinary schema changes: `CREATE TABLE`, `ALTER TABLE`, data migrations.

**`atomic`** - all files in the entry share a single transaction.
Either every file is applied and committed together, or nothing is.
Use this for changes that must land as one unit - e.g. a new table and its seed data, or a set of related schema changes in a release.

**`no-tx`** - statements execute outside any `BEGIN`/`COMMIT` block.
Required for statements PostgreSQL refuses to run inside a transaction:
`CREATE INDEX CONCURRENTLY`, `DROP INDEX CONCURRENTLY`, `VACUUM`, `ALTER SYSTEM`.
If the history write fails after execution, a `NoTxHistoryError` is returned with the exact recovery SQL to run before retrying.

**`repeatable`** - applied on every run where the file checksum has changed since last apply.
Idempotent SQL only: `CREATE OR REPLACE FUNCTION`, `CREATE OR REPLACE VIEW`, trigger definitions.
Each file in the entry is checked and re-applied independently.

| Mode         | Transaction             | Behaviour                                       |
|--------------|-------------------------|-------------------------------------------------|
| *(default)*  | one tx per file         | runs once; checksum-guarded                     |
| `atomic`     | one tx across all files | all succeed or all roll back                    |
| `no-tx`      | none                    | for `CREATE INDEX CONCURRENTLY`, `VACUUM`, etc. |
| `repeatable` | one tx per file         | re-runs whenever the file checksum changes      |

### Manifest rules

- `id` is required, must match `[a-zA-Z0-9._-]`, and must be unique within the manifest.
- `files` is required and must not be empty.
- File paths must be unique across all entries.
- Files within the same entry must have unique basenames (they share the same `id` prefix in `migration_id`).

---

## CLI

```
gopgmigrate apply     --dsn <dsn> --manifest <path> [--table <table>]
gopgmigrate plan      --dsn <dsn> --manifest <path> [--table <table>]
gopgmigrate status    --dsn <dsn> --manifest <path> [--table <table>]
gopgmigrate validate  --manifest <path>
```

| Command    | Description                                                                           |
|------------|---------------------------------------------------------------------------------------|
| `apply`    | Apply all pending migrations in manifest order                                        |
| `plan`     | Show pending migrations without applying. Exits 2 if any are pending, 0 if up to date |
| `status`   | Print a table of all manifest entries with their applied state                        |
| `validate` | Check that all files referenced in the manifest exist (no DB required)                |

All flags fall back to environment variables:

| Flag         | Env var              | Default                    |
|--------------|----------------------|----------------------------|
| `--dsn`      | `PGMIGRATE_DSN`      | -                          |
| `--manifest` | `PGMIGRATE_MANIFEST` | `migrations/manifest.yaml` |
| `--table`    | `PGMIGRATE_TABLE`    | `schema_migrations`        |

### `plan` exit codes

| Code | Meaning                                          |
|------|--------------------------------------------------|
| `0`  | Nothing to apply, database is up to date         |
| `1`  | Error (connection failure, manifest error, etc.) |
| `2`  | Pending migrations exist                         |

This makes `plan` composable in CI pipelines - a non-zero exit can gate a deploy or trigger an alert.

### `status` output

```
PATH                                   KIND        APPLIED_AT
sql/001_create_users.sql               once        2024-03-12 14:02:11
sql/002_add_roles.sql                  once        2024-03-12 14:02:11
sql/003_seed_roles.sql                 once        2024-03-12 14:02:11
sql/004_create_index_concurrently.sql  no-tx       2024-03-12 14:02:12
sql/views/vw_users.sql                 repeatable  -
```

Pending entries show `-` in `APPLIED_AT`. Column widths adjust to the longest value in the result set.

---

## Library usage

```go
m, err := migrator.NewWithDSN("postgres://user:pass@localhost/mydb", migrator.Config{
    ManifestPath: "./migrations/manifest.yaml",
})
if err != nil {
    log.Fatal(err)
}
defer m.Close()

if err := m.Run(ctx); err != nil {
    var noTxErr *migrator.NoTxHistoryError
    if errors.As(err, &noTxErr) {
        // migration ran but the history record failed to write
        // execute noTxErr.RecoverySQL() manually, then re-run
        fmt.Println(noTxErr.RecoverySQL())
    }
    log.Fatal(err)
}
```

Use `NewWithDB` to supply an existing `*sql.DB`, or `NewValidateOnly` to validate the manifest without a database connection.

---

## History table

Migrations are tracked in `schema_migrations` (configurable via `--table`):

```sql
create table schema_migrations (
    record_id    serial      primary key,
    migration_id text        not null unique,  -- <entry-id>/<filename>, e.g. rel-1.0.users/001_create_users.sql
    path         text        not null,         -- manifest-relative path, e.g. sql/001_create_users.sql
    kind         text        not null,         -- once | no-tx | repeatable
    checksum     text        not null,         -- sha256 of file contents at apply time
    description  text,
    applied_by   name        not null default session_user,
    applied_at   timestamptz not null default transaction_timestamp(),
    txid         text        not null default pg_current_xact_id()::text
);
```

The table is created automatically on first run.

---

## License

Apache License 2.0 - see [LICENSE](LICENSE).
