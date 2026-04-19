package output

import (
	"encoding/json"
	"os"
	"path/filepath"

	"cloudrift/internal/models"
)

func WriteJSON(path string, findings []models.Finding) error {
	b, err := json.MarshalIndent(findings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
