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

## Installation

**CLI:**
```sh
go install github.com/hashmap-kz/gopgmigrate/v2/cmd/gopgmigrate@latest
```

**Library:**
```sh
go get github.com/hashmap-kz/gopgmigrate/v2
```

---

## CLI

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

## Library

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

## License

Apache License 2.0 - see [LICENSE](LICENSE).
