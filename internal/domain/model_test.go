package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func reviewedActionPlan(t *testing.T, id, actionType string, inputs map[string]string, requester Actor) ActionPlan {
	t.Helper()
	definition, ok := ActionDefinitionFor(actionType)
	if !ok {
		t.Fatalf("missing action definition %q", actionType)
	}
	execution, err := definition.ExecutionFor(inputs)
	if err != nil {
		t.Fatal(err)
	}
	return ActionPlan{TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: ActionPlanKind}, Metadata: ObjectMeta{ID: id, Name: "reviewed action"}, Spec: ActionPlanSpec{ProjectID: "project-a", RepositoryID: "repo-a", WorktreeID: "primary", ActionType: actionType, Risk: definition.Risk, Inputs: inputs, Execution: execution, ExecutionContext: WorktreeExecutionContext{ProjectID: "project-a", RepositoryID: "repo-a", WorktreeID: "primary", CanonicalPath: "C:/fixture", PathFingerprint: "sha256:path", Head: "abc123", Branch: "main"}, Prechecks: definition.Prechecks, Postchecks: definition.Postchecks, PolicyDecision: definition.PolicyDecision, ApprovalRequired: definition.ApprovalRequired, RequestedBy: requester, RequestedAt: time.Now().UTC()}}
}

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

func TestProposalRequiresBoundedEvidenceAndReviewState(t *testing.T) {
	now := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	proposal := Proposal{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: ProposalKind},
		Metadata: ObjectMeta{ID: "proposal-1", Name: "package script test"},
		Spec:     ProposalSpec{ProjectID: "project-1", RepositoryID: "repo-1", WorktreeID: "primary", Head: "abc", SourcePath: "package.json", SourceDigest: "sha256:" + strings.Repeat("a", 64), CommandKind: "package_script", Command: "npm run test", Inference: "deterministic", State: ProposalPending, CreatedAt: now},
	}
	if err := proposal.Validate(); err != nil {
		t.Fatal(err)
	}
	proposal.Spec.SourcePath = "../outside"
	if err := proposal.Validate(); err == nil {
		t.Fatal("proposal accepted source outside selected worktree")
	}
	proposal.Spec.SourcePath = "package.json"
	proposal.Spec.State = ProposalApplied
	if err := proposal.Validate(); err == nil {
		t.Fatal("applied proposal without review time was accepted")
	}
}

func TestChecksetRejectsShellAndUnknownDependencies(t *testing.T) {
	checkset := Checkset{TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: ChecksetKind}, Metadata: ObjectMeta{ID: "checks-1", Name: "Checks"}, Spec: ChecksetSpec{ProjectID: "project-1", RepositoryID: "repo-1", WorktreeID: "primary", Head: "abc", ProposalID: "proposal-1", Name: "Checks", State: ChecksetDraft, Steps: []CheckStep{{ID: "test", Name: "Test", Command: CheckCommand{Executable: "git", Arguments: []string{"status", "--porcelain"}, TimeoutSeconds: 5}}}}}
	if err := checkset.Validate(); err != nil {
		t.Fatal(err)
	}
	checkset.Spec.Steps[0].Command.Executable = "sh"
	if err := checkset.Validate(); err == nil {
		t.Fatal("shell executable accepted")
	}
	checkset.Spec.Steps[0].Command.Executable = "git"
	checkset.Spec.Steps[0].DependsOn = []string{"missing"}
	if err := checkset.Validate(); err == nil {
		t.Fatal("missing dependency accepted")
	}
}

func TestHighImpactActionRequiresFreshIndependentApproval(t *testing.T) {
	plan := reviewedActionPlan(t, "plan-1", "release.production", map[string]string{"commit": "abc123"}, Actor{Kind: ActorAgent, ID: "claude"})
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
			DecidedAt:   time.Now().UTC(),
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

func TestActionPlanRejectsForgedProductionPolicy(t *testing.T) {
	plan := ActionPlan{TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: ActionPlanKind}, Metadata: ObjectMeta{ID: "plan-1", Name: "Production"}, Spec: ActionPlanSpec{ProjectID: "project-a", RepositoryID: "repo-a", WorktreeID: "primary", ActionType: "release.production", Risk: RiskSafeLocal, Inputs: map[string]string{"commit": "abc123"}, PolicyDecision: PolicyAllowed, RequestedBy: Actor{Kind: ActorAgent, ID: "agent"}, RequestedAt: time.Now().UTC()}}
	if err := plan.Validate(); err == nil {
		t.Fatal("forged low-risk production plan was accepted")
	}
}

func TestActionPlanBindsReviewedExecutionAndEvidenceContracts(t *testing.T) {
	plan := reviewedActionPlan(t, "plan-execution", "release.production", map[string]string{"commit": "abc123"}, Actor{Kind: ActorHuman, ID: "local-user"})
	if plan.Spec.Execution.Executable != "devroom-release-production" || plan.Spec.Execution.Arguments[1] != "abc123" || len(plan.Spec.Prechecks) != 2 || len(plan.Spec.Postchecks) != 1 {
		t.Fatalf("reviewed execution snapshot = %#v", plan.Spec)
	}
	plan.Spec.Execution.Arguments[1] = "forged"
	if err := plan.Validate(); err == nil {
		t.Fatal("forged action argv was accepted")
	}
	definition, _ := ActionDefinitionFor("release.production")
	definition.Execution.EnvironmentAllowlist = []string{"TOKEN", "token"}
	if _, err := definition.ExecutionFor(map[string]string{"commit": "abc123"}); err == nil {
		t.Fatal("duplicate allowlisted environment names were accepted")
	}
}

func TestExecutionTrustRequiresVerifiedReadOnlyObservedWorktree(t *testing.T) {
	now := time.Now().UTC()
	worktree := Worktree{TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: WorktreeKind}, Metadata: ObjectMeta{ID: "primary", Name: "Primary"}, Spec: WorktreeSpec{ProjectID: "project-a", RepositoryID: "repo-a", CanonicalPath: "C:/fixture", PathFingerprint: "sha256:path", Trust: WorktreeTrustVerifiedReadOnly, Primary: true, Head: "abc123", LastObserved: now}}
	trust, err := NewWorktreeExecutionTrust(worktree, now)
	if err != nil || trust.Context.Head != "abc123" {
		t.Fatalf("execution trust = %#v, %v", trust, err)
	}
	worktree.Spec.Trust = WorktreeTrustUnverified
	if _, err := NewWorktreeExecutionTrust(worktree, now); err == nil {
		t.Fatal("unverified worktree became trusted for execution")
	}
}

func TestSingleHumanCanApproveOwnRequestAndApprovalBindsToPlanDigest(t *testing.T) {
	plan := reviewedActionPlan(t, "plan-1", "release.production", map[string]string{"commit": "abc123"}, Actor{Kind: ActorHuman, ID: "local-user"})
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
			DecidedAt: time.Now().UTC(),
		},
	}
	if err := approval.ValidateFor(plan); err != nil {
		t.Fatalf("single human approval should be valid: %v", err)
	}
	mutated := plan
	mutated.Spec.Inputs = map[string]string{"commit": "different"}
	if err := approval.ValidateFor(mutated); err == nil {
		t.Fatal("expected approval to reject a mutated action plan")
	}
}

func TestApprovalMustMatchActionPlan(t *testing.T) {
	plan := reviewedActionPlan(t, "plan-1", "repository.refresh", nil, Actor{Kind: ActorHuman, ID: "local-user"})
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
			DecidedAt: time.Now().UTC(),
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
