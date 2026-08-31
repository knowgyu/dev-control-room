package app

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/measurement"
	"github.com/knowgyu/dev-control-room/internal/store"
)

const (
	MeasurementComparisonEmpty       = "empty"
	MeasurementComparisonComparable  = "comparable"
	MeasurementComparisonMissing     = "missing"
	MeasurementComparisonUnavailable = "unavailable"
)

type MeasurementRunSummary struct {
	RunID               string                 `json:"runId"`
	Status              measurement.Status     `json:"status"`
	Commit              string                 `json:"commit"`
	Head                string                 `json:"head"`
	DirtyState          measurement.DirtyState `json:"dirtyState"`
	OS                  string                 `json:"os"`
	Arch                string                 `json:"arch"`
	ToolVersions        map[string]string      `json:"toolVersions"`
	ConfigurationDigest string                 `json:"configurationDigest"`
	StartedAt           time.Time              `json:"startedAt"`
	EndedAt             time.Time              `json:"endedAt"`
	RequiredFailures    []string               `json:"requiredFailures"`
	Measurements        []MeasurementSummary   `json:"measurements"`
}

type MeasurementSummary struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Category    measurement.Category   `json:"category"`
	Status      measurement.Status     `json:"status"`
	Provenance  measurement.Provenance `json:"provenance"`
	Unit        string                 `json:"unit"`
	SampleCount int                    `json:"sampleCount"`
	Min         *float64               `json:"min"`
	P50         *float64               `json:"p50"`
	P95         *float64               `json:"p95"`
	Max         *float64               `json:"max"`
	Baseline    *float64               `json:"baseline,omitempty"`
	Delta       *float64               `json:"delta,omitempty"`
	CommandID   string                 `json:"commandId,omitempty"`
	ExitCode    *int                   `json:"exitCode,omitempty"`
	Required    bool                   `json:"required"`
}

type MeasurementComparison struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Unit        string   `json:"unit"`
	State       string   `json:"state"`
	CurrentP50  *float64 `json:"currentP50"`
	PreviousP50 *float64 `json:"previousP50"`
	DeltaP50    *float64 `json:"deltaP50"`
	CurrentP95  *float64 `json:"currentP95"`
	PreviousP95 *float64 `json:"previousP95"`
	DeltaP95    *float64 `json:"deltaP95"`
}

type MeasurementNextAction struct {
	Code   string `json:"code"`
	Label  string `json:"label"`
	Reason string `json:"reason"`
}

type MeasurementDashboard struct {
	Latest             *MeasurementRunSummary  `json:"latest"`
	PreviousComparable *MeasurementRunSummary  `json:"previousComparable"`
	ComparisonState    string                  `json:"comparisonState"`
	Comparisons        []MeasurementComparison `json:"comparisons"`
	NextActions        []MeasurementNextAction `json:"nextActions"`
}

func (a *App) MeasurementRuns(ctx context.Context) ([]MeasurementRunSummary, error) {
	items, err := a.store.ListMeasurementRuns(ctx)
	if err != nil {
		return nil, err
	}
	summaries := make([]MeasurementRunSummary, 0, len(items))
	for _, item := range items {
		summaries = append(summaries, measurementRunSummary(item))
	}
	return summaries, nil
}

func (a *App) MeasurementRun(ctx context.Context, id string) (MeasurementRunSummary, error) {
	item, err := a.store.GetMeasurementRun(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MeasurementRunSummary{}, contract.NotFound("measurement run not found")
		}
		return MeasurementRunSummary{}, err
	}
	return measurementRunSummary(item), nil
}

func (a *App) MeasurementDashboard(ctx context.Context) (MeasurementDashboard, error) {
	items, err := a.MeasurementRuns(ctx)
	if err != nil {
		return MeasurementDashboard{}, err
	}
	return measurementDashboard(items), nil
}

func (a *App) ImportMeasurementRun(ctx context.Context, item measurement.Run) (MeasurementRunSummary, error) {
	if err := item.Validate(); err != nil {
		return MeasurementRunSummary{}, contract.InvalidInput("invalid measurement manifest")
	}
	a.store.SetMasker(a.masker)
	if err := a.store.SaveMeasurementRun(ctx, item); err != nil {
		if errors.Is(err, store.ErrMeasurementRunDuplicate) {
			return MeasurementRunSummary{}, contract.Conflict("measurement run has already been imported")
		}
		return MeasurementRunSummary{}, err
	}
	stored, err := a.store.GetMeasurementRun(ctx, item.Metadata.ID)
	if err != nil {
		return MeasurementRunSummary{}, err
	}
	return measurementRunSummary(stored), nil
}

