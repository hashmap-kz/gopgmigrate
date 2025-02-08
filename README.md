# PostgreSQL Migration Tool

This tool automates the execution of PostgreSQL migrations, supporting **versioned**, **repeatable**, and **data**
migrations. It ensures consistency and prevents duplicate executions using **advisory locks** and **transactional
execution**.

---

## Features

- **Versioned Migrations**: Runs migrations sequentially, ensuring each version is applied once.
- **Repeatable Migrations**: Re-applies if the SQL script changes (hash-based detection).
- **Data Migrations**: Similar to schema migrations but for seed/data scripts.
- **Transactional Execution**: Ensures all migrations run within a single transaction.

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

# default: public.migrate_history
export PGMIGRATE_HISTORY_TABLE_NAME=migrate_history_dev

# one of: (debug, info, warn, error)
# default: info
export PGMIGRATE_LOG_LEVEL=debug

# one of: (console, json)
# default: console
export PGMIGRATE_LOG_MODE=console
```

---

## Migration Directory Structure

Migrations must follow a structured format:

```
migrations/
  ├── schema/       # Versioned migrations (5-digit prefix, .do.sql)
  ├── repeatable/   # Repeatable migrations
  ├── data/         # Data migrations (5-digit prefix, .do.sql)
```

Example files:

```
schema/
  ├── 00001-init.do.sql
  ├── 00002-add_users.do.sql

repeatable/
  ├── fn_get_users.sql
  ├── update_schema.sql

data/
  ├── 00001-seed_data.do.sql
```

---

## Running Migrations

Run migrations using:

TODO

--- 

## Error Handling

- Logs errors and stops execution on failure.
- Uses advisory locks to prevent concurrent runs.
- Ensures directories exist before execution.

--- 

## Implementation Details

- **Regex Validation**:
    - Versioned migrations: `^\d{5}-[^.]+\.do\.sql$`
    - Repeatable migrations: `.*\.sql$`
- **Sorting**:
    - Versioned migrations are sorted numerically using their prefix.
- **Transaction Safety**:
    - All migrations run inside a **single transaction**.

---

## **Contributing**

We welcome contributions! To contribute: see the [Contribution](CONTRIBUTING.md) guidelines.

---

## **License**

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

---

## Notes:

- Version numbers must be unique across all directories when scanned recursively; otherwise, an error will be raised (
  e.g., data/00001-users.sql and schema/00001-roles.sql is an error).
- Version numbers must increment sequentially (e.g., 00001, 00002, 00003); any gaps or inconsistencies will result in an
  error.
- No distinction is made between schema/ and data/, but nested directories are allowed for organizing versions and other
  files.
- Support for tag-based rollbacks is provided using special migration files that write a tag into the history table.
- Repeatable migrations can be managed using a separate table.
- Every *.do.sql file must have a corresponding *.undo.sql file. These files can be placed in any directory, and the *
  .undo.sql file may be empty if no undo action is required.
- Any stray file inside 'migration-dir' must cause an error (if the file is not a versioned-migration, and not a repeatable one).






