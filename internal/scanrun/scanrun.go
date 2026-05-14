package scanrun

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Zero0x00/cloudrift/internal/models"
)

// Run executes the current scan pipeline entrypoint.
//
// Current behavior intentionally mirrors existing CLI scan behavior: create a
// scan directory with metadata + empty findings.
func Run(_ context.Context, outputDir, toolVersion string) (string, error) {
	scanID := time.Now().UTC().Format("20060102-150405")
	scanPath := filepath.Join(outputDir, scanID)
	if err := os.MkdirAll(scanPath, 0o755); err != nil {
		return "", err
	}
	findings := []models.Finding{}
	meta := models.ScanSnapshot{
		ScanID:       scanID,
		Timestamp:    time.Now().UTC(),
		AccountIDs:   []string{},
		ToolVersion:  toolVersion,
		FindingCount: len(findings),
	}
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(scanPath, "scan-metadata.json"), b, 0o644); err != nil {
		return "", err
	}
	if err := writeFindings(filepath.Join(scanPath, "findings.json"), findings); err != nil {
		return "", err
	}
	return scanID, nil
}

func writeFindings(path string, findings []models.Finding) error {
	b, err := json.MarshalIndent(findings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
