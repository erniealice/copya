# seeds/

Bootstrap seed data for all business types. These CSVs are loaded into a fresh
database by `go run ./cmd/setup --drop` (via `copya.Seed`) to produce a working,
realistic app environment.

## Structure

```
seeds/
  common/          ← shared across all business types (roles, collection_method, ...)
  general/         ← defaults for all types, overrides common (clients, products, ...)
  professional/    ← professional overrides (legal/consulting-specific data)
  service/         ← service overrides (salon/spa/clinic-specific data)
  retail/          ← retail overrides (POS-specific data)
```

Later tiers replace earlier tiers for the **same table name**. A table present in
`professional/` completely replaces the version from `general/` for that business type.

## File naming

Each `.csv` file stem is the table name:

```
revenue_category.csv  →  table "revenue_category"
client.csv            →  table "client"
```

## ID stability

Seed IDs are stable across resets (e.g. `client-001`, `loc-main`, `revcat-legal`).
They are referenced by fixture data and hardcoded in test helpers — do not change them.

## Adding or modifying seeds

1. Edit or add a `.csv` file in the appropriate tier directory
2. First row = column names (must match the live database schema)
3. Empty cells or `NULL` → SQL NULL
4. All inserts use `ON CONFLICT (id) DO NOTHING` — safe to re-run
5. Insertion follows `InsertOrder` in `../golang/v1/types.go` — if you add a new table,
   add it to `InsertOrder` at the correct FK-dependency level

## Loading seeds

```bash
# Full reset (preferred)
go run ./cmd/setup --drop          # from apps/service-admin

# Seed only (no drop/migrate)
go run ./cmd/seeder                # from apps/service-admin
```
