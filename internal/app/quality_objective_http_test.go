package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/assurance"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestQualityObjectiveRevalidationHTTPRejectsUntrustedInputAndPreservesState(t *testing.T) {
	service, objective := newQualityObjectiveDecisionFixture(t)
	decided := postQualityObjectiveDecision(t, service, objective.Metadata.ID, QualityObjectiveDecisionInput{
		ExpectedRevision: objective.Spec.Revision,
		Disposition:      domain.QualityObjectiveDispositionPursue,
		Action:           "add focused tests",
		Actor:            "knowgyu",
	}, service.mutationToken, "")
	if decided.Code != http.StatusOK {
		t.Fatalf("decision response = %d: %s", decided.Code, decided.Body.String())
	}

	tests := []struct {
		name       string
		id         string
		rawBody    string
		token      string
		origin     string
		wantStatus int
		wantCode   contract.ErrorCode
	}{
		{name: "missing token", id: objective.Metadata.ID, rawBody: `{"expectedRevision":2,"findingId":"finding-1"}`, wantStatus: http.StatusForbidden, wantCode: contract.ErrorForbidden},
		{name: "wrong origin", id: objective.Metadata.ID, rawBody: `{"expectedRevision":2,"findingId":"finding-1"}`, token: "fixture", origin: "https://example.invalid", wantStatus: http.StatusForbidden, wantCode: contract.ErrorForbidden},
		{name: "forged outcome", id: objective.Metadata.ID, rawBody: `{"expectedRevision":2,"findingId":"finding-1","outcome":"improved"}`, token: "fixture", wantStatus: http.StatusBadRequest, wantCode: contract.ErrorInvalidInput},
		{name: "two sources", id: objective.Metadata.ID, rawBody: `{"expectedRevision":2,"findingId":"finding-1","qualityRunId":"run-1"}`, token: "fixture", wantStatus: http.StatusBadRequest, wantCode: contract.ErrorInvalidInput},
		{name: "no source", id: objective.Metadata.ID, rawBody: `{"expectedRevision":2}`, token: "fixture", wantStatus: http.StatusBadRequest, wantCode: contract.ErrorInvalidInput},
		{name: "missing objective", id: "objective-missing", rawBody: `{"expectedRevision":1,"findingId":"finding-1"}`, token: "fixture", wantStatus: http.StatusNotFound, wantCode: contract.ErrorNotFound},
		{name: "stale revision", id: objective.Metadata.ID, rawBody: `{"expectedRevision":1,"findingId":"finding-1"}`, token: "fixture", wantStatus: http.StatusConflict, wantCode: contract.ErrorConflict},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			token := testCase.token
			if token == "fixture" {
				token = service.mutationToken
			}
			response := postQualityObjectiveRaw(t, service, "/api/quality/objectives/"+testCase.id+"/revalidations", testCase.rawBody, token, testCase.origin)
			if response.Code != testCase.wantStatus {
				t.Fatalf("response = %d, want %d: %s", response.Code, testCase.wantStatus, response.Body.String())
			}
			var envelope contract.Envelope[map[string]any]
			if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.OK || envelope.Error == nil || envelope.Error.Code != testCase.wantCode {
				t.Fatalf("error envelope = %#v", envelope)
			}
		})
	}

	loaded, err := service.QualityObjective(context.Background(), objective.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Spec.Revision != objective.Spec.Revision+1 || loaded.Spec.State != domain.QualityObjectiveStateReady || len(loaded.Spec.Revalidations) != 0 {
		t.Fatalf("rejected requests mutated objective = %#v", loaded)
	}
}

