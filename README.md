# PostgreSQL Migration Tool

This tool automates the execution of PostgreSQL migrations, supporting **versioned**, and **repeatable** migrations.
It ensures consistency and prevents duplicate executions using **advisory locks** and **transactional
execution**. It scans all migration files **recursively**, ensuring that version numbers are consistent and
strictly ordered.

---

## Features

- **Strictness**: This tool is designed to be simple in use and strict by rules.
- **Versioned Migrations**: Runs migrations sequentially, ensuring each version is applied once.
- **Repeatable Migrations**: Re-applies if the SQL script changes (hash-based detection).
- **Transactional Execution**: Ensures all migrations run within a single transaction.

--- 

## Naming conventions

![Migration Naming Convention](assets/migration-names.png)

---

## Installation

TODO

---

## Configuration

The database connection URL is retrieved from an environment variable:

```sh
### REQUIRED

export PGMIGRATE_CONNSTR="postgres://user:password@localhost:5432/dbname"
export PGMIGRATE_DIRNAME="examples/basic"

### OPTIONAL

# default: public.history
export PGMIGRATE_HISTORY_TABLE_NAME=migrate_history_dev

# one of: (debug, info, warn, error)
# default: info
export PGMIGRATE_LOG_LEVEL=debug

# one of: (console, json)
# default: console
export PGMIGRATE_LOG_MODE=console
```

---

## Strict Versioning Rules

The tool enforces **strict versioning policies** to maintain database integrity and ensure consistency across
migrations.

### 1. Unique Version Numbers Across All Directories

- Version numbers **must be globally unique** across **all directories**.
- Example (**Invalid Case – Causes an Error**):

  ```
  migrations/
    schema/00001-roles.sql
    data/00001-users.sql  ❌ (Duplicate version 00001)
  ```
    - The tool will **abort execution** if the same version is found in multiple directories.

### 2. Strictly Sequential Versioning

- Version numbers **must increment sequentially**.
- Example (**Invalid – Causes an Error**):

  ```
  migrations/
    00001-create-tables.sql
    00003-insert-default-values.sql ❌ (Missing 00002)
  ```
    - **Gaps in versioning are not allowed**.
    - The tool will reject any migration that does not fit a sequential order.

---

## Directory Structure

- **Nested directories are allowed** for organization, but all migrations must still adhere to the global versioning
  rules.

### Valid Example:

```
migrations/
  00001-init.sql
  00002-users.sql
  subdir/
    00003-orders.sql
    00004-products.sql
```

- The tool **scans all directories recursively** and ensures that:
    - Version numbers are **unique**.
    - Version numbers are **strictly sequential**.

---

## Tag-Based Rollbacks

Tag-based rollbacks allow migrations to be reverted to a specific **tagged checkpoint**.

### How It Works

- A special migration file writes a **tag marker** into the migration history table.
- The tool can then rollback all migrations **after** this tag.

### Example:

```
migrations/
  00010-add-users.sql
  00011-add-orders.sql
  00012-release-v1.tag.sql  ✅ (Tag marker)
  00013-modify-orders.sql
```

- Running a rollback to `00012-release-v1.tag.sql` will **undo** `00013-modify-orders.sql` and all subsequent
  migrations.

---

## Repeatable Migrations

Repeatable migrations are **not versioned** but **re-executed** whenever their contents change.

- Stored in a **separate tracking table**.
- Their **checksums** are used to detect changes.
- Example Naming:

  ```
  migrations/
    refresh_views.r.sql
    update_permissions.r.sql
  ```

- Every time the tool runs, it:
    - **Checks the checksum** of each repeatable migration.
    - If the file has changed, it **re-applies** the migration.

---

## Strict File Naming Rules

- **Each `*.do.sql` migration must have a corresponding `*.undo.sql` file**.
- **The `*.undo.sql` file can be empty** if no rollback is required.
- **The `*.r.sql`** stands for repeatable migrations.

### Example:

```
migrations/
  00001-init.do.sql
  00001-init.undo.sql  ✅ (Mandatory, even if empty)
```

- If a `.do.sql` file exists **without** a `.undo.sql` counterpart, **an error will be raised**.

---

## Handling Stray Files

