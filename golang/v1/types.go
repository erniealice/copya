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
	// Tax lookup tables with no FKs — must precede tax_registration_kind,
	// tax_class, and product (which FK to tax_treatment + tax_class).
	"tax_authority",
	"tax_treatment",

	// Level 1: depend on user or base tables
	"admin",
	"delegate",
	"staff",
	"workspace",
	// Tax classification tables that FK to tax_authority — must precede
	// product (product.withholding_class_id FKs to tax_class, and
	// product.tax_treatment_id FKs to tax_treatment which is Level 0 above).
	"tax_registration_kind",
	"tax_class",
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
	// work_request_type: catalog FKs to workspace only; must precede
	// work_request (work_request.work_request_type_id FK).
	"work_request_type",
	"supplier_category",
	"supplier",
	"product_variant",
	"product_attribute",
	"inventory_item",
	// Tax rate + registration: FK to workspace (Level 1), tax_authority,
	// tax_registration_kind, and client (polymorphic party_id — no DB FK).
	// tax_rate must precede tax_registration for correctness; both go at
	// Level 2 after client.
	"tax_rate",
	"tax_registration",

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
	// work_request: FKs to work_request_type (Level 2), client, user,
	// workspace_user, and optionally to subscription, subscription_seat,
	// workflow, and job. Placed after job (Level 5) to satisfy all FKs.
	"work_request",

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

	// Level 8: communication domain (Plan-4 2026-06-03)
	// FK order: conversation before all three children; conversation_post before
	// conversation_read_receipt (receipt.last_read_post_id FKs to conversation_post).
	"conversation",              // FKs: workspace, client, user (assigned_to, created_by)
	"conversation_post",         // FKs: conversation, workspace, client, user (sender)
	"conversation_read_receipt", // FKs: conversation, user, conversation_post (last_read)
	"conversation_participant",  // FKs: conversation, workspace, user [v2 seam — no seeds in v1]

	// Level 9: evaluation domain (20260604-performance-evaluation)
	// outcome_criteria has no entity-domain FKs (workspace_id is optional) — seeds after
	// workspace but logically belongs before templates that reference it.
	"outcome_criteria",           // FKs: workspace (opt), outcome_criteria self (supersedes/overrides)
	// Templates must precede evaluations (evaluation.evaluation_template_id FK).
	"evaluation_template",        // FKs: workspace, evaluation_template self (copied_from)
	"evaluation_template_item",   // FKs: evaluation_template, workspace, outcome_criteria
	// Cycles must precede evaluations that stamp evaluation_cycle_id.
	"evaluation_cycle",           // FKs: workspace, subscription
	// Evaluation header: FKs to workspace, client, subscription, subscription_seat,
	// evaluation_template, workspace_user (evaluator/signoff arcs), staff (subject),
	// evaluation_cycle (nullable).
	"evaluation",                 // FKs: workspace, client, subscription (opt), subscription_seat (opt),
	                              //      evaluation_template (opt), workspace_user (opt), staff (opt),
	                              //      evaluation_cycle (opt)
	// Response: FKs to evaluation + workspace + outcome_criteria.
	"evaluation_response",        // FKs: evaluation, workspace, outcome_criteria
	// Cycle members: FKs to evaluation_cycle + workspace + client + staff.
	"evaluation_cycle_member",    // FKs: evaluation_cycle, workspace, client, staff
}
