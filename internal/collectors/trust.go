package collectors

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/Zero0x00/cloudrift/internal/config"
	"github.com/Zero0x00/cloudrift/internal/models"
)

var accountIDInIAMArn = regexp.MustCompile(`^arn:aws:iam::([0-9]{12}):`)

type IAMAPI interface {
	ListRoles(ctx context.Context, params *iam.ListRolesInput, optFns ...func(*iam.Options)) (*iam.ListRolesOutput, error)
	ListAttachedRolePolicies(ctx context.Context, params *iam.ListAttachedRolePoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error)
	ListRolePolicies(ctx context.Context, params *iam.ListRolePoliciesInput, optFns ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error)
	GetRolePolicy(ctx context.Context, params *iam.GetRolePolicyInput, optFns ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error)
}

var newIAMClient = func(cfg awsv2.Config) IAMAPI {
	return iam.NewFromConfig(cfg)
}

func CollectTrust(ctx context.Context, accounts []Account) ([]models.AssetNode, []models.Relationship, error) {
	return CollectTrustWithConfig(ctx, config.Default(), accounts)
}

func CollectTrustWithConfig(ctx context.Context, cfg *config.Config, accounts []Account) ([]models.AssetNode, []models.Relationship, error) {
	sem := make(chan struct{}, max(1, cfg.Scan.RoleAssumptionConcurrency))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	nodeByARN := make(map[string]models.AssetNode)
	relByKey := make(map[string]models.Relationship)

	for _, account := range accounts {
		account := account
		if account.Session == nil {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			localNodes, localRels, err := collectTrustFromClient(ctx, account.ID, newIAMClient(*account.Session))
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			for _, n := range localNodes {
				nodeByARN[n.ARN] = n
			}
			for _, r := range localRels {
				key := r.SourceARN + "|" + string(r.RelType) + "|" + r.TargetARN
				relByKey[key] = r
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	if firstErr != nil {
		return nil, nil, firstErr
	}

	nodes := make([]models.AssetNode, 0, len(nodeByARN))
	for _, node := range nodeByARN {
		nodes = append(nodes, node)
	}
	rels := make([]models.Relationship, 0, len(relByKey))
	for _, rel := range relByKey {
		rels = append(rels, rel)
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].ARN == nodes[j].ARN {
			return nodes[i].AssetType < nodes[j].AssetType
		}
		return nodes[i].ARN < nodes[j].ARN
	})
	sort.Slice(rels, func(i, j int) bool {
		if rels[i].SourceARN == rels[j].SourceARN {
			return rels[i].TargetARN < rels[j].TargetARN
		}
		return rels[i].SourceARN < rels[j].SourceARN
	})

	return nodes, rels, nil
}

func collectTrustFromClient(ctx context.Context, accountID string, client IAMAPI) ([]models.AssetNode, []models.Relationship, error) {
	var roleNodes []models.AssetNode
	var principalNodes []models.AssetNode
	var rels []models.Relationship

	var marker *string
	for {
		out, err := client.ListRoles(ctx, &iam.ListRolesInput{Marker: marker})
		if err != nil {
			return nil, nil, err
		}

		for _, role := range out.Roles {
			principals := extractExternalPrincipals(accountID, awsv2.ToString(role.AssumeRolePolicyDocument))
			if len(principals) == 0 {
				continue
			}

			roleNode := models.AssetNode{
				ARN:       awsv2.ToString(role.Arn),
				AssetType: models.AssetIAMRole,
				Name:      awsv2.ToString(role.RoleName),
				AccountID: accountID,
				Region:    "global",
				Properties: map[string]any{
					"path": awsv2.ToString(role.Path),
				},
			}
			attachedPolicyNames, inlinePolicyDocuments, parseOK := collectRolePermissionInputs(ctx, client, roleNode.Name)
			roleNode.Properties["attached_policy_names"] = attachedPolicyNames
			roleNode.Properties["inline_policy_documents"] = inlinePolicyDocuments
			roleNode.Properties["policy_parse_ok"] = parseOK
			roleNodes = append(roleNodes, roleNode)

			for _, p := range principals {
				extNode := models.AssetNode{
					ARN:       externalPrincipalARN(p.PrincipalType, p.Value),
					AssetType: models.AssetExternalPrincipal,
					Name:      p.Value,
					AccountID: accountID,
					Region:    "global",
					Properties: map[string]any{
						"principal_type": p.PrincipalType,
						"principal_value": p.Value,
					},
				}
				principalNodes = append(principalNodes, extNode)
				rels = append(rels, models.Relationship{
					SourceARN: roleNode.ARN,
					TargetARN: extNode.ARN,
					RelType:   models.RelTrusts,
					Properties: map[string]any{
						"principal_type": p.PrincipalType,
						"principal_value": p.Value,
					},
				})
			}
		}

		if !out.IsTruncated || out.Marker == nil {
			break
		}
		marker = out.Marker
	}

	nodes := append(roleNodes, principalNodes...)
	return nodes, rels, nil
}

type externalPrincipal struct {
	PrincipalType string
	Value         string
}

func extractExternalPrincipals(accountID, rawPolicy string) []externalPrincipal {
	doc, ok := parsePolicyDocument(rawPolicy)
	if !ok {
		return nil
	}

	var principals []externalPrincipal
	for _, statement := range doc.Statement {
		if !statementAllowsAssumeRole(statement) {
			continue
		}

		awsValues := principalValues(statement.Principal["AWS"])
		for _, v := range awsValues {
			if normalized, keep := normalizeExternalAWSPrincipal(accountID, v); keep {
				principals = append(principals, externalPrincipal{
					PrincipalType: "aws_account",
					Value:         normalized,
				})
			}
		}

		federatedValues := principalValues(statement.Principal["Federated"])
		for _, v := range federatedValues {
			pt := classifyFederatedPrincipal(v)
			if pt == "" {
				continue
			}
			principals = append(principals, externalPrincipal{
				PrincipalType: pt,
				Value:         strings.TrimSpace(v),
			})
		}
	}

	if len(principals) == 0 {
		return nil
	}

	dedup := make(map[string]externalPrincipal)
	for _, p := range principals {
		key := p.PrincipalType + "|" + p.Value
		dedup[key] = p
	}
	out := make([]externalPrincipal, 0, len(dedup))
	for _, p := range dedup {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PrincipalType == out[j].PrincipalType {
			return out[i].Value < out[j].Value
		}
		return out[i].PrincipalType < out[j].PrincipalType
	})
	return out
}

