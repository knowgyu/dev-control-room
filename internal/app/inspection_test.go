package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/assurance"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/qualitysetup"
	"github.com/knowgyu/dev-control-room/internal/store"
)

func TestGenerateInspectionPlanIsDeterministicAndNoAI(t *testing.T) {
	ctx := context.Background()
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	project, err := service.AddProject(ctx, AddProjectInput{Name: "Inspection", Path: tempGoGitRepository(t, "inspection")})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(ctx, "manual"); err != nil {
		t.Fatal(err)
	}

	plan, err := service.GenerateInspectionPlan(ctx, InspectionPlanGenerateInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Generation.Source != inspectionGenerationSource || plan.Generation.AIProposal || plan.Generation.Reason != inspectionGenerationReason || plan.Spec.State != domain.InspectionPlanStateProposed || plan.Spec.Digest == "" {
		t.Fatalf("unexpected generated plan: %#v", plan)
	}
	if plan.Spec.Checks == nil || len(plan.Spec.Checks) == 0 {
		t.Fatal("deterministic plan has no checks")
	}
}

func TestInspectionResultFromAdapterMapsTestsFailed(t *testing.T) {
	result := inspectionResultFromAdapter(assurance.QualityOutcomeTestsFailed, nil, domain.InspectionCheck{ID: domain.InspectionCheckPytest, ComponentID: "backend"})
	if result.Outcome != domain.InspectionOutcomeTestsFailed {
		t.Fatalf("tests-failed adapter outcome = %q, want %q", result.Outcome, domain.InspectionOutcomeTestsFailed)
	}
}

func TestDeterministicQualityImprovementsOnlyAddsRunnableChecks(t *testing.T) {
	root := t.TempDir()
	setup := qualitysetup.Report{Components: []qualitysetup.Component{{
		ID:     "backend",
		Checks: []qualitysetup.Check{{ID: "pytest"}},
	}}}
	plan := domain.InspectionPlan{Spec: domain.InspectionPlanSpec{
		Checks: []domain.InspectionCheck{{ID: domain.InspectionCheckRuff, ComponentID: "backend"}},
	}}
	results := []domain.InspectionResult{{
		ComponentID: "backend",
		CheckID:     domain.InspectionCheckRuff,
		Outcome:     domain.InspectionOutcomeFindings,
	}}

	changes, _ := deterministicQualityImprovements(plan, setup, root, results)
	if len(changes) != 0 {
		t.Fatalf("improvements added unavailable checks: %#v", changes)
	}
}

func TestPythonInspectionUsesTheSelectedVirtualEnvironment(t *testing.T) {
	root := t.TempDir()
	firstInterpreter := filepath.Join(root, ".venv", "Scripts", "python.exe")
	secondInterpreter := filepath.Join(root, "venv", "Scripts", "python.exe")
	for _, interpreter := range []string{firstInterpreter, secondInterpreter} {
		if err := os.MkdirAll(filepath.Dir(interpreter), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(interpreter, []byte("python"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	firstPackages := filepath.Join(root, ".venv", "Lib", "site-packages")
	secondPackages := filepath.Join(root, "venv", "Lib", "site-packages")
	if err := os.MkdirAll(firstPackages, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(secondPackages, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(firstPackages, "ruff-1.2.3.dist-info"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(secondPackages, "ruff-9.8.7.dist-info"), 0o700); err != nil {
		t.Fatal(err)
	}

	selected, scope := resolveQualityPython(root, root, nil)
	if selected != firstInterpreter || scope != "project" {
		t.Fatalf("selected Python environment = %q, %q; want %q, project", selected, scope, firstInterpreter)
	}
	if !qualityPythonPackageInstalled(selected, domain.InspectionCheckRuff) || pythonPackageVersion(selected, "ruff") != "1.2.3" {
		t.Fatalf("selected environment package resolution failed: installed=%v version=%q", qualityPythonPackageInstalled(selected, domain.InspectionCheckRuff), pythonPackageVersion(selected, "ruff"))
	}
	if qualityPythonPackageInstalled(secondInterpreter, domain.InspectionCheckRuff) == false {
		t.Fatal("test fixture did not create the second environment package")
	}

	checks := []domain.InspectionCheck{{ID: domain.InspectionCheckRuff, ComponentID: "component-1"}}
	firstDigest := inspectionToolDigestForRoot(checks, root)
	if err := os.RemoveAll(filepath.Join(firstPackages, "ruff-1.2.3.dist-info")); err != nil {
		t.Fatal(err)
	}
	if qualityPythonPackageInstalled(selected, domain.InspectionCheckRuff) {
		t.Fatal("package detection fell back to a different virtual environment")
	}
	if firstDigest == inspectionToolDigestForRoot(checks, root) {
		t.Fatal("tool digest did not change when the selected environment package disappeared")
	}
}

func TestGenerateInspectionPlanReusesCurrentGenerationAndRegeneratesRejectedPlan(t *testing.T) {
	ctx := context.Background()
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	project, err := service.AddProject(ctx, AddProjectInput{Name: "Regeneration", Path: tempGoGitRepository(t, "regeneration")})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(ctx, "manual"); err != nil {
		t.Fatal(err)
	}
	input := InspectionPlanGenerateInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary"}
	first, err := service.GenerateInspectionPlan(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	reused, err := service.GenerateInspectionPlan(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if reused.Metadata.ID != first.Metadata.ID {
		t.Fatalf("same generation was duplicated: first %q, reused %q", first.Metadata.ID, reused.Metadata.ID)
	}
	rejected, err := service.ReviewInspectionPlan(ctx, first.Metadata.ID, InspectionPlanReviewInput{ExpectedRevision: 1, Decision: "reject"})
	if err != nil || rejected.Spec.State != domain.InspectionPlanStateRejected {
		t.Fatalf("reject plan = %#v, err=%v", rejected, err)
	}
	regenerated, err := service.GenerateInspectionPlan(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if regenerated.Metadata.ID == first.Metadata.ID || regenerated.Spec.State != domain.InspectionPlanStateProposed {
		t.Fatalf("rejected plan was not regenerated safely: %#v", regenerated)
	}
	plans, err := service.InspectionPlans(ctx)
	if err != nil || len(plans) != 2 {
		t.Fatalf("regenerated plan history = %d, err=%v", len(plans), err)
	}
}

func TestGenerateInspectionPlanReportsNoApplicableChecks(t *testing.T) {
	ctx := context.Background()
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	project, err := service.AddProject(ctx, AddProjectInput{Name: "Empty inspection", Path: tempGitRepository(t, "empty-inspection")})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(ctx, "manual"); err != nil {
		t.Fatal(err)
	}
	_, err = service.GenerateInspectionPlan(ctx, InspectionPlanGenerateInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary"})
	if contract.Classify(err).Code != contract.ErrorUnavailable || !strings.Contains(err.Error(), "no applicable inspection checks") {
		t.Fatalf("empty inspection error = %v", err)
	}
}

func TestInspectionPlanRoutesUseReviewAndCAS(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	plan := inspectionRoutePlanFixture(t, service)
	if err := service.store.SaveInspectionPlan(context.Background(), plan); err != nil {
		t.Fatal(err)
	}

	listed := callUICheckset[[]InspectionPlanView](t, service, http.MethodGet, "/api/quality/inspection-plans", nil)
	if len(listed) != 1 || listed[0].Generation.Source != inspectionGenerationSource {
		t.Fatalf("listed plans = %#v", listed)
	}
	got := callUICheckset[InspectionPlanView](t, service, http.MethodGet, "/api/quality/inspection-plans/"+plan.Metadata.ID, nil)
	if got.Spec.Digest == "" {
		t.Fatal("stored plan digest is missing")
	}

	reviewed := callUICheckset[InspectionPlanView](t, service, http.MethodPost, "/api/quality/inspection-plans/"+plan.Metadata.ID+"/review", []byte(`{"expectedRevision":1,"decision":"review"}`))
	if reviewed.Spec.State != domain.InspectionPlanStateReviewed || reviewed.Spec.Revision != 2 {
		t.Fatalf("reviewed plan = %#v", reviewed.Spec)
	}
	assertInspectionPlanViewDigest(t, reviewed)
	storedReviewed, err := service.store.GetInspectionPlan(context.Background(), plan.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedReviewed.Spec.Digest != reviewed.Spec.Digest {
		t.Fatalf("reviewed digest differs from persisted plan: returned=%q persisted=%q", reviewed.Spec.Digest, storedReviewed.Spec.Digest)
	}
	assertUIError(t, service, http.MethodPost, "/api/quality/inspection-plans/"+plan.Metadata.ID+"/review", []byte(`{"expectedRevision":1,"decision":"approve"}`), service.mutationToken, "http://127.0.0.1:38471", http.StatusConflict, contract.ErrorConflict)
	approved := callUICheckset[InspectionPlanView](t, service, http.MethodPost, "/api/quality/inspection-plans/"+plan.Metadata.ID+"/review", []byte(`{"expectedRevision":2,"decision":"approve"}`))
	if approved.Spec.State != domain.InspectionPlanStateApproved || approved.Spec.Revision != 3 {
		t.Fatalf("approved plan = %#v", approved.Spec)
	}
	assertInspectionPlanViewDigest(t, approved)
}

func TestLoadInspectionRunArtifactRejectsManifestMismatch(t *testing.T) {
	content, err := json.Marshal(inspectionRunArtifact{
		PlanID:  "inspection-plan",
		Results: []domain.InspectionResult{{ComponentID: "component-1", CheckID: domain.InspectionCheckGoTest, Outcome: domain.InspectionOutcomeClean}},
		ScoreID: "score-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{name: "size", mutate: func(data []byte) []byte { return append(data, '\n') }},
		{name: "sha256", mutate: func(data []byte) []byte {
			mutated := append([]byte{}, data...)
			mutated[0] ^= 1
			return mutated
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := New(t.TempDir(), "127.0.0.1:38471")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = service.Close() })
			artifact, err := service.SaveAssuranceArtifact(context.Background(), ArtifactInput{
				SourceType: inspectionResultArtifactType,
				SourceID:   "inspection-run",
				Name:       "inspection-run.json",
				MIME:       inspectionResultArtifactMIME,
				Content:    content,
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(artifact.Spec.Path, tt.mutate(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := service.loadInspectionRunArtifact(context.Background(), artifact.Metadata.ID); contract.Classify(err).Code != contract.ErrorUnavailable {
				t.Fatalf("tampered artifact error = %v, want unavailable", err)
			}
		})
	}
}

func assertInspectionPlanViewDigest(t *testing.T, view InspectionPlanView) {
	t.Helper()
	want, err := view.InspectionPlan.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if view.Spec.Digest != want {
		t.Fatalf("returned plan digest = %q, want %q", view.Spec.Digest, want)
	}
	if err := view.InspectionPlan.Validate(); err != nil {
		t.Fatalf("returned plan is invalid: %v", err)
	}
}

func TestLatestInspectionRunRecoversStoredArtifactByWorktree(t *testing.T) {
	ctx := context.Background()
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	plan := inspectionRoutePlanFixture(t, service)
	if err := service.store.SaveInspectionPlan(ctx, plan); err != nil {
		t.Fatal(err)
	}
	storedPlan, err := service.store.GetInspectionPlan(ctx, plan.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	results := []domain.InspectionResult{{ComponentID: "component-1", CheckID: domain.InspectionCheckGoTest, Outcome: domain.InspectionOutcomeClean}}
	score, err := CalculateRepositoryQualityScore(RepositoryQualityScoreInput{Plan: storedPlan, Results: results, Current: ptrInspectionFreshness(storedPlan)})
	if err != nil {
		t.Fatal(err)
	}
	score.Metadata.ID = "score-latest"
	if err := service.store.SaveRepositoryQualityScore(ctx, score); err != nil {
		t.Fatal(err)
	}
	content, err := json.Marshal(inspectionRunArtifact{PlanID: storedPlan.Metadata.ID, Results: results, ScoreID: score.Metadata.ID})
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := service.SaveAssuranceArtifact(ctx, ArtifactInput{SourceType: inspectionResultArtifactType, SourceID: "run-latest", Name: "run-latest.json", MIME: inspectionResultArtifactMIME, Content: content})
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.LatestInspectionRun(ctx, InspectionRunQueryInput{ProjectID: storedPlan.Spec.ProjectID, RepositoryID: storedPlan.Spec.RepositoryID, WorktreeID: storedPlan.Spec.WorktreeID})
	if err != nil {
		t.Fatal(err)
	}
	if got.ResultArtifact != artifact.Metadata.ID || got.Plan.Metadata.ID != storedPlan.Metadata.ID || got.Score.Metadata.ID != score.Metadata.ID {
		t.Fatalf("latest run = %#v", got)
	}
}

func TestRunInspectionPlanRejectsUnreviewedCheckDefinition(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	plan := inspectionRoutePlanFixture(t, service)
	plan.Spec.State = domain.InspectionPlanStateApproved
	plan.Spec.Checks[0].AdapterID = "arbitrary-executable"
	plan.Spec.Checks[0].ParserID = "arbitrary-parser"
	if err := service.store.SaveInspectionPlan(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RunInspectionPlan(context.Background(), plan.Metadata.ID, InspectionPlanRunInput{ExpectedRevision: 1}); contract.Classify(err).Code != contract.ErrorInvalidInput {
		t.Fatalf("unreviewed check error = %v", err)
	}
}

func TestToolInstallRouteIsPreviewOnly(t *testing.T) {
	ctx := context.Background()
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	project, err := service.AddProject(ctx, AddProjectInput{Name: "Tool install preview", Path: tempGoGitRepository(t, "tool-install-preview")})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(ctx, "manual"); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(QualityToolInstallPlanInput{
		ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary",
		Kind: "python.ruff", Version: "1.0.0", AffectedFiles: []string{"go.mod"},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/quality/tool-installs/plan", bytes.NewReader(payload))
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	request.Header.Set("Origin", "http://127.0.0.1:38471")
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("tool preview status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response contract.Envelope[QualityToolInstallPreview]
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if !response.OK || response.Error != nil || response.Data == nil {
		t.Fatalf("tool preview response = %#v", response)
	}
}

func TestApplyQualityImprovementProposalRecomputesDigestAndReloads(t *testing.T) {
	ctx := context.Background()
	service, plan, proposal, readEvidence := newInspectionApplyFixture(t)

	approved, err := service.ReviewQualityImprovementProposal(ctx, proposal.Metadata.ID, QualityImprovementProposalReviewInput{
		ExpectedRevision: 1,
		Decision:         "approve",
	})
	if err != nil {
		t.Fatal(err)
	}
	if approved.Spec.State != domain.QualityImprovementStateApproved || approved.Spec.Revision != 2 {
		t.Fatalf("approved proposal = %#v", approved.Spec)
	}

	applied, err := service.applyQualityImprovementProposal(ctx, proposal.Metadata.ID, QualityImprovementProposalApplyInput{
		ExpectedRevision: approved.Spec.Revision,
	}, readEvidence)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Plan.Spec.Revision != plan.Spec.Revision+1 || applied.Proposal.Spec.State != domain.QualityImprovementStateApplied {
		t.Fatalf("applied result = %#v", applied)
	}
	if len(applied.Plan.Spec.Checks) != 2 || applied.Plan.Spec.ToolDigest != inspectionToolDigestForCurrent(applied.Plan.Spec.Checks) {
		t.Fatalf("applied check set or tool digest = %#v", applied.Plan.Spec)
	}
	wantDigest, err := applied.Plan.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if applied.Plan.Spec.Digest != wantDigest {
		t.Fatalf("applied plan digest = %q, want %q", applied.Plan.Spec.Digest, wantDigest)
	}

	reloaded, err := service.InspectionPlan(ctx, plan.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := reloaded.Validate(); err != nil {
		t.Fatal(err)
	}
	if reloaded.Spec.Digest != applied.Plan.Spec.Digest {
		t.Fatalf("reloaded digest = %q, applied digest = %q", reloaded.Spec.Digest, applied.Plan.Spec.Digest)
	}
	if reloaded.Spec.EvidenceDigest != applied.Plan.Spec.EvidenceDigest || reloaded.Spec.Head != applied.Plan.Spec.Head {
		t.Fatalf("reloaded freshness fields = %#v", reloaded.Spec)
	}
	_, current, _, err := readEvidence(ctx, reloaded.InspectionPlan)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.IsStale(current) {
		t.Fatalf("reloaded plan is stale against current evidence: %#v", reloaded.StaleReasons(current))
	}

	reloadedProposal, err := service.QualityImprovementProposal(ctx, proposal.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloadedProposal.Spec.State != domain.QualityImprovementStateApplied || reloadedProposal.Spec.Revision != 3 {
		t.Fatalf("reloaded proposal = %#v", reloadedProposal.Spec)
	}
}

func TestApplyQualityImprovementProposalMarksFreshnessChangesStale(t *testing.T) {
	tests := []struct {
		name   string
		change func(*domain.InspectionFreshness)
	}{
		{name: "HEAD", change: func(freshness *domain.InspectionFreshness) { freshness.Head = "head-2" }},
		{name: "config", change: func(freshness *domain.InspectionFreshness) { freshness.ConfigDigest = digestForScore('x') }},
		{name: "evidence", change: func(freshness *domain.InspectionFreshness) { freshness.EvidenceDigest = digestForScore('x') }},
		{name: "tool", change: func(freshness *domain.InspectionFreshness) { freshness.ToolDigest = digestForScore('x') }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			service, plan, proposal, readEvidence := newInspectionApplyFixture(t)
			approved, err := service.ReviewQualityImprovementProposal(ctx, proposal.Metadata.ID, QualityImprovementProposalReviewInput{
				ExpectedRevision: 1,
				Decision:         "approve",
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.applyQualityImprovementProposal(ctx, proposal.Metadata.ID, QualityImprovementProposalApplyInput{
				ExpectedRevision: approved.Spec.Revision,
			}, func(ctx context.Context, currentPlan domain.InspectionPlan) (qualitysetup.Report, domain.InspectionFreshness, string, error) {
				setup, freshness, root, err := readEvidence(ctx, currentPlan)
				if err != nil {
					return setup, freshness, root, err
				}
				tt.change(&freshness)
				return setup, freshness, root, nil
			})
			if contract.Classify(err).Code != contract.ErrorConflict {
				t.Fatalf("stale apply error = %v", err)
			}

			staleProposal, err := service.QualityImprovementProposal(ctx, proposal.Metadata.ID)
			if err != nil {
				t.Fatal(err)
			}
			if staleProposal.Spec.State != domain.QualityImprovementStateStale || staleProposal.Spec.Revision != 3 {
				t.Fatalf("stale proposal = %#v", staleProposal.Spec)
			}
			unchangedPlan, err := service.InspectionPlan(ctx, plan.Metadata.ID)
			if err != nil {
				t.Fatal(err)
			}
			if unchangedPlan.Spec.Revision != plan.Spec.Revision || unchangedPlan.Spec.State != plan.Spec.State {
				t.Fatalf("plan changed during stale apply = %#v", unchangedPlan.Spec)
			}
		})
	}
}

func TestInspectionFreshnessUsesReviewedPlanChecks(t *testing.T) {
	setup := qualitysetup.Report{
		Head:   "head-1",
		Digest: strings.TrimPrefix(digestForScore('e'), "sha256:"),
		Components: []qualitysetup.Component{{
			ID: "component-1",
		}},
	}
	plan := inspectionRoutePlan()
	components, _ := inspectionPlanParts(setup)
	freshness := inspectionFreshnessForSetup(setup, plan, components)

	if freshness.Head != setup.Head || freshness.EvidenceDigest != "sha256:"+setup.Digest {
		t.Fatalf("freshness identity = %#v", freshness)
	}
	if freshness.ConfigDigest != inspectionConfigDigestForSetup(setup, components) {
		t.Fatalf("freshness config digest = %q", freshness.ConfigDigest)
	}
	if freshness.ToolDigest != inspectionToolDigestForCurrent(plan.Spec.Checks) {
		t.Fatalf("freshness tool digest = %q", freshness.ToolDigest)
	}
}

func newInspectionApplyFixture(t *testing.T) (*App, domain.InspectionPlan, domain.QualityImprovementProposal, inspectionEvidenceReader) {
	t.Helper()
	service := newInMemoryInspectionApp(t)
	seedInspectionScope(t, service)
	setup := qualitysetup.Report{
		Head:   "head-1",
		Digest: strings.TrimPrefix(digestForScore('e'), "sha256:"),
		Components: []qualitysetup.Component{{
			ID: "component-1",
		}},
	}
	plan := inspectionRoutePlan()
	components, _ := inspectionPlanParts(setup)
	plan.Spec.ConfigDigest = inspectionConfigDigestForSetup(setup, components)
	plan.Spec.EvidenceDigest = "sha256:" + setup.Digest
	plan.Spec.ToolDigest = inspectionToolDigestForCurrent(plan.Spec.Checks)
	if err := service.store.SaveInspectionPlan(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	storedPlan, err := service.store.GetInspectionPlan(context.Background(), plan.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	score, err := CalculateRepositoryQualityScore(RepositoryQualityScoreInput{
		Plan:    storedPlan,
		Results: []domain.InspectionResult{{ComponentID: "component-1", CheckID: domain.InspectionCheckGoTest, Outcome: domain.InspectionOutcomeClean}},
		Current: ptrInspectionFreshness(storedPlan),
	})
	if err != nil {
		t.Fatal(err)
	}
	score.Metadata.ID = "score-1"
	if err := service.store.SaveRepositoryQualityScore(context.Background(), score); err != nil {
		t.Fatalf("save parent score: %v", err)
	}
	changes := []domain.QualityImprovementChange{{
		Action:      domain.QualityImprovementActionAddCheck,
		ComponentID: "component-1",
		CheckID:     domain.InspectionCheckGoTestRace,
		Reason:      domain.QualityImprovementReasonFindingObserved,
	}}
	now := time.Now().UTC()
	proposal := domain.QualityImprovementProposal{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.QualityImprovementProposalKind},
		Metadata: domain.ObjectMeta{ID: "quality-improvement-apply", Name: "Quality improvement apply"},
		Spec: domain.QualityImprovementProposalSpec{
			ProjectID: storedPlan.Spec.ProjectID, RepositoryID: storedPlan.Spec.RepositoryID, WorktreeID: storedPlan.Spec.WorktreeID,
			PlanID: storedPlan.Metadata.ID, BaseScoreID: score.Metadata.ID, BaseScoreDigest: digestText(score),
			BasePlanRevision: storedPlan.Spec.Revision, BasePlanDigest: storedPlan.Spec.Digest, Head: storedPlan.Spec.Head,
			ConfigDigest: storedPlan.Spec.ConfigDigest, EvidenceDigest: storedPlan.Spec.EvidenceDigest, ToolDigest: storedPlan.Spec.ToolDigest,
			Changes: changes, RationaleDigest: digestForScore('a'), RationaleCode: domain.QualityImprovementReasonFindingObserved,
			Source: domain.InspectionGenerationDeterministic, State: domain.QualityImprovementStateProposed, Revision: 1,
			CreatedAt: now, UpdatedAt: now,
		},
	}
	if err := service.store.SaveQualityImprovementProposal(context.Background(), proposal); err != nil {
		t.Fatalf("save proposal: %v", err)
	}
	readEvidence := func(_ context.Context, currentPlan domain.InspectionPlan) (qualitysetup.Report, domain.InspectionFreshness, string, error) {
		return setup, inspectionFreshnessForSetup(setup, currentPlan, components), "/virtual/inspection", nil
	}
	return service, storedPlan, proposal, readEvidence
}

func seedInspectionScope(t *testing.T, service *App) {
	t.Helper()
	ctx := context.Background()
	project := domain.NewProject(
		"project-1",
		"Inspection fixture",
		[]domain.Repository{domain.NewRepository("repo-1", "Inspection fixture", "/virtual/inspection")},
	)
	if err := service.store.SaveProject(ctx, project); err != nil {
		t.Fatalf("save inspection project: %v", err)
	}
	now := time.Now().UTC()
	worktree := domain.Worktree{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.WorktreeKind},
		Metadata: domain.ObjectMeta{ID: "primary", Name: "Inspection fixture worktree"},
		Spec: domain.WorktreeSpec{
			ProjectID: "project-1", RepositoryID: "repo-1", CanonicalPath: "/virtual/inspection",
			PathFingerprint: digestForScore('p'), Trust: domain.WorktreeTrustVerifiedReadOnly, Primary: true,
			Head: "head-1", Branch: "main", LastObserved: now,
		},
	}
	if err := service.store.ReplaceWorktrees(ctx, "project-1", "repo-1", []domain.Worktree{worktree}, true); err != nil {
		t.Fatalf("save inspection worktree: %v", err)
	}
}

func newInMemoryInspectionApp(t *testing.T) *App {
	t.Helper()
	database, err := sql.Open("sqlite", "file:"+strings.ReplaceAll(t.Name(), "/", "-")+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	if err := store.Migrate(context.Background(), database); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	persistence, err := store.New(database, nil)
	if err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = persistence.Close() })
	return &App{store: persistence}
}

func inspectionRoutePlan() domain.InspectionPlan {
	now := time.Now().UTC()
	return domain.InspectionPlan{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.InspectionPlanKind},
		Metadata: domain.ObjectMeta{ID: "inspection-plan-route", Name: "Inspection route"},
		Spec: domain.InspectionPlanSpec{
			ProjectID: "project-1", RepositoryID: "repo-1", WorktreeID: "primary", Branch: "main", Head: "head-1",
			BaselineDigest: digestForScore('a'), ConfigDigest: digestForScore('b'), EvidenceDigest: digestForScore('c'), ToolDigest: digestForScore('d'),
			Components: []domain.InspectionComponent{{ID: "component-1"}}, Checks: []domain.InspectionCheck{inspectionCheck(domain.InspectionCheckGoTest, "component-1")},
			State: domain.InspectionPlanStateProposed, Revision: 1, Version: domain.InspectionPlanVersion, CreatedAt: now, UpdatedAt: now,
		},
	}
}

func inspectionRoutePlanFixture(t *testing.T, service *App) domain.InspectionPlan {
	t.Helper()
	project, err := service.AddProject(context.Background(), AddProjectInput{
		Name: "Inspection route fixture",
		Path: tempGoGitRepository(t, "inspection-route"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	worktree, err := service.Worktree(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil {
		t.Fatal(err)
	}
	plan := inspectionRoutePlan()
	plan.Spec.ProjectID = project.Metadata.ID
	plan.Spec.RepositoryID = "repo-1"
	plan.Spec.WorktreeID = "primary"
	plan.Spec.Branch = worktree.Spec.Branch
	plan.Spec.Head = worktree.Spec.Head
	return plan
}
