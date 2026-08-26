package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestUnattendedApprovalScopeLifecycleBindsPlanAndReportsRevocation(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	repository := tempGitRepository(t, "approval-scope")
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Approval scope", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	deadline := now.Add(time.Hour)
	scope, err := service.CreateUnattendedApprovalScope(context.Background(), UnattendedApprovalScopeInput{
		Name:                 "Codex local refresh",
		ProjectID:            project.Metadata.ID,
		RepositoryID:         "repo-1",
		WorktreeID:           "primary",
		ProviderProfile:      "codex",
		ActionTypes:          []string{"repository.refresh"},
		RiskClasses:          []domain.ActionRisk{domain.RiskSafeLocal},
		Techniques:           []string{domain.QualityTechniqueStaticSecurity},
		ToolVersion:          "go1.26.7",
		ToolConfigDigest:     appApprovalDigest,
		ArgumentSchemaDigest: appApprovalDigest,
		WritablePaths:        []string{repository},
		NetworkPolicy:        domain.NetworkPolicyAllowlist,
		DiskLimitBytes:       1 << 20,
		Deadline:             deadline,
		Prohibited:           []string{"ci_edit", "commit", "delete", "pull_request", "push", "remote_dispatch", "scope_expansion"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if scope.Spec.State != domain.UnattendedScopeDraft || scope.Spec.Revision != 1 {
		t.Fatalf("created scope = %#v", scope.Spec)
	}
	approveRequest := httptest.NewRequest(http.MethodPost, "/api/assurance/approval-scopes/"+scope.Metadata.ID+"/approve", nil)
	approveRecorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(approveRecorder, approveRequest)
	if approveRecorder.Code != http.StatusForbidden {
		t.Fatalf("unprotected approval = %d: %s", approveRecorder.Code, approveRecorder.Body.String())
	}
	approveRequest = httptest.NewRequest(http.MethodPost, "/api/assurance/approval-scopes/"+scope.Metadata.ID+"/approve", bytes.NewReader(nil))
	approveRequest.Header.Set("X-Control-Room-Token", service.mutationToken)
	approveRecorder = httptest.NewRecorder()
	service.Handler().ServeHTTP(approveRecorder, approveRequest)
	var scopeEnvelope contract.Envelope[domain.UnattendedApprovalScope]
	if approveRecorder.Code != http.StatusOK || json.Unmarshal(approveRecorder.Body.Bytes(), &scopeEnvelope) != nil || scopeEnvelope.Data == nil {
		t.Fatalf("approval HTTP = %d: %s", approveRecorder.Code, approveRecorder.Body.String())
	}
	scope = *scopeEnvelope.Data
	if err != nil {
		t.Fatal(err)
	}
	if scope.Spec.State != domain.UnattendedScopeApproved || scope.Spec.ApprovedBy != "local-user" || scope.Spec.Revision != 2 {
		t.Fatalf("approved scope = %#v", scope.Spec)
	}
	listed, err := service.UnattendedApprovalScopes(context.Background())
	if err != nil || len(listed) != 1 || listed[0].Metadata.ID != scope.Metadata.ID {
		t.Fatalf("scopes = %#v, %v", listed, err)
	}

	plan, err := service.PlanAction(context.Background(), ActionPlanInput{
		ID:                   "scope-plan",
		Name:                 "Scoped refresh",
		ProjectID:            project.Metadata.ID,
		RepositoryID:         "repo-1",
		WorktreeID:           "primary",
		ActionType:           "repository.refresh",
		ApprovalScopeID:      scope.Metadata.ID,
		ProviderProfile:      "codex",
		Techniques:           []string{domain.QualityTechniqueStaticSecurity},
		ToolVersion:          "go1.26.7",
		ToolConfigDigest:     appApprovalDigest,
		ArgumentSchemaDigest: appApprovalDigest,
		WritablePaths:        []string{filepath.Join(repository, "internal")},
		NetworkPolicy:        domain.NetworkPolicyAllowlist,
		DiskLimitBytes:       1024,
		ScopeDeadline:        deadline,
		ProhibitedOperations: []string{"ci_edit", "commit", "delete", "pull_request", "push", "remote_dispatch", "scope_expansion"},
	})
	if err != nil || !plan.Spec.ScopeMatch {
		t.Fatalf("scoped plan = %#v, %v", plan.Spec, err)
	}
	checkRequest := httptest.NewRequest(http.MethodGet, "/api/assurance/action-plans/"+plan.Metadata.ID+"/approval-scope", nil)
	checkRecorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(checkRecorder, checkRequest)
	var matchEnvelope contract.Envelope[domain.UnattendedApprovalMatch]
	if checkRecorder.Code != http.StatusOK || json.Unmarshal(checkRecorder.Body.Bytes(), &matchEnvelope) != nil || matchEnvelope.Data == nil || !matchEnvelope.Data.Matched || len(matchEnvelope.Data.Reasons) != 1 || matchEnvelope.Data.Reasons[0] != "exact_match" {
		t.Fatalf("scope check HTTP = %d: %s", checkRecorder.Code, checkRecorder.Body.String())
	}

	revokeRequest := httptest.NewRequest(http.MethodPost, "/api/assurance/approval-scopes/"+scope.Metadata.ID+"/revoke", bytes.NewReader(nil))
	revokeRequest.Header.Set("X-Control-Room-Token", service.mutationToken)
	revokeRecorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(revokeRecorder, revokeRequest)
	if revokeRecorder.Code != http.StatusOK || json.Unmarshal(revokeRecorder.Body.Bytes(), &scopeEnvelope) != nil || scopeEnvelope.Data == nil {
		t.Fatalf("revocation HTTP = %d: %s", revokeRecorder.Code, revokeRecorder.Body.String())
	}
	scope = *scopeEnvelope.Data
	if err != nil {
		t.Fatal(err)
	}
	if scope.Spec.State != domain.UnattendedScopeRevoked || scope.Spec.Revision != 3 {
		t.Fatalf("revoked scope = %#v", scope.Spec)
	}
	checkRecorder = httptest.NewRecorder()
	service.Handler().ServeHTTP(checkRecorder, checkRequest)
	if checkRecorder.Code != http.StatusOK || json.Unmarshal(checkRecorder.Body.Bytes(), &matchEnvelope) != nil || matchEnvelope.Data == nil || matchEnvelope.Data.Matched || !hasAppApprovalString(matchEnvelope.Data.Reasons, "scope_not_approved") {
		t.Fatalf("revoked scope check HTTP = %d: %s", checkRecorder.Code, checkRecorder.Body.String())
	}
	_, err = service.PlanAction(context.Background(), ActionPlanInput{
		ID:                   "revoked-scope-plan",
		Name:                 "Revoked refresh",
		ProjectID:            project.Metadata.ID,
		RepositoryID:         "repo-1",
		WorktreeID:           "primary",
		ActionType:           "repository.refresh",
		ApprovalScopeID:      scope.Metadata.ID,
		ProviderProfile:      "codex",
		Techniques:           []string{domain.QualityTechniqueStaticSecurity},
		ToolVersion:          "go1.26.7",
		ToolConfigDigest:     appApprovalDigest,
		ArgumentSchemaDigest: appApprovalDigest,
		WritablePaths:        []string{filepath.Join(repository, "internal")},
		NetworkPolicy:        domain.NetworkPolicyAllowlist,
		DiskLimitBytes:       1024,
		ScopeDeadline:        deadline,
		ProhibitedOperations: []string{"ci_edit", "commit", "delete", "pull_request", "push", "remote_dispatch", "scope_expansion"},
	})
	if contract.Classify(err).Code != contract.ErrorPolicyDenied {
		t.Fatalf("revoked scope plan error = %v", err)
	}
}

func hasAppApprovalString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

const appApprovalDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
