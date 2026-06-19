package scans

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Zero0x00/cloudrift/internal/models"
)

// LoadAssets merges every assets/*.json file (each a JSON array of AssetNode) for a scan,
// in sorted filename order. A missing assets/ directory is not an error (empty slice).
// Used by the Neo4j export so the projected graph includes asset nodes and relationships,
// not just findings.
func LoadAssets(outputDir, scanID string) ([]models.AssetNode, error) {
	resolved, err := ResolveScanDirectoryName(outputDir, scanID)
	if err != nil {
		return nil, err
	}
	assetsDir := filepath.Join(outputDir, resolved, "assets")
	entries, err := os.ReadDir(assetsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var out []models.AssetNode
	for _, name := range names {
		b, err := os.ReadFile(filepath.Join(assetsDir, name))
		if err != nil {
			return nil, err
		}
		if len(b) == 0 {
			continue
		}
		var nodes []models.AssetNode
		if err := json.Unmarshal(b, &nodes); err != nil {
			return nil, err
		}
		out = append(out, nodes...)
	}
	return out, nil
}

// LoadRelationships reads relationships.json (a JSON array of Relationship) for a scan.
// A missing or empty file is not an error (empty slice).
func LoadRelationships(outputDir, scanID string) ([]models.Relationship, error) {
	resolved, err := ResolveScanDirectoryName(outputDir, scanID)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(outputDir, resolved, "relationships.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return nil, nil
	}
	var rels []models.Relationship
	if err := json.Unmarshal(b, &rels); err != nil {
		return nil, err
	}
	return rels, nil
}
