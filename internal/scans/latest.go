package scans

import (
	"sort"
	"time"

	"github.com/Zero0x00/cloudrift/internal/models"
)

// ResolveLatestScanID returns the scan directory name under outputDir that is "newest"
// by scan-metadata.json timestamp (descending), with directory name ascending as a stable tie-breaker
// when timestamps are equal. Malformed scan directories are skipped (same as GET /api/scans).
// Returns ("", nil) when there are no loadable scans.
func ResolveLatestScanID(outputDir string) (string, error) {
	scanIDs, err := ListScanIDs(outputDir)
	if err != nil {
		return "", err
	}
	return resolveLatestScanIDWithLoader(outputDir, scanIDs, LoadScanArtifacts)
}

type latestLoader func(outputDir, dirName string) (*models.ScanSnapshot, []models.Finding, error)

func resolveLatestScanIDWithLoader(outputDir string, scanIDs []string, load latestLoader) (string, error) {
	type row struct {
		dir string
		ts  time.Time
	}
	rows := make([]row, 0, len(scanIDs))
	for _, dir := range scanIDs {
		meta, _, err := load(outputDir, dir)
		if err != nil {
			continue
		}
		rows = append(rows, row{dir: dir, ts: SortTimestamp(meta)})
	}
	if len(rows) == 0 {
		return "", nil
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ts.Equal(rows[j].ts) {
			return rows[i].dir < rows[j].dir
		}
		return rows[i].ts.After(rows[j].ts)
	})
	return rows[0].dir, nil
}