type policyDocument struct {
	Statement []policyStatement
}

type policyStatement struct {
	Effect    string
	Action    any
	Principal map[string]any
}

func parsePolicyDocument(raw string) (policyDocument, bool) {
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		decoded = raw
	}

	var anyDoc map[string]any
	if err := json.Unmarshal([]byte(decoded), &anyDoc); err != nil {
		return policyDocument{}, false
	}

	statementsAny, ok := anyDoc["Statement"]
	if !ok {
		return policyDocument{}, false
	}

	var statementMaps []map[string]any
	switch typed := statementsAny.(type) {
	case map[string]any:
		statementMaps = append(statementMaps, typed)
	case []any:
		for _, item := range typed {
			if m, ok := item.(map[string]any); ok {
				statementMaps = append(statementMaps, m)
			}
		}
	default:
		return policyDocument{}, false
	}

	out := policyDocument{Statement: make([]policyStatement, 0, len(statementMaps))}
	for _, m := range statementMaps {
		stmt := policyStatement{
			Effect: strings.TrimSpace(stringValue(m["Effect"])),
			Action: m["Action"],
		}
		if principalMap, ok := m["Principal"].(map[string]any); ok {
			stmt.Principal = principalMap
		} else {
			stmt.Principal = map[string]any{}
		}
		out.Statement = append(out.Statement, stmt)
	}
	return out, true
}

