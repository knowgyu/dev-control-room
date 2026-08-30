package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestQualityObjectiveServiceAndQualityHomeUsePersistedData(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{
		Name: "Quality home",
		Path: tempGoGitRepository(t, "quality-home"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	campaign, err := service.CreateQualityCampaign(context.Background(), QualityCampaignInput{
		ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Name: "Quality home campaign",
	})
	if err != nil {
		t.Fatal(err)
	}
	objective, err := service.CreateQualityObjective(context.Background(), QualityObjectiveInput{
		ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary",
		Owner: "owner", Title: "Improve regression confidence", CampaignID: campaign.Metadata.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if objective.Spec.State != domain.QualityObjectiveStateDraft || objective.Spec.Revision != 1 {
		t.Fatalf("objective = %#v", objective)
	}
	items, err := service.QualityObjectives(context.Background())
	if err != nil || len(items) != 1 || items[0].Metadata.ID != objective.Metadata.ID {
		t.Fatalf("objectives = %#v, err = %v", items, err)
	}
	loaded, err := service.QualityObjective(context.Background(), objective.Metadata.ID)
	if err != nil || loaded.Spec.Title != objective.Spec.Title {
		t.Fatalf("loaded objective = %#v, err = %v", loaded, err)
	}

	now := time.Now().UTC()
	completed := now.Add(time.Second)
	if err := service.store.SaveQualityRun(context.Background(), domain.QualityRun{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.QualityRunKind},
		Metadata: domain.ObjectMeta{ID: "run-failed", Name: "Failed run"},
		Spec: domain.QualityRunSpec{
			CampaignID: campaign.Metadata.ID, ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary",
			Head: objective.Spec.Head, Technique: domain.QualityTechniqueProperty, Runner: "quality.test",
			State: domain.AssuranceStateFailed, StartedAt: now, CompletedAt: &completed,
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.store.SaveBaseline(context.Background(), domain.PRCIBaseline{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.PRCIBaselineKind},
		Metadata: domain.ObjectMeta{ID: "baseline-stale", Name: "Stale baseline"},
		Spec: domain.PRCIBaselineSpec{
			ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Head: objective.Spec.Head,
			SourceDigest: "sha256:baseline", CapturedAt: now.Add(-time.Hour), FreshUntil: now.Add(-time.Minute), State: "fresh",
			Entries: []domain.BaselineEntry{{ID: "entry-1", Name: "test", Classification: domain.BaselineObserved}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.store.SaveAssuranceProposal(context.Background(), domain.AssuranceProposal{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AssuranceProposalKind},
		Metadata: domain.ObjectMeta{ID: "assurance-proposal", Name: "Proposal"},
		Spec: domain.AssuranceProposalSpec{
			SessionID: "session-1", ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary",
			BaseHead: objective.Spec.Head, IsolationPath: "C:/isolated", PatchDigest: "sha256:patch",
			Purpose: "add a property", State: "proposed", CreatedAt: now,
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.store.SaveFinding(context.Background(), domain.Finding{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.FindingKind},
		Metadata: domain.ObjectMeta{ID: "finding-open", Name: "Open finding"},
		Spec: domain.FindingSpec{
			ProjectID: project.Metadata.ID, RepositoryID: "repo-1", FindingType: "quality",
			Fingerprint: "sha256:finding", Severity: domain.SeverityAttention, Confidence: domain.ConfidenceConfirmed,
			Summary: "property coverage is missing", RecommendedNext: "review the property proposal",
			FirstObserved: now, LastObserved: now, State: domain.FindingOpen,
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateQualityObjective(context.Background(), QualityObjectiveInput{
		ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary",
		Owner: "owner", Title: "Improve property coverage", FindingIDs: []string{"finding-open"},
	}); err != nil {
		t.Fatal(err)
	}

	home, err := service.QualityHome(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if home.Objectives == nil || home.Queue == nil || home.Summary.ActiveObjectives != 2 {
		t.Fatalf("quality home empty fields = %#v", home)
	}
	if home.Summary.FailedRuns != 1 || home.Summary.StaleBaselines != 1 || home.Summary.UnreviewedProposals != 1 || home.Summary.OpenFindings != 2 || home.Summary.QueueItems != 4 {
		t.Fatalf("quality home summary = %#v, queue = %#v", home.Summary, home.Queue)
	}
	for _, item := range home.Queue {
		if item.ID == "finding:finding-open" {
			t.Fatalf("attached finding was duplicated in quality home queue: %#v", home.Queue)
		}
	}
}

func TestCreateQualityObjectiveValidatesRelationshipReferences(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	project, err := service.AddProject(context.Background(), AddProjectInput{
		Name: "Quality objective links",
		Path: tempGoGitRepository(t, "quality-objective-links"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	base := QualityObjectiveInput{
		ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary",
		Owner: "owner", Title: "Validate links",
	}

	_, err = service.CreateQualityObjective(context.Background(), QualityObjectiveInput{
		ProjectID: base.ProjectID, RepositoryID: base.RepositoryID, WorktreeID: base.WorktreeID,
		Owner: base.Owner, Title: base.Title, FindingIDs: []string{"finding-missing"},
	})
	if err == nil || contract.Classify(err).Code != contract.ErrorNotFound {
		t.Fatalf("missing finding error = %v, code = %s", err, contract.Classify(err).Code)
	}

	otherProject, err := service.AddProject(context.Background(), AddProjectInput{
		Name: "Other quality objective project",
		Path: tempGoGitRepository(t, "quality-objective-other-project"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := service.store.SaveFinding(context.Background(), domain.Finding{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.FindingKind},
		Metadata: domain.ObjectMeta{ID: "finding-other-project", Name: "Other project finding"},
		Spec: domain.FindingSpec{
			ProjectID: otherProject.Metadata.ID, RepositoryID: "repo-1", FindingType: "quality",
			Fingerprint: "sha256:other-project", Severity: domain.SeverityAttention, Confidence: domain.ConfidenceConfirmed,
			Summary: "finding belongs to another project", RecommendedNext: "select the matching project",
			FirstObserved: now, LastObserved: now, State: domain.FindingOpen,
		},
	}); err != nil {
		t.Fatal(err)
	}
	_, err = service.CreateQualityObjective(context.Background(), QualityObjectiveInput{
		ProjectID: base.ProjectID, RepositoryID: base.RepositoryID, WorktreeID: base.WorktreeID,
		Owner: base.Owner, Title: "Reject out of scope finding", FindingIDs: []string{"finding-other-project"},
	})
	if err == nil || contract.Classify(err).Code != contract.ErrorInvalidInput {
		t.Fatalf("out of scope finding error = %v, code = %s", err, contract.Classify(err).Code)
	}

	_, err = service.CreateQualityObjective(context.Background(), QualityObjectiveInput{
		ProjectID: base.ProjectID, RepositoryID: base.RepositoryID, WorktreeID: base.WorktreeID,
		Owner: base.Owner, Title: "Reject missing campaign", CampaignID: "campaign-missing",
	})
	if err == nil || contract.Classify(err).Code != contract.ErrorNotFound {
		t.Fatalf("missing campaign error = %v, code = %s", err, contract.Classify(err).Code)
	}
}

func TestQualityObjectiveHTTPUsesEnvelopeAndMutationProtection(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	getRequest := httptest.NewRequest(http.MethodGet, "/api/quality/home", nil)
	getRecorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("quality home response = %d: %s", getRecorder.Code, getRecorder.Body.String())
	}
	var homeEnvelope contract.Envelope[QualityHome]
	if err := json.NewDecoder(getRecorder.Body).Decode(&homeEnvelope); err != nil || !homeEnvelope.OK || homeEnvelope.Data == nil || homeEnvelope.Data.Queue == nil || homeEnvelope.Data.Objectives == nil {
		t.Fatalf("quality home envelope = %#v, err = %v", homeEnvelope, err)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/quality/objectives", nil)
	listRecorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("quality objective list response = %d: %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listEnvelope contract.Envelope[[]domain.QualityObjective]
	if err := json.NewDecoder(listRecorder.Body).Decode(&listEnvelope); err != nil || !listEnvelope.OK || listEnvelope.Data == nil || *listEnvelope.Data == nil {
		t.Fatalf("quality objective list envelope = %#v, err = %v", listEnvelope, err)
	}

	detailRequest := httptest.NewRequest(http.MethodGet, "/api/quality/objectives/objective-missing", nil)
	detailRecorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusNotFound {
		t.Fatalf("quality objective detail response = %d: %s", detailRecorder.Code, detailRecorder.Body.String())
	}
	var detailEnvelope contract.Envelope[domain.QualityObjective]
	if err := json.NewDecoder(detailRecorder.Body).Decode(&detailEnvelope); err != nil || detailEnvelope.OK || detailEnvelope.Error == nil || detailEnvelope.Error.Code != contract.ErrorNotFound {
		t.Fatalf("quality objective detail envelope = %#v, err = %v", detailEnvelope, err)
	}

	body, err := json.Marshal(QualityObjectiveInput{Owner: "owner", Title: "missing scope"})
	if err != nil {
		t.Fatal(err)
	}
	postRequest := httptest.NewRequest(http.MethodPost, "/api/quality/objectives", bytes.NewReader(body))
	postRecorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(postRecorder, postRequest)
	if postRecorder.Code != http.StatusForbidden {
		t.Fatalf("unprotected objective response = %d: %s", postRecorder.Code, postRecorder.Body.String())
	}
	var forbidden contract.Envelope[domain.QualityObjective]
	if err := json.NewDecoder(postRecorder.Body).Decode(&forbidden); err != nil || forbidden.OK || forbidden.Error == nil || forbidden.Error.Code != contract.ErrorForbidden {
		t.Fatalf("forbidden envelope = %#v, err = %v", forbidden, err)
	}
}

func TestQualityObjectiveHTTPAuthenticatedPostReturnsEnvelopePayload(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	project, err := service.AddProject(context.Background(), AddProjectInput{
		Name: "Quality objective HTTP",
		Path: tempGoGitRepository(t, "quality-objective-http"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}

	input := QualityObjectiveInput{
		ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary",
		Owner: "owner", Title: "Improve HTTP contract",
	}
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/quality/objectives", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://127.0.0.1:38471")
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("authenticated objective response = %d: %s", recorder.Code, recorder.Body.String())
	}

	var envelope contract.Envelope[domain.QualityObjective]
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.OK || envelope.Error != nil || envelope.Data == nil || envelope.Schema != contract.EnvelopeSchema {
		t.Fatalf("authenticated objective envelope = %#v", envelope)
	}
	if envelope.Data.Spec.ProjectID != input.ProjectID || envelope.Data.Spec.RepositoryID != input.RepositoryID || envelope.Data.Spec.WorktreeID != input.WorktreeID || envelope.Data.Spec.Owner != input.Owner || envelope.Data.Spec.Title != input.Title {
		t.Fatalf("authenticated objective payload = %#v", envelope.Data)
	}
}
