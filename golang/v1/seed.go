package copya

import (
	"context"
	"database/sql"
	"fmt"

	seeds "github.com/erniealice/copya"
)

// Seed loads CSV seed data for the given business type and inserts it into the
// database in dependency order. Uses ON CONFLICT DO NOTHING so it is idempotent.
//
// businessType must match a seeds/ sub-directory: common, general, service, professional.
// dialect controls quoting and INSERT syntax.
func Seed(ctx context.Context, db *sql.DB, businessType string, dialect Dialect) error {
	p := NewSeedProvider(seeds.SeedsFS)
	set, err := p.Load(businessType)
	if err != nil {
		return fmt.Errorf("seed: load %s: %w", businessType, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("seed: begin tx: %w", err)
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
			return fmt.Errorf("seed: insert %s: %w", name, err)
		}
		fmt.Printf("  seeded %-32s %d rows\n", name, len(t.Rows))
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("seed: commit: %w", err)
	}
	return nil
}
