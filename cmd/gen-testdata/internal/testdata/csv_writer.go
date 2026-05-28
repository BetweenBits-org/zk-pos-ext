package testdata

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
)

// writeCSV writes a single file with a header row + the given rows.
// Creates the parent directory if missing. Used by every model's
// synthesis to emit accounts.csv / cex_assets.csv / tier_ratios.csv.
func writeCSV(outDir, name string, header []string, rows [][]string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", outDir, err)
	}
	path := filepath.Join(outDir, name)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %q: %w", path, err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		return fmt.Errorf("write header %q: %w", name, err)
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return fmt.Errorf("write row %q: %w", name, err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("flush %q: %w", name, err)
	}
	return nil
}