- **Any unrecognized file inside the migration directory will cause an error.**
- Only the following file types are allowed:
    - **Versioned migrations (`00001-description.sql`)**
    - **Repeatable migrations (`description.r.sql`)**
    - **Tag migrations (`00012-release-v1.tag.sql`)**
    - **Undo migrations (`00001-description.undo.sql`)**

### Example (Invalid Case – Causes an Error)

```
migrations/
  00001-init.sql
  notes.txt  ❌ (Invalid file)
```

- **The tool will reject execution if stray files exist**.

---

## Key Design Enforcements

| Rule                                               | Enforcement                                                                    |
|----------------------------------------------------|--------------------------------------------------------------------------------|
| **Version numbers must be unique**                 | A version number cannot be reused across files, even in different directories. |
| **Version numbers must be sequential**             | No gaps (e.g., `00001`, `00002`, `00004` is invalid).                          |
| **Schema and data migrations are treated equally** | No separate rules; all are versioned together.                                 |
| **Nested directories are allowed**                 | The tool scans directories recursively.                                        |
| **Tag-based rollbacks are supported**              | Allows rolling back to a tagged migration checkpoint.                          |
| **Repeatable migrations are tracked separately**   | They are reapplied if modified.                                                |
| **Each `*.do.sql` must have a `*.undo.sql`**       | If no rollback is required, the `*.undo.sql` file can be empty.                |
| **Stray files are prohibited**                     | Any unrecognized file in `migration-dir` will cause an error.                  |

---

## Error Handling

| Error Type                                 | Cause                                       | Resolution                                          |
|--------------------------------------------|---------------------------------------------|-----------------------------------------------------|
| **Duplicate version**                      | Same version exists in multiple files       | Ensure each version number is unique globally.      |
| **Missing sequence**                       | Version numbers are not sequential          | Rename files to maintain sequential order.          |
| **Unrecognized file**                      | Stray files detected in migration directory | Remove or move non-migration files elsewhere.       |
| **Missing undo file**                      | `*.do.sql` exists without a `*.undo.sql`    | Add an empty `*.undo.sql` if no rollback is needed. |
| **Repeatable migration checksum mismatch** | A repeatable migration changed              | The tool will reapply the migration automatically.  |

---

## Conclusion

This migration tool ensures **strict versioning**, **repeatable migrations**, and **tag-based rollbacks** while
enforcing **file integrity**. By scanning all files recursively and enforcing strict rules, it provides a robust
migration process for PostgreSQL.

---

## **Contributing**

We welcome contributions! To contribute: see the [Contribution](CONTRIBUTING.md) guidelines.

---

## **License**

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## TODO

- no-transaction support
- https://github.com/flyway/flyway/blob/main/flyway-database/flyway-database-postgresql/src/main/java/org/flywaydb/database/postgresql/PostgreSQLParser.java#L35
- versioned and repeatable migrations should have order of execution, i.e. both should have a version (they may be
  placed in different dirs, but the order is matters, when some data migrations rely on some functions).
- CLI - print history, recursively traverse dir, sort files; add new; find latest; etc. etc.

```
Pattern COPY_FROM_STDIN_REGEX = Pattern.compile("^COPY( .*)? FROM STDIN");
Pattern CREATE_DATABASE_TABLESPACE_SUBSCRIPTION_REGEX = Pattern.compile("^(CREATE|DROP) (DATABASE|TABLESPACE|SUBSCRIPTION)");
Pattern ALTER_SYSTEM_REGEX = Pattern.compile("^ALTER SYSTEM");
Pattern CREATE_INDEX_CONCURRENTLY_REGEX = Pattern.compile("^(CREATE|DROP)( UNIQUE)? INDEX CONCURRENTLY");
Pattern REINDEX_REGEX = Pattern.compile("^REINDEX( VERBOSE)? (SCHEMA|DATABASE|SYSTEM)");
Pattern VACUUM_REGEX = Pattern.compile("^VACUUM");
Pattern DISCARD_ALL_REGEX = Pattern.compile("^DISCARD ALL");
Pattern ALTER_TYPE_ADD_VALUE_REGEX = Pattern.compile("^ALTER TYPE( .*)? ADD VALUE");
```
