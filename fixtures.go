package copya

import "embed"

//go:embed fixtures/revenue-reporting fixtures/expense-reporting fixtures/disbursement-reporting
var FixturesFS embed.FS
