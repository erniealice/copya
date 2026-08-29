package bundle

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	rootcopya "github.com/erniealice/copya"
	copya "github.com/erniealice/copya/golang/v1"
	"golang.org/x/crypto/bcrypt"
)

type Plan struct {
	BundleID       string
	BundleVersion  string
	SchemaRelease  string
	Profile        string
	BusinessType   string
	WorkspaceID    string
	WorkspaceSlug  string
	PermissionRows int
	Digest         string
}

type Result struct {
	Plan
	TargetKey string
	Applied   bool
	NoOp      bool
}

type permissionRow struct {
	ID                       string
	Name                     string
	Code                     string
	Type                     string
	ApplicablePrincipalTypes string
}

func BuildPlan(manifest Manifest, digest string) (Plan, error) {
	if err := manifest.Validate(); err != nil {
		return Plan{}, err
	}
	if len(digest) != 64 {
		return Plan{}, errors.New("copya bundle: invalid manifest digest")
	}
	permissions, err := loadActivePermissions(manifest.BusinessType)
	if err != nil {
		return Plan{}, err
	}
	return Plan{
		BundleID:       manifest.ID,
		BundleVersion:  manifest.Version,
		SchemaRelease:  manifest.SchemaRelease,
		Profile:        manifest.Profile,
		BusinessType:   manifest.BusinessType,
		WorkspaceID:    manifest.Workspace.ID,
		WorkspaceSlug:  manifest.Workspace.Slug,
		PermissionRows: len(permissions),
		Digest:         digest,
	}, nil
}