func measurementRunSummary(item measurement.Run) MeasurementRunSummary {
	reproducibility := item.Spec.Reproducibility
	summary := MeasurementRunSummary{
		RunID:               item.Metadata.ID,
		Status:              item.Spec.Status,
		Commit:              reproducibility.Commit,
		Head:                reproducibility.Head,
		DirtyState:          reproducibility.DirtyState,
		OS:                  reproducibility.OS,
		Arch:                reproducibility.Arch,
		ToolVersions:        make(map[string]string, len(reproducibility.ToolVersions)),
		ConfigurationDigest: reproducibility.ConfigurationDigest,
		StartedAt:           reproducibility.StartedAt,
		EndedAt:             reproducibility.EndedAt,
		RequiredFailures:    append([]string{}, item.Spec.RequiredFailures...),
		Measurements:        make([]MeasurementSummary, 0, len(item.Spec.Measurements)),
	}
	for name, version := range reproducibility.ToolVersions {
		summary.ToolVersions[name] = version
	}
	for _, current := range item.Spec.Measurements {
		summary.Measurements = append(summary.Measurements, measurementSummary(current))
	}
	return summary
}

func measurementSummary(item measurement.Measurement) MeasurementSummary {
	spec := item.Spec
	return MeasurementSummary{
		ID:          item.Metadata.ID,
		Name:        spec.Name,
		Category:    spec.Category,
		Status:      spec.Status,
		Provenance:  spec.Provenance,
		Unit:        spec.Unit,
		SampleCount: spec.SampleCount,
		Min:         cloneMeasurementFloat(spec.Min),
		P50:         cloneMeasurementFloat(spec.P50),
		P95:         cloneMeasurementFloat(spec.P95),
		Max:         cloneMeasurementFloat(spec.Max),
		Baseline:    cloneMeasurementFloat(spec.Baseline),
		Delta:       cloneMeasurementFloat(spec.Delta),
		CommandID:   spec.CommandID,
		ExitCode:    cloneMeasurementInt(spec.ExitCode),
		Required:    spec.Required,
	}
}

func measurementDashboard(items []MeasurementRunSummary) MeasurementDashboard {
	dashboard := MeasurementDashboard{
		ComparisonState: MeasurementComparisonEmpty,
		Comparisons:     []MeasurementComparison{},
		NextActions:     []MeasurementNextAction{},
	}
	if len(items) == 0 {
		return dashboard
	}
	sortMeasurementRuns(items)
	latest := items[0]
	dashboard.Latest = &latest
	dashboard.ComparisonState = MeasurementComparisonUnavailable
	for _, candidate := range items[1:] {
		if !comparableMeasurementRuns(latest, candidate) {
			continue
		}
		previous := candidate
		dashboard.PreviousComparable = &previous
		dashboard.ComparisonState = MeasurementComparisonComparable
		break
	}
	if dashboard.PreviousComparable == nil && knownMeasurementRunIdentity(latest) {
		dashboard.ComparisonState = MeasurementComparisonMissing
	}
	dashboard.Comparisons, dashboard.NextActions = measurementComparisons(latest, dashboard.PreviousComparable)
	return dashboard
}

func sortMeasurementRuns(items []MeasurementRunSummary) {
	sort.SliceStable(items, func(index, next int) bool {
		left, right := items[index], items[next]
		if left.EndedAt.Equal(right.EndedAt) {
			return left.RunID > right.RunID
		}
		return left.EndedAt.After(right.EndedAt)
	})
}

func comparableMeasurementRuns(left, right MeasurementRunSummary) bool {
	if !knownMeasurementRunIdentity(left) || !knownMeasurementRunIdentity(right) {
		return false
	}
	if left.Commit != right.Commit || left.Head != right.Head || left.DirtyState != right.DirtyState || left.OS != right.OS || left.Arch != right.Arch || left.ConfigurationDigest != right.ConfigurationDigest {
		return false
	}
	if len(left.ToolVersions) != len(right.ToolVersions) {
		return false
	}
	for name, version := range left.ToolVersions {
		if right.ToolVersions[name] != version {
			return false
		}
	}
	return right.EndedAt.Before(left.EndedAt)
}

func knownMeasurementRunIdentity(item MeasurementRunSummary) bool {
	for _, value := range []string{item.Commit, item.Head, item.OS, item.Arch, item.ConfigurationDigest} {
		if value == "" || value == "unknown" || value == "unavailable" {
			return false
		}
	}
	if item.DirtyState == measurement.DirtyUnknown || len(item.ToolVersions) == 0 {
		return false
	}
	for _, version := range item.ToolVersions {
		if version == "" || version == "unknown" || version == "unavailable" {
			return false
		}
	}
	return true
}

