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
- **Migration Modes**: `plain`, `group`, `mixed` modes, with robust transaction handling model.

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

## Quick start

TODO

```
go run .\main.go migrate --dirname=examples/basic --connstr postgres://postgres:postgres@localhost:5432/bookstore --dry-run --mode=mixed
```

---

# Migration Documentation

## Migration Modes

There are three migration modes:

- **plain (default)** - Executes all pending files one by one.
    - If a file is **transactional** (`*.do.sql`, `*.r.sql`, `*.undo.sql`), all statements within the file are executed
      in a **single transaction**.
    - If a file is **non-transactional** (`*.ntx.do.sql`, `*.ntx.r.sql`, `*.ntx.undo.sql`), the file's content is split
      into individual SQL statements, which are executed **one by one**.

- **group** - Executes all pending files as a single **group**.
    - All files must either be executed within **one transaction** (if transactional) or all must be **non-transactional
      ** (`*.ntx.*`).

- **mixed** - Splits pending migrations into **separate transactional and non-transactional groups**.
    - If a group is **transactional**, all files within it are applied **within a single transaction**.
    - If a group is **non-transactional**, each file is executed **individually**, following the behavior of **plain
      mode**.

## Migration Directory Structure

Migration files can be placed in directories and subdirectories.  
The discovery process is recursive.

File names **always** start with a version number (`00001-`) and have the `.sql` extension.  
Each file's version number should increase sequentially.

Example layout:

  ```
  migrations/
    00001-init.sql
    00002-users.sql
    subdir/
      00003-orders.sql
      00004-products.sql
  ```

Since scanning is recursive, **version numbering is global** across all directories.  
A subdirectory **cannot contain a file with a version number that is already used** in a higher-level directory.

Example (**Invalid Case – Causes an Error**):

  ```
  migrations/
    schema/00001-roles.sql
    data/00001-users.sql  ❌ (Duplicate version 00001)
  ```

As a result, the following Bash command should return a **complete, version-ordered list** of all migrations, scanning
all directories:

```sh
find migrations/ -type f \( -iname \*.r.sql -o -iname \*.do.sql \) -exec basename {} \; | sort
```

---

## Planning Migration Steps

You are responsible for carefully planning each migration step.

### Example Considerations:

- **PostgreSQL** allows most **DDL statements** to be executed within transactions, while some other database systems do
  not.
- In PostgreSQL, **only a few statements cannot be executed within a transaction**, some examples:
    - `VACUUM`
    - `ALTER SYSTEM`
    - `REINDEX`
- However, most of these statements are maintenance-related, making it **relatively easy to structure migration steps
  accordingly**.

---

## Handling Unexpected Database States

This **iteration-based** approach (with group/mixed modes) is chosen because it is the most robust.
For example, during the development process, you may need to apply multiple migrations each week.
When applying migrations in a production environment, if a script fails, you must resolve the issue quickly, as some
scripts may have already been applied while others have not.
This situation can become a nightmare.
A better approach is to apply all scripts within a transaction (if your DBMS supports it—see the notes above).
This way, if something fails, you don’t have to worry because your database remains unchanged.
You can then resolve the issue and retry the process without panic.

There may be multiple reasons why this situation occurs.  
However, the key issue is that **the database is not in the expected state**.

For example, your backend services may fail to function correctly due to an **incomplete database state**.  
Even though the database remains **ACID-compliant and technically consistent**, **missing migrations** can cause
business rules to fail.


---

## **Contributing**

We welcome contributions! To contribute: see the [Contribution](CONTRIBUTING.md) guidelines.

---

## **License**

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

























