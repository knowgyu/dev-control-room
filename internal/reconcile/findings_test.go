package reconcile

import (
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/collector"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestRepositoryFindingsAreDeterministicAndResolveClearedState(t *testing.T) {
	now := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	state := collector.State{Path: "/tmp/fixture", Dirty: true, Ahead: 1, Behind: 2, Detached: true, UnsafeCleanup: true}
	first := RepositoryFindings("project", "repo", state, "observation-1", now, nil)
	second := RepositoryFindings("project", "repo", state, "observation-2", now.Add(time.Minute), first)
	if len(first) != len(second) {
		t.Fatalf("finding set changed shape: %d vs %d", len(first), len(second))
	}
	for index := range first {
		if first[index].Metadata.ID != second[index].Metadata.ID || first[index].Spec.Fingerprint != second[index].Spec.Fingerprint {
			t.Fatalf("finding identity was not stable: %#v %#v", first[index], second[index])
		}
		if second[index].Spec.EvidenceRefs[0] != "observation-2" {
			t.Fatalf("finding evidence was not refreshed: %#v", second[index])
		}
	}
	cleared := RepositoryFindings("project", "repo", collector.State{}, "observation-3", now.Add(2*time.Minute), first)
	resolved := 0
	for _, finding := range cleared {
		if finding.Spec.State == domain.FindingResolved {
			resolved++
		}
	}
	if resolved == 0 {
		t.Fatal("cleared finding set did not resolve prior findings")
	}
}
