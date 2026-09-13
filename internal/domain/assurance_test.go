package domain

import (
	"strings"
	"testing"
	"time"
)

func TestArtifactEvidenceStateRequiresReason(t *testing.T) {
	artifact := Artifact{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: ArtifactKind},
		Metadata: ObjectMeta{ID: "artifact-1", Name: "coverage.out"},
		Spec: ArtifactSpec{
			SourceType: "quality_run", SourceID: "run-1", Path: "coverage.out",
			SHA256: strings.Repeat("a", 64), Retention: ArtifactRetentionActive,
			EvidenceState: ArtifactEvidenceStatePartial, CreatedAt: time.Now().UTC(),
		},
	}
	if err := artifact.Validate(); err == nil {
		t.Fatal("partial artifact without an evidence reason was accepted")
	}
	artifact.Spec.EvidenceReason = "runner.timeout"
	if err := artifact.Validate(); err != nil {
		t.Fatalf("partial artifact with an evidence reason was rejected: %v", err)
	}
	artifact.Spec.EvidenceState = ""
	artifact.Spec.EvidenceReason = ""
	if err := artifact.Validate(); err != nil || !artifact.EvidenceValid() {
		t.Fatalf("legacy artifact evidence state is not valid: %v", err)
	}
}
