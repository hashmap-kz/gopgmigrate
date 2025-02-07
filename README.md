# PostgreSQL Migration Tool

This tool automates the execution of PostgreSQL migrations, supporting **versioned**, **repeatable**, and **data**
migrations. It ensures consistency and prevents duplicate executions using **advisory locks** and **transactional
execution**.

## Features

- **Versioned Migrations**: Runs migrations sequentially, ensuring each version is applied once.
- **Repeatable Migrations**: Re-applies if the SQL script changes (hash-based detection).
- **Data Migrations**: Similar to schema migrations but for seed/data scripts.
- **Transactional Execution**: Ensures all migrations run within a single transaction.

## Installation

TODO

## Configuration

The database connection URL is retrieved from an environment variable:

```sh
export DATABASE_URL="postgres://user:password@localhost:5432/dbname"
```

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

## Running Migrations

Run migrations using:

```sh
go run main.go
```

## Error Handling

- Logs errors and stops execution on failure.
- Uses advisory locks to prevent concurrent runs.
- Ensures directories exist before execution.

## Implementation Details

- **Regex Validation**:
    - Versioned migrations: `^\d{5}-[^.]+\.do\.sql$`
    - Repeatable migrations: `.*\.r\.sql$`
- **Sorting**:
    - Versioned migrations are sorted numerically using their prefix.
- **Transaction Safety**:
    - All migrations run inside a **single transaction**.

## License

MIT License

## Contributing

1. Fork the repository
2. Create a feature branch
3. Submit a pull request
