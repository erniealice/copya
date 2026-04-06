# copya

Business-type data provider for seeds and E2E fixtures. Embeds CSV data for all
business types and exposes a Go API for inserting it into any SQL database.

Two distinct datasets live here:

| Dataset | Directory | Purpose | Command |
|---------|-----------|---------|---------|
| **Seeds** | `seeds/` | Bootstrap a working app with realistic data | `go run ./cmd/setup --drop` |
| **Fixtures** | `fixtures/` | Assertion-grade data for E2E report testing | `go run ./cmd/fixtures load <suite>` |

---

## Seeds

`seeds/` holds the data that makes a fresh database usable: roles, clients, products,
suppliers, locations, permissions, categories, and more.

### 3-tier cascade

```
seeds/common/     ← shared across all business types
seeds/general/    ← defaults for all types (overrides common)
seeds/{type}/     ← business-type overrides (overrides general)
```

Later tiers replace earlier tiers for the same table name.

### Business types

| Type | Directory | Use case |
|------|-----------|----------|
| `professional` | `seeds/professional/` | Legal, consulting, accounting |
| `service` | `seeds/service/` | Salon, spa, clinic |
| `retail` | `seeds/retail/` | Retail POS |

### Go API

```go
import (
    "github.com/erniealice/copya"
    v1 "github.com/erniealice/copya/golang/v1"
)

// Seed a database in one call (preferred)
err := v1.Seed(ctx, db, "professional", v1.Postgres)

// Lower-level: load + inspect
provider := v1.NewSeedProvider(copya.SeedsFS)
set, err := provider.Load("professional")   // returns SeedSet (map[table]→SeedTable)
table := set["revenue_category"]
fmt.Println(table.Headers) // [id, name, active, ...]
sql := table.ToSQL("revenue_category", v1.Postgres)
```

`Seed()` inserts all tables in `InsertOrder` (least-dependent first), using
`ON CONFLICT (id) DO NOTHING` — safe to re-run.

### Adding a new business type

1. Create `seeds/{type}/` directory
2. Add CSV files — each file stem becomes a table name
3. CSV header row = column names; data rows = values
4. Tables in `{type}/` override same-named tables in `general/`
5. The `embed.go` glob already covers `seeds/**` — no changes needed

---

## Fixtures

`fixtures/` holds assertion-grade test data for E2E report tests. These are NOT
bootstrap seeds — they are loaded before a test suite and cleared after, providing
a stable dataset with known totals.

### 2-tier cascade

```
fixtures/{suite}/common/        ← shared across all business types
fixtures/{suite}/{businessType} ← professional / service / retail overrides
```

### Suites

| Suite | Tables | Purpose |
|-------|--------|---------|
| `revenue-reporting` | `location_area`, `revenue`, `revenue_line_item` | Revenue totals, aging receivables, category breakdowns |
| `expense-reporting` | `location_area`, `expenditure`, `expenditure_line_item`, `purchase_order`, `purchase_order_line_item` | Expenditure totals, PO tracking |
| `disbursement-reporting` | `location_area`, `disbursement_method`, `treasury_disbursement`, `disbursement_schedule` | Disbursement totals, schedule aging |

### ID convention

All fixture rows use the `e2e-*` prefix so they are visually distinct from seed data
and can be precisely targeted by `ClearFixtures`:

```
e2e-rev-001, e2e-exp-001, e2e-td-001, e2e-la-cebu-bp, e2e-dm-bank, ...
```

### Go API

```go
import v1 "github.com/erniealice/copya/golang/v1"

// Load fixture data (idempotent — ON CONFLICT DO NOTHING)
err := v1.LoadFixtures(ctx, db, "revenue-reporting", "professional", v1.Postgres)

// Clear fixture data (deletes by ID, reverse InsertOrder for FK safety)
err := v1.ClearFixtures(ctx, db, "revenue-reporting", "professional", v1.Postgres)
```

### CLI (from apps/service-admin)

```bash
go run ./cmd/fixtures load revenue-reporting
go run ./cmd/fixtures load revenue-reporting professional
go run ./cmd/fixtures clear revenue-reporting
go run ./cmd/fixtures clear --all
```

### Adding a new suite

1. Create `fixtures/{suite-name}/common/` and `fixtures/{suite-name}/professional/`
2. Add CSV files with `e2e-*` IDs; first column must always be `id`
3. Add the suite to the embed glob in `fixtures.go`
4. Add the suite name to `allSuites` in `apps/service-admin/cmd/fixtures/main.go`
5. If new tables are introduced, add them to `InsertOrder` in `golang/v1/types.go` at the correct FK-dependency level

---

## Data format

Both seeds and fixtures use the same CSV format:

- Header row = column names (must match database schema)
- Empty values or `NULL` → SQL `NULL`
- SQL output uses `ON CONFLICT (id) DO NOTHING`
- Insert order follows `InsertOrder` in `golang/v1/types.go` (least-dependent → most-dependent)

---

## CLI tool

`cmd/copya/` is a standalone CLI for inspecting seed data without a running database:

```bash
go build -o copya ./cmd/copya

copya --business-type professional --format list      # list all tables
copya --business-type professional --table revenue    # SQL for one table
copya --business-type professional --format csv       # CSV output
```

Useful for verifying seed content or piping to `psql` in dev.
