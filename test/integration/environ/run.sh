#!/usr/bin/env bash
set -euo pipefail

(
  make restart
  export PGPASSWORD=test

  # Wait for PostgreSQL to become ready inside the container
  until docker exec pg-primary pg_isready -U test >/dev/null 2>&1; do
    echo "Waiting for PostgreSQL to be ready..."
    sleep 1
  done
)
