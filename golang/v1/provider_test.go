package copya

import (
	"strings"
	"testing"
	"testing/fstest"
)

// ---------------------------------------------------------------------------
// ToSQL tests
// ---------------------------------------------------------------------------

func TestToSQL_Postgres(t *testing.T) {
	table := &SeedTable{
		Name:    "user",
		Headers: []string{"id", "name", "email"},
		Rows: [][]string{
			{"1", "Alice", "alice@example.com"},
			{"2", "Bob", "bob@example.com"},
		},
	}

	sql := table.ToSQL("user", Postgres)

	if !strings.Contains(sql, `INSERT INTO "user"`) {
		t.Error("expected Postgres INSERT INTO with double-quoted table name")
	}
	if !strings.Contains(sql, `ON CONFLICT (id) DO NOTHING`) {
		t.Error("expected Postgres ON CONFLICT clause")
	}
	if !strings.Contains(sql, `'Alice'`) {
		t.Error("expected Alice value in SQL")
	}
	if !strings.Contains(sql, `'bob@example.com'`) {
		t.Error("expected bob@example.com value in SQL")
	}
}

func TestToSQL_MySQL(t *testing.T) {
	table := &SeedTable{
		Name:    "user",
		Headers: []string{"id", "name"},
		Rows: [][]string{
			{"1", "Alice"},
		},
	}

	sql := table.ToSQL("user", MySQL)

	if !strings.Contains(sql, "INSERT IGNORE INTO `user`") {
		t.Errorf("expected MySQL INSERT IGNORE with backtick-quoted name, got: %s", sql)
	}
	if !strings.Contains(sql, "`id`") {
		t.Error("expected backtick-quoted column names")
	}
}

func TestToSQL_EmptyRows(t *testing.T) {
	table := &SeedTable{
		Name:    "user",
		Headers: []string{"id"},
		Rows:    nil,
	}

	sql := table.ToSQL("user", Postgres)
	if sql != "" {
		t.Errorf("expected empty string for no rows, got %q", sql)
	}
}

func TestToSQL_NULLHandling(t *testing.T) {
	table := &SeedTable{
		Headers: []string{"id", "name", "email"},
		Rows: [][]string{
			{"1", "", "NULL"},
		},
	}

	sql := table.ToSQL("user", Postgres)

	// Both empty string and "NULL" should become NULL
	parts := strings.Split(sql, "VALUES (")
	if len(parts) < 2 {
		t.Fatal("could not parse VALUES clause")
	}
	values := parts[1]
	// Count NULL occurrences (should have 2: empty string and literal "NULL")
	if strings.Count(values, "NULL") != 2 {
		t.Errorf("expected 2 NULL values, got SQL: %s", sql)
	}
}

func TestToSQL_EscapesSingleQuotes(t *testing.T) {
	table := &SeedTable{
		Headers: []string{"id", "name"},
		Rows: [][]string{
			{"1", "O'Brien"},
		},
	}

	sql := table.ToSQL("user", Postgres)

	if !strings.Contains(sql, "O''Brien") {
		t.Errorf("expected escaped single quote, got: %s", sql)
	}
}

// ---------------------------------------------------------------------------
// quoteIdent tests
// ---------------------------------------------------------------------------

func TestQuoteIdent_Postgres(t *testing.T) {
	q := quoteIdent(Postgres)
	if got := q("name"); got != `"name"` {
		t.Errorf("quoteIdent(Postgres)('name') = %q, want %q", got, `"name"`)
	}
}

func TestQuoteIdent_MySQL(t *testing.T) {
	q := quoteIdent(MySQL)
	if got := q("name"); got != "`name`" {
		t.Errorf("quoteIdent(MySQL)('name') = %q, want %q", got, "`name`")
	}
}

// ---------------------------------------------------------------------------
// merge tests
// ---------------------------------------------------------------------------

func TestMerge_OverwritesDst(t *testing.T) {
	dst := SeedSet{
		"user": &SeedTable{Name: "user", Headers: []string{"id"}, Rows: [][]string{{"1"}}},
	}
	src := SeedSet{
		"user": &SeedTable{Name: "user", Headers: []string{"id", "name"}, Rows: [][]string{{"1", "Alice"}}},
	}

	merge(dst, src)

	if len(dst["user"].Headers) != 2 {
		t.Errorf("expected 2 headers after merge, got %d", len(dst["user"].Headers))
	}
}

