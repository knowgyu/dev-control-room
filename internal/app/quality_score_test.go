package app

import (
	"testing"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestCalculateRepositoryQualityScoreUsesDeterministicOutcomes(t *testing.T) {
	plan := scorePlanFixture()
	results := []domain.InspectionResult{
		{ComponentID: "component-api", CheckID: domain.InspectionCheckRuff, Outcome: domain.InspectionOutcomeFindings, Findings: []domain.InspectionFinding{{Severity: domain.SeverityHigh}}},
		{ComponentID: "component-web", CheckID: domain.InspectionCheckVitest, Outcome: domain.InspectionOutcomeTestsFailed, Prompt: "ignored", Model: "ignored", Usage: map[string]any{"tokens": 999}},
	}
	score, err := CalculateRepositoryQualityScore(RepositoryQualityScoreInput{Plan: plan, Results: results, Current: ptrInspectionFreshness(plan)})
	if err != nil {
		t.Fatal(err)
	}
	if score.Spec.Components[0].Value != 70 || score.Spec.Components[1].Value != 0 || score.Spec.Overall != 35 || score.Spec.Status != domain.QualityScoreStatusFresh {
		t.Fatalf("score = %#v", score.Spec)
	}

	results[0].Findings[0].Severity = domain.SeverityCritical
	changed, err := CalculateRepositoryQualityScore(RepositoryQualityScoreInput{Plan: plan, Results: results, Current: ptrInspectionFreshness(plan)})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Spec.Components[0].Value == score.Spec.Components[0].Value {
		t.Fatal("severity did not affect score")
	}
}

func TestCalculateRepositoryQualityScoreRequiresCompleteFreshEvidence(t *testing.T) {
	plan := scorePlanFixture()
	score, err := CalculateRepositoryQualityScore(RepositoryQualityScoreInput{
		Plan: plan,
		Results: []domain.InspectionResult{{
			ComponentID: "component-api", CheckID: domain.InspectionCheckRuff, Outcome: domain.InspectionOutcomeClean,
		}},
		Current: ptrInspectionFreshness(plan),
	})
	if err != nil {
		t.Fatal(err)
	}
	if score.Spec.Status != domain.QualityScoreStatusInconclusive || score.Spec.Components[1].Outcome != domain.InspectionOutcomeInconclusive {
		t.Fatalf("incomplete score = %#v", score.Spec)
	}

	allResults := []domain.InspectionResult{
		{ComponentID: "component-api", CheckID: domain.InspectionCheckRuff, Outcome: domain.InspectionOutcomeClean},
		{ComponentID: "component-web", CheckID: domain.InspectionCheckVitest, Outcome: domain.InspectionOutcomeClean},
	}
	score, err = CalculateRepositoryQualityScore(RepositoryQualityScoreInput{Plan: plan, Results: allResults})
	if err != nil {
		t.Fatal(err)
	}
	if score.Spec.Status != domain.QualityScoreStatusInconclusive {
		t.Fatalf("score without current freshness = %#v", score.Spec)
	}
	partialFreshness := ptrInspectionFreshness(plan)
	partialFreshness.EvidenceDigest = ""
	score, err = CalculateRepositoryQualityScore(RepositoryQualityScoreInput{Plan: plan, Results: allResults, Current: partialFreshness})
	if err != nil {
		t.Fatal(err)
	}
	if score.Spec.Status == domain.QualityScoreStatusFresh {
		t.Fatalf("score with incomplete current freshness was reported fresh: %#v", score.Spec)
	}

	allResults = append(allResults, domain.InspectionResult{ComponentID: "component-api", CheckID: "unknown", Outcome: domain.InspectionOutcomeClean})
	if _, err = CalculateRepositoryQualityScore(RepositoryQualityScoreInput{Plan: plan, Results: allResults, Current: ptrInspectionFreshness(plan)}); err == nil {
		t.Fatal("unknown result was accepted")
	}
}

func TestQualityComparisonRequiresCompatibleExplicitPair(t *testing.T) {
	plan := scorePlanFixture()
	before, err := CalculateRepositoryQualityScore(RepositoryQualityScoreInput{Plan: plan, Results: []domain.InspectionResult{{ComponentID: "component-api", CheckID: domain.InspectionCheckRuff, Outcome: domain.InspectionOutcomeClean}, {ComponentID: "component-web", CheckID: domain.InspectionCheckVitest, Outcome: domain.InspectionOutcomeClean}}, Current: ptrInspectionFreshness(plan)})
	if err != nil {
		t.Fatal(err)
	}
	afterPlan := plan
	after, err := CalculateRepositoryQualityScore(RepositoryQualityScoreInput{Plan: afterPlan, Results: []domain.InspectionResult{{ComponentID: "component-api", CheckID: domain.InspectionCheckRuff, Outcome: domain.InspectionOutcomeFindings, Findings: []domain.InspectionFinding{{Severity: domain.SeverityInfo}}}, {ComponentID: "component-web", CheckID: domain.InspectionCheckVitest, Outcome: domain.InspectionOutcomeClean}}, Current: ptrInspectionFreshness(afterPlan)})
	if err != nil {
		t.Fatal(err)
	}
	comparison := CompareQualityScores(before, after)
	if comparison.Status != domain.QualityComparisonComparable || comparison.Delta != -3 {
		t.Fatalf("comparison = %#v", comparison)
	}
	after.Spec.ToolDigest = digestForScore('z')
	if comparison = CompareQualityScores(before, after); !comparison.IsInconclusive() {
		t.Fatalf("incompatible comparison = %#v", comparison)
	}
	if comparison = CompareQualityScorePair(domain.QualityComparisonInput{Before: &before}); !comparison.IsInconclusive() {
		t.Fatalf("missing after comparison = %#v", comparison)
	}
	afterPlan.Spec.Head = "head-2"
	after, err = CalculateRepositoryQualityScore(RepositoryQualityScoreInput{Plan: afterPlan, Results: []domain.InspectionResult{{ComponentID: "component-api", CheckID: domain.InspectionCheckRuff, Outcome: domain.InspectionOutcomeClean}, {ComponentID: "component-web", CheckID: domain.InspectionCheckVitest, Outcome: domain.InspectionOutcomeClean}}, Current: ptrInspectionFreshness(afterPlan)})
	if err != nil {
		t.Fatal(err)
	}
	if comparison = CompareQualityScores(before, after); comparison.Reason != "head_mismatch" {
		t.Fatalf("head-incompatible comparison = %#v", comparison)
	}
}

func scorePlanFixture() domain.InspectionPlan {
	plan := domain.InspectionPlan{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.InspectionPlanKind}, Metadata: domain.ObjectMeta{ID: "plan-1", Name: "Plan"}, Spec: domain.InspectionPlanSpec{
		ProjectID: "project-1", RepositoryID: "repo-1", WorktreeID: "primary", Branch: "main", Head: "head-1", BaselineDigest: digestForScore('a'), ConfigDigest: digestForScore('b'), EvidenceDigest: digestForScore('d'), ToolDigest: digestForScore('c'),
		Components: []domain.InspectionComponent{{ID: "component-api"}, {ID: "component-web"}}, Checks: []domain.InspectionCheck{{ID: domain.InspectionCheckRuff, ComponentID: "component-api", AdapterID: "adapter.ruff.v1", ParserID: "parser.ruff.v1"}, {ID: domain.InspectionCheckVitest, ComponentID: "component-web", AdapterID: "adapter.vitest.v1", ParserID: "parser.vitest.v1"}}, State: domain.InspectionPlanStateApproved, Revision: 1, Version: domain.InspectionPlanVersion,
	}}
	return plan
}

func digestForScore(char byte) string {
	value := make([]byte, 64)
	for index := range value {
		value[index] = char
	}
	return "sha256:" + string(value)
}

func ptrInspectionFreshness(plan domain.InspectionPlan) *domain.InspectionFreshness {
	return &domain.InspectionFreshness{Head: plan.Spec.Head, ConfigDigest: plan.Spec.ConfigDigest, ToolDigest: plan.Spec.ToolDigest, EvidenceDigest: plan.Spec.EvidenceDigest}
}
