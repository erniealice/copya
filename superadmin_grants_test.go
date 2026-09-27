package copya_test

import (
	"encoding/csv"
	"testing"

	copya "github.com/erniealice/copya"
)

// TestSuperAdminRoleHoldsEveryActivePermission keeps the seeded super-admin
// role ("role-admin", "Full system access" in seeds/common/role.csv) in step
// with the permission catalog. Every new active row in
// seeds/general/permission.csv needs a matching role-admin grant in
// seeds/general/role_permission.csv, or a fresh install's super-admin is
// denied the new feature. Grants were found missing for 95 codes on 2026-09-27
// (docs/wiki/articles/data-seeder-guide.md).
func TestSuperAdminRoleHoldsEveryActivePermission(t *testing.T) {
	permissions := readSeedCSV(t, "seeds/general/permission.csv")
	grants := readSeedCSV(t, "seeds/general/role_permission.csv")

	granted := make(map[string]bool)
	for _, grant := range grants {
		if grant["role_id"] == "role-admin" && grant["active"] == "true" {
			granted[grant["permission_id"]] = true
		}
	}
	seenCodes := make(map[string]string)
	for _, permission := range permissions {
		if permission["active"] != "true" {
			continue
		}
		key := permission["permission_code"] + "|" + permission["permission_type"]
		if previous, ok := seenCodes[key]; ok {
			t.Errorf("permission %s duplicates %s (%s)", permission["id"], previous, key)
		}
		seenCodes[key] = permission["id"]
		if !granted[permission["id"]] {
			t.Errorf("active permission %s (%s) has no role-admin grant; add rp-role-admin-%s to seeds/general/role_permission.csv",
				permission["id"], permission["permission_code"], permission["id"])
		}
	}
}

func readSeedCSV(t *testing.T, path string) []map[string]string {
	t.Helper()
	file, err := copya.SeedsFS.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if len(records) == 0 {
		t.Fatalf("%s: empty", path)
	}
	rows := make([]map[string]string, 0, len(records)-1)
	for _, record := range records[1:] {
		row := make(map[string]string, len(records[0]))
		for index, header := range records[0] {
			row[header] = record[index]
		}
		rows = append(rows, row)
	}
	return rows
}
