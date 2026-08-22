package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestProjectContractAndValidation(t *testing.T) {
	project := NewProject("sample-project", "Sample Project", []Repository{
		NewRepository("backend", "Backend", `C:\work\backend`),
		NewRepository("frontend", "Frontend", `C:\work\frontend`),
	})
	if err := project.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(project)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, field := range []string{`"apiVersion":"devroom/v1alpha1"`, `"kind":"Project"`, `"repositories"`, `"path":"C:\\work\\backend"`} {
		if !strings.Contains(text, field) {
			t.Fatalf("contract field %q missing from %s", field, text)
		}
	}
}

func TestFindingRejectsInvalidLifecycleValues(t *testing.T) {
	now := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	finding := Finding{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: FindingKind},
		Metadata: ObjectMeta{ID: "finding-1", Name: "dirty repository"},
		Spec: FindingSpec{
			ProjectID: "project-1", FindingType: "git.dirty", Fingerprint: "abc",
			Severity: Severity("urgent"), Confidence: ConfidenceConfirmed, State: FindingOpen,
			Summary: "repository has changes", RecommendedNext: "review the diff",
			FirstObserved: now, LastObserved: now,
		},
	}
	if err := finding.Validate(); err == nil {
		t.Fatal("expected invalid severity to be rejected")
	}
}

func TestFindingRequiresUsefulSummaryAndOrderedTimes(t *testing.T) {
	now := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	finding := Finding{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: FindingKind},
		Metadata: ObjectMeta{ID: "finding-1", Name: "dirty repository"},
		Spec: FindingSpec{
			ProjectID: "project-1", FindingType: "git.dirty", Fingerprint: "abc",
			Severity: SeverityAttention, Confidence: ConfidenceConfirmed, State: FindingOpen,
			Summary: "repository has changes", RecommendedNext: "review the diff",
			FirstObserved: now, LastObserved: now.Add(time.Minute),
		},
	}
	if err := finding.Validate(); err != nil {
		t.Fatal(err)
	}
	finding.Spec.Summary = ""
	if err := finding.Validate(); err == nil {
		t.Fatal("expected an empty summary to fail")
	}
	finding.Spec.Summary = "repository has changes"
	finding.Spec.LastObserved = now.Add(-time.Second)
	if err := finding.Validate(); err == nil {
		t.Fatal("expected reversed observation times to fail")
	}
}

func TestRepositoryIdentifiersAreProjectScopedAndValidated(t *testing.T) {
	project := NewProject("project-a", "Project A", []Repository{
		NewRepository("backend", "Backend", `C:\work\backend`),
		NewRepository("frontend", "Frontend", `C:\work\frontend`),
	})
	if err := project.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := RepositoryKey("project-a", "backend"); got != "project-a/backend" {
		t.Fatalf("unexpected repository key: %s", got)
	}
	duplicate := NewProject("project-a", "Project A", []Repository{
		NewRepository("backend", "Backend", `C:\work\one`),
		NewRepository("backend", "Backend copy", `C:\work\two`),
	})
	if err := duplicate.Validate(); err == nil {
		t.Fatal("expected duplicate repository identifier to be rejected")
	}
	invalid := NewRepository("backend/name", "Backend", `C:\work\backend`)
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected invalid repository identifier to be rejected")
	}
}

func TestHighImpactActionRequiresFreshIndependentApproval(t *testing.T) {
	plan := ActionPlan{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: ActionPlanKind},
		Metadata: ObjectMeta{ID: "plan-1", Name: "production promotion"},
		Spec: ActionPlanSpec{
			ProjectID: "project-a", ActionType: "release.production", Risk: RiskHighImpact,
			PolicyDecision: PolicyApprovalRequired, ApprovalRequired: true,
		},
	}
	if err := plan.Validate(); err != nil {
		t.Fatal(err)
	}
	digest, err := plan.Digest()
	if err != nil {
		t.Fatal(err)
	}
	missingApproval := plan
	missingApproval.Spec.ApprovalRequired = false
	if err := missingApproval.Validate(); err == nil {
		t.Fatal("expected high-impact plan without approval requirement to fail")
	}
	selfApproved := Approval{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: ApprovalKind},
		Metadata: ObjectMeta{ID: "approval-1", Name: "production approval"},
		Spec: ApprovalSpec{
			ActionPlanID: "plan-1", ActionPlanDigest: digest, Status: ApprovalGranted,
			RequestedBy: Actor{Kind: ActorAgent, ID: "claude"},
			ApprovedBy:  &Actor{Kind: ActorAgent, ID: "claude"},
			ExpiresAt:   futureTime(),
		},
	}
	if err := selfApproved.ValidateFor(plan); err == nil {
		t.Fatal("expected agent approval to fail")
	}
	valid := selfApproved
	valid.Spec.ApprovedBy = &Actor{Kind: ActorHuman, ID: "local-user"}
	if err := valid.ValidateFor(plan); err != nil {
		t.Fatal(err)
	}
}

