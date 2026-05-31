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
- **Dry-run** - prints pending work without touching the database
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

Apply:

```sh
gopgmigrate up --dsn postgres://user:pass@localhost/mydb --manifest migrations/manifest.yaml
```

---

## Manifest

Each entry in `migrations` declares the SQL files to apply, the execution mode, and a required stable ID.

### `id`

Required. Uniquely identifies the entry within the manifest. Used together with the file basename to form the `migration_id` stored in the history table (`id/filename.sql`).

Allowed characters: `[a-zA-Z0-9._-]`

IDs must be unique within the manifest. Duplicates are rejected at load time with a descriptive error.

### `files`

Required. One or more paths to SQL files, relative to the manifest file location. File paths must be globally unique across all entries.

### `mode`

Optional. Controls the transaction behaviour. See [Modes](#modes) below.

### `description`

Optional. Free-form text stored in the history table alongside the migration record.

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
gopgmigrate up        --dsn <dsn> --manifest <path> [--table <table>] [--dry-run]
gopgmigrate status    --dsn <dsn> --manifest <path> [--table <table>]
gopgmigrate validate  --manifest <path>
```

All flags fall back to environment variables:

| Flag         | Env var              | Default                    |
|--------------|----------------------|----------------------------|
| `--dsn`      | `PGMIGRATE_DSN`      | -                          |
| `--manifest` | `PGMIGRATE_MANIFEST` | `migrations/manifest.yaml` |
| `--table`    | `PGMIGRATE_TABLE`    | `schema_migrations`        |
| `--dry-run`  | `PGMIGRATE_DRY_RUN`  | `false`                    |

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
    path         text        not null,         -- resolved file path on disk
    kind         text        not null,         -- once | atomic | repeatable | no-tx
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
