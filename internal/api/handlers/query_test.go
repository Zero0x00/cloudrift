package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cloudrift/internal/models"
	"cloudrift/internal/queryv2"
)

func TestQueryHandler_Query(t *testing.T) {
	outDir := t.TempDir()
	scanID := "scan-1"
	scanPath := filepath.Join(outDir, scanID)
	if err := os.MkdirAll(scanPath, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := models.ScanSnapshot{ScanID: scanID, Timestamp: time.Now().UTC()}
	metaB, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(scanPath, "scan-metadata.json"), metaB, 0o644); err != nil {
		t.Fatal(err)
	}
	findings := []models.Finding{{ID: "f-1", Severity: models.SeverityHigh, MonthlyRiskCost: 42}}
	findingsB, _ := json.Marshal(findings)
	if err := os.WriteFile(filepath.Join(scanPath, "findings.json"), findingsB, 0o644); err != nil {
		t.Fatal(err)
	}

	h := NewQueryHandler(queryv2.NewService(outDir, nil, nil))
	body := bytes.NewBufferString(`{"query":"what should i fix first","scan_id":"scan-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/query", body)
	rec := httptest.NewRecorder()

	h.Query().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["intent"] == "" || got["answer"] == "" {
		t.Fatalf("unexpected response: %v", got)
	}
}
