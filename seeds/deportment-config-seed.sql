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
-- (Days Present / School Days) are deliberately UNSCOPED — attendance is a
-- ledger materialized as job_outcome_line rows, never a graded band. Per the
-- D-B decision (OR-4) absent is DERIVED (school_days - present) at load time,
-- so days-absent / tardies are not seeded as source criteria here.
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

-- 2. outcome_criteria: conduct x2 (scored) + attendance x2 (ledger, unscoped).
--    Per D-B (OR-4) absent is derived, so days-absent/tardies are not seeded.
INSERT INTO outcome_criteria
  (id, criteria_group_id, version, version_status, scope, workspace_id, name, description,
   criteria_type, min_score, max_score, score_increment, aggregation_method, weight, required, active, created_by, date_created, date_modified)
VALUES
  ('seed-deportment-oc-conduct', 'seed-deportment-oc-conduct-grp', 1, 'VERSION_STATUS_PUBLISHED', 'CRITERIA_SCOPE_WORKSPACE', :'ws', 'Conduct', 'Deportment / conduct',
   'CRITERIA_TYPE_NUMERIC_SCORE', 0, 100, 0.01, 'AGGREGATION_METHOD_MAXIMUM', 1.0, true, true, 'system', now(), now()),
  ('seed-homeroom-oc-conduct', 'seed-homeroom-oc-conduct-grp', 1, 'VERSION_STATUS_PUBLISHED', 'CRITERIA_SCOPE_WORKSPACE', :'ws', 'Conduct', 'Homeroom / conduct',
   'CRITERIA_TYPE_NUMERIC_SCORE', 0, 100, 0.01, 'AGGREGATION_METHOD_MAXIMUM', 1.0, true, true, 'system', now(), now()),
  ('seed-homeroom-oc-days-present', 'seed-homeroom-oc-days-present-grp', 1, 'VERSION_STATUS_PUBLISHED', 'CRITERIA_SCOPE_WORKSPACE', :'ws', 'Days Present', 'Homeroom attendance / days present',
   'CRITERIA_TYPE_NUMERIC_SCORE', 0, 31, 0.5, 'AGGREGATION_METHOD_INDIVIDUAL', 1.0, true, true, 'system', now(), now()),
  ('seed-homeroom-oc-school-days', 'seed-homeroom-oc-school-days-grp', 1, 'VERSION_STATUS_PUBLISHED', 'CRITERIA_SCOPE_WORKSPACE', :'ws', 'School Days', 'Homeroom attendance / school days (denominator)',
   'CRITERIA_TYPE_NUMERIC_SCORE', 0, 31, 1, 'AGGREGATION_METHOD_LATEST', 1.0, true, true, 'system', now(), now())
ON CONFLICT (id) DO NOTHING;

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
