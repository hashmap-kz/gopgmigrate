#!/usr/bin/env bash
# rename-migrations.sh
#
# Renames migration files from the old convention to the new one:
#
#   OLD                              NEW
#   ---------------------------------------------------------------
#   NNNNN-name.do.sql            →  NNNNNNN-name.do.sql
#   NNNNN-name.ntx.do.sql        →  NNNNNNN-name.notx.do.sql
#   NNNNN-name.r.sql             →  NNNNNNN-name.r.do.sql
#   NNNNN-name.ntx.r.sql         →  NNNNNNN-name.rnotx.do.sql
#   NNNNN-name.undo.sql          →  NNNNNNN-name.undo.sql
#
#   5-digit zero-padded rev      →  7-digit zero-padded rev
#
# Usage:
#   ./rename-migrations.sh <root-dir>
#   ./rename-migrations.sh examples
#
# Dry-run (print only, no changes):
#   DRY_RUN=1 ./rename-migrations.sh examples

set -euo pipefail

ROOT="${1:-.}"
DRY_RUN="${DRY_RUN:-0}"

renamed=0
skipped=0
errors=0

# ── helpers ──────────────────────────────────────────────────────────────────

log()  { echo "  $*"; }
ok()   { echo "  ✓  $*"; }
skip() { echo "  –  $*"; ((skipped++)) || true; }
err()  { echo "  ✗  $*" >&2; ((errors++)) || true; }

do_rename() {
    local src="$1" dst="$2"
    if [[ "$src" == "$dst" ]]; then
        skip "unchanged: $src"
        return
    fi
    if [[ -e "$dst" ]]; then
        err "destination already exists, skipping: $dst"
        return
    fi
    if [[ "$DRY_RUN" == "1" ]]; then
        ok "[dry-run] $src  →  $(basename "$dst")"
    else
        mv "$src" "$dst"
        ok "$src  →  $(basename "$dst")"
    fi
    ((renamed++)) || true
}

# Pad a decimal string to 7 digits.
pad7() {
    printf '%07d' "$((10#$1))"
}

# ── per-file transformation ───────────────────────────────────────────────────

process_file() {
    local path="$1"
    local dir base name new_base new_path

    dir="$(dirname "$path")"
    base="$(basename "$path")"

    # skip non-sql and already-correct files, stray files etc.
    if [[ "$base" != *.sql ]]; then
        skip "not .sql: $path"
        return
    fi

    # ── extract 5-digit rev prefix ───────────────────────────────────────────
    # expected prefix: NNNNN-  (exactly 5 digits then a dash)
    if [[ ! "$base" =~ ^([0-9]{5})-(.+)$ ]]; then
        skip "no 5-digit rev prefix, skipping: $path"
        return
    fi

    local rev5="${BASH_REMATCH[1]}"
    local rest="${BASH_REMATCH[2]}"   # everything after "NNNNN-"
    local rev7
    rev7="$(pad7 "$rev5")"

    # ── classify and build new basename ──────────────────────────────────────

    # 1. NNNNN-name.ntx.r.sql  →  NNNNNNN-name.rnotx.do.sql
    if [[ "$rest" =~ ^(.+)\.ntx\.r\.sql$ ]]; then
        name="${BASH_REMATCH[1]}"
        new_base="${rev7}-${name}.rnotx.do.sql"

    # 2. NNNNN-name.ntx.do.sql  →  NNNNNNN-name.notx.do.sql
    elif [[ "$rest" =~ ^(.+)\.ntx\.do\.sql$ ]]; then
        name="${BASH_REMATCH[1]}"
        new_base="${rev7}-${name}.notx.do.sql"

    # 3. NNNNN-name.r.sql  →  NNNNNNN-name.r.do.sql
    elif [[ "$rest" =~ ^(.+)\.r\.sql$ ]]; then
        name="${BASH_REMATCH[1]}"
        new_base="${rev7}-${name}.r.do.sql"

    # 4. NNNNN-name.undo.sql  →  NNNNNNN-name.undo.sql
    elif [[ "$rest" =~ ^(.+)\.undo\.sql$ ]]; then
        name="${BASH_REMATCH[1]}"
        new_base="${rev7}-${name}.undo.sql"

    # 5. NNNNN-name.do.sql  →  NNNNNNN-name.do.sql
    elif [[ "$rest" =~ ^(.+)\.do\.sql$ ]]; then
        name="${BASH_REMATCH[1]}"
        new_base="${rev7}-${name}.do.sql"

    # 6. unrecognised pattern
    else
        skip "unrecognised extension pattern, skipping: $path"
        return
    fi

    new_path="${dir}/${new_base}"
    do_rename "$path" "$new_path"
}

# ── main ──────────────────────────────────────────────────────────────────────

echo ""
echo "rename-migrations.sh"
echo "  root    : $ROOT"
echo "  dry-run : $DRY_RUN"
echo ""

# collect all .sql files, process deepest paths first to avoid
# renaming a parent before its children (not an issue for flat files
# but safe practice for any nested structure)
while IFS= read -r -d '' file; do
    process_file "$file"
done < <(find "$ROOT" -type f -name "*.sql" -print0 | sort -z)

echo ""
echo "done — renamed: $renamed  skipped: $skipped  errors: $errors"
if (( errors > 0 )); then
    echo "errors occurred — review output above" >&2
    exit 1
fi

## preview — no changes made
#DRY_RUN=1 ./rename-migrations.sh examples
#
## apply
#./rename-migrations.sh examples
