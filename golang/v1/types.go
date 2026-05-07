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
	// job_template must be inserted before plan because plan.job_template_id
	// is a FK reference to job_template(id) (auto-spawn-jobs-from-subscription).
	"job_template",
	"job_template_phase",
	"job_template_task",
	"job_template_relation",
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
	"price_schedule",
	"price_plan",
	"inventory_serial",
	"inventory_attribute",
	"supplier_contract",
	"supplier_contract_line",
	// Supplier-pricing-symmetry (SPS) schedule layer — date-windowed
	// pricing scoped to a supplier_contract. Mirrors the sales-side
	// price_schedule + price_plan pair. Schedule must precede its line
	// (Line FKs to both schedule and supplier_contract_line).
	"supplier_contract_price_schedule",
	"supplier_contract_price_schedule_line",
	// Procurement domain (supplier-subscriptions plan P4):
	// Level 3a — no FK dependencies on each other; both are top-level
	// procurement graph roots.
	"cost_schedule",
	"supplier_plan",
	// Level 3b — cost_plan FKs to supplier_plan + cost_schedule;
	// supplier_product_plan FKs to supplier_plan + product + product_variant.
	"cost_plan",
	"supplier_product_plan",
	// Level 3c — supplier_product_cost_plan FKs to cost_plan + supplier_product_plan.
	"supplier_product_cost_plan",
	"procurement_request",
	"procurement_request_line",
	"purchase_order",
	"purchase_order_line_item",

	// Level 4: transactional
	"subscription",
	"subscription_attribute",
	// supplier_subscription FKs to supplier + cost_plan + procurement_request + location.
	// Placed adjacent to selling-side subscription for symmetry.
	"supplier_subscription",
	"revenue",
	"revenue_line_item",
	"revenue_payment",
	"expenditure",
	"expenditure_line_item",
	// SPS recognition + accrual layer. accrued_expense FKs to
	// supplier_contract only, so it sits at the top of Level 4.
	// expense_recognition FKs to expenditure + supplier_contract +
	// accrued_expense, so it must follow all three. Lines + settlement
	// follow their parents (settlement also FKs to expenditure).
	"accrued_expense",
	"expense_recognition",
	"expense_recognition_line",
	"accrued_expense_settlement",
	"treasury_collection",
	"treasury_disbursement",
	"disbursement_schedule",
	"journal_entry",
	"journal_line",
	"asset",
	"depreciation_schedule",
	"asset_maintenance",

	// Level 5: operations
	// job_template / job_template_phase / job_template_task / job_template_relation
	// were promoted to Level 1 (above) so plan.job_template_id can resolve at
	// insert time.
	// product_price_plan FK references job_template_phase (milestone billing),
	// so it must be inserted after job_template_phase even though it's a
	// pricing-graph entity (Level 3).
	"product_price_plan",
	"job",
	"job_phase",
	"job_task",
	"job_activity",

	// Level 6: fulfillment
	"fulfillment",
	"fulfillment_item",

	// Level 7: payroll policy (jurisdiction overlay)
	"rate_table",
	"rate_band",
	"leave_type",
	"leave_balance",
	"deduction_schedule_rule",
	"pay_cycle",
	"leave_request",
	"supplier_dependent",
	"supplier_lifecycle_event",
}
