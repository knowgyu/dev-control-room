package domain

import (
	"testing"
	"time"
)

func TestInspectionPlanValidatesOpaqueChecksAndDigest(t *testing.T) {
	plan := inspectionPlanFixture()
	digest, err := plan.Digest()
	if err != nil {
		t.Fatal(err)
	}
	plan.Spec.Digest = digest
	if err := plan.Validate(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func(*InspectionPlan)
	}{
		{name: "unknown check", mutate: func(item *InspectionPlan) { item.Spec.Checks[0].ID = "shell" }},
		{name: "unknown component", mutate: func(item *InspectionPlan) { item.Spec.Checks[0].ComponentID = "other" }},
		{name: "digest mismatch", mutate: func(item *InspectionPlan) { item.Spec.Digest = "sha256:" + repeatedHex(64, 'b') }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := plan
			test.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("invalid inspection plan accepted")
			}
		})
	}
}

func TestInspectionPlanReportsEachFreshnessMismatch(t *testing.T) {
	plan := inspectionPlanFixture()
	current := InspectionFreshness{Head: "head-2", ConfigDigest: digestForInspection('e'), ToolDigest: digestForInspection('f'), EvidenceDigest: digestForInspection('g')}
	reasons := plan.StaleReasons(current)
	want := []string{"head", "config", "tool", "evidence"}
	if len(reasons) != len(want) {
		t.Fatalf("reasons = %#v, want %#v", reasons, want)
	}
	for index := range want {
		if reasons[index] != want[index] {
			t.Fatalf("reason[%d] = %q, want %q", index, reasons[index], want[index])
		}
	}
}

func inspectionPlanFixture() InspectionPlan {
	now := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	return InspectionPlan{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: InspectionPlanKind}, Metadata: ObjectMeta{ID: "plan-1", Name: "Plan"},
		Spec: InspectionPlanSpec{
			ProjectID: "project-1", RepositoryID: "repo-1", WorktreeID: "primary", Branch: "main", Head: "head-1",
			BaselineDigest: digestForInspection('a'), ConfigDigest: digestForInspection('b'), EvidenceDigest: digestForInspection('d'), ToolDigest: digestForInspection('c'),
			Components: []InspectionComponent{{ID: "component-api"}}, Checks: []InspectionCheck{{ID: InspectionCheckRuff, ComponentID: "component-api", AdapterID: "adapter.ruff.v1", ParserID: "parser.ruff.v1"}},
			State: InspectionPlanStateApproved, Revision: 1, Version: InspectionPlanVersion, CreatedAt: now, UpdatedAt: now,
		},
	}
}

func digestForInspection(char byte) string { return "sha256:" + repeatedHex(64, char) }
func repeatedHex(count int, char byte) string {
	value := make([]byte, count)
	for index := range value {
		value[index] = char
	}
	return string(value)
}
