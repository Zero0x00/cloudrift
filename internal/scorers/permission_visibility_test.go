package scorers

import (
	"testing"

	"github.com/Zero0x00/cloudrift/internal/models"
)

func TestDeriveRolePermissionVisibility_AdminByManagedPolicyName(t *testing.T) {
	role := testRole(map[string]any{
		"attached_policy_names": []string{"AdministratorAccess"},
	})
	v := DeriveRolePermissionVisibility(role)
	if v.Classification != models.PermissionTierAdmin {
		t.Fatalf("expected admin, got %s", v.Classification)
	}
	if !v.Capabilities.AdminLike {
		t.Fatalf("expected admin_like true")
	}
	if v.Confidence != models.PermissionConfidenceHigh {
		t.Fatalf("expected high confidence, got %s", v.Confidence)
	}
}

func TestDeriveRolePermissionVisibility_AdminByWildcardPolicy(t *testing.T) {
	role := testRole(map[string]any{
		"inline_policy_documents": []string{`{"Statement":[{"Effect":"Allow","Action":"*","Resource":"*"}]}`},
	})
	v := DeriveRolePermissionVisibility(role)
	if v.Classification != models.PermissionTierAdmin {
		t.Fatalf("expected admin, got %s", v.Classification)
	}
}

func TestDeriveRolePermissionVisibility_PrivilegedByIAMWriteAndAssumeRole(t *testing.T) {
	role := testRole(map[string]any{
		"inline_policy_documents": []string{`{"Statement":[{"Effect":"Allow","Action":["iam:CreateRole","sts:AssumeRole"],"Resource":"*"}]}`},
	})
	v := DeriveRolePermissionVisibility(role)
	if v.Classification != models.PermissionTierPrivileged {
		t.Fatalf("expected privileged, got %s", v.Classification)
	}
	if !v.Capabilities.IAMWriteAccess || !v.Capabilities.CanAssumeRole {
		t.Fatalf("expected iam_write_access and can_assume_role true")
	}
}

func TestDeriveRolePermissionVisibility_LimitedReadOnly(t *testing.T) {
	role := testRole(map[string]any{
		"inline_policy_documents": []string{`{"Statement":[{"Effect":"Allow","Action":["s3:GetObject","s3:ListBucket"],"Resource":"*"}]}`},
	})
	v := DeriveRolePermissionVisibility(role)
	if v.Classification != models.PermissionTierLimited {
		t.Fatalf("expected limited, got %s", v.Classification)
	}
}

func TestDeriveRolePermissionVisibility_UnknownOnParseFailure(t *testing.T) {
	role := testRole(map[string]any{
		"inline_policy_documents": []string{"{not-json"},
	})
	v := DeriveRolePermissionVisibility(role)
	if v.Classification != models.PermissionTierUnknown {
		t.Fatalf("expected unknown, got %s", v.Classification)
	}
	if v.PolicyParseOK {
		t.Fatalf("expected policy_parse_ok false")
	}
}

func TestDeriveRolePermissionVisibility_UnknownOnUnsupportedComplexity(t *testing.T) {
	role := testRole(map[string]any{
		"inline_policy_documents": []string{`{"Statement":[{"Effect":"Allow","NotAction":"iam:DeleteRole","Resource":"*"}]}`},
	})
	v := DeriveRolePermissionVisibility(role)
	if v.Classification != models.PermissionTierUnknown {
		t.Fatalf("expected unknown, got %s", v.Classification)
	}
	if !v.ComplexPolicyDetected {
		t.Fatalf("expected complex policy detected")
	}
}

func TestDeriveRolePermissionVisibility_DeterministicMetadataPopulated(t *testing.T) {
	role := testRole(map[string]any{
		"attached_policy_names":  []string{"IAMFullAccess"},
		"inline_policy_documents": []string{`{"Statement":[{"Effect":"Allow","Action":["sts:AssumeRole"],"Resource":"*"}]}`},
	})
	v := DeriveRolePermissionVisibility(role)
	if v.AnalysisMode == "" {
		t.Fatalf("expected analysis_mode to be populated")
	}
	if len(v.Reasons) == 0 {
		t.Fatalf("expected reasons to be populated")
	}
	if v.Confidence == "" {
		t.Fatalf("expected confidence to be populated")
	}
	if !v.UsedManagedPolicyNameHeuristics {
		t.Fatalf("expected managed policy heuristic usage true")
	}
	if v.Confidence != models.PermissionConfidenceLow {
		t.Fatalf("expected low confidence for non-admin managed-policy-name heuristics, got %s", v.Confidence)
	}
	if v.ManagedPolicyDocumentsInspected {
		t.Fatalf("expected managed policy documents inspected false")
	}
}

func TestDeriveRolePermissionVisibility_AdministratorAccessKeepsHighConfidence(t *testing.T) {
	role := testRole(map[string]any{
		"attached_policy_names": []string{"AdministratorAccess"},
	})
	v := DeriveRolePermissionVisibility(role)
	if v.Classification != models.PermissionTierAdmin {
		t.Fatalf("expected admin, got %s", v.Classification)
	}
	if v.Confidence != models.PermissionConfidenceHigh {
		t.Fatalf("expected high confidence, got %s", v.Confidence)
	}
}

func testRole(props map[string]any) models.AssetNode {
	return models.AssetNode{
		ARN:        "arn:aws:iam::111111111111:role/TestRole",
		AssetType:  models.AssetIAMRole,
		Name:       "TestRole",
		AccountID:  "111111111111",
		Region:     "global",
		Properties: props,
	}
}
