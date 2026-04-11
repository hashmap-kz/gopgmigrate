# gopgmigrate

A small, strict migration tool for **PostgreSQL only**.

`gopgmigrate` is built for teams that want plain SQL files, predictable rules, and a migration directory that can be organized in a way that fits a real project. It scans migrations **recursively**, supports **repeatable** scripts, supports **non-transactional** PostgreSQL statements via explicit filename suffixes, and protects execution with a **PostgreSQL advisory lock**.

It is intentionally focused: no multi-database abstraction, no ORM coupling, no hidden DSL. Just PostgreSQL and SQL files.

## Why this tool

- **PostgreSQL-only by design**
- **Recursive migration discovery** across nested directories
- **Flexible directory layout** with one global version space
- **Strict file naming** so migrations stay easy to reason about
- **Repeatable migrations** with SHA-256 change detection
- **Transactional and non-transactional** migrations
- **Rollback by count** using matching `down` scripts
- **Dry-run mode** to inspect what would be executed
- **Advisory locking** to prevent concurrent migration runs

## Migration file naming

Migration filenames are strict and explicit:

```text
0000001-create-users.up.sql
0000002-refresh-user-stats.r.up.sql
0000003-vacuum-big-table.notx.up.sql
0000004-refresh-heavy-view.rnotx.up.sql
0000005-create-users.down.sql
```

Supported kinds:

- `*.up.sql` — versioned, transactional
- `*.r.up.sql` — repeatable, transactional
- `*.notx.up.sql` — versioned, non-transactional
- `*.rnotx.up.sql` — repeatable, non-transactional
- `*.down.sql` — rollback script

Version prefix format:

- exactly **7 digits**
- one global version sequence across the whole migration tree

Examples:

```text
0000001-init-schema.up.sql
0000002-seed-roles.up.sql
0000003-refresh-view.r.up.sql
0000004-create-index-concurrently.notx.up.sql
0000002-seed-roles.down.sql
```

## Flexible migration directory structure

The migration directory is scanned **recursively**. You are free to split files by domain, release, environment, or purpose.

That means this is valid:

```text
migrations/
├── schema/
│   ├── 0000001-init.up.sql
│   └── 0000002-users.up.sql
├── data/
│   └── 0000003-seed-roles.up.sql
├── views/
│   └── 0000004-refresh-user-stats.r.up.sql
└── maintenance/
    └── 0000005-reindex.notx.up.sql
```

The only hard rule is:

**versions are global across all subdirectories.**

So this is invalid:

```text
migrations/
├── schema/0000001-init.up.sql
└── data/0000001-seed-users.up.sql
```

because version `0000001` is duplicated.

This layout is one of the main ideas behind the tool: you are not forced into a single flat folder, but you still keep a strict, sortable migration history.

## How repeatable migrations work

Repeatable migrations are tracked by filename and content hash.

- if a repeatable file was never applied, it is executed
- if its content changed, it is executed again
- if its content did not change, it is skipped

Versioned migrations are stricter:

- if not applied yet, they are executed
- if already applied and the file changed, execution stops with an error

## Transactional vs non-transactional scripts

By default, `gopgmigrate` runs migrations in a transaction.

For PostgreSQL statements that must not run inside a transaction, use the `notx` variants:

- `*.notx.up.sql`
- `*.rnotx.up.sql`

The tool also performs a pre-check for common PostgreSQL statements that usually require non-transactional execution, such as:

- `CREATE INDEX CONCURRENTLY`
- `ALTER SYSTEM`
- `VACUUM`
- `REINDEX` on database/system/schema targets
- `ALTER TYPE ... ADD VALUE`
- `COPY ... FROM STDIN`

This helps catch mistakes before migration execution starts.

## Commands

```text
gopgmigrate migrate
gopgmigrate last
gopgmigrate rollback-count <steps>
```

Examples:

```bash
gopgmigrate migrate \
  --dirname ./migrations \
  --connstr 'postgres://user:pass@localhost:5432/app' \
  --history-table public.migrate_history

gopgmigrate rollback-count 2 \
  --dirname ./migrations \
  --connstr 'postgres://user:pass@localhost:5432/app' \
  --history-table public.migrate_history
```

Dry-run example:

```bash
gopgmigrate migrate \
  --dirname ./migrations \
  --connstr 'postgres://user:pass@localhost:5432/app' \
  --dry-run
```

## Configuration

Flags:

- `--dirname` — migration directory
- `--connstr` — PostgreSQL connection string
- `--history-table` — history table in `schema.table` format
- `--log-enc` — `text` or `json`
- `--log-level` — `debug`, `info`, `warn`, `error`
- `--dry-run` — print pending migrations without applying them

Environment variables:

```bash
export PGMIGRATE_DIRNAME=./migrations
export PGMIGRATE_CONNSTR='postgres://postgres:postgres@localhost:5432/app'
export PGMIGRATE_HISTORY_TABLE_NAME=public.migrate_history
```

## Quick start

Create a migration tree:

```text
migrations/
├── schema/
│   ├── 0000001-init.up.sql
│   └── 0000002-users.up.sql
├── data/
│   └── 0000003-seed-admin.up.sql
└── rollback/
    ├── 0000003-seed-admin.down.sql
    └── 0000002-users.down.sql
```

Run migrations:

```bash
go run ./main.go migrate \
  --dirname ./migrations \
  --connstr 'postgres://postgres:postgres@localhost:5432/app'
```

Inspect pending migrations without changing the database:

```bash
go run ./main.go migrate \
  --dirname ./migrations \
  --connstr 'postgres://postgres:postgres@localhost:5432/app' \
  --dry-run
```

Rollback the last migration:

```bash
go run ./main.go rollback-count 1 \
  --dirname ./migrations \
  --connstr 'postgres://postgres:postgres@localhost:5432/app'
```

## Project structure

Current layout:

```text
.
├── main.go
├── internal/
│   ├── filters/
│   ├── history/
│   ├── migration/
│   ├── naming/
│   ├── resolver/
│   └── stmt/
├── pkg/
│   └── logger/
└── examples/
    └── tree/
```

Package roles:

- `internal/naming` — filename parsing and migration kind rules
- `internal/resolver` — recursive file discovery and validation
- `internal/filters` — decide what should be applied or rolled back
- `internal/history` — PostgreSQL migration history repository and advisory lock
- `internal/migration` — migration execution flow
- `internal/stmt` — SQL statement splitting for non-transactional scripts
- `pkg/logger` — logger setup

## Scope

This project is for **PostgreSQL migrations**.

It is not trying to be:

- a generic SQL migration framework
- a multi-database tool
- a schema diff engine
- an ORM migration generator

The goal is to stay small, readable, and strict.

## Current status

The project is currently **CLI-first**. The repository has internal packages and a clean code layout, but the public contract is the command-line tool.

If you need PostgreSQL migrations with plain SQL, nested directories, repeatable scripts, and explicit `notx` handling, this is what `gopgmigrate` is built for.
