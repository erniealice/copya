package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/erniealice/copya/golang/v1/bundle"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	manifestPath := flag.String("manifest", "", "path to the immutable bundle manifest")
	targetKey := flag.String("target", "", "target key selected by the Esqyma initializer")
	schemaRelease := flag.String("schema-release", "", "required Esqyma calendar release")
	apply := flag.Bool("apply", false, "apply the bundle transaction")
	flag.Parse()

	if *manifestPath == "" || *targetKey == "" || *schemaRelease == "" || flag.NArg() != 0 {
		fatalf("usage: copya-bundle --manifest PATH --target CLIENT/TARGET --schema-release postgres/YYYY.MM.N [--apply]")
	}
	manifest, _, digest, err := bundle.Load(*manifestPath)
	if err != nil {
		fatalf("%v", err)
	}
	plan, err := bundle.BuildPlan(manifest, digest)
	if err != nil {
		fatalf("%v", err)
	}
	if manifest.SchemaRelease != *schemaRelease {
		fatalf("bundle release %s does not match required %s", manifest.SchemaRelease, *schemaRelease)
	}
	if !*apply {
		writeJSON(struct {
			Mode string `json:"mode"`
			bundle.Plan
		}{Mode: "plan", Plan: plan})
		return
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fatalf("DATABASE_URL is required for --apply")
	}
	password := os.Getenv(manifest.User.PasswordEnv)
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		fatalf("connect database: %v", err)
	}
	result, err := bundle.Apply(context.Background(), db, *targetKey, *schemaRelease, password, digest, manifest)
	if err != nil {
		fatalf("%v", err)
	}
	writeJSON(result)
}

func writeJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fatalf("encode output: %v", err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "copya-bundle: "+format+"\n", args...)
	os.Exit(1)
}
