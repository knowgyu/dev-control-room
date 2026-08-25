package assurance

import (
	"context"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestFixtureTechniqueReportsCoverEveryV1Technique(t *testing.T) {
	for _, technique := range []string{domain.QualityTechniqueStaticSecurity, domain.QualityTechniqueMutation, domain.QualityTechniqueProperty, domain.QualityTechniqueFuzz, domain.QualityTechniqueTargetedE2E} {
		report, err := RunFixtureTechnique(context.Background(), technique, t.TempDir())
		if err != nil || report.State != "succeeded" || report.Mode != "fixture" || report.Technique != technique {
			t.Fatalf("%s report = %#v, %v", technique, report, err)
		}
	}
}
