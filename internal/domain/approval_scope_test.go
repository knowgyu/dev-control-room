package domain

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUnattendedApprovalScopeRequiresBoundedApprovalContract(t *testing.T) {
	now := time.Now().UTC()
	scope := testApprovalScope(t, now)
	if err := scope.Validate(); err != nil {
		t.Fatal(err)
	}
	scope.Spec.Prohibited = []string{"commit"}
	if err := scope.Validate(); err == nil || !strings.Contains(err.Error(), "must prohibit") {
		t.Fatalf("scope without mandatory prohibitions was accepted: %v", err)
	}
}

func TestUnattendedApprovalScopeMatchesOnlyExactBoundedRequest(t *testing.T) {
	now := time.Now().UTC()
	scope := testApprovalScope(t, now)
	approvedAt := now.Add(-time.Second)
	scope.Spec.State = UnattendedScopeApproved
	scope.Spec.ApprovedBy = "local-user"
	scope.Spec.ApprovedAt = &approvedAt
	digest, err := scope.Digest()
	if err != nil {
		t.Fatal(err)
	}
	request := UnattendedApprovalRequest{
		ScopeID:              scope.Metadata.ID,
		ScopeDigest:          digest,
		ProjectID:            scope.Spec.ProjectID,
		RepositoryID:         scope.Spec.RepositoryID,
		WorktreeID:           scope.Spec.WorktreeID,
		ProviderProfile:      scope.Spec.ProviderProfile,
		ActionType:           "repository.refresh",
		Risk:                 RiskSafeLocal,
		Techniques:           []string{QualityTechniqueStaticSecurity},
		ToolVersion:          scope.Spec.ToolVersion,
		ToolConfigDigest:     scope.Spec.ToolConfigDigest,
		ArgumentSchemaDigest: scope.Spec.ArgumentSchemaDigest,
		WritablePaths:        []string{filepath.Join(scope.Spec.WritablePaths[0], "internal")},
		NetworkPolicy:        scope.Spec.NetworkPolicy,
		DiskBytes:            1024,
		Deadline:             now.Add(30 * time.Minute),
		Prohibited:           append([]string(nil), scope.Spec.Prohibited...),
	}
	matched := scope.Match(request, now)
	if !matched.Matched || len(matched.Reasons) != 1 || matched.Reasons[0] != "exact_match" {
		t.Fatalf("exact request did not match: %#v", matched)
	}

	request.ProviderProfile = "other-profile"
	mismatched := scope.Match(request, now)
	if mismatched.Matched || !containsString(mismatched.Reasons, "provider_profile_mismatch") {
		t.Fatalf("provider mismatch was accepted: %#v", mismatched)
	}

	request = requestForApprovalScope(scope, digest, now)
	request.WritablePaths = []string{filepath.Join(filepath.Dir(scope.Spec.WritablePaths[0]), "outside")}
	mismatched = scope.Match(request, now)
	if mismatched.Matched || !containsString(mismatched.Reasons, "writable_path_not_allowed") {
		t.Fatalf("path outside scope was accepted: %#v", mismatched)
	}
}

func TestUnattendedApprovalScopeRejectsExpiredOrRevokedState(t *testing.T) {
	now := time.Now().UTC()
	scope := testApprovalScope(t, now)
	approvedAt := now.Add(-time.Second)
	scope.Spec.State = UnattendedScopeApproved
	scope.Spec.ApprovedBy = "local-user"
	scope.Spec.ApprovedAt = &approvedAt
	digest, err := scope.Digest()
	if err != nil {
		t.Fatal(err)
	}
	request := requestForApprovalScope(scope, digest, now)
	scope.Spec.Deadline = now.Add(-time.Second)
	match := scope.Match(request, now)
	if match.Matched || !containsString(match.Reasons, "scope_expired") {
		t.Fatalf("expired scope was accepted: %#v", match)
	}
	scope.Spec.Deadline = now.Add(time.Hour)
	scope.Spec.State = UnattendedScopeRevoked
	match = scope.Match(request, now)
	if match.Matched || !containsString(match.Reasons, "scope_not_approved") {
		t.Fatalf("revoked scope was accepted: %#v", match)
	}
}

func testApprovalScope(t *testing.T, now time.Time) UnattendedApprovalScope {
	t.Helper()
	root := t.TempDir()
	return UnattendedApprovalScope{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: UnattendedApprovalScopeKind},
		Metadata: ObjectMeta{ID: "scope-1", Name: "Safe local scope"},
		Spec: UnattendedApprovalSpec{
			ProjectID:            "project-a",
			RepositoryID:         "repo-a",
			WorktreeID:           "primary",
			ProviderProfile:      "codex",
			ActionTypes:          []string{"repository.refresh"},
			RiskClasses:          []ActionRisk{RiskSafeLocal},
			Techniques:           []string{QualityTechniqueStaticSecurity},
			ToolVersion:          "go1.26.7",
			ToolConfigDigest:     testApprovalDigest,
			ArgumentSchemaDigest: testApprovalDigest,
			WritablePaths:        []string{filepath.Join(root, "repo")},
			NetworkPolicy:        NetworkPolicyAllowlist,
			DiskLimitBytes:       1 << 20,
			Deadline:             now.Add(time.Hour),
			Prohibited:           append([]string(nil), requiredUnattendedProhibitions...),
			State:                UnattendedScopeDraft,
			Revision:             1,
			CreatedAt:            now.Add(-time.Second),
			UpdatedAt:            now,
		},
	}
}

func requestForApprovalScope(scope UnattendedApprovalScope, digest string, now time.Time) UnattendedApprovalRequest {
	return UnattendedApprovalRequest{
		ScopeID:              scope.Metadata.ID,
		ScopeDigest:          digest,
		ProjectID:            scope.Spec.ProjectID,
		RepositoryID:         scope.Spec.RepositoryID,
		WorktreeID:           scope.Spec.WorktreeID,
		ProviderProfile:      scope.Spec.ProviderProfile,
		ActionType:           scope.Spec.ActionTypes[0],
		Risk:                 scope.Spec.RiskClasses[0],
		Techniques:           append([]string(nil), scope.Spec.Techniques...),
		ToolSetup:            append([]string(nil), scope.Spec.ToolSetup...),
		ToolVersion:          scope.Spec.ToolVersion,
		ToolConfigDigest:     scope.Spec.ToolConfigDigest,
		ArgumentSchemaDigest: scope.Spec.ArgumentSchemaDigest,
		WritablePaths:        []string{filepath.Join(scope.Spec.WritablePaths[0], "internal")},
		NetworkPolicy:        scope.Spec.NetworkPolicy,
		DiskBytes:            1024,
		Deadline:             now.Add(30 * time.Minute),
		Prohibited:           append([]string(nil), scope.Spec.Prohibited...),
	}
}

const testApprovalDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
