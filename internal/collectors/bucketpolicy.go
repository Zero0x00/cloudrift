package collectors

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"sync"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/Zero0x00/cloudrift/internal/config"
	"github.com/Zero0x00/cloudrift/internal/models"
)

// CollectBucketPolicies inspects S3 bucket resource policies for cross-account or public
// grants — exposure that IAM role-trust scanning (CollectTrust) misses entirely. For each
// bucket it reads the bucket policy and, for every Allow statement granting access to an
// external account or to "*", emits an external-principal node plus a bucket->principal
// relationship (RelTrusts, tagged source_kind=s3_bucket_policy) carrying the granted
// actions. Returns the external-principal nodes, the relationships, and the IDs of accounts
// whose policy read was denied (folded into scan coverage by the caller).
//
// It is given the already-collected bucket assets (from CollectStorage) to avoid a second
// ListBuckets pass; it only needs each owning account's session for GetBucketPolicy.
func CollectBucketPolicies(ctx context.Context, accounts []Account, buckets []models.AssetNode) ([]models.AssetNode, []models.Relationship, []string, error) {
	return CollectBucketPoliciesWithConfig(ctx, config.Default(), accounts, buckets)
}

func CollectBucketPoliciesWithConfig(ctx context.Context, cfg *config.Config, accounts []Account, buckets []models.AssetNode) ([]models.AssetNode, []models.Relationship, []string, error) {
	sessionByAccount := make(map[string]*awsv2.Config, len(accounts))
	for i := range accounts {
		if accounts[i].Session != nil {
			sessionByAccount[accounts[i].ID] = accounts[i].Session
		}
	}

	var (
		mu         sync.Mutex
		principals []models.AssetNode
		rels       []models.Relationship
		failed     []string
		firstErr   error
		seenPrin   = map[string]bool{}
	)
	sem := make(chan struct{}, max(1, cfg.Scan.RoleAssumptionConcurrency))
	var wg sync.WaitGroup

	for _, b := range buckets {
		if b.AssetType != models.AssetS3Bucket {
			continue
		}
		sess := sessionByAccount[b.AccountID]
		if sess == nil {
			continue
		}
		b := b
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			c := s3.NewFromConfig(*sess)
			out, err := c.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: awsv2.String(b.Name)})
			if err != nil {
				// The overwhelmingly common case is "no bucket policy" (NoSuchBucketPolicy),
				// which is not a coverage gap. Only an access-denied read is a real gap.
				if isAccessDeniedErr(err) {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					failed = append(failed, b.AccountID)
					mu.Unlock()
				}
				return
			}
			if out.Policy == nil {
				return
			}
			grants := externalGrantsFromBucketPolicy(b.AccountID, awsv2.ToString(out.Policy))
			if len(grants) == 0 {
				return
			}
			mu.Lock()
			for _, g := range grants {
				if !seenPrin[g.principalARN] {
					seenPrin[g.principalARN] = true
					principals = append(principals, models.AssetNode{
						ARN:       g.principalARN,
						AssetType: models.AssetExternalPrincipal,
						Name:      g.principalValue,
						Properties: map[string]any{
							"principal_type":      g.principalType,
							"principal_value":     g.principalValue,
							"external_account_id": g.externalAccountID,
						},
					})
				}
				rels = append(rels, models.Relationship{
					SourceARN: b.ARN,
					TargetARN: g.principalARN,
					RelType:   models.RelTrusts,
					Properties: map[string]any{
						"source_kind":         "s3_bucket_policy",
						"principal_type":      g.principalType,
						"external_account_id": g.externalAccountID,
						"is_public":           g.isPublic,
						"write_access":        g.write,
						"actions":             g.actions,
					},
				})
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	warnPartial("bucket_policy", firstErr)
	return principals, rels, failed, nil
}