func measurementComparisons(latest MeasurementRunSummary, previous *MeasurementRunSummary) ([]MeasurementComparison, []MeasurementNextAction) {
	comparisons := make([]MeasurementComparison, 0, len(latest.Measurements))
	previousByID := make(map[string]MeasurementSummary)
	if previous != nil {
		previousByID = make(map[string]MeasurementSummary, len(previous.Measurements))
		for _, item := range previous.Measurements {
			previousByID[item.ID] = item
		}
	}
	unknownNames := []string{}
	regressionNames := []string{}
	for _, current := range latest.Measurements {
		if current.Status == measurement.StatusUnknown || current.Provenance == measurement.ProvenanceUnavailable {
			unknownNames = append(unknownNames, current.Name)
		}
		comparison := MeasurementComparison{
			ID:         current.ID,
			Name:       current.Name,
			Unit:       current.Unit,
			State:      MeasurementComparisonUnavailable,
			CurrentP50: cloneMeasurementFloat(current.P50),
			CurrentP95: cloneMeasurementFloat(current.P95),
		}
		if previousItem, found := previousByID[current.ID]; found && comparableMeasurementValues(current, previousItem) {
			comparison.State = MeasurementComparisonComparable
			comparison.PreviousP50 = cloneMeasurementFloat(previousItem.P50)
			comparison.DeltaP50 = subtractMeasurementFloat(current.P50, previousItem.P50)
			comparison.PreviousP95 = cloneMeasurementFloat(previousItem.P95)
			comparison.DeltaP95 = subtractMeasurementFloat(current.P95, previousItem.P95)
			if measurementIsRegression(current.Name, comparison.DeltaP50, comparison.DeltaP95) {
				regressionNames = append(regressionNames, current.Name)
			}
		}
		comparisons = append(comparisons, comparison)
	}

	actions := []MeasurementNextAction{}
	if len(latest.RequiredFailures) > 0 {
		actions = append(actions, MeasurementNextAction{Code: "failed_required_check", Label: "필수 검사 실패", Reason: "실패한 필수 검사: " + strings.Join(latest.RequiredFailures, ", ")})
	}
	if len(unknownNames) > 0 {
		actions = append(actions, MeasurementNextAction{Code: "unavailable_probe", Label: "사용할 수 없는 측정값 확인", Reason: "측정되지 않았거나 사용할 수 없는 항목: " + strings.Join(unknownNames, ", ")})
	}
	if !knownMeasurementRunIdentity(latest) {
		actions = append(actions, MeasurementNextAction{Code: "incomplete_reproducibility", Label: "재현성 metadata 보완", Reason: "commit, HEAD, 변경 상태, 플랫폼, 구성 digest, 도구 버전이 모두 알려진 manifest를 가져오세요."})
	}
	if previous == nil && knownMeasurementRunIdentity(latest) {
		actions = append(actions, MeasurementNextAction{Code: "missing_comparable_baseline", Label: "비교 가능한 이전 실행 없음", Reason: "같은 commit, HEAD, 구성 digest, 플랫폼, 도구 버전의 이전 실행을 가져오세요."})
	}
	if len(regressionNames) > 0 {
		actions = append(actions, MeasurementNextAction{Code: "regression_comparable_metric", Label: "비교 가능한 지표 회귀", Reason: "이전 비교 실행보다 불리해진 지표: " + strings.Join(regressionNames, ", ")})
	}
	return comparisons, actions
}

func comparableMeasurementValues(left, right MeasurementSummary) bool {
	if left.Name != right.Name || left.Unit != right.Unit || left.Category != right.Category || left.CommandID != right.CommandID {
		return false
	}
	if left.Status != measurement.StatusPass || right.Status != measurement.StatusPass || left.Provenance != measurement.ProvenanceMeasured || right.Provenance != measurement.ProvenanceMeasured {
		return false
	}
	return left.P50 != nil && right.P50 != nil
}

func measurementIsRegression(name string, deltaP50, deltaP95 *float64) bool {
	if strings.HasSuffix(name, "coverage_percent") {
		return deltaP50 != nil && *deltaP50 < 0
	}
	if !strings.HasPrefix(name, "quality.") && !strings.Contains(name, "duration") && !strings.Contains(name, "latency") {
		return false
	}
	return (deltaP50 != nil && *deltaP50 > 0) || (deltaP95 != nil && *deltaP95 > 0)
}

func subtractMeasurementFloat(left, right *float64) *float64 {
	if left == nil || right == nil {
		return nil
	}
	value := *left - *right
	return &value
}

func cloneMeasurementFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneMeasurementInt(value *int) *int {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
