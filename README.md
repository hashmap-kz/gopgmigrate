# gopgmigrate

SQL-first PostgreSQL migrations - no config files, no hidden DSL, no ORM coupling.

[![License](https://img.shields.io/github/license/hashmap-kz/gopgmigrate)](https://github.com/hashmap-kz/gopgmigrate/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/hashmap-kz/gopgmigrate/v2)](https://goreportcard.com/report/github.com/hashmap-kz/gopgmigrate/v2)
[![Go Reference](https://pkg.go.dev/badge/github.com/hashmap-kz/gopgmigrate/v2.svg)](https://pkg.go.dev/github.com/hashmap-kz/gopgmigrate/v2)
[![Workflow Status](https://img.shields.io/github/actions/workflow/status/hashmap-kz/gopgmigrate/ci.yml?branch=master)](https://github.com/hashmap-kz/gopgmigrate/actions/workflows/ci.yml?query=branch:master)

Drop SQL files in a directory. The filename encodes the execution mode. Done.

---

## Naming convention

```
{0000000}-{name}.{ext}
```

| Extension    | Behaviour                                                              |
|--------------|------------------------------------------------------------------------|
| `.up.sql`    | Versioned · runs once · in a transaction                               |
| `.r.sql`     | Repeatable · re-runs when file content changes · in a transaction      |
| `.notx.sql`  | Versioned · runs once · outside a transaction                          |
| `.rnotx.sql` | Repeatable · re-runs when file content changes · outside a transaction |

The 7-digit prefix controls execution order **globally across all subdirectories**.
Every file in the migrations directory must match - any stray file is an error (exit 3).

**Example:**

```
migrations/
  0000001-schemas.up.sql
  0000002-lookup-tables.up.sql
  0000003-users.up.sql
  0000004-v-users.r.sql
  0000005-concurrent-indexes.notx.sql
```

See [`examples/migrations/`](examples/migrations/) for a working schema.

---

## Install

### CLI

**Using installation script**

```bash
curl -fsSL https://raw.githubusercontent.com/hashmap-kz/gopgmigrate/master/scripts/install.sh | sh
```

**Using Go:**

```bash
go install github.com/hashmap-kz/gopgmigrate@latest
```

**Using Homebrew:**

```bash
brew tap hashmap-kz/homebrew-tap
brew install gopgmigrate
```

Or download a binary from the [Releases page](https://github.com/hashmap-kz/gopgmigrate/releases).

### Library

```sh
go get github.com/hashmap-kz/gopgmigrate/v2
```

---

## Usage

### CLI

```sh
gopgmigrate apply    --dsn <dsn> --dir <path>
gopgmigrate plan     --dsn <dsn> --dir <path>
gopgmigrate status   --dsn <dsn> --dir <path>
gopgmigrate validate --dir <path>
```

| Command    | Description                                                         |
|------------|---------------------------------------------------------------------|
| `apply`    | Apply all pending migrations in revision order                      |
| `plan`     | Show pending migrations without applying                            |
| `status`   | Print applied/pending state of all migrations                       |
| `validate` | Scan the directory and verify all files match the naming convention |

**Flags:**

| Flag          | Env var           | Default             |
|---------------|-------------------|---------------------|
| `--dsn`       | `PGMIGRATE_DSN`   | -                   |
| `--dir`, `-d` | `PGMIGRATE_DIR`   | `migrations`        |
| `--table`     | `PGMIGRATE_TABLE` | `schema_migrations` |
| `--log-level` | -                 | `warn`              |

`--dsn` is optional when standard PostgreSQL environment
variables are set (`PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD`).
When none are present, the tool fails immediately with a clear error before attempting any connection.

`--table` accepts a schema-qualified name (e.g. `myschema.migrations`).

**Exit codes:**

| Code | Meaning                                       |
|------|-----------------------------------------------|
| `0`  | Success                                       |
| `1`  | Error                                         |
| `2`  | Pending migrations exist (`plan` only)        |
| `3`  | Stray files found in the migrations directory |

---

### Library

```go
m, err := migrator.NewWithDSN(os.Getenv("DATABASE_URL"), migrator.Config{
    Dir: "./migrations",
})
if err != nil {
    log.Fatal(err)
}
defer m.Close()

if err := m.Run(ctx); err != nil {
    var noTxErr *migrator.NoTxHistoryError
    if errors.As(err, &noTxErr) {
        // migration ran but history record failed to write
        // execute noTxErr.RecoverySQL() manually, then re-run
        fmt.Println(noTxErr.RecoverySQL())
    }
    log.Fatal(err)
}
```

`NewWithDSN` accepts an empty DSN when PG* environment variables are configured.
`NewWithDB` accepts an existing `*sql.DB`. `NewValidateOnly` validates without a database connection.

---

## How it works

1. Walks the migrations directory recursively, collects files matching the naming convention
2. Sorts by the 7-digit revision prefix; duplicate revisions anywhere in the tree are an error
3. Cross-checks the scan against the history table: missing files or changed execution modes are hard errors
4. Applies pending migrations in order; records each with its checksum, timestamp, and transaction ID

Modifying an applied `.up.sql` or `.notx.sql` file is a hard error - the checksum no longer matches.
Modifying a `.r.sql` or `.rnotx.sql` file triggers a re-apply on the next run.

Advisory locking prevents concurrent runs against the same database.

---

## Non-transactional migrations

PostgreSQL supports transactional DDL - most `CREATE`, `ALTER`, and `DROP` statements can be wrapped in `BEGIN/COMMIT`
and rolled back on failure. The tool defaults to transactional execution; `.notx.sql` and `.rnotx.sql`
make the exception explicit in the filename.

Statements that cannot run inside a transaction:

- `VACUUM`
- `ALTER SYSTEM`
- `REINDEX SCHEMA / DATABASE / SYSTEM`
- `CREATE INDEX CONCURRENTLY`
- `DROP INDEX CONCURRENTLY`
- `ALTER TYPE ... ADD VALUE` (before PostgreSQL 12)

---

## Directory layouts

Files can live anywhere under the root directory - the tool walks recursively and sorts globally by revision.
Organise however makes sense for your project.

**Flat:**

```
migrations/
  0000001-create-users-table.up.sql
  0000002-add-roles-table.up.sql
  0000003-fn-get-users.r.sql
  0000004-vacuum-users.notx.sql
```

**By concern:**

```
migrations/
  schema/
    0000001-create-users-table.up.sql
    0000002-add-roles-table.up.sql
  data/
    0000003-seed-roles.up.sql
  functions/
    0000004-fn-get-users.r.sql
  indexes/
    0000005-vacuum-users.notx.sql
```

**By release:**

```
migrations/
  rel-1.0/
    0000001-create-users-table.up.sql
    0000002-add-roles-table.up.sql
    0000003-fn-get-users.r.sql
  rel-2.0/
    0000004-add-audit-columns.up.sql
    0000005-vacuum-users.notx.sql
```

One rule applies everywhere: **revision numbers are global**.
Two files with the same revision anywhere in the tree is an error.

---

## History table

Created automatically on first run (`schema_migrations` by default, override with `--table`):

```sql
create table schema_migrations
(
    record_id    serial primary key,
    migration_id int         not null unique, -- 7-digit revision number
    kind         text        not null,        -- once | no-tx | repeatable | repeatable-notx
    checksum     text        not null,        -- sha256 of file contents at apply time
    applied_by   name        not null default session_user,
    applied_at   timestamptz not null default transaction_timestamp(),
    txid         text        not null default pg_current_xact_id()::text
);
```

---

## License

Apache License 2.0 - see [LICENSE](LICENSE).