func statementAllowsAssumeRole(statement policyStatement) bool {
	if !strings.EqualFold(statement.Effect, "Allow") {
		return false
	}
	actions := principalValues(statement.Action)
	for _, action := range actions {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(action)), "sts:assumerole") {
			return true
		}
	}
	return false
}

func principalValues(raw any) []string {
	switch typed := raw.(type) {
	case string:
		v := strings.TrimSpace(typed)
		if v == "" {
			return nil
		}
		return []string{v}
	case []string:
		var out []string
		for _, item := range typed {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []any:
		var out []string
		for _, item := range typed {
			s, ok := item.(string)
			if !ok {
				continue
			}
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func normalizeExternalAWSPrincipal(accountID, value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "*" {
		return "", false
	}

	if len(value) == 12 && isDigits(value) {
		if value == accountID {
			return "", false
		}
		return "arn:aws:iam::" + value + ":root", true
	}

	m := accountIDInIAMArn.FindStringSubmatch(value)
	if len(m) == 2 {
		if m[1] == accountID {
			return "", false
		}
		return value, true
	}
	return "", false
}

func classifyFederatedPrincipal(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.HasSuffix(lower, ".amazonaws.com") || strings.Contains(lower, ".amazonaws.com") {
		return ""
	}
	if strings.Contains(lower, "saml-provider/") {
		return "saml"
	}
	return "oidc"
}

func externalPrincipalARN(principalType, principalValue string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(strings.TrimSpace(principalValue)))
	return "arn:cloudrift:external-principal:::" + principalType + "/" + encoded
}

func collectRolePermissionInputs(ctx context.Context, client IAMAPI, roleName string) ([]string, []string, bool) {
	attachedPolicyNames := make([]string, 0, 4)
	inlinePolicyDocuments := make([]string, 0, 4)
	parseOK := true

	var attachedMarker *string
	for {
		out, err := client.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
			RoleName: awsv2.String(roleName),
			Marker:   attachedMarker,
		})
		if err != nil {
			parseOK = false
			break
		}
		for _, ap := range out.AttachedPolicies {
			name := strings.TrimSpace(awsv2.ToString(ap.PolicyName))
			if name != "" {
				attachedPolicyNames = append(attachedPolicyNames, name)
			}
		}
		if !out.IsTruncated || out.Marker == nil {
			break
		}
		attachedMarker = out.Marker
	}

	inlinePolicyNames := make([]string, 0, 4)
	var inlineMarker *string
	for {
		out, err := client.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{
			RoleName: awsv2.String(roleName),
			Marker:   inlineMarker,
		})
		if err != nil {
			parseOK = false
			break
		}
		for _, pn := range out.PolicyNames {
			if s := strings.TrimSpace(pn); s != "" {
				inlinePolicyNames = append(inlinePolicyNames, s)
			}
		}
		if !out.IsTruncated || out.Marker == nil {
			break
		}
		inlineMarker = out.Marker
	}

	for _, pn := range inlinePolicyNames {
		out, err := client.GetRolePolicy(ctx, &iam.GetRolePolicyInput{
			RoleName:   awsv2.String(roleName),
			PolicyName: awsv2.String(pn),
		})
		if err != nil {
			parseOK = false
			continue
		}
		decoded, err := url.QueryUnescape(awsv2.ToString(out.PolicyDocument))
		if err != nil {
			parseOK = false
			continue
		}
		inlinePolicyDocuments = append(inlinePolicyDocuments, decoded)
	}

	sort.Strings(attachedPolicyNames)
	sort.Strings(inlinePolicyDocuments)
	return attachedPolicyNames, inlinePolicyDocuments, parseOK
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func isDigits(v string) bool {
	for _, ch := range v {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
