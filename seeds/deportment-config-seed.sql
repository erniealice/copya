-- =============================================================================
-- Deportment scoring-config seed (education1) — the two conduct/formation
-- scoring graphs for job_category codes subject_deportment + homeroom_deportment.
-- Mirrors the academic scoring-config-seed.sql / config-junction-seed.sql
-- convention (protojson SCREAMING enum names, TEXT columns, idempotent
-- ON CONFLICT (id) DO NOTHING). Apply AFTER the academic seeds.
--
-- Design source: docs/plan/20260714-job-category-primitive/research/
--   deportment-scoring-model.md (DM§1-§3).
--
-- LOAD-BEARING: both schemes carry score_scale_id = NULL (D-A A1 "no band").
-- Deportment therefore does NOT run through ComputePhaseOutcome/ComputeJobOutcome
-- (they fail-loud without a scale); the loader direct-writes the summaries. The
-- config rows still exist to (a) bind the grade-entry grid, (b) give the resolver
-- a scheme to snapshot, (c) keep the graph shape identical to academic.
--
-- Only CONDUCT is scoped (scoring_component_criteria). The attendance criteria
-- (Days Present / School Days / Times Tardy) are deliberately UNSCOPED —
-- attendance is a ledger materialized as job_outcome_line rows, never a graded
-- band. Days-absent stays DERIVED (school_days - present) at load time and is
-- not a source criterion; Times Tardy IS a source criterion (entered live
-- through the grade grid), bound to the monthly tasks by the grade loader.
--
-- Run with:  psql -d education1 -v ON_ERROR_STOP=1 -f deportment-config-seed.sql
-- =============================================================================

\set ws '019ecb8e-d83f-74ab-aa13-5a6c27afd112'

-- 1. scoring_scheme x2 (SUM composite, NO scale = no band). ------------------
INSERT INTO scoring_scheme
  (id, workspace_id, active, date_created, date_modified, scheme_group_id, version, version_status, name, composite_method, score_scale_id, weights_must_sum_to_one, rounding_mode)
VALUES
  ('seed-deportment-scheme', :'ws', true, (extract(epoch from now())*1000)::bigint, (extract(epoch from now())*1000)::bigint,
   'seed-deportment-scheme-grp', 1, 'VERSION_STATUS_PUBLISHED', 'Deportment (Conduct)', 'SCORING_METHOD_SUM', NULL, false, 'ROUNDING_MODE_HALF_UP'),
  ('seed-homeroom-scheme', :'ws', true, (extract(epoch from now())*1000)::bigint, (extract(epoch from now())*1000)::bigint,
   'seed-homeroom-scheme-grp', 1, 'VERSION_STATUS_PUBLISHED', 'Homeroom (Conduct + Attendance)', 'SCORING_METHOD_SUM', NULL, false, 'ROUNDING_MODE_HALF_UP')
ON CONFLICT (id) DO NOTHING;

-- 2. outcome_criteria: conduct x2 (scored) + attendance x3 (ledger, unscoped).
--    Days-absent stays derived (not seeded); Times Tardy is a live-entry source
--    criterion (integer 0..31, latest-wins). The homeroom criteria carry the
--    stable machine `code` the generic document template pivots on; the
--    subject-deportment conduct criterion stays uncoded.
--
--    Repair-upsert (NOT DO NOTHING): a pre-existing environment that predates
--    the code column would otherwise keep NULL codes and drift the render path
--    blank. This fills a NULL code (never overwriting an established one — code
--    is the stable map key) and reconciles the config fields this wave fixed
--    (aggregation_method + decimal_places, e.g. the Times-Tardy latest-wins /
--    integer row). Change-guarded so a re-seed is a clean no-op.
INSERT INTO outcome_criteria
  (id, criteria_group_id, version, version_status, scope, workspace_id, name, description,
   criteria_type, decimal_places, min_score, max_score, score_increment, aggregation_method, weight, required, active, created_by, date_created, date_modified, code)
