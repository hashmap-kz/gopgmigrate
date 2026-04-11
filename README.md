# gopgmigrate

SQL-first PostgreSQL migrations - rollbacks, repeatable scripts, any directory layout

Runs migrations sequentially with advisory locking, transactional safety, and hash-based change detection - no config
files, no YAML, no ORM coupling, no hidden DSL, no magic. Just SQL files and a clear naming convention.

<!-- TOC -->
* [gopgmigrate](#gopgmigrate)
  * [How it works](#how-it-works)
  * [Usage](#usage)
    * [CLI](#cli)
      * [Flags](#flags)
      * [Examples](#examples)
    * [Library](#library)
  * [Naming conventions](#naming-conventions)
    * [Why extensions - not directories or prefixes](#why-extensions---not-directories-or-prefixes)
    * [Design rationale](#design-rationale)
    * [Transaction behaviour](#transaction-behaviour)
  * [Directory layouts](#directory-layouts)
    * [Flat](#flat)
    * [By concern](#by-concern)
    * [By release and concern](#by-release-and-concern)
    * [By environment](#by-environment)
  * [Contributing](#contributing)
  * [License](#license)
<!-- TOC -->

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

---

## Naming conventions

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

Other tools made choices that this tool deliberately avoids.

**Flat-only layouts** are impractical for projects that grow across release cycles,
separate schema and data concerns, or multiple environments. `gopgmigrate` imposes no
structure - organise directories however your project demands.

**Pseudo-DSL and magic comments** inside SQL files - especially when up and down logic
live in the same file - make it impossible to open a file in a database IDE and run it
directly. Every file here is plain executable SQL, nothing else.

**Rollback scripts alongside forward migrations** break shell-based workflows. A plain
`find *.sql` glob picks up both directions at once. `gopgmigrate` keeps them separate so
`find` and `psql` always work safely without the tool.

**Vendor lock-in** means your migration files become useless without the tool that owns
them. Here, every file is a plain SQL file. The tool is optional. `find | sort | psql`
reproduces your database from scratch with no binary required.

**Repeatable migrations and non-transactional execution** are not edge cases - they are
everyday requirements for managing views, functions, and maintenance operations. Both
are first-class citizens, declared in the filename, requiring no special configuration.

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
