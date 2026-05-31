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

**Examples:**

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

| Command    | Description                                                           |
|------------|-----------------------------------------------------------------------|
| `apply`    | Apply all pending migrations in revision order                        |
| `plan`     | Show pending migrations without applying                              |
| `status`   | Print applied/pending state of all migrations                         |
| `validate` | Scan the directory and verify all files match the naming convention   |

**Flags:**

| Flag          | Env var           | Default              |
|---------------|-------------------|----------------------|
| `--dsn`       | `PGMIGRATE_DSN`   | —                    |
| `--dir`, `-d` | `PGMIGRATE_DIR`   | `migrations`         |
| `--table`     | `PGMIGRATE_TABLE` | `schema_migrations`  |
| `--log-level` | —                 | `warn`               |

`--dsn` is optional when standard PostgreSQL environment variables are set (`PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD`). When none are present, the tool fails immediately with a clear error before attempting any connection.

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
m, err := migrator.NewWithDSN("postgres://user:pass@localhost/mydb", migrator.Config{
    Dir: "./migrations",
})
if err != nil {
    log.Fatal(err)
}
defer m.Close()

if err := m.Run(ctx); err != nil {
    var noTxErr *migrator.NoTxHistoryError
    if errors.As(err, &noTxErr) {
        // the migration ran but the history record failed to write
        // run noTxErr.RecoverySQL() manually, then retry
        fmt.Println(noTxErr.RecoverySQL())
    }
    log.Fatal(err)
}
```

`NewWithDB` accepts an existing `*sql.DB`. `NewValidateOnly` validates without a database connection.

---

## How it works

1. Walks the migrations directory recursively, collects files matching the naming convention
2. Sorts by the 7-digit revision prefix
3. Compares against the history table; applies only what is pending
4. Records every applied migration with its checksum, timestamp, and transaction ID

Modifying an applied `.up.sql` or `.notx.sql` file is a hard error - the checksum no longer matches.
Modifying a `.r.sql` or `.rnotx.sql` file triggers a re-apply on the next run.

Advisory locking prevents concurrent runs against the same database.

---

### Design rationale

This tool is built around one simple idea: your migration files should stay plain, usable SQL.

**Flexible directory layouts**  
Real projects rarely fit into one flat folder. You may want to split schema and data changes, group migrations by
release, or organise them by module or environment. This tool does not force a directory structure, so you can arrange
files in the way that makes sense for your project.

**Plain SQL, nothing hidden**  
Migration files should be easy to read, review, copy, and run directly in your database IDE or with `psql`. That is why
every migration here is just normal executable SQL, with no embedded DSL, no magic comments, and no mixed control syntax
inside the file.

**Undo is intentionally omitted by design**  
Keeping rollback files mixed together with forward migrations makes simple shell workflows harder and riskier. Those 
undo scripts are mostly untested and redundant. When you really need to UNDO something - it's a straightforward plain
migration file. This approach keeps everything predictable. 

**No lock-in**  
Your SQL files should still be useful even without this tool. They remain normal SQL files that can be sorted, reviewed,
and executed independently. The tool helps manage migrations, but it does not own your migration format.

**Repeatables and non-transactional migrations are built in**  
Updating views, functions, triggers, extensions, or maintenance logic is a normal part of working with PostgreSQL. Some
operations also need to run outside a transaction. These cases are supported naturally and are expressed in the
filename, without extra configuration or custom syntax.

### Transaction behaviour

PostgreSQL supports transactional DDL - most `CREATE`, `ALTER`, and `DROP` statements can be wrapped in `BEGIN/COMMIT`
and rolled back on failure. This tool defaults to transactional execution and makes the non-transactional case explicit
in the filename.

Statements that **cannot** run inside a transaction and require `.notx.sql` or `.rnotx.sql`:

- `VACUUM`
- `ALTER SYSTEM`
- `REINDEX SCHEMA / DATABASE / SYSTEM`
- `CREATE INDEX CONCURRENTLY`
- `DROP INDEX CONCURRENTLY`
- `ALTER TYPE ... ADD VALUE` (before PostgreSQL 12)

---

## Directory layouts

Migration files can live anywhere under the root directory. The tool walks recursively and sorts globally by revision.
Organise however makes sense for your project.

### Flat

```
migrations/
  0000001-create-users-table.up.sql
  0000002-add-roles-table.up.sql
  0000003-fn-get-users.r.sql
  0000004-vacuum-users.notx.sql
```

### By concern

```
migrations/
  schema/
    0000001-create-users-table.up.sql
    0000002-add-roles-table.up.sql
  data/
    0000003-seed-roles.up.sql
    0000004-seed-users.up.sql
  functions/
    0000005-fn-get-users.r.sql
    0000006-fn-get-roles.r.sql
  no-transaction/
    0000007-vacuum-users.notx.sql
```

### By release and concern

```
migrations/
  v1.0.0/
    schema/
      0000001-create-users-table.up.sql
      0000002-add-roles-table.up.sql
    data/
      0000003-seed-roles.up.sql
    functions/
      0000004-fn-get-users.r.sql
  v1.1.0/
    schema/
      0000005-add-audit-columns.up.sql
    no-transaction/
      0000006-vacuum-users.notx.sql
```

One rule applies in all layouts: **version numbers are global**. Two files with the same revision number anywhere in the
tree is an error.

---

## License

Apache License 2.0 - see [LICENSE](LICENSE).
