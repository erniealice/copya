package copya

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// loadDir reads every .csv file in dir from fsys and returns a SeedSet.
// Each CSV file becomes one SeedTable keyed by its file stem.
func loadDir(fsys fs.FS, dir string) (SeedSet, error) {
	set := make(SeedSet)

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return set, nil // dir doesn't exist — not an error, just empty
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".csv") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".csv")
		path := filepath.Join(dir, e.Name())

		table, err := readCSV(fsys, path)
		if err != nil {
			return nil, fmt.Errorf("copya: read %s: %w", path, err)
		}
		table.Name = name
		set[name] = table
	}

	return set, nil
}

func readCSV(fsys fs.FS, path string) (*SeedTable, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true

	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return &SeedTable{}, nil
	}

	return &SeedTable{
		Headers: records[0],
		Rows:    records[1:],
	}, nil
}

// merge overlays src onto dst. Tables in src replace tables in dst entirely.
func merge(dst, src SeedSet) {
	for k, v := range src {
		dst[k] = v
	}
}