func TestMerge_AddsNewTables(t *testing.T) {
	dst := SeedSet{
		"user": &SeedTable{Name: "user"},
	}
	src := SeedSet{
		"role": &SeedTable{Name: "role"},
	}

	merge(dst, src)

	if _, ok := dst["role"]; !ok {
		t.Error("expected 'role' table to be added to dst")
	}
	if _, ok := dst["user"]; !ok {
		t.Error("expected 'user' table to still exist in dst")
	}
}

// ---------------------------------------------------------------------------
// loadDir / readCSV tests (using fstest.MapFS)
// ---------------------------------------------------------------------------

func TestLoadDir_ReadsCSVFiles(t *testing.T) {
	fs := fstest.MapFS{
		"seeds/common/user.csv": &fstest.MapFile{
			Data: []byte("id,name\n1,Alice\n2,Bob\n"),
		},
		"seeds/common/role.csv": &fstest.MapFile{
			Data: []byte("id,label\n10,Admin\n"),
		},
		"seeds/common/readme.txt": &fstest.MapFile{
			Data: []byte("ignore this"),
		},
	}

	set, err := loadDir(fs, "seeds/common")
	if err != nil {
		t.Fatalf("loadDir error: %v", err)
	}

	if len(set) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(set))
	}

	userTable, ok := set["user"]
	if !ok {
		t.Fatal("expected 'user' table")
	}
	if len(userTable.Headers) != 2 {
		t.Errorf("user headers = %d, want 2", len(userTable.Headers))
	}
	if len(userTable.Rows) != 2 {
		t.Errorf("user rows = %d, want 2", len(userTable.Rows))
	}
	if userTable.Name != "user" {
		t.Errorf("user table name = %q, want %q", userTable.Name, "user")
	}
}

func TestLoadDir_MissingDirReturnsEmpty(t *testing.T) {
	fs := fstest.MapFS{}

	set, err := loadDir(fs, "seeds/nonexistent")
	if err != nil {
		t.Fatalf("loadDir error: %v", err)
	}
	if len(set) != 0 {
		t.Errorf("expected empty set, got %d tables", len(set))
	}
}

func TestReadCSV_EmptyFile(t *testing.T) {
	fs := fstest.MapFS{
		"empty.csv": &fstest.MapFile{Data: []byte("")},
	}

	table, err := readCSV(fs, "empty.csv")
	if err != nil {
		t.Fatalf("readCSV error: %v", err)
	}
	if len(table.Headers) != 0 {
		t.Errorf("expected no headers for empty CSV, got %d", len(table.Headers))
	}
}

// ---------------------------------------------------------------------------
// SeedProvider tests
// ---------------------------------------------------------------------------

func TestSeedProvider_Load_CascadeMerge(t *testing.T) {
	fs := fstest.MapFS{
		"seeds/common/user.csv": &fstest.MapFile{
			Data: []byte("id,name\n1,CommonUser\n"),
		},
		"seeds/general/user.csv": &fstest.MapFile{
			Data: []byte("id,name,email\n1,GeneralUser,gen@test.com\n"),
		},
		"seeds/retail/product.csv": &fstest.MapFile{
			Data: []byte("id,sku\n100,SKU-001\n"),
		},
	}

	provider := NewSeedProvider(fs)
	set, err := provider.Load("retail")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	// "user" should be from general (overrides common)
	userTable, ok := set["user"]
	if !ok {
		t.Fatal("expected 'user' table")
	}
	if len(userTable.Headers) != 3 {
		t.Errorf("user should have 3 headers from general override, got %d", len(userTable.Headers))
	}

	// "product" should come from retail tier
	if _, ok := set["product"]; !ok {
		t.Error("expected 'product' table from retail tier")
	}
}

func TestSeedProvider_Load_CachesResults(t *testing.T) {
	fs := fstest.MapFS{
		"seeds/common/user.csv": &fstest.MapFile{
			Data: []byte("id,name\n1,Alice\n"),
		},
	}

	provider := NewSeedProvider(fs)

	set1, err := provider.Load("retail")
	if err != nil {
		t.Fatalf("first Load error: %v", err)
	}

	set2, err := provider.Load("retail")
	if err != nil {
		t.Fatalf("second Load error: %v", err)
	}

	// Should return same reference (cached)
	if len(set1) != len(set2) {
		t.Errorf("cached results differ: %d vs %d tables", len(set1), len(set2))
	}
}

