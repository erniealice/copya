package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/erniealice/copya"
	v1 "github.com/erniealice/copya/golang/v1"
)

func main() {
	businessType := flag.String("business-type", "service", "Business type (e.g. service, retail, professional)")
	dialect := flag.String("dialect", "postgres", "SQL dialect: postgres or mysql")
	table := flag.String("table", "", "Specific table to output (omit for all)")
	format := flag.String("format", "sql", "Output format: sql, csv, or list")
	schemaFile := flag.String("schema-file", "", "Path to DDL schema file to prepend before INSERT statements")
	wrapTx := flag.Bool("wrap-transaction", false, "Wrap output in BEGIN/COMMIT transaction")

	// Custom CSV overrides
	userCSV := flag.String("user-csv", "", "Path to custom user CSV file")
	locationCSV := flag.String("location-csv", "", "Path to custom location CSV file")
	productCSV := flag.String("product-csv", "", "Path to custom product CSV file")

	flag.Parse()

	d := v1.Postgres
	if *dialect == "mysql" {
		d = v1.MySQL
	}

	provider := v1.NewSeedProvider(copya.SeedsFS)

	// Load the full set first
	set, err := provider.Load(*businessType)
	if err != nil {
		fatal(err)
	}

	// Apply custom CSV overrides
	overrides := map[string]string{
		"user":     *userCSV,
		"location": *locationCSV,
		"product":  *productCSV,
	}
	for tableName, path := range overrides {
		if path == "" {
			continue
		}
		if err := provider.OverrideFromFile(set, tableName, path); err != nil {
			fatal(err)
		}
	}

	switch *format {
	case "list":
		listTables(set)
	case "csv":
		outputCSV(set, *table)
	case "sql":
		outputSQL(set, *table, d, *businessType, *schemaFile, *wrapTx)
	default:
		fmt.Fprintf(os.Stderr, "unknown format: %s (use sql, csv, or list)\n", *format)
		os.Exit(1)
	}
}

func listTables(set v1.SeedSet) {
	names := sortedKeys(set)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "TABLE\tROWS\tCOLUMNS\n")
	for _, name := range names {
		t := set[name]
		fmt.Fprintf(w, "%s\t%d\t%s\n", name, len(t.Rows), strings.Join(t.Headers, ", "))
	}
	w.Flush()
}

func outputCSV(set v1.SeedSet, tableName string) {
	if tableName != "" {
		t, ok := set[tableName]
		if !ok {
			fatal(fmt.Errorf("table %q not found", tableName))
		}
		printCSV(t)
		return
	}
	names := sortedKeys(set)
	for i, name := range names {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("-- %s\n", name)
		printCSV(set[name])
	}
}

func outputSQL(set v1.SeedSet, tableName string, d v1.Dialect, businessType, schemaFile string, wrapTx bool) {
	if wrapTx {
		fmt.Println("BEGIN;")
	}

	if schemaFile != "" {
		ddl, err := os.ReadFile(schemaFile)
		if err != nil {
			fatal(fmt.Errorf("read schema-file: %w", err))
		}
		fmt.Printf("%s\n", ddl)
	}

	if tableName != "" {
		t, ok := set[tableName]
		if !ok {
			fatal(fmt.Errorf("table %q not found", tableName))
		}
		fmt.Print(t.ToSQL(tableName, d))
	} else {
		// Full seed: header + dependency-ordered output
		fmt.Printf("-- Copya seed data for business type: %s\n", businessType)
		fmt.Printf("-- Generated: %s\n", time.Now().UTC().Format(time.RFC3339))
		fmt.Printf("-- Dialect: %s\n", d)
		fmt.Printf("-- Tables: %d\n\n", len(set))
		fmt.Print(v1.ToSQLAll(set, d))
	}

	if wrapTx {
		fmt.Println("COMMIT;")
	}
}

func printCSV(t *v1.SeedTable) {
	fmt.Println(strings.Join(t.Headers, ","))
	for _, row := range t.Rows {
		fmt.Println(strings.Join(row, ","))
	}
}

func sortedKeys(set v1.SeedSet) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