type bucketGrant struct {
	principalType     string // aws_account | public
	principalValue    string
	principalARN      string
	externalAccountID string
	isPublic          bool
	actions           []string
	write             bool
}

// resourceStatement keeps Principal as `any` (unlike the assume-role parser) so the
// string form `"Principal": "*"` used for public buckets is not lost.
type resourceStatement struct {
	Effect    string `json:"Effect"`
	Action    any    `json:"Action"`
	Principal any    `json:"Principal"`
}

func externalGrantsFromBucketPolicy(ownerAccountID, rawPolicy string) []bucketGrant {
	stmts := parseResourceStatements(rawPolicy)
	var grants []bucketGrant
	for _, stmt := range stmts {
		if !strings.EqualFold(strings.TrimSpace(stmt.Effect), "allow") {
			continue
		}
		actions := lowerAll(principalValues(stmt.Action))
		write := actionsImplyWrite(actions)

		// Principal: "*" (string) → public.
		if s, ok := stmt.Principal.(string); ok && strings.TrimSpace(s) == "*" {
			grants = append(grants, publicGrant(actions, write))
			continue
		}
		pm, ok := stmt.Principal.(map[string]any)
		if !ok {
			continue
		}
		for _, p := range principalValues(pm["AWS"]) {
			if strings.TrimSpace(p) == "*" {
				grants = append(grants, publicGrant(actions, write))
				continue
			}
			normalized, keep := normalizeExternalAWSPrincipal(ownerAccountID, p)
			if !keep {
				continue
			}
			grants = append(grants, bucketGrant{
				principalType:     "aws_account",
				principalValue:    normalized,
				principalARN:      externalPrincipalARN("aws_account", normalized),
				externalAccountID: accountIDFromARN(normalized),
				actions:           actions,
				write:             write,
			})
		}
	}
	return grants
}

func publicGrant(actions []string, write bool) bucketGrant {
	return bucketGrant{
		principalType:  "public",
		principalValue: "*",
		principalARN:   externalPrincipalARN("public", "*"),
		isPublic:       true,
		actions:        actions,
		write:          write,
	}
}

func parseResourceStatements(raw string) []resourceStatement {
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		decoded = raw
	}
	var doc struct {
		Statement any `json:"Statement"`
	}
	if json.Unmarshal([]byte(decoded), &doc) != nil {
		return nil
	}
	switch s := doc.Statement.(type) {
	case []any:
		out := make([]resourceStatement, 0, len(s))
		for _, item := range s {
			if rs, ok := toResourceStatement(item); ok {
				out = append(out, rs)
			}
		}
		return out
	case map[string]any:
		if rs, ok := toResourceStatement(s); ok {
			return []resourceStatement{rs}
		}
	}
	return nil
}

func toResourceStatement(item any) (resourceStatement, bool) {
	b, err := json.Marshal(item)
	if err != nil {
		return resourceStatement{}, false
	}
	var rs resourceStatement
	if json.Unmarshal(b, &rs) != nil {
		return resourceStatement{}, false
	}
	return rs, true
}

func actionsImplyWrite(actions []string) bool {
	for _, a := range actions {
		a = strings.TrimSpace(a)
		if a == "*" || a == "s3:*" {
			return true
		}
		if strings.HasPrefix(a, "s3:put") ||
			strings.HasPrefix(a, "s3:delete") ||
			strings.HasPrefix(a, "s3:restore") ||
			strings.HasPrefix(a, "s3:replicate") ||
			strings.HasPrefix(a, "s3:create") {
			return true
		}
	}
	return false
}

func lowerAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, strings.ToLower(strings.TrimSpace(s)))
	}
	return out
}

func accountIDFromARN(arn string) string {
	if m := accountIDInIAMArn.FindStringSubmatch(arn); len(m) == 2 {
		return m[1]
	}
	return ""
}

func isAccessDeniedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "accessdenied") || strings.Contains(msg, "forbidden") || strings.Contains(msg, "not authorized")
}
