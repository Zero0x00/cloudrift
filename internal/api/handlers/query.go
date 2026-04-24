package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"cloudrift/internal/api/schema"
	"cloudrift/internal/queryv2"
)

type QueryHandler struct {
	service *queryv2.Service
}

func NewQueryHandler(service *queryv2.Service) *QueryHandler {
	return &QueryHandler{service: service}
}

func (h *QueryHandler) Query() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.service == nil {
			writeError(w, http.StatusServiceUnavailable, "query_service_unavailable", "query service is unavailable", nil)
			return
		}
		var req schema.QueryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request JSON", nil)
			return
		}
		if strings.TrimSpace(req.Query) == "" {
			writeError(w, http.StatusBadRequest, "invalid_query", "query is required", nil)
			return
		}
		resp, err := h.service.Execute(r.Context(), queryv2.QueryRequest{
			Query:       req.Query,
			ScanID:      req.ScanID,
			AccountID:   req.AccountID,
			ModeHint:    req.ModeHint,
			TopK:        req.TopK,
			FindingID:   req.FindingID,
			EntityID:    req.EntityID,
			PrincipalID: req.PrincipalID,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "query_execution_failed", err.Error(), nil)
			return
		}
		out := schema.QueryResponse{
			Answer:              resp.Answer,
			AnswerType:          resp.AnswerType,
			Intent:              string(resp.Intent),
			Confidence:          resp.Confidence,
			SupportLevel:        resp.SupportLevel,
			ScanID:              resp.ScanID,
			GraphUsed:           resp.GraphUsed,
			SemanticUsed:        resp.SemanticUsed,
			DomainUsed:          resp.DomainUsed,
			RecommendedActions:  resp.RecommendedAction,
			FollowUpSuggestions: resp.FollowUps,
			Notes:               resp.Notes,
		}
		for _, f := range resp.SupportingFacts {
			out.SupportingFacts = append(out.SupportingFacts, schema.QuerySupportingFact{
				Label: f.Label, Value: f.Value, Source: f.Source,
			})
		}
		for _, ro := range resp.RelatedObjects {
			out.RelatedObjects = append(out.RelatedObjects, schema.QueryRelatedObject{
				Type: ro.Type, ID: ro.ID, Label: ro.Label, URL: ro.URL,
			})
		}
		writeJSON(w, http.StatusOK, out)
	}
}