func Apply(ctx context.Context, db *sql.DB, targetKey, requiredRelease, password, digest string, manifest Manifest) (Result, error) {
	plan, err := BuildPlan(manifest, digest)
	if err != nil {
		return Result{}, err
	}
	result := Result{Plan: plan, TargetKey: targetKey}
	if db == nil {
		return result, errors.New("copya bundle: nil database")
	}
	if !strings.Contains(targetKey, "/") {
		return result, fmt.Errorf("copya bundle: invalid target key %q", targetKey)
	}
	if manifest.SchemaRelease != requiredRelease {
		return result, fmt.Errorf("copya bundle: manifest release %s does not match required %s", manifest.SchemaRelease, requiredRelease)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("copya bundle: begin: %w", err)
	}
	defer tx.Rollback()

	state, err := receiptState(ctx, tx, targetKey, manifest)
	if err != nil {
		return result, err
	}
	if state.exists {
		if state.digest != digest || state.schemaRelease != manifest.SchemaRelease || state.businessType != manifest.BusinessType || state.workspaceID != manifest.Workspace.ID {
			return result, fmt.Errorf("copya bundle: receipt conflict for %s/%s", manifest.ID, manifest.Version)
		}
		if err := verifyRows(ctx, tx, manifest, plan.PermissionRows); err != nil {
			return result, fmt.Errorf("copya bundle: receipt exists but data verification failed: %w", err)
		}
		result.NoOp = true
		return result, nil
	}
	if password == "" {
		return result, fmt.Errorf("copya bundle: %s is empty", manifest.User.PasswordEnv)
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return result, fmt.Errorf("copya bundle: hash password: %w", err)
	}
	permissions, err := loadActivePermissions(manifest.BusinessType)
	if err != nil {
		return result, err
	}

	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO public."user" (id, first_name, last_name, email_address, mobile_number, password_hash, active, timezone) VALUES ($1,$2,$3,$4,$5,$6,true,$7)`, []any{manifest.User.ID, manifest.User.FirstName, manifest.User.LastName, manifest.User.EmailAddress, manifest.User.MobileNumber, string(passwordHash), manifest.User.Timezone}},
		{`INSERT INTO public.workspace (id, name, description, private, active, timezone, slug) VALUES ($1,$2,$3,false,true,$4,$5)`, []any{manifest.Workspace.ID, manifest.Workspace.Name, manifest.Workspace.Description, manifest.Workspace.Timezone, manifest.Workspace.Slug}},
		{`INSERT INTO public.admin (id, user_id, active) VALUES ($1,$2,true)`, []any{manifest.AdminID, manifest.User.ID}},
		{`INSERT INTO public.workspace_user (id, workspace_id, user_id, active, member_status) VALUES ($1,$2,$3,true,'active')`, []any{manifest.WorkspaceUserID, manifest.Workspace.ID, manifest.User.ID}},
		{`INSERT INTO public.role (id, workspace_id, name, description, color, active, applicable_principal_types) VALUES ($1,$2,$3,$4,$5,true,'{1,2}'::integer[])`, []any{manifest.Role.ID, manifest.Workspace.ID, manifest.Role.Name, manifest.Role.Description, manifest.Role.Color}},
		{`INSERT INTO public.workspace_user_role (id, workspace_user_id, role_id, active) VALUES ($1,$2,$3,true)`, []any{manifest.WorkspaceUserRoleID, manifest.WorkspaceUserID, manifest.Role.ID}},
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			return result, fmt.Errorf("copya bundle: insert identity graph: %w", err)
		}
	}

	for _, permission := range permissions {
		permissionID := manifest.Workspace.Slug + "-" + permission.ID
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO public.permission
				(id, workspace_id, user_id, granted_by_user_id, name, permission_code, permission_type, active, applicable_principal_types)
			VALUES ($1,$2,$3,$3,$4,$5,$6,true,$7::integer[])`,
			permissionID, manifest.Workspace.ID, manifest.User.ID, permission.Name, permission.Code, permission.Type, permission.ApplicablePrincipalTypes); err != nil {
			return result, fmt.Errorf("copya bundle: insert permission %s: %w", permission.Code, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO public.role_permission (id, role_id, permission_id, permission_type, active)
			VALUES ($1,$2,$3,$4,true)`, rolePermissionID(manifest.Role.ID, permissionID), manifest.Role.ID, permissionID, permission.Type); err != nil {
			return result, fmt.Errorf("copya bundle: grant permission %s: %w", permission.Code, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO ichizen_deploy.data_bundle_receipts
			(target_key, bundle_id, bundle_version, bundle_digest, schema_release, business_type, workspace_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		targetKey, manifest.ID, manifest.Version, digest, manifest.SchemaRelease, manifest.BusinessType, manifest.Workspace.ID); err != nil {
		return result, fmt.Errorf("copya bundle: insert receipt: %w", err)
	}
	if err := verifyRows(ctx, tx, manifest, len(permissions)); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("copya bundle: commit: %w", err)
	}
	result.Applied = true
	return result, nil
}

type existingReceipt struct {
	exists        bool
	digest        string
	schemaRelease string
	businessType  string
	workspaceID   string
}

func receiptState(ctx context.Context, tx *sql.Tx, targetKey string, manifest Manifest) (existingReceipt, error) {
	var state existingReceipt
	err := tx.QueryRowContext(ctx, `
		SELECT bundle_digest, schema_release, business_type, workspace_id
		FROM ichizen_deploy.data_bundle_receipts
		WHERE target_key=$1 AND bundle_id=$2 AND bundle_version=$3`, targetKey, manifest.ID, manifest.Version).
		Scan(&state.digest, &state.schemaRelease, &state.businessType, &state.workspaceID)
	if errors.Is(err, sql.ErrNoRows) {
		return state, nil
	}
	if err != nil {
		return state, fmt.Errorf("copya bundle: read receipt: %w", err)
	}
	state.exists = true
	return state, nil
}

func verifyRows(ctx context.Context, tx *sql.Tx, manifest Manifest, permissionCount int) error {
	checks := []struct {
		name  string
		query string
		args  []any
		want  int
	}{
		{"user", `SELECT count(*) FROM public."user" WHERE id=$1 AND active AND email_address=$2`, []any{manifest.User.ID, manifest.User.EmailAddress}, 1},
		{"workspace", `SELECT count(*) FROM public.workspace WHERE id=$1 AND active AND slug=$2`, []any{manifest.Workspace.ID, manifest.Workspace.Slug}, 1},
		{"admin", `SELECT count(*) FROM public.admin WHERE id=$1 AND active AND user_id=$2`, []any{manifest.AdminID, manifest.User.ID}, 1},
		{"workspace user", `SELECT count(*) FROM public.workspace_user WHERE id=$1 AND active AND workspace_id=$2 AND user_id=$3`, []any{manifest.WorkspaceUserID, manifest.Workspace.ID, manifest.User.ID}, 1},
		{"role", `SELECT count(*) FROM public.role WHERE id=$1 AND active AND workspace_id=$2`, []any{manifest.Role.ID, manifest.Workspace.ID}, 1},
		{"workspace role", `SELECT count(*) FROM public.workspace_user_role WHERE id=$1 AND active AND workspace_user_id=$2 AND role_id=$3`, []any{manifest.WorkspaceUserRoleID, manifest.WorkspaceUserID, manifest.Role.ID}, 1},
		{"permissions", `SELECT count(*) FROM public.permission WHERE active AND workspace_id=$1 AND user_id=$2`, []any{manifest.Workspace.ID, manifest.User.ID}, permissionCount},
		{"role grants", `SELECT count(*) FROM public.role_permission WHERE active AND role_id=$1`, []any{manifest.Role.ID}, permissionCount},
	}
	for _, check := range checks {
		var got int
		if err := tx.QueryRowContext(ctx, check.query, check.args...).Scan(&got); err != nil {
			return fmt.Errorf("copya bundle: verify %s: %w", check.name, err)
		}
		if got != check.want {
			return fmt.Errorf("copya bundle: verify %s got %d, want %d", check.name, got, check.want)
		}
	}
	return nil
}

func loadActivePermissions(businessType string) ([]permissionRow, error) {
	provider := copya.NewSeedProvider(rootcopya.SeedsFS)
	table, err := provider.Table(businessType, "permission")
	if err != nil {
		return nil, err
	}
	headers := make(map[string]int, len(table.Headers))
	for index, header := range table.Headers {
		headers[header] = index
	}
	required := []string{"id", "name", "permission_code", "permission_type", "active", "applicable_principal_types"}
	for _, header := range required {
		if _, ok := headers[header]; !ok {
			return nil, fmt.Errorf("copya bundle: permission seed missing %s", header)
		}
	}
	permissions := make([]permissionRow, 0, len(table.Rows))
	seenCodes := make(map[string]bool, len(table.Rows))
	for rowIndex, row := range table.Rows {
		if len(row) != len(table.Headers) {
			return nil, fmt.Errorf("copya bundle: permission row %d has %d columns, want %d", rowIndex+2, len(row), len(table.Headers))
		}
		active, err := strconv.ParseBool(row[headers["active"]])
		if err != nil {
			return nil, fmt.Errorf("copya bundle: permission row %d active: %w", rowIndex+2, err)
		}
		if !active {
			continue
		}
		code := row[headers["permission_code"]]
		if code == "" {
			return nil, errors.New("copya bundle: empty active permission code")
		}
		candidate := permissionRow{
			ID:                       row[headers["id"]],
			Name:                     row[headers["name"]],
			Code:                     code,
			Type:                     row[headers["permission_type"]],
			ApplicablePrincipalTypes: row[headers["applicable_principal_types"]],
		}
		if seenCodes[code] {
			for _, existing := range permissions {
				if existing.Code == code && existing != candidate {
					return nil, fmt.Errorf("copya bundle: conflicting active permission code %q", code)
				}
			}
			continue
		}
		seenCodes[code] = true
		permissions = append(permissions, candidate)
	}
	if len(permissions) == 0 {
		return nil, errors.New("copya bundle: no active permissions")
	}
	return permissions, nil
}

func rolePermissionID(roleID, permissionID string) string {
	sum := sha256.Sum256([]byte(roleID + "\x00" + permissionID))
	return "rp-" + hex.EncodeToString(sum[:12])
}
