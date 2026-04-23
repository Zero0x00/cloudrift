package blastradius

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cloudrift/internal/api/schema"
	"cloudrift/internal/models"
)

func TestEncodeDecodeExternalEntityID(t *testing.T) {
	id := EncodeExternalEntityID("arn:aws:iam::1:root", "oidc", "ext-9")
	ep, pt, ea, ok := DecodeExternalEntityID(id)
	if !ok || ep != "arn:aws:iam::1:root" || pt != "oidc" || ea != "ext-9" {
		t.Fatalf("decode: ok=%v ep=%q pt=%q ea=%q", ok, ep, pt, ea)
	}
	matched := MatchExternalEntityFindings([]models.Finding{
		{
			Module: models.ModuleExternalAccess,
			Evidence: map[string]any{
				"external_principal":  "arn:aws:iam::1:root",
				"principal_type":      "OIDC",
				"external_account_id": "ext-9",
				"role_arn":            "arn:aws:iam::2:role/R",
			},
			AffectedARN: "arn:aws:iam::2:role/R",
		},
	}, ep, pt, ea)
	if len(matched) != 1 {
		t.Fatalf("match count %d", len(matched))
	}
}

func TestBuildSummaryPayload_unavailableGraph(t *testing.T) {
	s := BuildSummaryPayload(
		schema.BlastRootFinding,
		"f1",
		"scan-a",
		ModeBlastRadius,
		nil,
		"",
		"f1",
		PrivilegeSignals{AdminLike: true},
		false,
		ReasonNeo4jDisabled,
	)
	if s.GraphAvailable || s.ReachableResourceCount != 0 {
		t.Fatalf("graph off: %+v", s)
	}
	if s.GraphUnavailableReason != string(ReasonNeo4jDisabled) {
		t.Fatalf("reason: %q", s.GraphUnavailableReason)
	}
	if s.EscalationPossible != true {
		t.Fatalf("escalation from evidence should pass through when graph nil")
	}
}

func TestBuildSummaryPayload_withGraph(t *testing.T) {
	g := newWorkingGraph()
	g.addTriples([]PathTriple{
		{Src: "arn:a:role/A", Dst: "arn:a:role/B", Type: "TRUSTS"},
		{Src: "arn:a:role/B", Dst: "account:222", Type: "OWNED_BY"},
	})
	s := BuildSummaryPayload(
		schema.BlastRootFinding,
		"f1",
		"scan-a",
		ModeBlastRadius,
		g,
		"arn:a:role/A",
		"f1",
		PrivilegeSignals{},
		true,
		ReasonNone,
	)
	if !s.GraphAvailable || !s.EscalationPossible {
		t.Fatalf("expected trusts escalation: %+v", s)
	}
	if s.ReachableResourceCount < 1 || s.ReachableAccountsCount < 1 {
		t.Fatalf("counts: %+v", s)
	}
}

func TestBuildExplorerPayload_criticalHighlights(t *testing.T) {
	g := newWorkingGraph()
	g.addTriples([]PathTriple{
		{Src: "arn:x:role/F", Dst: "arn:x:role/Y", Type: "TRUSTS"},
	})
	sum := BuildSummaryPayload(schema.BlastRootFinding, "f1", "s", ModeBlastRadius, g, "arn:x:role/F", "f1", PrivilegeSignals{AdminLike: true}, true, ReasonNone)
	ex := BuildExplorerPayload(sum, "arn:x:role/F", ModeBlastRadius, "f1", g)
	if len(ex.Nodes) == 0 || len(ex.Edges) == 0 {
		t.Fatalf("expected nodes/edges, got %d %d", len(ex.Nodes), len(ex.Edges))
	}
	var trustEdge bool
	for _, e := range ex.Edges {
		if e.Type == "ASSUME_ROLE" && e.IsCriticalPath {
			trustEdge = true
		}
	}
	if !trustEdge {
		t.Fatalf("ASSUME_ROLE should be critical path: %#v", ex.Edges)
	}
}

func TestPrincipalIDEncodeDecodeRoundTrip(t *testing.T) {
	arn := "arn:aws:iam::123456789012:role/SecurityAudit"
	pid := EncodePrincipalID(arn, "role", "123456789012")
	gotARN, gotType, gotAcct, ok := DecodePrincipalID(pid)
	if !ok {
		t.Fatalf("expected decode success")
	}
	if gotARN != arn || gotType != "role" || gotAcct != "123456789012" {
		t.Fatalf("unexpected decode values %q %q %q", gotARN, gotType, gotAcct)
	}
}

