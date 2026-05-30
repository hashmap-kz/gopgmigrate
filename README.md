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
  - files:
      - sql/001_create_users.sql

  # atomic: all files share one transaction - all succeed or all roll back
  - files:
      - sql/002_add_roles.sql
      - sql/003_seed_roles.sql
    mode: atomic
    description: "release-1.0"

  # repeatable: re-applied whenever the file checksum changes
  - files:
      - sql/views/vw_users.sql
    mode: repeatable

  # no-tx: runs outside any transaction (required for VACUUM, CREATE INDEX CONCURRENTLY, etc.)
  - files:
      - sql/004_create_index_concurrently.sql
    mode: no-tx
```

Apply:

```sh
gopgmigrate up --dsn postgres://user:pass@localhost/mydb --manifest migrations/manifest.yaml
```

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
Each repeatable entry must list exactly one file.

| Mode         | Transaction             | Behaviour                                       |
|--------------|-------------------------|-------------------------------------------------|
| *(default)*  | one tx per file         | runs once; checksum-guarded                     |
| `atomic`     | one tx across all files | all succeed or all roll back                    |
| `no-tx`      | none                    | for `CREATE INDEX CONCURRENTLY`, `VACUUM`, etc. |
| `repeatable` | one tx per file         | re-runs whenever the file checksum changes      |

### Manifest rules

- `files` is required and must not be empty.
- Duplicate file paths across any entries are rejected at load time.
- `repeatable` + more than one file per entry is a hard error.

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
    path        text primary key,
    kind        text        not null,  -- once | repeatable | no-tx
    checksum    text        not null,  -- sha256 of file contents
    description text,
    applied_by  name        not null default session_user,
    applied_at  timestamptz not null default transaction_timestamp(),
    txid        text        not null default pg_current_xact_id()::text
);
```

The table is created automatically on first run.

---

## License

Apache License 2.0 - see [LICENSE](LICENSE).
