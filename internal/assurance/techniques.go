package assurance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

type TechniqueReport struct {
	Technique    string         `json:"technique"`
	Mode         string         `json:"mode"`
	State        string         `json:"state"`
	Summary      string         `json:"summary"`
	Findings     []string       `json:"findings,omitempty"`
	Seed         string         `json:"seed,omitempty"`
	Reproduction string         `json:"reproduction,omitempty"`
	Mutants      []MutantResult `json:"mutants,omitempty"`
	Corpus       []string       `json:"corpus,omitempty"`
	Scenario     string         `json:"scenario,omitempty"`
}

type MutantResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Target string `json:"target"`
}

// RunFixtureTechnique is the deterministic local adapter used by the three
// repository fixture and by automated tests. It produces reproducible reports
// without pretending that a missing third-party analyzer ran successfully.
func RunFixtureTechnique(ctx context.Context, technique, root string) (TechniqueReport, error) {
	select {
	case <-ctx.Done():
		return TechniqueReport{Technique: technique, Mode: "fixture", State: "cancelled"}, ctx.Err()
	default:
	}
	if root == "" {
		return TechniqueReport{}, errors.New("technique root is required")
	}
	if _, err := os.Stat(root); err != nil {
		return TechniqueReport{}, err
	}
	report := TechniqueReport{Technique: technique, Mode: "fixture", State: "succeeded"}
	switch technique {
	case domain.QualityTechniqueStaticSecurity:
		report.Summary = "fixture static/security rules completed"
		report.Findings = []string{"fixture.rule.no-secret-output:pass", "fixture.rule.typed-command:pass"}
	case domain.QualityTechniqueMutation:
		report.Summary = "fixture mutation sample completed"
		report.Mutants = []MutantResult{{ID: "mutant-1", Status: "killed", Target: "input-validation"}, {ID: "mutant-2", Status: "survived", Target: "error-mapping"}}
		report.Reproduction = "fixture mutation report is reproducible at the recorded HEAD"
	case domain.QualityTechniqueProperty:
		report.Summary = "fixture property sample completed"
		report.Seed = "devroom-fixture-seed-1"
		report.Findings = []string{"property:invalid-input-rejected"}
	case domain.QualityTechniqueFuzz:
		report.Summary = "fixture fuzz sample completed"
		report.Seed = "devroom-fuzz-seed-1"
		report.Corpus = []string{"empty", "unicode-boundary", "bounded-argv"}
		report.Reproduction = "fixture fuzz corpus is retained in the report artifact"
	case domain.QualityTechniqueTargetedE2E:
		report.Summary = "fixture targeted E2E sample completed"
		report.Scenario = filepath.Base(strings.TrimRight(root, string(filepath.Separator))) + ": first-use-to-quality-run"
		report.Findings = []string{"register", "observe", "review", "run"}
	default:
		return TechniqueReport{}, fmt.Errorf("unsupported fixture technique %q", technique)
	}
	return report, nil
}