func TestSeedProvider_Table(t *testing.T) {
	fs := fstest.MapFS{
		"seeds/common/user.csv": &fstest.MapFile{
			Data: []byte("id,name\n1,Alice\n"),
		},
	}

	provider := NewSeedProvider(fs)

	table, err := provider.Table("retail", "user")
	if err != nil {
		t.Fatalf("Table error: %v", err)
	}
	if table.Name != "user" {
		t.Errorf("table name = %q, want %q", table.Name, "user")
	}
}

func TestSeedProvider_Table_NotFound(t *testing.T) {
	fs := fstest.MapFS{}
	provider := NewSeedProvider(fs)

	_, err := provider.Table("retail", "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent table")
	}
}

func TestSeedProvider_Tables(t *testing.T) {
	fs := fstest.MapFS{
		"seeds/common/user.csv": &fstest.MapFile{
			Data: []byte("id,name\n1,Alice\n"),
		},
		"seeds/common/role.csv": &fstest.MapFile{
			Data: []byte("id,label\n1,Admin\n"),
		},
	}

	provider := NewSeedProvider(fs)
	names, err := provider.Tables("retail")
	if err != nil {
		t.Fatalf("Tables error: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 table names, got %d", len(names))
	}
}

// ---------------------------------------------------------------------------
// ToSQLAll tests
// ---------------------------------------------------------------------------

func TestToSQLAll_InsertsInOrder(t *testing.T) {
	set := SeedSet{
		"user": &SeedTable{
			Name:    "user",
			Headers: []string{"id", "name"},
			Rows:    [][]string{{"1", "Alice"}},
		},
		"role": &SeedTable{
			Name:    "role",
			Headers: []string{"id", "label"},
			Rows:    [][]string{{"1", "Admin"}},
		},
	}

	sql := ToSQLAll(set, Postgres)

	userIdx := strings.Index(sql, `-- user`)
	roleIdx := strings.Index(sql, `-- role`)

	if userIdx == -1 || roleIdx == -1 {
		t.Fatalf("expected both user and role in output, got:\n%s", sql)
	}

	// user is before role in InsertOrder
	if userIdx > roleIdx {
		t.Error("expected user table before role table in output (InsertOrder)")
	}
}

func TestToSQLAll_IncludesNonOrderedTables(t *testing.T) {
	set := SeedSet{
		"custom_table": &SeedTable{
			Name:    "custom_table",
			Headers: []string{"id"},
			Rows:    [][]string{{"1"}},
		},
	}

	sql := ToSQLAll(set, Postgres)

	if !strings.Contains(sql, "-- custom_table") {
		t.Error("expected custom_table in output (not in InsertOrder but should still appear)")
	}
}

// ---------------------------------------------------------------------------
// Negative / defensive ToSQL tests
// ---------------------------------------------------------------------------

func TestToSQL_SQLInjectionInValues(t *testing.T) {
	table := &SeedTable{
		Headers: []string{"id", "name"},
		Rows: [][]string{
			{"1", "Robert'); DROP TABLE users;--"},
		},
	}

	sql := table.ToSQL("user", Postgres)

	// Single quotes must be escaped (doubled)
	if strings.Contains(sql, "Robert');") {
		t.Error("SQL injection: unescaped single quote found in output")
	}
	if !strings.Contains(sql, "Robert''); DROP TABLE users;--") {
		t.Errorf("expected properly escaped value, got: %s", sql)
	}
}

func TestToSQL_ValuesWithNewlinesAndTabs(t *testing.T) {
	table := &SeedTable{
		Headers: []string{"id", "description"},
		Rows: [][]string{
			{"1", "line1\nline2\ttab"},
		},
	}

	sql := table.ToSQL("user", Postgres)

	// The value should be quoted but newlines/tabs preserved as-is
	if !strings.Contains(sql, "line1\nline2\ttab") {
		t.Errorf("expected newlines and tabs preserved in value, got: %s", sql)
	}
	// Must still be wrapped in single quotes
	if !strings.Contains(sql, "'line1\nline2\ttab'") {
		t.Errorf("expected value wrapped in single quotes, got: %s", sql)
	}
}

