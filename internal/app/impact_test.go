package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestAssuranceImpactAndTraceExposeMeasuredEvidenceWithoutLocalPaths(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Impact", Path: tempGitRepository(t, "impact")})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateAssuranceSession(context.Background(), AssuranceSessionInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary"})
	if err != nil {
		t.Fatal(err)
	}
	invocation, err := service.RunAgentInvocation(context.Background(), AgentInvocationInput{SessionID: session.Metadata.ID, Provider: "fake", ProfileID: "fake", RequestedModel: "fixture-model"})
	if err != nil {
		t.Fatal(err)
	}
	if len(invocation.Spec.ArtifactIDs) != 1 || invocation.Spec.TraceID == "" || invocation.Spec.ProjectID != project.Metadata.ID {
		t.Fatalf("invocation trace metadata = %#v", invocation)
	}
	artifacts, err := service.AssuranceArtifacts(context.Background())
	if err != nil || len(artifacts) != 1 {
		t.Fatalf("invocation artifacts = %#v, %v", artifacts, err)
	}
	previousStart := time.Now().UTC().Add(-8 * 24 * time.Hour)
	previous := domain.AgentInvocation{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AgentInvocationKind}, Metadata: domain.ObjectMeta{ID: "old-invocation", Name: "Previous invocation"}, Spec: domain.AgentInvocationSpec{SessionID: session.Metadata.ID, ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Head: invocation.Spec.Head, Provider: "fake", ProfileID: "fake", RequestedModel: "fixture-model", SelectionSource: "user", State: domain.AssuranceStateSucceeded, IdempotencyKey: "old-invocation", StartedAt: previousStart, CompletedAt: timePtr(previousStart.Add(time.Second))}}
	if err := service.store.SaveAgentInvocation(context.Background(), previous); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveAssuranceArtifact(context.Background(), ArtifactInput{SourceType: "unrelated", SourceID: "other-project", Name: "unrelated.json", MIME: "application/json", Content: []byte(`{"scope":"other"}`)}); err != nil {
		t.Fatal(err)
	}
	value := 12.5
	effect, err := service.CreateEffect(context.Background(), EffectInput{
		ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Fingerprint: "sha256:impact-trace", Kind: domain.EffectMeasured,
		SourceRunID: invocation.Metadata.ID, EvidenceIDs: invocation.Spec.ArtifactIDs, Adopted: true, Reverified: true, AdoptedCommit: invocation.Spec.Head, ReverificationRunID: invocation.Spec.TraceID, ReverifiedCommit: invocation.Spec.Head, Label: "검증된 개선", Value: value, ValueKnown: true, Unit: "분",
		MetricKey: "time_saved", Outcome: "improved",
	})
	if err != nil {
		t.Fatal(err)
	}
	impact, err := service.AssuranceImpact(context.Background(), AssuranceImpactQuery{Provider: "fake", Model: "fixture-model", ProjectID: project.Metadata.ID, Days: 7, Now: time.Now().UTC().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if impact.DataQuality.MeasuredEffects != 1 || impact.Traceability.CompleteEffects != 1 || impact.Traceability.Status != "complete" {
		t.Fatalf("impact evidence summary = %#v", impact)
	}
	if impact.DataQuality.RecordsTotal != 3 {
		t.Fatalf("filtered artifact leaked into impact records: %#v", impact.DataQuality)
	}
	metric := metricByKey(impact.Metrics, "verified_effects")
	if metric == nil || metric.Value == nil || *metric.Value != 1 || metric.State != "measured" {
		t.Fatalf("verified effect metric = %#v", metric)
	}
	agentRate := metricByKey(impact.Metrics, "agent_success_rate")
	if agentRate == nil || agentRate.Comparison == nil || agentRate.Comparison.PreviousValue == nil || *agentRate.Comparison.PreviousValue != 100 {
		t.Fatalf("agent comparison = %#v", agentRate)
	}
	if agentRate.SampleCount != 1 || agentRate.Comparison.State != "neutral" {
		t.Fatalf("agent comparison semantics = %#v", agentRate)
	}
	trace, err := service.AssuranceTrace(context.Background(), effect.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !trace.Complete || len(trace.Artifacts) != 1 || trace.Artifacts[0].SHA256 == "" || trace.Artifacts[0].Present != true {
		t.Fatalf("trace = %#v", trace)
	}
	encoded, err := json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), artifacts[0].Spec.Path) || strings.Contains(string(encoded), service.home) {
		t.Fatalf("trace leaked local path: %s", encoded)
	}
	if !strings.Contains(string(encoded), `"fromId"`) || !strings.Contains(string(encoded), `"toId"`) {
		t.Fatalf("trace did not expose typed link IDs: %s", encoded)
	}
	if !hasTraceRelation(trace.Links, "reverification") || !hasTraceRelation(trace.Links, "adopted_commit") || !hasTraceRelation(trace.Links, "reverified_commit") {
		t.Fatalf("trace omitted adoption chain = %#v", trace.Links)
	}
	report, err := service.ExportAssuranceReport(context.Background(), AssuranceReportQuery{Format: "json", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(report.Body), artifacts[0].Spec.Path) || strings.Contains(string(report.Body), service.home) {
		t.Fatalf("report leaked local path: %s", report.Body)
	}
	if strings.Contains(string(report.Body), "unrelated.json") {
		t.Fatalf("report included an unrelated artifact: %s", report.Body)
	}
}

func TestAssuranceArtifactArchiveManifestPinAndRestore(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	artifact, err := service.SaveAssuranceArtifact(context.Background(), ArtifactInput{SourceType: "fixture", SourceID: "fixture-1", Name: "report.json", MIME: "application/json", Content: []byte(`{"ok":true}`)})
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "assurance-pack")
	exported, err := service.ExportAssuranceArtifacts(context.Background(), []string{artifact.Metadata.ID}, archive)
	if err != nil || !exported.Verified || exported.Manifest != archiveManifestName || exported.ManifestSHA == "" {
		t.Fatalf("export = %#v, %v", exported, err)
	}
	if _, err := os.Stat(filepath.Join(archive, archiveManifestName)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(artifact.Spec.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetAssuranceArtifactRetention(context.Background(), artifact.Metadata.ID, domain.ArtifactRetentionPinned); err != nil {
		t.Fatal(err)
	}
	if _, err := service.DeleteAssuranceArtifact(context.Background(), artifact.Metadata.ID, "DELETE"); err == nil {
		t.Fatal("pinned artifact deletion was accepted")
	}
	restored, err := service.RestoreAssuranceArtifact(context.Background(), artifact.Metadata.ID)
	if err != nil || restored.Spec.RestoredAt == nil || restored.Spec.Retention != domain.ArtifactRetentionPinned {
		t.Fatalf("restored = %#v, %v", restored, err)
	}
	if data, err := os.ReadFile(restored.Spec.Path); err != nil || string(data) != `{"ok":true}` {
		t.Fatalf("restored bytes = %q, %v", data, err)
	}
	if _, err := service.SetAssuranceArtifactRetention(context.Background(), artifact.Metadata.ID, domain.ArtifactRetentionActive); err != nil {
		t.Fatal(err)
	}
	deleted, err := service.DeleteAssuranceArtifact(context.Background(), artifact.Metadata.ID, "DELETE")
	if err != nil || deleted.Spec.Retention != domain.ArtifactRetentionDeleted {
		t.Fatalf("deleted = %#v, %v", deleted, err)
	}
}

func TestAssuranceImpactHTTPContracts(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	artifact, err := service.SaveAssuranceArtifact(context.Background(), ArtifactInput{SourceType: "fixture", SourceID: "http-1", Name: "result.json", MIME: "application/json", Content: []byte(`{"passed":true}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateEffect(context.Background(), EffectInput{Fingerprint: "sha256:http-effect", Kind: domain.EffectUnavailable, SourceRunID: "run-missing", EvidenceIDs: []string{artifact.Metadata.ID}, Label: "근거 부족"}); err != nil {
		t.Fatal(err)
	}
	handler := service.Handler()
	request := httptest.NewRequest(http.MethodGet, "/api/assurance/impact?days=7", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"metrics"`) || !strings.Contains(recorder.Body.String(), `"baselineState"`) {
		t.Fatalf("impact response = %d %s", recorder.Code, recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/api/assurance/impact/export?format=csv&days=7", nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Header().Get("Content-Type"), "text/csv") || !strings.Contains(recorder.Body.String(), "metric,label,value") {
		t.Fatalf("impact export response = %d %s", recorder.Code, recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "/api/assurance/artifacts/"+artifact.Metadata.ID+"/retention", strings.NewReader(`{"retention":"pinned"}`))
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"retention":"pinned"`) {
		t.Fatalf("retention response = %d %s", recorder.Code, recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/api/assurance/artifacts/"+artifact.Metadata.ID, nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || strings.Contains(recorder.Body.String(), artifact.Spec.Path) || strings.Contains(recorder.Body.String(), service.home) {
		t.Fatalf("artifact detail leaked local path = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestAssuranceArtifactRetentionRejectsCorruptLocalFile(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	artifact, err := service.SaveAssuranceArtifact(context.Background(), ArtifactInput{SourceType: "fixture", SourceID: "corrupt-1", Name: "report.json", MIME: "application/json", Content: []byte(`{"ok":true}`)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact.Spec.Path, []byte(`{"ok":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetAssuranceArtifactRetention(context.Background(), artifact.Metadata.ID, domain.ArtifactRetentionPinned); err == nil {
		t.Fatal("corrupt local artifact was pinned")
	}
}

func TestAssuranceImpactTimeSavedUsesExplicitMetricAndConsistentUnits(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	if _, err := service.CreateEffect(context.Background(), EffectInput{Fingerprint: "sha256:not-time", Kind: domain.EffectMeasured, MetricKey: "defects_prevented", Value: 99, ValueKnown: true, Unit: "건", Label: "방지 결함"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateEffect(context.Background(), EffectInput{Fingerprint: "sha256:time", Kind: domain.EffectMeasured, MetricKey: "time_saved", Value: 12.5, ValueKnown: true, Unit: "분", Label: "절감 시간"}); err != nil {
		t.Fatal(err)
	}

	impact, err := service.AssuranceImpact(context.Background(), AssuranceImpactQuery{Days: 7, Now: time.Now().UTC().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	metric := metricByKey(impact.Metrics, "time_saved")
	if metric == nil || metric.Value == nil || *metric.Value != 12.5 || metric.SampleCount != 1 || metric.Unit != "분" {
		t.Fatalf("time saved metric = %#v", metric)
	}
}

func TestAssuranceImpactSeparatesMeasuredAndUserEstimatedTime(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	if _, err := service.CreateEffect(context.Background(), EffectInput{Fingerprint: "sha256:measured-time", Kind: domain.EffectMeasured, MetricKey: "time_saved", Value: 12, ValueKnown: true, Unit: "분", Label: "측정 시간"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateEffect(context.Background(), EffectInput{Fingerprint: "sha256:estimated-time", Kind: domain.EffectUserEstimated, MetricKey: "time_saved", Value: 30, ValueKnown: true, Unit: "분", Label: "추정 시간"}); err != nil {
		t.Fatal(err)
	}

	impact, err := service.AssuranceImpact(context.Background(), AssuranceImpactQuery{Days: 7, Now: time.Now().UTC().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	measured := metricByKey(impact.Metrics, "time_saved")
	estimated := metricByKey(impact.Metrics, "time_saved_estimated")
	if measured == nil || measured.Value == nil || *measured.Value != 12 || measured.State != "measured" {
		t.Fatalf("measured time metric = %#v", measured)
	}
	if estimated == nil || estimated.Value == nil || *estimated.Value != 30 || estimated.State != "user_estimated" {
		t.Fatalf("estimated time metric = %#v", estimated)
	}
	if impact.DataQuality.MeasuredEffects != 1 || impact.DataQuality.UserEstimated != 1 {
		t.Fatalf("effect classifications = %#v", impact.DataQuality)
	}
}

func TestEffectVerificationRequiresSuccessfulReverificationAtAdoptedCommit(t *testing.T) {
	now := time.Now().UTC()
	run := domain.QualityRun{Spec: domain.QualityRunSpec{Head: "head-good", State: domain.AssuranceStateSucceeded}}
	effect := domain.Effect{Spec: domain.EffectSpec{Adopted: true, Reverified: true, AdoptedCommit: "head-good", ReverificationRunID: "run-1", ReverifiedCommit: "head-bad", CreatedAt: now}}
	if effectAdoptionComplete(effect, nil, map[string]domain.QualityRun{"run-1": run}) {
		t.Fatal("mismatched reverification commit was accepted")
	}
	effect.Spec.AdoptedCommit = "another-commit"
	effect.Spec.ReverifiedCommit = "head-good"
	if effectAdoptionComplete(effect, nil, map[string]domain.QualityRun{"run-1": run}) {
		t.Fatal("reverification at a different adopted commit was accepted")
	}
	effect.Spec.AdoptedCommit = "head-good"
	effect.Spec.ReverifiedCommit = "head-good"
	if !effectAdoptionComplete(effect, nil, map[string]domain.QualityRun{"run-1": run}) {
		t.Fatal("successful reverification at adopted commit was rejected")
	}
	effect.Spec.ReverificationRunID = " run-1 "
	run.Spec.Head = " head-good "
	if !effectAdoptionComplete(effect, nil, map[string]domain.QualityRun{"run-1": run}) {
		t.Fatal("trimmed successful reverification metadata was rejected")
	}
	run.Spec.State = domain.AssuranceStateFailed
	if effectAdoptionComplete(effect, nil, map[string]domain.QualityRun{"run-1": run}) {
		t.Fatal("failed reverification was accepted")
	}
}

func TestAssuranceImpactAttributesTraceIDAsSource(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Trace source", Path: tempGitRepository(t, "trace-source")})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateAssuranceSession(context.Background(), AssuranceSessionInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary"})
	if err != nil {
		t.Fatal(err)
	}
	invocation, err := service.RunAgentInvocation(context.Background(), AgentInvocationInput{SessionID: session.Metadata.ID, Provider: "trace-provider", ProfileID: "fake", RequestedModel: "trace-model"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateEffect(context.Background(), EffectInput{
		ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Fingerprint: "sha256:trace-source", Kind: domain.EffectUnavailable,
		TraceIDs: []string{invocation.Spec.TraceID}, Label: "Trace source", Reason: "source uses trace identity",
	}); err != nil {
		t.Fatal(err)
	}
	impact, err := service.AssuranceImpact(context.Background(), AssuranceImpactQuery{Provider: "trace-provider", Model: "trace-model", ProjectID: project.Metadata.ID, Days: 7, Now: time.Now().UTC().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if impact.Traceability.EffectsTotal != 1 || impact.Traceability.PartialEffects != 1 || impact.Traceability.UnresolvedEffects != 0 {
		t.Fatalf("trace-id source was not attributed = %#v", impact.Traceability)
	}
}

func hasTraceRelation(links []AssuranceTraceLink, relation string) bool {
	for _, link := range links {
		if link.Relation == relation {
			return true
		}
	}
	return false
}

func metricByKey(metrics []AssuranceMetric, key string) *AssuranceMetric {
	for index := range metrics {
		if metrics[index].Key == key {
			return &metrics[index]
		}
	}
	return nil
}
