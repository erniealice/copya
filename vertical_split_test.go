package copya_test

import (
	"reflect"
	"sort"
	"testing"

	copya "github.com/erniealice/copya"
	v1 "github.com/erniealice/copya/golang/v1"
)

func TestLeasingSeedSetsAreDistinctAndClosed(t *testing.T) {
	provider := v1.NewSeedProvider(copya.SeedsFS)
	property, err := provider.Load("leasing")
	if err != nil {
		t.Fatal(err)
	}
	equipment, err := provider.Load("equipment_leasing")
	if err != nil {
		t.Fatal(err)
	}

	assertSeedGraph(t, property, []string{
		"Commercial Lease",
		"Residential Lease",
	}, []string{
		"Emergency Property Repair",
		"Move-In Inspection",
		"Move-Out Inspection",
		"Routine Property Maintenance",
	})
	assertSeedGraph(t, equipment, []string{
		"Fixed-Term Equipment Lease",
		"Short-Term Equipment Lease",
	}, []string{
		"Equipment Deployment",
		"Equipment Preventive Maintenance",
		"Equipment Repair",
		"Equipment Return Inspection",
	})

	if reflect.DeepEqual(property["job_template"].Rows, equipment["job_template"].Rows) {
		t.Fatal("property and equipment leasing must not share one job-template set")
	}
	if !reflect.DeepEqual(property["permission"], equipment["permission"]) {
		t.Fatal("leasing vertical split must preserve the canonical permission catalog")
	}
}

func assertSeedGraph(t *testing.T, set v1.SeedSet, wantPlans, wantTemplates []string) {
	t.Helper()
	templates := set["job_template"]
	phases := set["job_template_phase"]
	tasks := set["job_template_task"]
	plans := set["plan"]
	if templates == nil || phases == nil || tasks == nil || plans == nil {
		t.Fatal("vertical seed graph is missing a required table")
	}

	templateIDs := columnSet(t, templates, "id")
	phaseIDs := columnSet(t, phases, "id")
	for _, value := range columnValues(t, phases, "job_template_id") {
		if !templateIDs[value] {
			t.Fatalf("phase references unknown job template %q", value)
		}
	}
	for _, value := range columnValues(t, tasks, "job_template_phase_id") {
		if !phaseIDs[value] {
			t.Fatalf("task references unknown job template phase %q", value)
		}
	}
	for _, value := range columnValues(t, plans, "job_template_id") {
		if !templateIDs[value] {
			t.Fatalf("plan references unknown job template %q", value)
		}
	}

	gotPlans := columnValues(t, plans, "name")
	gotTemplates := columnValues(t, templates, "name")
	sort.Strings(gotPlans)
	sort.Strings(gotTemplates)
	if !reflect.DeepEqual(gotPlans, wantPlans) {
		t.Fatalf("plan names = %v, want %v", gotPlans, wantPlans)
	}
	if !reflect.DeepEqual(gotTemplates, wantTemplates) {
		t.Fatalf("job template names = %v, want %v", gotTemplates, wantTemplates)
	}
}

func columnSet(t *testing.T, table *v1.SeedTable, name string) map[string]bool {
	t.Helper()
	result := make(map[string]bool)
	for _, value := range columnValues(t, table, name) {
		if result[value] {
			t.Fatalf("duplicate %s value %q in %s", name, value, table.Name)
		}
		result[value] = true
	}
	return result
}

func columnValues(t *testing.T, table *v1.SeedTable, name string) []string {
	t.Helper()
	index := -1
	for candidate, header := range table.Headers {
		if header == name {
			index = candidate
			break
		}
	}
	if index < 0 {
		t.Fatalf("%s has no %s column", table.Name, name)
	}
	result := make([]string, 0, len(table.Rows))
	for _, row := range table.Rows {
		if len(row) != len(table.Headers) {
			t.Fatalf("%s row has %d values, want %d", table.Name, len(row), len(table.Headers))
		}
		result = append(result, row[index])
	}
	return result
}
