package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"github.com/Zero0x00/cloudrift/internal/models"
)

// Statement is one parameterized Cypher write for deterministic batching and tests.
type Statement struct {
	Cypher string
	Params map[string]any
}

// Execer runs a single Cypher write (tests use fakes; production uses DriverExecer).
type Execer interface {
	Run(ctx context.Context, cypher string, params map[string]any) error
}

// RunStatements executes writes in order. Empty stmts is a no-op.
func RunStatements(ctx context.Context, ex Execer, stmts []Statement) error {
	for _, s := range stmts {
		params := s.Params
		if params == nil {
			params = map[string]any{}
		}
		if err := ex.Run(ctx, s.Cypher, params); err != nil {
			return err
		}
	}
	return nil
}

// CompileWriteScan builds ordered MERGE/SET statements from existing scan artifacts.
// JSON files remain source of truth; this graph projection is derived only.
//
// Asset modeling note:
//   - All infrastructure resources are stored under a single :Asset label.
//   - models.AssetNode.AssetType is persisted as `asset_type` and is the discriminator.
//   - There are intentionally no per-type labels like :DnsRecord or :S3Bucket in this phase.
//
// OWNED_BY projection note:
//   - Canonical ownership is always projected from AssetNode.account_id.
//   - Relationship rows with RelOwnedBy are additionally projected when TargetARN looks like an
//     IAM account-root ARN (arn:aws:iam::<account-id>:root or :*), mapping to AwsAccount.
//
// JSON map fields note:
//   - Asset properties and finding evidence are persisted as JSON strings
//     (`properties_json`, `evidence_json`) for deterministic writes and model fidelity.
//   - Nested map keys are not promoted to first-class graph properties in this foundation step.
//
// Order: AwsAccount → ScanSnapshot → Asset → Finding → Asset-OWNED_BY → graph rels → CAPTURED → AFFECTS.
func CompileWriteScan(
	meta models.ScanSnapshot,
	assets []models.AssetNode,
	rels []models.Relationship,
	findings []models.Finding,
) []Statement {
	var out []Statement

	accountIDs := collectAccountIDs(meta, assets, findings)
	for _, id := range accountIDs {
		out = append(out, Statement{
			Cypher: `MERGE (a:AwsAccount {account_id: $account_id})`,
			Params: map[string]any{"account_id": id},
		})
	}

	out = append(out, mergeScanSnapshotStatement(meta))

	sortedAssets := append([]models.AssetNode(nil), assets...)
	sort.Slice(sortedAssets, func(i, j int) bool { return sortedAssets[i].ARN < sortedAssets[j].ARN })
	for _, n := range sortedAssets {
		out = append(out, mergeAssetStatement(n))
	}

	sortedFindings := append([]models.Finding(nil), findings...)
	sort.Slice(sortedFindings, func(i, j int) bool { return sortedFindings[i].ID < sortedFindings[j].ID })
	for _, f := range sortedFindings {
		out = append(out, mergeFindingStatement(f))
	}

	for _, n := range sortedAssets {
		if strings.TrimSpace(n.AccountID) == "" {
			continue
		}
		out = append(out, Statement{
			Cypher: `
MATCH (x:Asset {arn: $arn})
MATCH (acc:AwsAccount {account_id: $account_id})
MERGE (x)-[:OWNED_BY]->(acc)
`,
			Params: map[string]any{"arn": n.ARN, "account_id": n.AccountID},
		})
	}

	sortedRels := append([]models.Relationship(nil), rels...)
	sort.Slice(sortedRels, func(i, j int) bool {
		if sortedRels[i].SourceARN != sortedRels[j].SourceARN {
			return sortedRels[i].SourceARN < sortedRels[j].SourceARN
		}
		if sortedRels[i].TargetARN != sortedRels[j].TargetARN {
			return sortedRels[i].TargetARN < sortedRels[j].TargetARN
		}
		return sortedRels[i].RelType < sortedRels[j].RelType
	})
	for _, r := range sortedRels {
		if r.RelType == models.RelOwnedBy {
			if accID, ok := accountIDFromIAMRootARN(r.TargetARN); ok {
				out = append(out, Statement{
					Cypher: `
MATCH (src:Asset {arn: $src_arn})
MERGE (acc:AwsAccount {account_id: $account_id})
MERGE (src)-[:OWNED_BY]->(acc)
`,
					Params: map[string]any{"src_arn": r.SourceARN, "account_id": accID},
				})
			}
			continue
		}
		typ, err := cypherRelType(r.RelType)
		if err != nil {
			continue
		}
		out = append(out, Statement{
			Cypher: fmt.Sprintf(`
MATCH (src:Asset {arn: $src_arn})
MATCH (dst:Asset {arn: $dst_arn})
MERGE (src)-[:%s]->(dst)
`, typ),
			Params: map[string]any{"src_arn": r.SourceARN, "dst_arn": r.TargetARN},
		})
	}

	for _, f := range sortedFindings {
		out = append(out, Statement{
			Cypher: `
MATCH (s:ScanSnapshot {scan_id: $scan_id})
MATCH (f:Finding {id: $finding_id})
MERGE (s)-[:CAPTURED]->(f)
`,
			Params: map[string]any{"scan_id": meta.ScanID, "finding_id": f.ID},
		})
	}

	assetARNs := make(map[string]struct{}, len(sortedAssets))
	for _, n := range sortedAssets {
		assetARNs[n.ARN] = struct{}{}
	}
	for _, f := range sortedFindings {
		if _, ok := assetARNs[f.AffectedARN]; !ok {
			continue
		}
		out = append(out, Statement{
			Cypher: `
MATCH (fn:Finding {id: $finding_id})
MATCH (ax:Asset {arn: $affected_arn})
MERGE (fn)-[:AFFECTS]->(ax)
`,
			Params: map[string]any{"finding_id": f.ID, "affected_arn": f.AffectedARN},
		})
	}

	return out
}

