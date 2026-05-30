<!-- omit in toc -->
# gopgmigrate

SQL-first PostgreSQL migrations - rollbacks, repeatable scripts, any directory layout

[![License](https://img.shields.io/github/license/hashmap-kz/gopgmigrate)](https://github.com/hashmap-kz/gopgmigrate/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/hashmap-kz/gopgmigrate)](https://goreportcard.com/report/github.com/hashmap-kz/gopgmigrate)
[![Go Reference](https://pkg.go.dev/badge/github.com/hashmap-kz/gopgmigrate.svg)](https://pkg.go.dev/github.com/hashmap-kz/gopgmigrate)
[![Workflow Status](https://img.shields.io/github/actions/workflow/status/hashmap-kz/gopgmigrate/ci.yml?branch=master)](https://github.com/hashmap-kz/gopgmigrate/actions/workflows/ci.yml?query=branch:master)
[![Go Version](https://img.shields.io/github/go-mod/go-version/hashmap-kz/gopgmigrate)](https://github.com/hashmap-kz/gopgmigrate/blob/master/go.mod#L3)
[![Latest Release](https://img.shields.io/github/v/release/hashmap-kz/gopgmigrate)](https://github.com/hashmap-kz/gopgmigrate/releases/latest)

Runs migrations sequentially with advisory locking, transactional safety, and hash-based change detection - no config
files, no YAML, no ORM coupling, no hidden DSL, no magic. Just SQL files and a clear naming convention.

<!-- Plugin used: Markdown TOC & Chapter Number -->

<!-- omit in toc -->
## Table of Contents

<!-- TOC tocDepth:2..3 chapterDepth:2..6 -->

- [How it works](#how-it-works)
- [Usage](#usage)
  - [CLI](#cli)
    - [Flags](#flags)
    - [Examples](#examples)
  - [Library](#library)
    - [Examples](#examples-1)
- [Naming conventions](#naming-conventions)
  - [Why extensions - not directories or prefixes](#why-extensions---not-directories-or-prefixes)
  - [Design rationale](#design-rationale)
  - [Transaction behaviour](#transaction-behaviour)
- [Directory layouts](#directory-layouts)
  - [Flat](#flat)
  - [By concern](#by-concern)
  - [By release and concern](#by-release-and-concern)
  - [By environment](#by-environment)
- [Contributing](#contributing)
- [License](#license)

<!-- /TOC -->

---

## How it works

1. Scans the migration directory recursively for `.sql` files
2. Compares them against the history table in your database
3. Applies only what is pending, in version order
4. Records every applied migration with its hash, timestamp, and transaction ID

Version ordering is **global** across all subdirectories. Subdirectories are purely for your own organisation - the tool
sorts only by the 7-digit revision prefix.

---

## Usage

**[`^        back to top        ^`](#table-of-contents)**

### CLI

```sh
gopgmigrate <command> [flags]

Commands:
  migrate          Apply all pending migrations
  rollback-count   Roll back the last N applied migrations
  last             Show the last applied migration
```

#### Flags

All commands share the same flags. Each flag falls back to an environment variable when not set.

| Flag              | Env var                        | Default                  | Description                               |
|-------------------|--------------------------------|--------------------------|-------------------------------------------|
| `--dirname`       | `PGMIGRATE_DIRNAME`            | -                        | Migration directory (required)            |
| `--connstr`       | `PGMIGRATE_CONNSTR`            | -                        | PostgreSQL connection string (required)   |
| `--history-table` | `PGMIGRATE_HISTORY_TABLE_NAME` | `public.migrate_history` | History table in `schema.table` format    |
| `--log-level`     | -                              | `info`                   | `debug` · `info` · `warn` · `error`       |
| `--dry-run`       | -                              | `false`                  | Print pending migrations without applying |

#### Examples

```sh
# apply all pending migrations
gopgmigrate migrate \
  --dirname ./migrations \
  --connstr postgres://user:pass@localhost:5432/mydb \
  --history-table public.migrate_history

# preview what would be applied
gopgmigrate migrate \
  --dirname ./migrations \
  --connstr postgres://user:pass@localhost:5432/mydb \
  --dry-run

# roll back the last 2 applied migrations
gopgmigrate rollback-count 2 \
  --dirname ./migrations \
  --connstr postgres://user:pass@localhost:5432/mydb

# using environment variables
export PGMIGRATE_DIRNAME=./migrations
export PGMIGRATE_CONNSTR=postgres://user:pass@localhost:5432/mydb

gopgmigrate migrate
gopgmigrate rollback-count 1 --dry-run
```

### Library

#### Examples

```go
err := migration.RunMigrationsUp(context.Background(), &migration.ApplyOpts{
	MigrationDir:     "./migrations",
	ConnStr:          "postgres://user:pass@localhost:5432/mydb",
	HistoryTableName: "public.migrate_history",
})
```

---

## Naming conventions

**[`^        back to top        ^`](#table-of-contents)**

Every migration file encodes its complete behaviour in its name.

![Migration Naming Convention](docs/assets/migration-names.svg)

```
{0000000}-{name}.{kind}.sql
```

| Extension       | Behaviour                                                  |
|-----------------|------------------------------------------------------------|
| `.up.sql`       | Versioned · runs once · transactional                      |
| `.r.up.sql`     | Repeatable · re-runs on content change · transactional     |
| `.notx.up.sql`  | Versioned · runs once · non-transactional                  |
| `.rnotx.up.sql` | Repeatable · re-runs on content change · non-transactional |
| `.down.sql`     | Rollback · always transactional                            |

The revision is exactly **7 zero-padded digits**. The name is free-form (hyphens and underscores, no dots). The
extension is the complete behaviour declaration - no other metadata needed.

```
0000001-create-users-table.up.sql
0000002-add-roles-table.up.sql
0000003-fn-get-users.r.up.sql        <- repeatable: re-applied when content changes
0000004-vacuum-users.notx.up.sql     <- non-transactional: runs outside BEGIN/COMMIT
0000005-refresh-stats.rnotx.up.sql   <- repeatable + non-transactional

0000001-create-users-table.down.sql  <- rollback for revision 1
0000002-add-roles-table.down.sql
```

### Why extensions - not directories or prefixes

The extension is what shell tools understand natively. No parsing, no convention memorisation:

```sh
# apply everything - reproduce the full database from scratch
find migrations/ -name "*.up.sql" | sort | xargs -I{} psql $DSN -f {}

# rollback in reverse
find migrations/ -name "*.down.sql" | sort -r | xargs -I{} psql $DSN -f {}

# repeatable files only - refresh all functions and views
find migrations/ -name "*.r.up.sql" -o -name "*.rnotx.up.sql" | sort | xargs -I{} psql $DSN -f {}

# non-transactional only
find migrations/ -name "*.notx.up.sql" -o -name "*.rnotx.up.sql" | sort | xargs -I{} psql $DSN -f {}
```

The tool adds safety on top: advisory locking, history tracking, hash verification, stray file detection. The bash path
is your emergency escape hatch - it always works.

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

**Safe separation of forward and rollback scripts**  
Keeping rollback files mixed together with forward migrations makes simple shell workflows harder and riskier. This tool
keeps them separate, so basic commands and file globs stay predictable and safe.

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

Statements that **cannot** run inside a transaction and require `.notx.up.sql` or `.rnotx.up.sql`:

- `VACUUM`
- `ALTER SYSTEM`
- `REINDEX SCHEMA / DATABASE / SYSTEM`
- `CREATE INDEX CONCURRENTLY`
- `DROP INDEX CONCURRENTLY`
- `ALTER TYPE ... ADD VALUE` (before PostgreSQL 12)

Non-transactional files are split into individual statements and executed one by one. If one fails, previously executed
statements in that file cannot be rolled back - plan accordingly.

---

## Directory layouts

**[`^        back to top        ^`](#table-of-contents)**

Migration files can live anywhere under the root directory. The tool walks recursively and sorts globally by revision.
Organise however makes sense for your project.

### Flat

```
migrations/
  0000001-create-users-table.up.sql
  0000001-create-users-table.down.sql
  0000002-add-roles-table.up.sql
  0000002-add-roles-table.down.sql
  0000003-fn-get-users.r.up.sql
  0000004-vacuum-users.notx.up.sql
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
    0000005-fn-get-users.r.up.sql
    0000006-fn-get-roles.r.up.sql
  no-transaction/
    0000007-vacuum-users.notx.up.sql
  down/
    0000001-create-users-table.down.sql
    0000002-add-roles-table.down.sql
    0000003-seed-roles.down.sql
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
      0000004-fn-get-users.r.up.sql
  v1.1.0/
    schema/
      0000005-add-audit-columns.up.sql
    no-transaction/
      0000006-vacuum-users.notx.up.sql
  down/
    0000001-create-users-table.down.sql
    0000002-add-roles-table.down.sql
    0000003-seed-roles.down.sql
    0000005-add-audit-columns.down.sql
    0000006-vacuum-users.down.sql
```

### By environment

```
migrations/
  dev/
    schema/
      0000001-create-users-table.up.sql
    data/
      0000002-seed-dev-users.up.sql
    functions/
      0000003-fn-get-users.r.up.sql
  prod/
    schema/
      0000001-create-users-table.up.sql
    functions/
      0000003-fn-get-users.r.up.sql
```

One rule applies in all layouts: **version numbers are global**. Two files with the same revision number anywhere in the
tree is an error.


---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache License 2.0 - see [LICENSE](LICENSE).
