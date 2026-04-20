package scorers

import (
	"encoding/json"
	"sort"
	"strings"

	"cloudrift/internal/models"
)

type permissionStatement struct {
	Effect    string `json:"Effect"`
	Action    any    `json:"Action"`
	NotAction any    `json:"NotAction"`
	Resource  any    `json:"Resource"`
}

type permissionPolicyDocument struct {
	Statement any `json:"Statement"`
}

func DeriveRolePermissionVisibility(role models.AssetNode) models.RolePermissionVisibility {
	v := models.RolePermissionVisibility{
		Classification:                  models.PermissionTierUnknown,
		Capabilities:                    models.RoleCapabilityFlags{},
		Reasons:                         []string{},
		Confidence:                      models.PermissionConfidenceLow,
		AnalysisMode:                    models.PermissionAnalysisModeAttachedNamesAndInlineDocs,
		PolicyParseOK:                   true,
		UsedManagedPolicyNameHeuristics: false,
		ComplexPolicyDetected:           false,
		ManagedPolicyDocumentsInspected: false,
	}

	if role.AssetType != models.AssetIAMRole {
		v.Reasons = append(v.Reasons, "asset is not iam_role")
		return v
	}

	attachedNames := stringSliceProperty(role.Properties, "attached_policy_names")
	inlineDocs := stringSliceProperty(role.Properties, "inline_policy_documents")

	if len(attachedNames) == 0 && len(inlineDocs) == 0 {
		v.PolicyParseOK = false
		v.Reasons = append(v.Reasons, "no permission artifacts attached")
		return v
	}

	wildcardAll := false
	iamWrite := false
	canAssumeRole := false
	s3Write := false
	cloudFrontControl := false
	hasAllow := false
	parseFailed := false

	for _, doc := range inlineDocs {
		pd, ok := parsePermissionPolicyDocument(doc)
		if !ok {
			parseFailed = true
			continue
		}
		for _, stmt := range pd {
			if stmt.NotAction != nil {
				v.ComplexPolicyDetected = true
				continue
			}
			if !strings.EqualFold(strings.TrimSpace(stmt.Effect), "allow") {
				continue
			}
			actions := policyStringList(stmt.Action)
			if len(actions) == 0 {
				continue
			}
			hasAllow = true
			resources := policyStringList(stmt.Resource)
			resourceStar := containsResourceStar(resources)
			if hasAction(actions, "*") && resourceStar {
				wildcardAll = true
			}
			if hasActionPrefix(actions, "sts:assumerole") || hasAction(actions, "sts:*") {
				canAssumeRole = true
			}
			if hasActionPrefix(actions, "iam:create") ||
				hasActionPrefix(actions, "iam:update") ||
				hasActionPrefix(actions, "iam:put") ||
				hasActionPrefix(actions, "iam:delete") ||
				hasActionPrefix(actions, "iam:attach") ||
				hasActionPrefix(actions, "iam:detach") ||
				hasAction(actions, "iam:passrole") ||
				hasActionPrefix(actions, "iam:set") ||
				hasAction(actions, "iam:*") {
				iamWrite = true
			}
			if hasActionPrefix(actions, "s3:put") ||
				hasActionPrefix(actions, "s3:delete") ||
				hasAction(actions, "s3:*") {
				s3Write = true
			}
			if hasActionPrefix(actions, "cloudfront:create") ||
				hasActionPrefix(actions, "cloudfront:update") ||
				hasActionPrefix(actions, "cloudfront:delete") ||
				hasAction(actions, "cloudfront:createinvalidation") ||
				hasActionPrefix(actions, "cloudfront:tag") ||
				hasAction(actions, "cloudfront:*") {
				cloudFrontControl = true
			}
		}
	}

	for _, name := range attachedNames {
		n := strings.TrimSpace(strings.ToLower(name))
		if n == "" {
			continue
		}
		switch n {
		case "administratoraccess":
			v.UsedManagedPolicyNameHeuristics = true
			wildcardAll = true
			v.Reasons = append(v.Reasons, "high-confidence managed policy heuristic matched: AdministratorAccess")
		case "iamfullaccess":
			v.UsedManagedPolicyNameHeuristics = true
			iamWrite = true
			v.Reasons = append(v.Reasons, "moderate-confidence managed policy heuristic matched: IAMFullAccess")
		case "amazoncloudfrontfullaccess":
			v.UsedManagedPolicyNameHeuristics = true
			cloudFrontControl = true
			v.Reasons = append(v.Reasons, "capability-level managed policy heuristic matched: AmazonCloudFrontFullAccess")
		case "amazons3fullaccess":
			v.UsedManagedPolicyNameHeuristics = true
			s3Write = true
			v.Reasons = append(v.Reasons, "capability-level managed policy heuristic matched: AmazonS3FullAccess")
		}
	}

	if parseFailed {
		v.PolicyParseOK = false
		v.Reasons = append(v.Reasons, "one or more inline policy documents failed to parse")
	}
	if !hasAllow && len(inlineDocs) > 0 {
		v.Reasons = append(v.Reasons, "no allow statements found in parsed inline policy documents")
	}

	adminLike := wildcardAll
	if !adminLike && canAssumeRole && iamWrite && cloudFrontControl {
		adminLike = true
	}

	v.Capabilities = models.RoleCapabilityFlags{
		CanAssumeRole:     canAssumeRole,
		IAMWriteAccess:    iamWrite,
		S3WriteAccess:     s3Write,
		CloudFrontControl: cloudFrontControl,
		AdminLike:         adminLike,
	}

	switch {
	case wildcardAll:
		v.Classification = models.PermissionTierAdmin
		v.Confidence = models.PermissionConfidenceHigh
		v.Reasons = append(v.Reasons, "high-confidence admin signal detected")
	case adminLike || (iamWrite && canAssumeRole):
		v.Classification = models.PermissionTierPrivileged
		v.Confidence = models.PermissionConfidenceMedium
		v.Reasons = append(v.Reasons, "privileged control-path capabilities detected")
	case iamWrite || s3Write || cloudFrontControl || canAssumeRole:
		v.Classification = models.PermissionTierScoped
		v.Confidence = models.PermissionConfidenceMedium
		v.Reasons = append(v.Reasons, "scoped elevated capabilities detected")
	case hasAllow:
		v.Classification = models.PermissionTierLimited
		v.Confidence = models.PermissionConfidenceMedium
		v.Reasons = append(v.Reasons, "allow statements present without elevated capability flags")
	default:
		v.Classification = models.PermissionTierUnknown
		v.Confidence = models.PermissionConfidenceLow
		v.Reasons = append(v.Reasons, "insufficient policy evidence for deterministic tier")
	}

	if !v.PolicyParseOK && v.Classification != models.PermissionTierAdmin {
		v.Classification = models.PermissionTierUnknown
		v.Confidence = models.PermissionConfidenceLow
		v.Reasons = append(v.Reasons, "parse uncertainty forced conservative unknown classification")
	}
	if v.Classification == models.PermissionTierUnknown {
		v.Reasons = append(v.Reasons, "unknown means analysis confidence is insufficient; treat as caution, not safety")
	}
	if v.Classification == models.PermissionTierLimited {
		v.Reasons = append(v.Reasons, "limited means no elevated flags matched in current conservative domains")
	}
	if v.UsedManagedPolicyNameHeuristics && !wildcardAll {
		v.Reasons = append(v.Reasons, "managed policy name heuristics are non-authoritative without managed policy document inspection")
		if v.Confidence == models.PermissionConfidenceMedium {
			v.Confidence = models.PermissionConfidenceLow
		}
	}

	// Deterministic output for stable tests/artifacts.
	sort.Strings(v.Reasons)
	v.Reasons = dedupeStrings(v.Reasons)
	return v
}

