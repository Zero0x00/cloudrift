package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cloudrift/internal/alerting"
	"cloudrift/internal/api/schema"
)

func TestAlertingRoutingCatalogGetPut(t *testing.T) {
	dir := t.TempDir()
	svc := alerting.NewService(dir, "http://127.0.0.1:8080")
	h := NewAlertingHandler(dir, svc)

	rr := httptest.NewRecorder()
	h.GetRoutingCatalog().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/alerts/routing", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get routing: %d", rr.Code)
	}
	var got schema.AlertRoutingCatalogResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}

	body := `{"catalog":{"teams":[{"team_id":"t1","display_name":"Team 1","slack_webhook_url":"https://hooks.slack.com/services/a/b/c"}],"account_teams":[{"account_id":"111111111111","team_id":"t1"}],"default_team_id":"t1"}}`
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPut, "/alerts/routing", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	h.PutRoutingCatalog().ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("put routing: %d %s", rr2.Code, rr2.Body.String())
	}

	rr3 := httptest.NewRecorder()
	h.GetRoutingCatalog().ServeHTTP(rr3, httptest.NewRequest(http.MethodGet, "/alerts/routing", nil))
	if err := json.Unmarshal(rr3.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Catalog.DefaultTeamID != "t1" || len(got.Catalog.Teams) != 1 {
		t.Fatalf("catalog %+v", got.Catalog)
	}
}

func TestAlertingRoutingCatalogPutValidation(t *testing.T) {
	dir := t.TempDir()
	svc := alerting.NewService(dir, "http://127.0.0.1:8080")
	h := NewAlertingHandler(dir, svc)
	body := `{"catalog":{"teams":[],"account_teams":[{"account_id":"1","team_id":"ghost"}]}}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/alerts/routing", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.PutRoutingCatalog().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d %s", rr.Code, rr.Body.String())
	}
}
