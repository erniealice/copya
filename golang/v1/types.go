package copya

// Dialect controls SQL syntax generation.
type Dialect string

const (
	Postgres Dialect = "postgres"
	MySQL    Dialect = "mysql"
)

// SeedTable represents a single CSV-backed seed table with headers and rows.
type SeedTable struct {
	Name    string     // file stem, e.g. "product"
	Headers []string   // column names from CSV header row
	Rows    [][]string // data rows (same order as Headers)
}

// SeedSet is the merged result for a business type: table name → SeedTable.
type SeedSet map[string]*SeedTable

// InsertOrder defines the dependency-safe insertion order for a full seed.
// Tables are listed from least-dependent to most-dependent.
var InsertOrder = []string{
	// Level 0: no foreign keys
	"user",
	"attribute",
	"attribute_value",
	"category",
	"group",
	"location",
	"payment_method",
	"collection_method",
	"disbursement_method",
	"event_recurrence",
	"account_group",
	"expenditure_category",
	"revenue_category",
	"location_area",

	// Level 1: depend on user or base tables
	"admin",
	"delegate",
	"staff",
	"workspace",
	"plan",
	"product",
	"collection",
	"event",
	"fiscal_period",
	"account",
	"asset_category",

	// Level 2: depend on level 1
	"workspace_user",
	"role",
	"permission",
	"client",
	"supplier_category",
	"supplier",
	"product_variant",
	"product_attribute",
	"inventory_item",

	// Level 3: depend on level 2
	"workspace_user_role",
	"role_permission",
	"client_category",
	"client_attribute",
	"delegate_client",
	"supplier_attribute",
	"location_attribute",
	"group_attribute",
	"staff_attribute",
	"delegate_attribute",
	"collection_attribute",
	"collection_parent",
	"collection_plan",
	"plan_attribute",
	"plan_location",
	"product_collection",
	"product_plan",
	"price_product",
	"price_plan",
	"inventory_serial",
	"inventory_attribute",
	"purchase_order",
	"purchase_order_line_item",

	// Level 4: transactional
	"subscription",
	"subscription_attribute",
	"revenue",
	"revenue_line_item",
	"revenue_payment",
	"expenditure",
	"expenditure_line_item",
	"treasury_collection",
	"treasury_disbursement",
	"disbursement_schedule",
	"journal_entry",
	"journal_line",
	"asset",
	"depreciation_schedule",
	"asset_maintenance",

	// Level 5: operations
	"job_template",
	"job_template_phase",
	"job_template_task",
	"job",
	"job_phase",
	"job_task",
	"job_activity",

	// Level 6: fulfillment
	"fulfillment",
	"fulfillment_item",
}