func TestSemanticEdgeType_TrustVariants(t *testing.T) {
	g := newWorkingGraph()
	g.addNode("arn:aws:iam::111111111111:role/A", "A", "asset", map[string]any{"asset_type": "iam_role", "account_id": "111111111111"})
	g.addNode("arn:aws:iam::222222222222:role/B", "B", "asset", map[string]any{"asset_type": "iam_role", "account_id": "222222222222"})
	g.addNode("arn:aws:iam::333333333333:root", "Root", "asset", map[string]any{"asset_type": "external_principal", "account_id": "333333333333"})
	if got := semanticEdgeType(g, rawEdge{Src: "arn:aws:iam::111111111111:role/A", Tgt: "arn:aws:iam::222222222222:role/B", Type: "TRUSTS"}); got != "CROSS_ACCOUNT_ASSUME_ROLE" {
		t.Fatalf("want CROSS_ACCOUNT_ASSUME_ROLE got %s", got)
	}
	if got := semanticEdgeType(g, rawEdge{Src: "arn:aws:iam::333333333333:root", Tgt: "arn:aws:iam::111111111111:role/A", Type: "TRUSTS"}); got != "EXTERNAL_TRUST" {
		t.Fatalf("want EXTERNAL_TRUST got %s", got)
	}
}

func TestBuildSummaryPayload_escalationFromPrivilegeSignals(t *testing.T) {
	g := newWorkingGraph()
	g.addTriples([]PathTriple{{Src: "arn:aws:iam::111111111111:role/A", Dst: "arn:aws:iam::111111111111:role/B", Type: "POINTS_TO"}})
	s := BuildSummaryPayload(
		schema.BlastRootPrincipal,
		"arn:aws:iam::111111111111:role/A",
		"scan-a",
		ModeAttackPath,
		g,
		"arn:aws:iam::111111111111:role/A",
		"",
		PrivilegeSignals{IAMWriteAccess: true, Classification: "privileged"},
		true,
		ReasonNone,
	)
	if !s.EscalationPossible {
		t.Fatalf("expected escalation true for IAM write signal")
	}
	if !strings.Contains(strings.ToLower(s.SummaryText), "privilege") {
		t.Fatalf("expected privilege narrative, got: %s", s.SummaryText)
	}
}

func TestFindingBlast_nilDriver(t *testing.T) {
	scanID := "scan-t"
	out := t.TempDir()
	writeMinimalScan(t, out, scanID, []models.Finding{{
		ID:          "f1",
		Title:       "t",
		Severity:    models.SeverityCritical,
		Module:      models.ModuleOrphanedEdge,
		AffectedARN: "arn:aws:iam::1:role/X",
	}})
	svc := NewService(nil, out)
	sum, _, _, re := svc.FindingBlast(context.Background(), scanID, "f1", ModeBlastRadius)
	if re != ReasonNeo4jDisabled {
		t.Fatalf("reason %v", re)
	}
	if sum.GraphAvailable || sum.SourceFindingID != "f1" {
		t.Fatalf("sum %+v", sum)
	}
}

func TestPrincipalBlast_nilDriver_usesEvidenceEnricher(t *testing.T) {
	scanID := "scan-p"
	out := t.TempDir()
	principalARN := "arn:aws:iam::111111111111:role/PivotRole"
	writeMinimalScan(t, out, scanID, []models.Finding{{
		ID:          "f-pivot",
		Title:       "external trust on privileged role",
		Severity:    models.SeverityHigh,
		Module:      models.ModuleExternalAccess,
		AffectedARN: principalARN,
		Evidence: map[string]any{
			"external_principal": "arn:aws:iam::999999999999:root",
			"permission_visibility": map[string]any{
				"classification": "privileged",
				"capabilities": map[string]any{
					"iam_write_access": true,
					"admin_like":       true,
				},
			},
		},
	}})
	svc := NewService(nil, out)
	sum, _, re := svc.PrincipalBlast(context.Background(), scanID, principalARN, ModeAttackPath)
	if re != ReasonNeo4jDisabled {
		t.Fatalf("reason %v", re)
	}
	if sum.GraphAvailable {
		t.Fatalf("expected graph unavailable fallback")
	}
	if !sum.EscalationPossible {
		t.Fatalf("expected escalation true from principal evidence enricher")
	}
	if !strings.Contains(strings.ToLower(sum.SummaryText), "confidence: high") {
		t.Fatalf("expected confidence marker in summary text, got: %s", sum.SummaryText)
	}
}

func writeMinimalScan(t *testing.T, outputDir, scanID string, findings []models.Finding) {
	t.Helper()
	d := filepath.Join(outputDir, scanID)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := models.ScanSnapshot{ScanID: scanID, Timestamp: time.Unix(1, 0).UTC(), FindingCount: len(findings)}
	mb, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(d, "scan-metadata.json"), mb, 0o644); err != nil {
		t.Fatal(err)
	}
	fb, _ := json.Marshal(findings)
	if err := os.WriteFile(filepath.Join(d, "findings.json"), fb, 0o644); err != nil {
		t.Fatal(err)
	}
}
