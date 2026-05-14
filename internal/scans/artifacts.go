// Package scans provides filesystem scan discovery and artifact loading shared by CLI and API.
package scans

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Zero0x00/cloudrift/internal/models"
)

var safeScanIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// IsSafeScanID reports whether id is an acceptable scan directory name (no path segments).
func IsSafeScanID(id string) bool {
	return id != "" &&
		safeScanIDPattern.MatchString(id) &&
		id != "." &&
		id != ".." &&
		!strings.Contains(id, "/") &&
		!strings.Contains(id, "\\")
}

// ResolveScanDirectoryName resolves the scan subdirectory name under outputDir for CLI/API use.
// If scanID is "latest" (after TrimSpace), it uses ResolveLatestScanID; otherwise the name must
// satisfy IsSafeScanID so filepath.Join(outputDir, name) never escapes via path segments.
func ResolveScanDirectoryName(outputDir, scanID string) (string, error) {
	id := strings.TrimSpace(scanID)
	if id != "latest" {
		if !IsSafeScanID(id) {
			return "", fmt.Errorf("scans: invalid scan id %q: use alphanumeric, dot, underscore, hyphen only (or \"latest\")", id)
		}
		return id, nil
	}
	return ResolveLatestScanID(outputDir)
}

// ListScanIDs returns sorted directory names under outputDir that match the safe scan ID pattern.
func ListScanIDs(outputDir string) ([]string, error) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !safeScanIDPattern.MatchString(name) {
			continue
		}
		ids = append(ids, name)
	}
	sort.Strings(ids)
	return ids, nil
}

// LoadScanArtifacts loads scan-metadata.json and findings.json for a scan directory name (dirName).
// dirName must satisfy IsSafeScanID (e.g. from ListScanIDs or ResolveScanDirectoryName) so joins
// under outputDir stay path-safe. Behavior matches the dashboard API: missing metadata falls back
// to directory mtime; malformed dirs error.
func LoadScanArtifacts(outputDir, dirName string) (*models.ScanSnapshot, []models.Finding, error) {
	if !IsSafeScanID(dirName) {
		return nil, nil, os.ErrInvalid
	}
	scanPath := filepath.Join(outputDir, dirName)
	metaPath := filepath.Join(scanPath, "scan-metadata.json")
	findingsPath := filepath.Join(scanPath, "findings.json")

	meta, err := readMetadata(metaPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	findings, err := readFindings(findingsPath)
	if err != nil {
		return nil, nil, err
	}
	if meta == nil {
		meta = &models.ScanSnapshot{
			ScanID:       dirName,
			Timestamp:    fileModTime(scanPath),
			FindingCount: len(findings),
		}
	}
	return meta, findings, nil
}

// SortTimestamp returns the timestamp used for newest-first ordering (matches API list semantics).
func SortTimestamp(meta *models.ScanSnapshot) time.Time {
	if meta == nil {
		return time.Unix(0, 0).UTC()
	}
	if meta.Timestamp.IsZero() {
		return time.Unix(0, 0).UTC()
	}
	return meta.Timestamp
}

func readMetadata(path string) (*models.ScanSnapshot, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return &models.ScanSnapshot{}, nil
	}
	var meta models.ScanSnapshot
	if err := json.Unmarshal(b, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func readFindings(path string) ([]models.Finding, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return []models.Finding{}, nil
	}
	var findings []models.Finding
	if err := json.Unmarshal(b, &findings); err != nil {
		return nil, err
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].AffectedARN == findings[j].AffectedARN {
			return findings[i].ID < findings[j].ID
		}
		return findings[i].AffectedARN < findings[j].AffectedARN
	})
	return findings, nil
}

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Unix(0, 0).UTC()
	}
	return info.ModTime().UTC()
}
