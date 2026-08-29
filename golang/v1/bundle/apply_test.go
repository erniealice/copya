package bundle

import (
	"strings"
	"testing"
)

const validManifest = `{
  "format_version": 1,
  "id": "gpagoda-base",
  "version": "2026.08.1",
  "schema_release": "postgres/2026.08.1",
  "profile": "client-minimal",
  "business_type": "leasing",
  "workspace": {"id":"workspace-gpagoda","name":"gpagoda","slug":"gpagoda","description":"local","timezone":"Australia/Sydney"},
  "user": {"id":"superadmin-001","first_name":"Super","last_name":"Admin","email_address":"admin@example.test","mobile_number":"+61400000000","timezone":"Australia/Sydney","password_env":"DB_INIT_ADMIN_PASSWORD"},
  "admin_id": "admin-gpagoda-superadmin",
  "workspace_user_id": "workspace-user-gpagoda-superadmin",
  "role": {"id":"role-gpagoda-super-admin","name":"Super Admin","description":"all grants","color":"#dc2626"},
  "workspace_user_role_id": "workspace-user-role-gpagoda-superadmin"
}`

func TestManifestAndPlan(t *testing.T) {
	manifest, err := Parse([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(manifest, Digest([]byte(validManifest)))
	if err != nil {
		t.Fatal(err)
	}
	if plan.PermissionRows < 1 {
		t.Fatal("expected active Copya permissions")
	}
	permissions, err := loadActivePermissions("leasing")
	if err != nil {
		t.Fatal(err)
	}
	foundHomeGrant := false
	for _, permission := range permissions {
		if permission.Code == "outcome_completion:read" {
			foundHomeGrant = true
		}
	}
	if !foundHomeGrant {
		t.Fatal("required home permission outcome_completion:read is absent")
	}
}

func TestManifestRejectsUnknownAndSecretFields(t *testing.T) {
	for _, raw := range []string{
		strings.Replace(validManifest, `"format_version": 1,`, `"format_version": 1, "unknown": true,`, 1),
		strings.Replace(validManifest, `"password_env":"DB_INIT_ADMIN_PASSWORD"`, `"password_env":"DB_INIT_ADMIN_PASSWORD", "password":"tracked-secret"`, 1),
	} {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Fatal("expected strict manifest rejection")
		}
	}
}

func TestReleaseCompatibilityIsExplicit(t *testing.T) {
	manifest, err := Parse([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaRelease != "postgres/2026.08.1" || manifest.Profile != "client-minimal" {
		t.Fatalf("unexpected compatibility contract: %+v", manifest)
	}
	if rolePermissionID(manifest.Role.ID, "p1") != rolePermissionID(manifest.Role.ID, "p1") {
		t.Fatal("role permission IDs must be deterministic")
	}
}