func parsePermissionPolicyDocument(raw string) ([]permissionStatement, bool) {
	var doc permissionPolicyDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, false
	}
	switch s := doc.Statement.(type) {
	case []any:
		out := make([]permissionStatement, 0, len(s))
		for _, item := range s {
			b, err := json.Marshal(item)
			if err != nil {
				return nil, false
			}
			var stmt permissionStatement
			if err := json.Unmarshal(b, &stmt); err != nil {
				return nil, false
			}
			out = append(out, stmt)
		}
		return out, true
	case map[string]any:
		b, err := json.Marshal(s)
		if err != nil {
			return nil, false
		}
		var stmt permissionStatement
		if err := json.Unmarshal(b, &stmt); err != nil {
			return nil, false
		}
		return []permissionStatement{stmt}, true
	default:
		return nil, false
	}
}

func stringSliceProperty(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	raw, ok := m[key]
	if !ok || raw == nil {
		return nil
	}
	switch t := raw.(type) {
	case []string:
		return append([]string(nil), t...)
	case []any:
		out := make([]string, 0, len(t))
		for _, v := range t {
			s, ok := v.(string)
			if ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	default:
		return nil
	}
}

func policyStringList(raw any) []string {
	switch t := raw.(type) {
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		return []string{strings.TrimSpace(strings.ToLower(t))}
	case []any:
		out := make([]string, 0, len(t))
		for _, v := range t {
			s, ok := v.(string)
			if !ok || strings.TrimSpace(s) == "" {
				continue
			}
			out = append(out, strings.TrimSpace(strings.ToLower(s)))
		}
		return out
	case []string:
		out := make([]string, 0, len(t))
		for _, s := range t {
			if strings.TrimSpace(s) == "" {
				continue
			}
			out = append(out, strings.TrimSpace(strings.ToLower(s)))
		}
		return out
	default:
		return nil
	}
}

func hasAction(actions []string, exact string) bool {
	e := strings.ToLower(strings.TrimSpace(exact))
	for _, a := range actions {
		if a == e {
			return true
		}
	}
	return false
}

func hasActionPrefix(actions []string, prefix string) bool {
	p := strings.ToLower(strings.TrimSpace(prefix))
	for _, a := range actions {
		if strings.HasPrefix(a, p) {
			return true
		}
	}
	return false
}

func containsResourceStar(resources []string) bool {
	for _, r := range resources {
		if strings.TrimSpace(r) == "*" {
			return true
		}
	}
	return false
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := make([]string, 0, len(in))
	prev := ""
	for i, s := range in {
		if i == 0 || s != prev {
			out = append(out, s)
		}
		prev = s
	}
	return out
}