// WriteScan compiles and runs all writes (schema must be applied separately).
func WriteScan(ctx context.Context, ex Execer, meta models.ScanSnapshot, assets []models.AssetNode, rels []models.Relationship, findings []models.Finding) error {
	return RunStatements(ctx, ex, CompileWriteScan(meta, assets, rels, findings))
}

func collectAccountIDs(meta models.ScanSnapshot, assets []models.AssetNode, findings []models.Finding) []string {
	seen := make(map[string]struct{})
	var ids []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for _, id := range meta.AccountIDs {
		add(id)
	}
	for _, a := range assets {
		add(a.AccountID)
	}
	for _, f := range findings {
		add(f.AccountID)
	}
	sort.Strings(ids)
	return ids
}

func mergeScanSnapshotStatement(meta models.ScanSnapshot) Statement {
	params := map[string]any{
		"scan_id":                meta.ScanID,
		"timestamp":              meta.Timestamp.UTC().Format(time.RFC3339Nano),
		"tool_version":           meta.ToolVersion,
		"finding_count":          meta.FindingCount,
		"critical_count":         meta.CriticalCount,
		"high_count":             meta.HighCount,
		"total_monthly_cost_usd": meta.TotalMonthlyCost,
		"account_ids":            append([]string(nil), meta.AccountIDs...),
	}
	embLines := ""
	gm := GraphEmbeddingMetaFromScanSnapshot(meta)
	// Require full identity (provider, non-empty model, positive dims) so Neo4j never stores
	// half-written metadata that retrieval would reject as incompatible.
	if gm.HasIdentity() && strings.TrimSpace(gm.Model) != "" {
		embLines = `,
    s.embedding_provider = $embedding_provider,
    s.embedding_model = $embedding_model,
    s.embedding_dimensions = $embedding_dimensions`
		params["embedding_provider"] = gm.Provider
		params["embedding_model"] = gm.Model
		params["embedding_dimensions"] = gm.Dimensions
	}
	return Statement{
		Cypher: fmt.Sprintf(`
MERGE (s:ScanSnapshot {scan_id: $scan_id})
SET s.timestamp = $timestamp,
    s.tool_version = $tool_version,
    s.finding_count = $finding_count,
    s.critical_count = $critical_count,
    s.high_count = $high_count,
    s.total_monthly_cost_usd = $total_monthly_cost_usd,
    s.account_ids = $account_ids%s
`, embLines),
		Params: params,
	}
}