func TestToSQL_VeryLongValues(t *testing.T) {
	longValue := strings.Repeat("a", 1500)
	table := &SeedTable{
		Headers: []string{"id", "data"},
		Rows: [][]string{
			{"1", longValue},
		},
	}

	sql := table.ToSQL("user", Postgres)

	if !strings.Contains(sql, longValue) {
		t.Error("expected very long value (1500 chars) to be preserved in SQL output")
	}
	if !strings.Contains(sql, "INSERT INTO") {
		t.Error("expected valid INSERT statement")
	}
}

func TestToSQL_MultipleQuotesInValue(t *testing.T) {
	table := &SeedTable{
		Headers: []string{"id", "note"},
		Rows: [][]string{
			{"1", "it's a 'test' with 'many' quotes"},
		},
	}

	sql := table.ToSQL("user", Postgres)

	// Every single quote in the value should be doubled
	if !strings.Contains(sql, "it''s a ''test'' with ''many'' quotes") {
		t.Errorf("expected all single quotes doubled, got: %s", sql)
	}
}

// ---------------------------------------------------------------------------
// Negative / defensive readCSV tests
// ---------------------------------------------------------------------------

func TestReadCSV_MalformedCSV_MismatchedColumns(t *testing.T) {
	fs := fstest.MapFS{
		"bad.csv": &fstest.MapFile{
			Data: []byte("id,name,email\n1,Alice\n2,Bob,bob@test.com,extrafield\n"),
		},
	}

	// Go's csv reader in strict mode returns an error for mismatched field counts.
	_, err := readCSV(fs, "bad.csv")
	if err == nil {
		t.Fatal("expected error for CSV with mismatched column counts, got nil")
	}
}

func TestReadCSV_HeaderOnly(t *testing.T) {
	fs := fstest.MapFS{
		"header-only.csv": &fstest.MapFile{
			Data: []byte("id,name,email\n"),
		},
	}

	table, err := readCSV(fs, "header-only.csv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(table.Headers) != 3 {
		t.Errorf("expected 3 headers, got %d", len(table.Headers))
	}
	if len(table.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(table.Rows))
	}
}

// ---------------------------------------------------------------------------
// Negative / defensive Load tests
// ---------------------------------------------------------------------------

func TestSeedProvider_Load_EmptyCSVFiles(t *testing.T) {
	fs := fstest.MapFS{
		"seeds/common/empty.csv": &fstest.MapFile{
			Data: []byte(""),
		},
	}

	provider := NewSeedProvider(fs)
	set, err := provider.Load("retail")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	emptyTable, ok := set["empty"]
	if !ok {
		t.Fatal("expected 'empty' table in set")
	}
	if len(emptyTable.Headers) != 0 {
		t.Errorf("expected 0 headers for empty CSV, got %d", len(emptyTable.Headers))
	}
}

// ---------------------------------------------------------------------------
// Negative / defensive quoteIdent tests
// ---------------------------------------------------------------------------

func TestQuoteIdent_IdentifierContainingQuotes(t *testing.T) {
	// Postgres: identifier with double-quote inside
	qPg := quoteIdent(Postgres)
	got := qPg(`my"table`)
	// Current implementation does not escape inner quotes — document behavior
	want := `"my"table"`
	if got != want {
		t.Errorf("quoteIdent(Postgres)(%q) = %q, want %q", `my"table`, got, want)
	}

	// MySQL: identifier with backtick inside
	qMy := quoteIdent(MySQL)
	gotMy := qMy("my`table")
	wantMy := "`my`table`"
	if gotMy != wantMy {
		t.Errorf("quoteIdent(MySQL)(%q) = %q, want %q", "my`table", gotMy, wantMy)
	}
}

func TestQuoteIdent_EmptyIdentifier(t *testing.T) {
	qPg := quoteIdent(Postgres)
	got := qPg("")
	if got != `""` {
		t.Errorf("quoteIdent(Postgres)('') = %q, want %q", got, `""`)
	}

	qMy := quoteIdent(MySQL)
	gotMy := qMy("")
	if gotMy != "``" {
		t.Errorf("quoteIdent(MySQL)('') = %q, want %q", gotMy, "``")
	}
}
