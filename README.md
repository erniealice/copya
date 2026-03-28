# copya

Business-type seed data provider. Provides ready-to-use CSV seed values per business type, following the same 3-tier cascade as [lyngua](../lyngua):

```
common → general → {businessType}
```

Later tiers replace earlier tiers for the same table name.

## Business types

| Type | Description |
|------|-------------|
| `service` | Salon, spa, and service-based businesses |

More business types (retail, professional, construction, etc.) can be added by creating a new directory under `seeds/`.

## Seed tables

### common (shared across all types)

| Table | Description |
|-------|-------------|
| `collection_method` | Payment collection methods (cash, card, gcash, etc.) |
| `role` | Default workspace roles |

### general (default for all types)

| Table | Description |
|-------|-------------|
| `attribute` | Product and general attributes |
| `client` | Sample client records |
| `expenditure_category` | Expense classification |
| `location` | Generic business locations |
| `supplier_category` | Vendor classification |

### service (overrides for service business type)

| Table | Description |
|-------|-------------|
| `asset_category` | Salon equipment, furniture, computers, vehicles, leasehold, office |
| `attribute` | Service-specific attributes (hair length, treatment type, staff specialty) |
| `location` | Cebu-area salon branches |
| `product` | 15 salon/spa services (haircuts, massage, facial, nails, makeup) |
| `revenue_category` | Hair, spa, nails, makeup, packages |
| `supplier` | L'Oreal, Takara Belmont, utilities, IT, janitorial, freelance |

## CLI usage

```bash
# Build the CLI
go build -o copya ./cmd/copya

# List all tables for a business type
copya --business-type service --format list

# Generate SQL INSERT statements (default)
copya --business-type service

# Generate SQL for a specific table
copya --business-type service --table product

# Output as CSV
copya --business-type service --format csv --table product
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--business-type` | `service` | Business type (maps to `seeds/{type}/` directory) |
| `--table` | *(all)* | Specific table name to output |
| `--format` | `sql` | Output format: `sql`, `csv`, or `list` |

### Pipe to psql

```bash
copya --business-type service | psql -d mydb
copya --business-type service --table product | psql -d mydb
```

## Go API usage

```go
import (
    "github.com/erniealice/copya"
    v1 "github.com/erniealice/copya/golang/v1"
)

// Create provider with embedded seeds
provider := v1.NewSeedProvider(copya.SeedsFS)

// Load all tables for a business type
set, err := provider.Load("service")

// Access a specific table
products := set["product"]
fmt.Println(products.Headers) // [id, name, description, price, currency, active]
fmt.Println(products.Rows[0]) // [prod-001, Haircut - Standard, ...]

// Load a single table
table, err := provider.Table("service", "product")

// Generate SQL
sql := table.ToSQL("product")

// List available tables
names, err := provider.Tables("service")
```

## Adding a new business type

1. Create `seeds/{type}/` directory
2. Add CSV files — each file becomes a seed table
3. CSV header row = column names, data rows = seed values
4. Tables in `{type}/` override same-named tables from `general/`
5. Update `embed.go` to include the new directory:

```go
//go:embed seeds/{type}/*.csv
```

## Data format

All seed data is stored as CSV:

- First row is the header (column names matching the database schema)
- Empty values or `NULL` are treated as SQL NULL in generated INSERTs
- SQL output uses `ON CONFLICT (id) DO NOTHING` for idempotency