func TestQualityObjectiveFindingRevalidationCannotBeConfirmed(t *testing.T) {
	service, projectID := newQualityObjectiveProject(t, "finding-revalidation")
	now := time.Now().UTC()
	if err := service.store.SaveFinding(context.Background(), domain.Finding{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.FindingKind},
		Metadata: domain.ObjectMeta{ID: "finding-revalidation", Name: "Finding revalidation"},
		Spec: domain.FindingSpec{
			ProjectID: projectID, RepositoryID: "repo-1", FindingType: "quality", Fingerprint: "sha256:finding-revalidation",
			Severity: domain.SeverityAttention, Confidence: domain.ConfidenceConfirmed, Summary: "missing focused test",
			RecommendedNext: "add a focused test", FirstObserved: now, LastObserved: now, State: domain.FindingResolved,
		},
	}); err != nil {
		t.Fatal(err)
	}
	objective, err := service.CreateQualityObjective(context.Background(), QualityObjectiveInput{
		ProjectID: projectID, RepositoryID: "repo-1", WorktreeID: "primary", Owner: "owner", Title: "Resolve finding",
		FindingIDs: []string{"finding-revalidation"},
	})
	if err != nil {
		t.Fatal(err)
	}
	decided := postQualityObjectiveDecision(t, service, objective.Metadata.ID, QualityObjectiveDecisionInput{
		ExpectedRevision: 1, Disposition: domain.QualityObjectiveDispositionPursue, Action: "keep the regression test", Actor: "knowgyu",
	}, service.mutationToken, "")
	if decided.Code != http.StatusOK {
		t.Fatalf("decision response = %d: %s", decided.Code, decided.Body.String())
	}
	revalidated := postQualityObjectiveRaw(t, service, "/api/quality/objectives/"+objective.Metadata.ID+"/revalidations", `{"expectedRevision":2,"findingId":"finding-revalidation"}`, service.mutationToken, "")
	if revalidated.Code != http.StatusOK {
		t.Fatalf("revalidation response = %d: %s", revalidated.Code, revalidated.Body.String())
	}
	var envelope contract.Envelope[domain.QualityObjective]
	if err := json.NewDecoder(revalidated.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data == nil || envelope.Data.Spec.State != domain.QualityObjectiveStateReview || len(envelope.Data.Spec.Revalidations) != 1 || envelope.Data.Spec.Revalidations[0].Outcome != domain.QualityObjectiveRevalidationImproved {
		t.Fatalf("finding revalidation = %#v", envelope.Data)
	}
	confirmed := postQualityObjectiveRaw(t, service, "/api/quality/objectives/"+objective.Metadata.ID+"/confirm", `{"expectedRevision":3}`, service.mutationToken, "")
	if confirmed.Code != http.StatusConflict {
		t.Fatalf("finding confirmation response = %d: %s", confirmed.Code, confirmed.Body.String())
	}
}

