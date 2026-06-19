package output

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Zero0x00/cloudrift/internal/models"
)

// SARIF 2.1.0 export. SARIF (Static Analysis Results Interchange Format) is consumed by
// GitHub code scanning and most security tooling, so emitting findings as SARIF lets
// Cloudrift results flow into existing security pipelines (PR annotations, dashboards).
// Spec: https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html

const (
	sarifVersion = "2.1.0"
	sarifSchema  = "https://json.schemastore.org/sarif-2.1.0.json"
	sarifToolURI = "https://github.com/Zero0x00/cloudrift"
)

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool      `json:"tool"`
	Results []sarifResult  `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	ShortDescription sarifText         `json:"shortDescription"`
	Properties       map[string]string `json:"properties,omitempty"`
}

type sarifResult struct {
	RuleID     string           `json:"ruleId"`
	Level      string           `json:"level"`
	Message    sarifText        `json:"message"`
	Locations  []sarifLocation  `json:"locations"`
	Properties map[string]any   `json:"properties,omitempty"`
}

type sarifText struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	LogicalLocations []sarifLogicalLocation `json:"logicalLocations,omitempty"`
}

type sarifLogicalLocation struct {
	Name               string `json:"name"`
	FullyQualifiedName string `json:"fullyQualifiedName,omitempty"`
	Kind               string `json:"kind,omitempty"`
}

// sarifLevel maps Cloudrift severity to the SARIF result level vocabulary.
func sarifLevel(s models.Severity) string {
	switch s {
	case models.SeverityCritical, models.SeverityHigh:
		return "error"
	case models.SeverityMedium:
		return "warning"
	default: // low, info, unknown
		return "note"
	}
}

// WriteSARIF writes findings as a SARIF 2.1.0 log. Each distinct module becomes a rule;
// each finding becomes a result whose level derives from its severity and whose logical
// location is the affected ARN (with the hostname as a friendly name).
func WriteSARIF(path string, findings []models.Finding) error {
	rows := append([]models.Finding(nil), findings...)
	sortFindings(rows)

	ruleIndex := map[string]sarifRule{}
	results := make([]sarifResult, 0, len(rows))
	for _, f := range rows {
		ruleID := string(f.Module)
		if ruleID == "" {
			ruleID = "finding"
		}
		if _, ok := ruleIndex[ruleID]; !ok {
			ruleIndex[ruleID] = sarifRule{
				ID:               ruleID,
				Name:             ruleName(f.Module),
				ShortDescription: sarifText{Text: ruleDescription(f.Module)},
			}
		}

		name := strings.TrimSpace(f.Hostname)
		if name == "" {
			name = f.AffectedARN
		}
		results = append(results, sarifResult{
			RuleID:  ruleID,
			Level:   sarifLevel(f.Severity),
			Message: sarifText{Text: resultMessage(f)},
			Locations: []sarifLocation{{
				LogicalLocations: []sarifLogicalLocation{{
					Name:               name,
					FullyQualifiedName: f.AffectedARN,
					Kind:               "resource",
				}},
			}},
			Properties: map[string]any{
				"severity":               string(f.Severity),
				"claimability":           string(f.Claimability),
				"accountId":              f.AccountID,
				"monthlyRiskCostUsd":     f.MonthlyRiskCost,
				"cloudriftFindingId":     f.ID,
			},
		})
	}

	rules := make([]sarifRule, 0, len(ruleIndex))
	for _, r := range ruleIndex {
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })

	log := sarifLog{
		Schema:  sarifSchema,
		Version: sarifVersion,
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name:           "Cloudrift",
				InformationURI: sarifToolURI,
				Rules:          rules,
			}},
			Results: results,
		}},
	}

	b, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sarif: %w", err)
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func ruleName(m models.Module) string {
	switch m {
	case models.ModuleOrphanedEdge:
		return "Orphaned edge asset"
	case models.ModuleExternalAccess:
		return "Risky external access"
	default:
		return "Finding"
	}
}

func ruleDescription(m models.Module) string {
	switch m {
	case models.ModuleOrphanedEdge:
		return "DNS still points at AWS edge infrastructure whose backing resource is gone or mis-linked (subdomain-takeover risk)."
	case models.ModuleExternalAccess:
		return "Cross-account or federated access (IAM trust or resource policy) that may be stale, over-privileged, or unapproved."
	default:
		return "Cloudrift finding."
	}
}

func resultMessage(f models.Finding) string {
	title := strings.TrimSpace(f.Title)
	if title == "" {
		title = string(f.Claimability)
	}
	if rec := strings.TrimSpace(f.Recommendation); rec != "" {
		return fmt.Sprintf("%s — %s", title, rec)
	}
	return title
}
