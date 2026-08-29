// Package bundle applies immutable, target-selected Copya data bundles.
package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var (
	identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	releasePattern    = regexp.MustCompile(`^postgres/[0-9]{4}\.[0-9]{2}\.[1-9][0-9]*$`)
	versionPattern    = regexp.MustCompile(`^[0-9]{4}\.[0-9]{2}\.[1-9][0-9]*$`)
	profilePattern    = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	businessPattern   = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	slugPattern       = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	envPattern        = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
)

type Manifest struct {
	FormatVersion       int       `json:"format_version"`
	ID                  string    `json:"id"`
	Version             string    `json:"version"`
	SchemaRelease       string    `json:"schema_release"`
	Profile             string    `json:"profile"`
	BusinessType        string    `json:"business_type"`
	Workspace           Workspace `json:"workspace"`
	User                User      `json:"user"`
	AdminID             string    `json:"admin_id"`
	WorkspaceUserID     string    `json:"workspace_user_id"`
	Role                Role      `json:"role"`
	WorkspaceUserRoleID string    `json:"workspace_user_role_id"`
}

type Workspace struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Timezone    string `json:"timezone"`
}

type User struct {
	ID           string `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	EmailAddress string `json:"email_address"`
	MobileNumber string `json:"mobile_number"`
	Timezone     string `json:"timezone"`
	PasswordEnv  string `json:"password_env"`
}

type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

func Load(path string) (Manifest, []byte, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, nil, "", fmt.Errorf("copya bundle: read %s: %w", path, err)
	}
	manifest, err := Parse(raw)
	if err != nil {
		return Manifest{}, nil, "", fmt.Errorf("copya bundle: %s: %w", path, err)
	}
	return manifest, raw, Digest(raw), nil
}

func Parse(raw []byte) (Manifest, error) {
	var manifest Manifest
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Manifest{}, errors.New("multiple JSON values")
		}
		return Manifest{}, err
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func Digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (manifest Manifest) Validate() error {
	if manifest.FormatVersion != 1 {
		return fmt.Errorf("unsupported format_version %d", manifest.FormatVersion)
	}
	if !identifierPattern.MatchString(manifest.ID) || !versionPattern.MatchString(manifest.Version) {
		return errors.New("invalid bundle id or version")
	}
	if !releasePattern.MatchString(manifest.SchemaRelease) {
		return fmt.Errorf("invalid schema release %q", manifest.SchemaRelease)
	}
	if !profilePattern.MatchString(manifest.Profile) || !businessPattern.MatchString(manifest.BusinessType) {
		return errors.New("invalid profile or business type")
	}
	for name, value := range map[string]string{
		"workspace id":           manifest.Workspace.ID,
		"user id":                manifest.User.ID,
		"admin id":               manifest.AdminID,
		"workspace user id":      manifest.WorkspaceUserID,
		"role id":                manifest.Role.ID,
		"workspace user role id": manifest.WorkspaceUserRoleID,
	} {
		if !identifierPattern.MatchString(value) {
			return fmt.Errorf("invalid %s", name)
		}
	}
	if manifest.Workspace.Name == "" || !slugPattern.MatchString(manifest.Workspace.Slug) || manifest.Workspace.Timezone == "" {
		return errors.New("invalid workspace")
	}
	if manifest.User.FirstName == "" || manifest.User.LastName == "" || !strings.Contains(manifest.User.EmailAddress, "@") || manifest.User.Timezone == "" {
		return errors.New("invalid user")
	}
	if !envPattern.MatchString(manifest.User.PasswordEnv) {
		return errors.New("invalid password environment key")
	}
	if manifest.Role.Name == "" {
		return errors.New("invalid role")
	}
	return nil
}
