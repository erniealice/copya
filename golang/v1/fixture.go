package copya

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"strings"

	seeds "github.com/erniealice/copya"
)

// FixtureProvider loads fixture CSV data for E2E report testing.
// Fixtures are assertion-grade test data — separate from bootstrap seed data.
// Each suite has a 2-tier cascade: fixtures/{suite}/common → fixtures/{suite}/{businessType}.
type FixtureProvider struct {
	fsys fs.FS
}

// NewFixtureProvider creates a FixtureProvider backed by the given fs.FS.
// Use copya.FixturesFS for the embedded default fixtures.
func NewFixtureProvider(fsys fs.FS) *FixtureProvider {
	return &FixtureProvider{fsys: fsys}
}

// Load returns the merged SeedSet for the given suite and business type.
// Cascade: fixtures/{suite}/common → fixtures/{suite}/{businessType}
func (p *FixtureProvider) Load(suite, businessType string) (SeedSet, error) {
	suite = strings.ToLower(suite)
	key := strings.ToLower(businessType)

	common, err := loadDir(p.fsys, "fixtures/"+suite+"/common")
	if err != nil {
		return nil, fmt.Errorf("copya fixtures: load %s/common: %w", suite, err)
	}

	bt, err := loadDir(p.fsys, "fixtures/"+suite+"/"+key)
	if err != nil {
		return nil, fmt.Errorf("copya fixtures: load %s/%s: %w", suite, key, err)
	}

	result := make(SeedSet)
	merge(result, common)
	merge(result, bt)

	return result, nil
}

// LoadFixtures loads fixture CSV data for the given suite and business type and inserts
// it into the database in dependency order. Uses ON CONFLICT DO NOTHING so it is idempotent.
//
// Available suites: revenue-reporting, expense-reporting, disbursement-reporting
func LoadFixtures(ctx context.Context, db *sql.DB, suite, businessType string, dialect Dialect) error {
	p := NewFixtureProvider(seeds.FixturesFS)
	set, err := p.Load(suite, businessType)
	if err != nil {
		return fmt.Errorf("load fixtures: load %s/%s: %w", suite, businessType, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("load fixtures: begin tx: %w", err)
	}

	// Build ordered table list: InsertOrder first, then any remainder.
	seen := make(map[string]bool)
	var ordered []string
	for _, name := range InsertOrder {
		if _, ok := set[name]; ok {
			ordered = append(ordered, name)
			seen[name] = true
		}
	}
	for name := range set {
		if !seen[name] {
			ordered = append(ordered, name)
		}
	}

	for _, name := range ordered {
		t := set[name]
		sqlStr := t.ToSQL(name, dialect)
		if sqlStr == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, sqlStr); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("load fixtures: insert %s: %w", name, err)
		}
		fmt.Printf("  loaded fixture %-32s %d rows\n", name, len(t.Rows))
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("load fixtures: commit: %w", err)
	}
	return nil
}

// ClearFixtures deletes fixture rows for the given suite and business type from the database.
// Rows are identified by reading IDs from the CSV files (first column = id).
// Deletion happens in reverse InsertOrder to respect foreign-key constraints.
func ClearFixtures(ctx context.Context, db *sql.DB, suite, businessType string, dialect Dialect) error {
	p := NewFixtureProvider(seeds.FixturesFS)
	set, err := p.Load(suite, businessType)
	if err != nil {
		return fmt.Errorf("clear fixtures: load %s/%s: %w", suite, businessType, err)
	}

	// Build deletion order: reverse InsertOrder, then non-ordered tables.
	seen := make(map[string]bool)
	// Collect tables present in the fixture set that are in InsertOrder
	var inOrder []string
	for _, name := range InsertOrder {
		if _, ok := set[name]; ok {
			inOrder = append(inOrder, name)
			seen[name] = true
		}
	}

	// Reverse the ordered list so child tables are deleted before parents
	var deleteOrder []string
	for i := len(inOrder) - 1; i >= 0; i-- {
		deleteOrder = append(deleteOrder, inOrder[i])
	}
	// Append any tables not in InsertOrder
	for name := range set {
		if !seen[name] {
			deleteOrder = append(deleteOrder, name)
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("clear fixtures: begin tx: %w", err)
	}

	q := quoteIdent(dialect)
	placeholder := "$1"
	if dialect == MySQL {
		placeholder = "?"
	}

	for _, name := range deleteOrder {
		t := set[name]
		if len(t.Rows) == 0 || len(t.Headers) == 0 {
			continue
		}

		// id is always the first column in fixture CSVs
		deleted := 0
		for _, row := range t.Rows {
			if len(row) == 0 {
				continue
			}
			id := row[0]
			if id == "" {
				continue
			}
			sqlStr := fmt.Sprintf(`DELETE FROM %s WHERE id = %s`, q(name), placeholder)
			if _, err := tx.ExecContext(ctx, sqlStr, id); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("clear fixtures: delete %s id=%q: %w", name, id, err)
			}
			deleted++
		}
		if deleted > 0 {
			fmt.Printf("  cleared fixture %-32s %d rows\n", name, deleted)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("clear fixtures: commit: %w", err)
	}
	return nil
}
