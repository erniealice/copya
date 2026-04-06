# fixtures/

Assertion-grade test data for E2E report testing. These CSVs are loaded before a
test suite and cleared after — they are **not** bootstrap seed data.

The key difference from seeds:

| | Seeds (`../seeds/`) | Fixtures (here) |
|---|---|---|
| Purpose | Make the app work | Assert exact report totals |
| Loaded by | `cmd/setup --drop` | `cmd/fixtures load <suite>` |
| Cleared by | `cmd/setup --drop` (full reset) | `cmd/fixtures clear <suite>` |
| IDs | Stable names (`client-001`) | `e2e-*` prefix |
| Cascade | 3-tier: common→general→type | 2-tier: common→type |

## Structure

```
fixtures/
  revenue-reporting/
    common/                    ← shared across all business types
      location_area.csv
    professional/              ← professional-specific data
      revenue.csv
      revenue_line_item.csv
  expense-reporting/
    common/
      location_area.csv
    professional/
      expenditure.csv
      expenditure_line_item.csv
      purchase_order.csv
      purchase_order_line_item.csv
  disbursement-reporting/
    common/
      location_area.csv
      disbursement_method.csv
    professional/
      treasury_disbursement.csv
      disbursement_schedule.csv
```

## ID convention

All fixture rows use the `e2e-*` prefix:

```
e2e-rev-001    revenue
e2e-rli-001    revenue_line_item
e2e-exp-001    expenditure
e2e-eli-001    expenditure_line_item
e2e-po-001     purchase_order
e2e-poli-001   purchase_order_line_item
e2e-td-001     treasury_disbursement
e2e-ds-001     disbursement_schedule
e2e-la-*       location_area
e2e-dm-*       disbursement_method
```

The prefix keeps fixture rows visually distinct from seed data and allows
`ClearFixtures` to delete precisely by ID (reads IDs from these CSVs, deletes
`WHERE id = $1`) rather than using a wildcard.

## Known totals (professional)

| Suite | Active records | Grand total | Notes |
|-------|---------------|-------------|-------|
| revenue-reporting | 22 revenues, 39 line items | ~PHP 385,500 | 3 cancelled excluded |
| expense-reporting | 21 expenditures, 47 line items | ~PHP 431,800 | 4 excluded-status records |
| disbursement-reporting | 18 paid, 4 overdue, 1 pending, 2 cancelled | ~PHP 348,000 paid | 10 schedule records |

Amounts are stored in centavos (÷100 for display).

## Loading and clearing

```bash
# From apps/service-admin
go run ./cmd/fixtures load revenue-reporting
go run ./cmd/fixtures clear revenue-reporting
go run ./cmd/fixtures clear --all
```

Load is idempotent (`ON CONFLICT (id) DO NOTHING`). Clear deletes rows in reverse
`InsertOrder` to avoid FK violations.

## Adding a new suite

1. Create `{suite-name}/common/` and `{suite-name}/professional/` (and other business types as needed)
2. Add CSV files; **first column must be `id`** with `e2e-*` values
3. Add the embed glob in `../fixtures.go`
4. Add the suite to `allSuites` in `apps/service-admin/cmd/fixtures/main.go`
5. Add any new tables to `InsertOrder` in `../golang/v1/types.go` at the correct FK level
