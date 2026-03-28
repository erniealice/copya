package copya

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
)

// SeedProvider loads and caches CSV seed data with a 3-tier cascade:
//
//	common → general → {businessType}
//
// Later tiers replace earlier tiers for the same table name.
type SeedProvider struct {
	fsys  fs.FS
	cache map[string]SeedSet
	mu    sync.RWMutex
}

// NewSeedProvider creates a provider backed by the given fs.FS.
// Use copya.SeedsFS for the embedded default seeds.
func NewSeedProvider(fsys fs.FS) *SeedProvider {
	return &SeedProvider{
		fsys:  fsys,
		cache: make(map[string]SeedSet),
	}
}

// Load returns the merged SeedSet for the given business type.
// Results are cached after first load.
func (p *SeedProvider) Load(businessType string) (SeedSet, error) {
	key := strings.ToLower(businessType)

	p.mu.RLock()
	if cached, ok := p.cache[key]; ok {
		p.mu.RUnlock()
		return cached, nil
	}
	p.mu.RUnlock()

	common, err := loadDir(p.fsys, "seeds/common")
	if err != nil {
		return nil, fmt.Errorf("copya: load common: %w", err)
	}

	general, err := loadDir(p.fsys, "seeds/general")
	if err != nil {
		return nil, fmt.Errorf("copya: load general: %w", err)
	}

	bt, err := loadDir(p.fsys, "seeds/"+key)
	if err != nil {
		return nil, fmt.Errorf("copya: load %s: %w", key, err)
	}

	result := make(SeedSet)
	merge(result, common)
	merge(result, general)
	merge(result, bt)

	p.mu.Lock()
	p.cache[key] = result
	p.mu.Unlock()

	return result, nil
}

// Table loads a single table by name for the given business type.
func (p *SeedProvider) Table(businessType, tableName string) (*SeedTable, error) {
	set, err := p.Load(businessType)
	if err != nil {
		return nil, err
	}
	t, ok := set[tableName]
	if !ok {
		return nil, fmt.Errorf("copya: table %q not found for business type %q", tableName, businessType)
	}
	return t, nil
}

// Tables returns all available table names for the given business type.
func (p *SeedProvider) Tables(businessType string) ([]string, error) {
	set, err := p.Load(businessType)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	return names, nil
}

// OverrideFromFile replaces a table in the set with data from a CSV file path.
func (p *SeedProvider) OverrideFromFile(set SeedSet, tableName, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("copya: open %s: %w", filePath, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return fmt.Errorf("copya: parse %s: %w", filePath, err)
	}
	if len(records) == 0 {
		return fmt.Errorf("copya: %s is empty", filePath)
	}

	set[tableName] = &SeedTable{
		Name:    tableName,
		Headers: records[0],
		Rows:    records[1:],
	}
	return nil
}

// ToSQL generates INSERT statements for a SeedTable.
func (t *SeedTable) ToSQL(tableName string, dialect Dialect) string {
	if len(t.Rows) == 0 {
		return ""
	}

	var b strings.Builder
	q := quoteIdent(dialect)

	cols := make([]string, len(t.Headers))
	for i, h := range t.Headers {
		cols[i] = q(h)
	}
	colList := strings.Join(cols, ", ")

	for _, row := range t.Rows {
		vals := make([]string, len(row))
		for i, v := range row {
			if v == "" || v == "NULL" {
				vals[i] = "NULL"
			} else {
				vals[i] = "'" + strings.ReplaceAll(v, "'", "''") + "'"
			}
		}
		valList := strings.Join(vals, ", ")

		switch dialect {
		case MySQL:
			fmt.Fprintf(&b, "INSERT IGNORE INTO %s (%s) VALUES (%s);\n",
				q(tableName), colList, valList)
		default: // Postgres
			fmt.Fprintf(&b, "INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (id) DO NOTHING;\n",
				q(tableName), colList, valList)
		}
	}

	return b.String()
}

// ToSQLAll generates all INSERT statements for a SeedSet in dependency order.
func ToSQLAll(set SeedSet, dialect Dialect) string {
	var b strings.Builder

	// First: tables in defined order
	seen := make(map[string]bool)
	for _, name := range InsertOrder {
		if t, ok := set[name]; ok {
			sql := t.ToSQL(name, dialect)
			if sql != "" {
				fmt.Fprintf(&b, "-- %s\n%s\n", name, sql)
			}
			seen[name] = true
		}
	}

	// Then: any remaining tables not in InsertOrder
	for name, t := range set {
		if seen[name] {
			continue
		}
		sql := t.ToSQL(name, dialect)
		if sql != "" {
			fmt.Fprintf(&b, "-- %s\n%s\n", name, sql)
		}
	}

	return b.String()
}

func quoteIdent(d Dialect) func(string) string {
	switch d {
	case MySQL:
		return func(s string) string { return "`" + s + "`" }
	default:
		return func(s string) string { return `"` + s + `"` }
	}
}