func TestQualityObjectiveCoverageRevalidationHTTPOutcomesAndConfirmation(t *testing.T) {
	tests := []struct {
		name        string
		state       string
		outcome     string
		percent     float64
		wantOutcome string
		wantReason  string
		confirm     bool
	}{
		{name: "improved", state: domain.AssuranceStateSucceeded, outcome: domain.QualityRunOutcomeCoverageCollected, percent: 90, wantOutcome: domain.QualityObjectiveRevalidationImproved, wantReason: "coverage.improved", confirm: true},
		{name: "below threshold", state: domain.AssuranceStateSucceeded, outcome: domain.QualityRunOutcomeCoverageCollected, percent: 70, wantOutcome: domain.QualityObjectiveRevalidationNotImproved, wantReason: qualityCoverageThresholdNotMet},
		{name: "runner inconclusive", state: domain.AssuranceStateQueued, outcome: "", percent: 90, wantOutcome: domain.QualityObjectiveRevalidationInconclusive, wantReason: qualityCoverageRunnerUnavailable},
	}

	for index, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			service, projectID := newQualityObjectiveProject(t, "coverage-"+testCase.name)
			worktree, err := service.Worktree(context.Background(), projectID, "repo-1", "primary")
			if err != nil {
				t.Fatal(err)
			}
			profilePath := filepath.Join(service.home, "runtime", "coverage", "coverage-"+testCase.name+".out")
			selection, err := assurance.NewQualityRunnerRegistry().Select(assurance.QualityRunnerSelectionRequest{
				TechniqueID: domain.QualityTechniqueGoTestCoverage, WorktreeRoot: worktree.Spec.CanonicalPath, CoveragePath: profilePath,
			})
			if err != nil || selection.State != assurance.QualityRunnerSelectionAvailable {
				t.Fatalf("coverage runner selection = %#v, err = %v", selection, err)
			}
			completed := time.Now().UTC()
			runID := "coverage-run-" + string(rune('1'+index))
			profileArtifact, err := service.SaveAssuranceArtifact(context.Background(), ArtifactInput{
				SourceType: "quality_run", SourceID: runID, Name: "profile.out", MIME: "text/plain",
				Content: []byte("mode: set\nexample/file.go:1.1,1.2 1 1\n"),
			})
			if err != nil {
				t.Fatal(err)
			}
			run := domain.QualityRun{
				TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.QualityRunKind},
				Metadata: domain.ObjectMeta{ID: runID, Name: "Coverage run"},
				Spec: domain.QualityRunSpec{
					CampaignID: "campaign-1", ProjectID: projectID, RepositoryID: "repo-1", WorktreeID: "primary", Head: worktree.Spec.Head,
					Technique: domain.QualityTechniqueGoTestCoverage, Runner: selection.Metadata.RunnerID,
					Command: domain.CheckCommand{Executable: selection.Command.Executable, Arguments: selection.Command.Arguments}, ConfigDigest: selection.Metadata.ConfigDigest,
					State: testCase.state, Outcome: testCase.outcome, Coverage: &domain.QualityCoverage{Mode: "set", FileCount: 1, TotalStatements: 10, CoveredStatements: int(testCase.percent / 10), Percent: testCase.percent, ProfileArtifactID: profileArtifact.Metadata.ID},
					StartedAt: completed.Add(-time.Minute), CompletedAt: &completed,
				},
			}
			if err := service.store.SaveQualityRun(context.Background(), run); err != nil {
				t.Fatal(err)
			}
			objective, err := service.CreateQualityObjective(context.Background(), QualityObjectiveInput{
				ProjectID: projectID, RepositoryID: "repo-1", WorktreeID: "primary", Owner: "owner", Title: "Improve coverage " + testCase.name,
				RunIDs: []string{run.Metadata.ID},
			})
			if err != nil {
				t.Fatal(err)
			}
			decided := postQualityObjectiveDecision(t, service, objective.Metadata.ID, QualityObjectiveDecisionInput{
				ExpectedRevision: 1, Disposition: domain.QualityObjectiveDispositionPursue, Action: "raise coverage", Actor: "knowgyu", MinimumPercent: 80,
			}, service.mutationToken, "")
			if decided.Code != http.StatusOK {
				t.Fatalf("decision response = %d: %s", decided.Code, decided.Body.String())
			}
			revalidated := postQualityObjectiveRaw(t, service, "/api/quality/objectives/"+objective.Metadata.ID+"/revalidations", `{"expectedRevision":2,"qualityRunId":"`+run.Metadata.ID+`"}`, service.mutationToken, "")
			if revalidated.Code != http.StatusOK {
				t.Fatalf("revalidation response = %d: %s", revalidated.Code, revalidated.Body.String())
			}
			var envelope contract.Envelope[domain.QualityObjective]
			if err := json.NewDecoder(revalidated.Body).Decode(&envelope); err != nil {
				t.Fatal(err)
			}
			latest := envelope.Data.Spec.Revalidations[0]
			if latest.Outcome != testCase.wantOutcome || latest.ReasonCode != testCase.wantReason {
				t.Fatalf("coverage revalidation = %#v, want outcome=%q reason=%q", latest, testCase.wantOutcome, testCase.wantReason)
			}
			if testCase.confirm {
				confirmed := postQualityObjectiveRaw(t, service, "/api/quality/objectives/"+objective.Metadata.ID+"/confirm", `{"expectedRevision":3}`, service.mutationToken, "")
				if confirmed.Code != http.StatusOK {
					t.Fatalf("confirmation response = %d: %s", confirmed.Code, confirmed.Body.String())
				}
			}
		})
	}
}

func TestQualityObjectiveCoverageLifecycleFailsClosedForInvalidProfileEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*App, domain.Artifact) error
	}{
		{name: "missing file", mutate: func(_ *App, artifact domain.Artifact) error { return os.Remove(artifact.Spec.Path) }},
		{name: "deleted artifact", mutate: func(service *App, artifact domain.Artifact) error {
			_, err := service.DeleteAssuranceArtifact(context.Background(), artifact.Metadata.ID, "DELETE")
			return err
		}},
		{name: "untrusted path", mutate: func(service *App, artifact domain.Artifact) error {
			path := filepath.Join(service.home, "outside-profile.out")
			if err := os.WriteFile(path, []byte("profile"), 0o600); err != nil {
				return err
			}
			artifact.Spec.Path = path
			return service.store.UpdateAssuranceArtifact(context.Background(), artifact)
		}},
		{name: "unreadable file", mutate: func(service *App, artifact domain.Artifact) error {
			path := filepath.Join(filepath.Dir(artifact.Spec.Path), "unreadable-profile")
			if err := os.MkdirAll(path, 0o700); err != nil {
				return err
			}
			artifact.Spec.Path = path
			return service.store.UpdateAssuranceArtifact(context.Background(), artifact)
		}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			service, objective, artifact := newQualityCoverageObjectiveFixture(t, "invalid-profile-"+testCase.name)
			if err := testCase.mutate(service, artifact); err != nil {
				t.Fatal(err)
			}
			revalidated := postQualityObjectiveRaw(t, service, "/api/quality/objectives/"+objective.Metadata.ID+"/revalidations", `{"expectedRevision":2,"qualityRunId":"coverage-run-invalid"}`, service.mutationToken, "")
			if revalidated.Code != http.StatusOK {
				t.Fatalf("revalidation response = %d: %s", revalidated.Code, revalidated.Body.String())
			}
			var envelope contract.Envelope[domain.QualityObjective]
			if err := json.NewDecoder(revalidated.Body).Decode(&envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Data == nil || len(envelope.Data.Spec.Revalidations) != 1 || envelope.Data.Spec.Revalidations[0].Outcome != domain.QualityObjectiveRevalidationInconclusive || envelope.Data.Spec.Revalidations[0].ReasonCode != qualityCoverageProfileMissing {
				t.Fatalf("invalid profile evidence was promoted = %#v", envelope.Data)
			}
			confirmed := postQualityObjectiveRaw(t, service, "/api/quality/objectives/"+objective.Metadata.ID+"/confirm", `{"expectedRevision":3}`, service.mutationToken, "")
			if confirmed.Code != http.StatusConflict {
				t.Fatalf("confirmation response = %d: %s", confirmed.Code, confirmed.Body.String())
			}
		})
	}
}

