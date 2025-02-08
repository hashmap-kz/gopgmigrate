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
- Versions should be unique recursively, error otherwise (data/00001-users.sql, schema/000001-roles.sql -> this is an error)
- Versions should be incremented sequentially (001,002,003), error otherwise
- Do not distinct between schema/data, but allow nested directories (versions, etc...)
- Add tag-based rollbacks (a special migration file, that writes a tag into history-table)
- For repeatable migrations it is possible to use another table
- Every *.do.sql MUST have corresponding *.undo.sql (it is possible to place all of them in any folder, and the file itself may be empty)