var iamRootARN = regexp.MustCompile(`^arn:aws:iam::(\d{12}):(root|\*)$`)

func accountIDFromIAMRootARN(arn string) (string, bool) {
	m := iamRootARN.FindStringSubmatch(strings.TrimSpace(arn))
	if len(m) < 2 {
		return "", false
	}
	return m[1], true
}

func mergeAssetStatement(n models.AssetNode) Statement {
	propsJSON, _ := json.Marshal(n.Properties)
	return Statement{
		Cypher: `
MERGE (x:Asset {arn: $arn})
SET x.asset_type = $asset_type,
    x.name = $name,
    x.account_id = $account_id,
    x.region = $region,
    x.scan_id = $scan_id,
    x.properties_json = $properties_json
`,
		Params: map[string]any{
			"arn":             n.ARN,
			"asset_type":      string(n.AssetType),
			"name":            n.Name,
			"account_id":      n.AccountID,
			"region":          n.Region,
			"scan_id":         n.ScanID,
			"properties_json": string(propsJSON),
		},
	}
}

func mergeFindingStatement(f models.Finding) Statement {
	ev, _ := json.Marshal(f.Evidence)
	embLine := ""
	params := map[string]any{
		"id":                      f.ID,
		"title":                   f.Title,
		"severity":                string(f.Severity),
		"module":                  string(f.Module),
		"claimability":            string(f.Claimability),
		"affected_arn":            f.AffectedARN,
		"account_id":              f.AccountID,
		"account_name":            f.AccountName,
		"ou_path":                 f.OUPath,
		"team":                    f.Team,
		"hostname":                f.Hostname,
		"monthly_direct_cost_usd": f.MonthlyDirectCost,
		"monthly_risk_cost_usd":   f.MonthlyRiskCost,
		"impact":                  f.Impact,
		"recommendation":          f.Recommendation,
		"remediation_command":     f.RemediationCmd,
		"evidence_json":           string(ev),
		"scan_id":                 f.ScanID,
	}
	if len(f.Embedding) == ExpectedVectorDimensions {
		emb := make([]float64, len(f.Embedding))
		for i, v := range f.Embedding {
			emb[i] = float64(v)
		}
		params["embedding"] = emb
		embLine = "    f.embedding = $embedding,\n"
	}
	return Statement{
		Cypher: fmt.Sprintf(`
MERGE (f:Finding {id: $id})
SET f.title = $title,
    f.severity = $severity,
    f.module = $module,
    f.claimability = $claimability,
    f.affected_arn = $affected_arn,
    f.account_id = $account_id,
    f.account_name = $account_name,
    f.ou_path = $ou_path,
    f.team = $team,
    f.hostname = $hostname,
    f.monthly_direct_cost_usd = $monthly_direct_cost_usd,
    f.monthly_risk_cost_usd = $monthly_risk_cost_usd,
    f.impact = $impact,
    f.recommendation = $recommendation,
    f.remediation_command = $remediation_command,
    f.evidence_json = $evidence_json,
%s    f.scan_id = $scan_id
`, embLine),
		Params: params,
	}
}

func cypherRelType(rt models.RelType) (string, error) {
	switch rt {
	case models.RelPointsTo:
		return "POINTS_TO", nil
	case models.RelUsesCert:
		return "USES_CERT", nil
	case models.RelFronts:
		return "FRONTS", nil
	case models.RelTrusts:
		return "TRUSTS", nil
	default:
		return "", fmt.Errorf("unsupported rel type: %q", rt)
	}
}

// driverExecer implements Execer using neo4j.DriverWithContext (one session per statement for simplicity).
type driverExecer struct {
	driver neo4j.DriverWithContext
	dbName string
}

// NewDriverExecer returns an Execer that runs each statement in a short write transaction.
func NewDriverExecer(driver neo4j.DriverWithContext, databaseName string) Execer {
	return &driverExecer{driver: driver, dbName: databaseName}
}

func (d *driverExecer) Run(ctx context.Context, cypher string, params map[string]any) error {
	session := d.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeWrite,
		DatabaseName: d.dbName,
	})
	defer session.Close(ctx)
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, strings.TrimSpace(cypher), params)
		return nil, err
	})
	return err
}