func TestSingleHumanCanApproveOwnRequestAndApprovalBindsToPlanDigest(t *testing.T) {
	plan := ActionPlan{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: ActionPlanKind},
		Metadata: ObjectMeta{ID: "plan-1", Name: "production promotion"},
		Spec: ActionPlanSpec{
			ProjectID: "project-a", ActionType: "release.production", Risk: RiskHighImpact,
			Inputs:         map[string]string{"target": "production", "commit": "abc123"},
			PolicyDecision: PolicyApprovalRequired, ApprovalRequired: true,
		},
	}
	digest, err := plan.Digest()
	if err != nil {
		t.Fatal(err)
	}
	human := Actor{Kind: ActorHuman, ID: "local-user"}
	approval := Approval{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: ApprovalKind},
		Metadata: ObjectMeta{ID: "approval-1", Name: "production approval"},
		Spec: ApprovalSpec{
			ActionPlanID: plan.Metadata.ID, ActionPlanDigest: digest,
			Status: ApprovalGranted, RequestedBy: human, ApprovedBy: &human,
			ExpiresAt: futureTime(),
		},
	}
	if err := approval.ValidateFor(plan); err != nil {
		t.Fatalf("single human approval should be valid: %v", err)
	}
	mutated := plan
	mutated.Spec.Inputs = map[string]string{"target": "production", "commit": "different"}
	if err := approval.ValidateFor(mutated); err == nil {
		t.Fatal("expected approval to reject a mutated action plan")
	}
}

func TestApprovalMustMatchActionPlan(t *testing.T) {
	plan := ActionPlan{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: ActionPlanKind},
		Metadata: ObjectMeta{ID: "plan-1", Name: "safe local action"},
		Spec: ActionPlanSpec{
			ProjectID: "project-a", ActionType: "local.format", Risk: RiskSafeLocal,
			PolicyDecision: PolicyAllowed, ApprovalRequired: false,
		},
	}
	digest, err := plan.Digest()
	if err != nil {
		t.Fatal(err)
	}
	requester := Actor{Kind: ActorHuman, ID: "local-user"}
	approver := Actor{Kind: ActorHuman, ID: "local-user"}
	approval := Approval{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: ApprovalKind},
		Metadata: ObjectMeta{ID: "approval-1", Name: "unexpected approval"},
		Spec: ApprovalSpec{
			ActionPlanID: "other-plan", ActionPlanDigest: digest, Status: ApprovalGranted,
			RequestedBy: requester, ApprovedBy: &approver,
		},
	}
	if err := approval.ValidateFor(plan); err == nil {
		t.Fatal("expected approval/action-plan mismatch to fail")
	}
}

func TestAgentProfileRequiresReviewedBoundaryLaunchModeAndEnvironment(t *testing.T) {
	profile := AgentProfile{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: AgentProfileKind},
		Metadata: ObjectMeta{ID: "claude-local", Name: "Claude Local"},
		Spec: AgentProfileSpec{
			Command: "claude-local", DataBoundary: AgentBoundaryLocal,
			LaunchMode:           AgentLaunchPowerShellProfile,
			EnvironmentAllowlist: []string{"CLAUDE_CONFIG_DIR"},
		},
	}
	if err := profile.Validate(); err != nil {
		t.Fatal(err)
	}
	profile.Spec.EnvironmentAllowlist = []string{"TOKEN", "token"}
	if err := profile.Validate(); err == nil {
		t.Fatal("expected case-insensitive duplicate environment names to fail")
	}
	profile.Spec.EnvironmentAllowlist = nil
	profile.Spec.DataBoundary = AgentDataBoundary("internet")
	if err := profile.Validate(); err == nil {
		t.Fatal("expected an unknown data boundary to fail")
	}
}

func TestAgentProfileRejectsShellSurfaces(t *testing.T) {
	profile := AgentProfile{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: AgentProfileKind},
		Metadata: ObjectMeta{ID: "safe-profile", Name: "Safe Profile"},
		Spec:     AgentProfileSpec{Command: "pwsh.exe -Command Write-Host", LaunchMode: AgentLaunchDirect, DataBoundary: AgentBoundaryLocal},
	}
	if err := profile.Validate(); err == nil {
		t.Fatal("direct profile accepted a generic shell command")
	}
	profile.Spec = AgentProfileSpec{Command: "Get-Command;Write-Host", LaunchMode: AgentLaunchPowerShellProfile, DataBoundary: AgentBoundaryLocal}
	if err := profile.Validate(); err == nil {
		t.Fatal("PowerShell profile accepted command injection syntax")
	}
	profile.Spec = AgentProfileSpec{Command: `C:\Program Files\Tool\tool.exe`, LaunchMode: AgentLaunchDirect, DataBoundary: AgentBoundaryLocal}
	if err := profile.Validate(); err != nil {
		t.Fatalf("safe absolute Windows executable was rejected: %v", err)
	}
	profile.Spec.VersionProbe = []string{"-Command", "Remove-Item"}
	if err := profile.Validate(); err == nil {
		t.Fatal("agent profile accepted an arbitrary version-probe command surface")
	}
	profile.Spec.VersionProbe = []string{"--version"}
	profile.Spec.EnvironmentAllowlist = []string{"INVALID NAME"}
	if err := profile.Validate(); err == nil {
		t.Fatal("agent profile accepted an invalid environment variable name")
	}
	profile.Spec.EnvironmentAllowlist = nil
	profile.Spec.Command = `\\server\share\tool.exe`
	if err := profile.Validate(); err == nil {
		t.Fatal("agent profile accepted a UNC executable")
	}
}

func futureTime() *time.Time {
	value := time.Now().UTC().Add(time.Hour)
	return &value
}