func newQualityCoverageObjectiveFixture(t *testing.T, name string) (*App, domain.QualityObjective, domain.Artifact) {
	t.Helper()
	service, projectID := newQualityObjectiveProject(t, name)
	worktree, err := service.Worktree(context.Background(), projectID, "repo-1", "primary")
	if err != nil {
		t.Fatal(err)
	}
	runID := "coverage-run-invalid"
	profilePath := filepath.Join(service.home, "runtime", "coverage", name+".out")
	selection, err := assurance.NewQualityRunnerRegistry().Select(assurance.QualityRunnerSelectionRequest{
		TechniqueID: domain.QualityTechniqueGoTestCoverage, WorktreeRoot: worktree.Spec.CanonicalPath, CoveragePath: profilePath,
	})
	if err != nil || selection.State != assurance.QualityRunnerSelectionAvailable {
		t.Fatalf("coverage runner selection = %#v, err = %v", selection, err)
	}
	artifact, err := service.SaveAssuranceArtifact(context.Background(), ArtifactInput{
		SourceType: "quality_run", SourceID: runID, Name: "profile.out", MIME: "text/plain", Content: []byte("mode: set\nexample/file.go:1.1,1.2 1 1\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	completed := time.Now().UTC()
	run := domain.QualityRun{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.QualityRunKind},
		Metadata: domain.ObjectMeta{ID: runID, Name: "Coverage run"},
		Spec: domain.QualityRunSpec{
			CampaignID: "campaign-1", ProjectID: projectID, RepositoryID: "repo-1", WorktreeID: "primary", Head: worktree.Spec.Head,
			Technique: domain.QualityTechniqueGoTestCoverage, Runner: selection.Metadata.RunnerID,
			Command: domain.CheckCommand{Executable: selection.Command.Executable, Arguments: selection.Command.Arguments}, ConfigDigest: selection.Metadata.ConfigDigest,
			State: domain.AssuranceStateSucceeded, Outcome: domain.QualityRunOutcomeCoverageCollected,
			Coverage:  &domain.QualityCoverage{Mode: "set", FileCount: 1, TotalStatements: 10, CoveredStatements: 9, Percent: 90, ProfileArtifactID: artifact.Metadata.ID},
			StartedAt: completed.Add(-time.Minute), CompletedAt: &completed,
		},
	}
	if err := service.store.SaveQualityRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	objective, err := service.CreateQualityObjective(context.Background(), QualityObjectiveInput{
		ProjectID: projectID, RepositoryID: "repo-1", WorktreeID: "primary", Owner: "owner", Title: "Improve coverage " + name, RunIDs: []string{runID},
	})
	if err != nil {
		t.Fatal(err)
	}
	decided := postQualityObjectiveDecision(t, service, objective.Metadata.ID, QualityObjectiveDecisionInput{
		ExpectedRevision: 1, Disposition: domain.QualityObjectiveDispositionPursue, Action: "raise coverage", Actor: "knowgyu", MinimumPercent: 80,
	}, service.mutationToken, "")
	if decided.Code != http.StatusOK {
		t.Fatalf("decision response = %d: %s", decided.Code, decided.Body.String())
	}
	return service, objective, artifact
}

func newQualityObjectiveProject(t *testing.T, name string) (*App, string) {
	t.Helper()
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: name, Path: tempGoGitRepository(t, name)})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	return service, project.Metadata.ID
}

func postQualityObjectiveRaw(t *testing.T, service *App, path, rawBody, token, origin string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(rawBody))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("X-Control-Room-Token", token)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response := httptest.NewRecorder()
	service.Handler().ServeHTTP(response, request)
	return response
}