VALUES
  ('seed-deportment-oc-conduct', 'seed-deportment-oc-conduct-grp', 1, 'VERSION_STATUS_PUBLISHED', 'CRITERIA_SCOPE_WORKSPACE', :'ws', 'Conduct', 'Deportment / conduct',
   'CRITERIA_TYPE_NUMERIC_SCORE', NULL, 0, 100, 0.01, 'AGGREGATION_METHOD_MAXIMUM', 1.0, true, true, 'system', now(), now(), NULL),
  ('seed-homeroom-oc-conduct', 'seed-homeroom-oc-conduct-grp', 1, 'VERSION_STATUS_PUBLISHED', 'CRITERIA_SCOPE_WORKSPACE', :'ws', 'Conduct', 'Homeroom / conduct',
   'CRITERIA_TYPE_NUMERIC_SCORE', NULL, 0, 100, 0.01, 'AGGREGATION_METHOD_MAXIMUM', 1.0, true, true, 'system', now(), now(), 'conduct'),
  ('seed-homeroom-oc-days-present', 'seed-homeroom-oc-days-present-grp', 1, 'VERSION_STATUS_PUBLISHED', 'CRITERIA_SCOPE_WORKSPACE', :'ws', 'Days Present', 'Homeroom attendance / days present',
   'CRITERIA_TYPE_NUMERIC_SCORE', NULL, 0, 31, 0.5, 'AGGREGATION_METHOD_INDIVIDUAL', 1.0, true, true, 'system', now(), now(), 'days_present'),
  ('seed-homeroom-oc-school-days', 'seed-homeroom-oc-school-days-grp', 1, 'VERSION_STATUS_PUBLISHED', 'CRITERIA_SCOPE_WORKSPACE', :'ws', 'School Days', 'Homeroom attendance / school days (denominator)',
   'CRITERIA_TYPE_NUMERIC_SCORE', NULL, 0, 31, 1, 'AGGREGATION_METHOD_LATEST', 1.0, true, true, 'system', now(), now(), 'school_days'),
  ('seed-homeroom-oc-times-tardy', 'seed-homeroom-oc-times-tardy-grp', 1, 'VERSION_STATUS_PUBLISHED', 'CRITERIA_SCOPE_WORKSPACE', :'ws', 'Times Tardy', 'Homeroom attendance / times tardy',
   'CRITERIA_TYPE_NUMERIC_SCORE', 0, 0, 31, 1, 'AGGREGATION_METHOD_LATEST', 1.0, true, true, 'system', now(), now(), 'times_tardy')
ON CONFLICT (id) DO UPDATE SET
  code               = COALESCE(outcome_criteria.code, EXCLUDED.code),
  aggregation_method = EXCLUDED.aggregation_method,
  decimal_places     = EXCLUDED.decimal_places,
  date_modified      = now()
WHERE outcome_criteria.code               IS DISTINCT FROM COALESCE(outcome_criteria.code, EXCLUDED.code)
   OR outcome_criteria.aggregation_method IS DISTINCT FROM EXCLUDED.aggregation_method
   OR outcome_criteria.decimal_places     IS DISTINCT FROM EXCLUDED.decimal_places;

-- 3. scoring_component x2 (conduct only). -----------------------------------
INSERT INTO scoring_component
  (id, scoring_scheme_id, code, label, weight, sequence_order, active, date_created, date_modified)
VALUES
  ('seed-deportment-sc-conduct', 'seed-deportment-scheme', 'conduct', 'Conduct', 1.0, 1, true, (extract(epoch from now())*1000)::bigint, (extract(epoch from now())*1000)::bigint),
  ('seed-homeroom-sc-conduct', 'seed-homeroom-scheme', 'conduct', 'Conduct', 1.0, 1, true, (extract(epoch from now())*1000)::bigint, (extract(epoch from now())*1000)::bigint)
ON CONFLICT (id) DO NOTHING;

-- 4. scoring_component_criteria x2 — bind ONLY conduct into each scheme. -----
INSERT INTO scoring_component_criteria
  (id, scoring_scheme_id, scoring_component_id, outcome_criteria_id, active, workspace_id, date_created, date_modified)
VALUES
  ('seed-deportment-scc-conduct', 'seed-deportment-scheme', 'seed-deportment-sc-conduct', 'seed-deportment-oc-conduct', true, :'ws', (extract(epoch from now())*1000)::bigint, (extract(epoch from now())*1000)::bigint),
  ('seed-homeroom-scc-conduct', 'seed-homeroom-scheme', 'seed-homeroom-sc-conduct', 'seed-homeroom-oc-conduct', true, :'ws', (extract(epoch from now())*1000)::bigint, (extract(epoch from now())*1000)::bigint)
ON CONFLICT (id) DO NOTHING;

-- 5. score_scale / score_scale_band = 0 rows (A1 no band; attendance never
--    banded). Add only under A2 (deportment-scoring-model.md §5) iff the printed
--    card later shows a conduct letter.

-- 6. template_task_criteria bindings (conduct + monthly days-present /
--    school-days / times-tardy) live in the grade loader, NOT here: they attach
--    to the per-section monthly tasks the loader creates, which do not exist at
--    config-seed time. This file seeds only the reusable scoring config.
