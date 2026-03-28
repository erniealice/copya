#!/usr/bin/env bash
# setup-db.sh — Create a fresh PostgreSQL database with schema + seed data
#
# Usage:
#   ./scripts/setup-db.sh                          # ichizen_dev + service
#   ./scripts/setup-db.sh professional1 professional
#   ./scripts/setup-db.sh mydb service
#   PGUSER=postgres ./scripts/setup-db.sh mydb professional
#
# What it does:
#   1. Generates 00_full_schema.sql from esqyma migrations (if missing)
#   2. Builds the copya binary (if missing)
#   3. DROP + CREATE the database
#   4. Applies the full schema (147 tables, all FKs, all indexes)
#   5. Seeds with business-type data (47 tables of realistic test data)

set -euo pipefail

DB_NAME="${1:-ichizen_dev}"
BIZ_TYPE="${2:-service}"
PG_USER="${PGUSER:-cradle}"

# Resolve paths relative to this script
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
COPYA_DIR="$(dirname "$SCRIPT_DIR")"
ROOT_DIR="$(dirname "$(dirname "$COPYA_DIR")")"
ESQYMA_DIR="$ROOT_DIR/packages/esqyma"
SCHEMA_FILE="$ESQYMA_DIR/migrations/postgres/00_full_schema.sql"
COPYA_BIN="$COPYA_DIR/copya"

echo "╔══════════════════════════════════════╗"
echo "║   Ichizen Database Setup             ║"
echo "╠══════════════════════════════════════╣"
echo "║  Database:      $DB_NAME"
echo "║  Business type: $BIZ_TYPE"
echo "║  PG user:       $PG_USER"
echo "╚══════════════════════════════════════╝"
echo ""

# --- Step 1: Schema DDL ---
if [ ! -f "$SCHEMA_FILE" ]; then
    echo "[1/4] Generating schema from esqyma migrations..."
    if [ ! -f "$ESQYMA_DIR/scripts/generate-full-schema.py" ]; then
        echo "ERROR: $ESQYMA_DIR/scripts/generate-full-schema.py not found"
        echo "       Make sure the esqyma submodule is checked out."
        exit 1
    fi
    (cd "$ESQYMA_DIR" && python3 scripts/generate-full-schema.py)
else
    echo "[1/4] Schema OK: $(wc -l < "$SCHEMA_FILE") lines"
fi

# --- Step 2: Build copya ---
if [ ! -f "$COPYA_BIN" ] || [ "$COPYA_DIR/cmd/copya/main.go" -nt "$COPYA_BIN" ]; then
    echo "[2/4] Building copya..."
    (cd "$COPYA_DIR" && go build -o copya ./cmd/copya)
else
    echo "[2/4] Copya binary OK"
fi

# --- Step 3: Create database ---
echo "[3/4] Creating database $DB_NAME (dropping if exists)..."
psql -U "$PG_USER" -d postgres -c "DROP DATABASE IF EXISTS \"$DB_NAME\";" 2>/dev/null || true
psql -U "$PG_USER" -d postgres -c "CREATE DATABASE \"$DB_NAME\";"

echo "      Applying schema..."
psql -U "$PG_USER" -d "$DB_NAME" -f "$SCHEMA_FILE" > /dev/null 2>&1
TABLE_COUNT=$(psql -U "$PG_USER" -d "$DB_NAME" -t -c "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE';")
echo "      $TABLE_COUNT tables created"

# --- Step 4: Seed data ---
echo "[4/4] Seeding $BIZ_TYPE data..."
"$COPYA_BIN" --business-type "$BIZ_TYPE" --dialect postgres | psql -U "$PG_USER" -d "$DB_NAME" > /dev/null 2>&1
SEED_COUNT=$(psql -U "$PG_USER" -d "$DB_NAME" -t -c "SELECT count(*) FROM \"user\";")
echo "      Done ($SEED_COUNT users seeded)"

echo ""
echo "Ready! Connect with:"
echo "  psql -U $PG_USER -d $DB_NAME"
