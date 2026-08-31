package measurement

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	APIVersion          = "devroom/measurement/v1"
	MeasurementRunKind  = "DogfoodMeasurementRun"
	MeasurementKind     = "Measurement"
	MaxManifestBytes    = 512 << 10
	MaxMeasurements     = 128
	MaxToolVersions     = 32
	MaxRequiredFailures = 128
)

type Category string

const (
	CategoryQuality     Category = "quality"
	CategoryPerformance Category = "performance"
	CategoryProcess     Category = "process"
	CategoryRuntime     Category = "runtime"
)

type Status string

const (
	StatusPass    Status = "pass"
	StatusFail    Status = "fail"
	StatusUnknown Status = "unknown"
)

type Provenance string

const (
	ProvenanceMeasured    Provenance = "measured"
	ProvenanceEstimated   Provenance = "estimated"
	ProvenanceInferred    Provenance = "inferred"
	ProvenanceUnavailable Provenance = "unavailable"
)

type DirtyState string

const (
	DirtyClean   DirtyState = "clean"
	DirtyDirty   DirtyState = "dirty"
	DirtyUnknown DirtyState = "unknown"
)

type ObjectMetadata struct {
	ID string `json:"id"`
}

// Run is the versioned machine-readable manifest for one dogfood run. Its
// status is a required-check gate, not an aggregate quality score.
type Run struct {
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Metadata   ObjectMetadata `json:"metadata"`
	Spec       RunSpec        `json:"spec"`
}

type RunSpec struct {
	Status           Status          `json:"status"`
	RequiredFailures []string        `json:"requiredFailures"`
	Reproducibility  Reproducibility `json:"reproducibility"`
	Measurements     []Measurement   `json:"measurements"`
}

// Reproducibility contains only bounded, non-secret identity metadata. It
// intentionally has no repository or output path field.
type Reproducibility struct {
	RunID               string            `json:"runId"`
	Commit              string            `json:"commit"`
	Head                string            `json:"head"`
	DirtyState          DirtyState        `json:"dirtyState"`
	OS                  string            `json:"os"`
	Arch                string            `json:"arch"`
	ToolVersions        map[string]string `json:"toolVersions"`
	ConfigurationDigest string            `json:"configurationDigest"`
	StartedAt           time.Time         `json:"startedAt"`
	EndedAt             time.Time         `json:"endedAt"`
}

// Measurement is one bounded observation. A zero-sample unknown measurement
// is valid for an unavailable optional source, such as an absent server.
type Measurement struct {
	APIVersion string          `json:"apiVersion"`
	Kind       string          `json:"kind"`
	Metadata   ObjectMetadata  `json:"metadata"`
	Spec       MeasurementSpec `json:"spec"`
}

type MeasurementSpec struct {
	Name        string     `json:"name"`
	Category    Category   `json:"category"`
	Status      Status     `json:"status"`
	Provenance  Provenance `json:"provenance"`
	Unit        string     `json:"unit"`
	SampleCount int        `json:"sampleCount"`
	RawSamples  []float64  `json:"rawSamples"`
	Min         *float64   `json:"min"`
	P50         *float64   `json:"p50"`
	P95         *float64   `json:"p95"`
	Max         *float64   `json:"max"`
	Baseline    *float64   `json:"baseline,omitempty"`
	Delta       *float64   `json:"delta,omitempty"`
	CommandID   string     `json:"commandId,omitempty"`
	Command     string     `json:"command,omitempty"`
	ExitCode    *int       `json:"exitCode,omitempty"`
	Required    bool       `json:"required"`
}

type MeasurementInput struct {
	ID         string
	Name       string
	Category   Category
	Status     Status
	Provenance Provenance
	Unit       string
	Samples    []float64
	Baseline   *float64
	Delta      *float64
	CommandID  string
	Command    string
	ExitCode   *int
	Required   bool
}

var (
	digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	idPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,95}$`)
	unitPattern   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]{0,31}$`)
)

// NewMeasurement constructs a validated measurement and computes its summary.
// Samples are copied so callers cannot mutate the exported raw sample set.
func NewMeasurement(input MeasurementInput) (Measurement, error) {
	samples := append([]float64{}, input.Samples...)
	summary, err := Summarize(samples)
	if errors.Is(err, ErrNoSamples) && input.Status == StatusUnknown {
		summary = Summary{}
		err = nil
	}
	if err != nil {
		return Measurement{}, err
	}
	measurement := Measurement{
		APIVersion: APIVersion,
		Kind:       MeasurementKind,
		Metadata:   ObjectMetadata{ID: input.ID},
		Spec: MeasurementSpec{
			Name:        input.Name,
			Category:    input.Category,
			Status:      input.Status,
			Provenance:  input.Provenance,
			Unit:        input.Unit,
			SampleCount: len(samples),
			RawSamples:  samples,
			Min:         summary.Min,
			P50:         summary.P50,
			P95:         summary.P95,
			Max:         summary.Max,
			Baseline:    cloneFloatPointer(input.Baseline),
			Delta:       cloneFloatPointer(input.Delta),
			CommandID:   input.CommandID,
			Command:     input.Command,
			ExitCode:    cloneIntPointer(input.ExitCode),
			Required:    input.Required,
		},
	}
	if err := measurement.Validate(); err != nil {
		return Measurement{}, fmt.Errorf("measurement is invalid: %w", err)
	}
	return measurement, nil
}

// NewRun constructs a validated run and derives its required-check status.
func NewRun(reproducibility Reproducibility, measurements []Measurement) (Run, error) {
	clonedMeasurements := make([]Measurement, len(measurements))
	for index, item := range measurements {
		clonedMeasurements[index] = cloneMeasurement(item)
	}
	status, failures := requiredStatus(clonedMeasurements)
	run := Run{
		APIVersion: APIVersion,
		Kind:       MeasurementRunKind,
		Metadata:   ObjectMetadata{ID: reproducibility.RunID},
		Spec: RunSpec{
			Status:           status,
			RequiredFailures: failures,
			Reproducibility:  cloneReproducibility(reproducibility),
			Measurements:     clonedMeasurements,
		},
	}
	if err := run.Validate(); err != nil {
		return Run{}, fmt.Errorf("measurement run is invalid: %w", err)
	}
	return run, nil
}

// SHA256Digest returns the repository's standard lowercase digest notation.
func SHA256Digest(value []byte) string {
	digest := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func (r Run) Validate() error {
	if r.APIVersion != APIVersion || r.Kind != MeasurementRunKind {
		return errors.New("measurement run type metadata is invalid")
	}
	if !validID(r.Metadata.ID) || r.Metadata.ID != r.Spec.Reproducibility.RunID {
		return errors.New("measurement run id is invalid or inconsistent")
	}
	if err := r.Spec.Reproducibility.Validate(); err != nil {
		return err
	}
	if r.Spec.Measurements == nil {
		return errors.New("measurement run measurements must be an array")
	}
	if len(r.Spec.Measurements) > MaxMeasurements {
		return errors.New("measurement run measurements are unbounded")
	}
	if r.Spec.RequiredFailures == nil || len(r.Spec.RequiredFailures) > MaxRequiredFailures {
		return errors.New("measurement run required failures must be a bounded array")
	}
	seen := make(map[string]struct{}, len(r.Spec.Measurements))
	for _, item := range r.Spec.Measurements {
		if err := item.Validate(); err != nil {
			return err
		}
		if _, exists := seen[item.Metadata.ID]; exists {
			return fmt.Errorf("duplicate measurement id %q", item.Metadata.ID)
		}
		seen[item.Metadata.ID] = struct{}{}
	}
	status, failures := requiredStatus(r.Spec.Measurements)
	if r.Spec.Status != status || !sameStrings(r.Spec.RequiredFailures, failures) {
		return errors.New("measurement run required status is inconsistent")
	}
	return nil
}

func (r Reproducibility) Validate() error {
	if !validID(r.RunID) || !validCommit(r.Commit) || !validCommit(r.Head) {
		return errors.New("measurement reproducibility identity is invalid")
	}
	if r.DirtyState != DirtyClean && r.DirtyState != DirtyDirty && r.DirtyState != DirtyUnknown {
		return errors.New("measurement reproducibility dirty state is invalid")
	}
	if !validToken(r.OS) || !validToken(r.Arch) {
		return errors.New("measurement reproducibility platform is invalid")
	}
	if r.ToolVersions == nil || len(r.ToolVersions) > MaxToolVersions {
		return errors.New("measurement reproducibility tool versions are invalid")
	}
	for name, version := range r.ToolVersions {
		if !validID(name) || !validSafeText(version, 256) {
			return errors.New("measurement reproducibility tool version is invalid")
		}
	}
	if !digestPattern.MatchString(r.ConfigurationDigest) || r.StartedAt.IsZero() || r.EndedAt.IsZero() || r.EndedAt.Before(r.StartedAt) {
		return errors.New("measurement reproducibility digest or time range is invalid")
	}
	return nil
}

func (m Measurement) Validate() error {
	if m.APIVersion != APIVersion || m.Kind != MeasurementKind || !validID(m.Metadata.ID) {
		return errors.New("measurement type metadata is invalid")
	}
	if !validSafeText(m.Spec.Name, 128) || !validCategory(m.Spec.Category) || !validStatus(m.Spec.Status) || !validProvenance(m.Spec.Provenance) || !unitPattern.MatchString(m.Spec.Unit) {
		return errors.New("measurement identity or classification is invalid")
	}
	if m.Spec.SampleCount < 0 || m.Spec.SampleCount > MaxRawSamples || m.Spec.SampleCount != len(m.Spec.RawSamples) || m.Spec.RawSamples == nil {
		return errors.New("measurement raw samples are invalid or unbounded")
	}
	if m.Spec.SampleCount == 0 {
		if m.Spec.Status != StatusUnknown || anySummaryPresent(m.Spec) || m.Spec.Provenance == ProvenanceMeasured {
			return errors.New("zero-sample measurement must be unknown and unsourced")
		}
	} else {
		summary, err := Summarize(m.Spec.RawSamples)
		if err != nil {
			return err
		}
		if !sameSummary(m.Spec, summary) {
			return errors.New("measurement summary does not match raw samples")
		}
		if m.Spec.Provenance == ProvenanceUnavailable {
			return errors.New("sampled measurement cannot be unavailable")
		}
	}
	if m.Spec.Baseline != nil && !validFloat(*m.Spec.Baseline) || m.Spec.Delta != nil && !validFloat(*m.Spec.Delta) {
		return errors.New("measurement comparison value is invalid")
	}
	if m.Spec.CommandID != "" && !validID(m.Spec.CommandID) {
		return errors.New("measurement command id is invalid")
	}
	if m.Spec.Command != "" && !validSafeText(m.Spec.Command, 256) {
		return errors.New("measurement command is invalid")
	}
	if m.Spec.ExitCode != nil && *m.Spec.ExitCode < 0 {
		return errors.New("measurement exit code is invalid")
	}
	if m.Spec.Provenance == ProvenanceUnavailable && m.Spec.Status != StatusUnknown {
		return errors.New("unavailable measurement must be unknown")
	}
	return nil
}

func requiredStatus(measurements []Measurement) (Status, []string) {
	failures := []string{}
	requiredCount := 0
	for _, item := range measurements {
		if !item.Spec.Required {
			continue
		}
		requiredCount++
		if item.Spec.Status != StatusPass {
			failures = append(failures, item.Metadata.ID)
		}
	}
	if requiredCount == 0 {
		return StatusUnknown, failures
	}
	if len(failures) > 0 {
		return StatusFail, failures
	}
	return StatusPass, failures
}

func validCategory(value Category) bool {
	return value == CategoryQuality || value == CategoryPerformance || value == CategoryProcess || value == CategoryRuntime
}

func validStatus(value Status) bool {
	return value == StatusPass || value == StatusFail || value == StatusUnknown
}

func validProvenance(value Provenance) bool {
	return value == ProvenanceMeasured || value == ProvenanceEstimated || value == ProvenanceInferred || value == ProvenanceUnavailable
}

func validID(value string) bool {
	return idPattern.MatchString(value) && validSafeText(value, 96)
}

func validToken(value string) bool {
	return idPattern.MatchString(value) && !strings.Contains(value, ":")
}

func validCommit(value string) bool {
	if value == "unknown" || value == "unavailable" {
		return true
	}
	if len(value) < 7 || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') && (character < 'A' || character > 'F') {
			return false
		}
	}
	return true
}

func validSafeText(value string, maxBytes int) bool {
	if value == "" || len(value) > maxBytes || strings.TrimSpace(value) != value || !utf8.ValidString(value) || hasAbsolutePathPrefix(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func hasAbsolutePathPrefix(value string) bool {
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, `\`) {
		return true
	}
	return len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' && (value[2] == '/' || value[2] == '\\')
}

func validFloat(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func anySummaryPresent(spec MeasurementSpec) bool {
	return spec.Min != nil || spec.P50 != nil || spec.P95 != nil || spec.Max != nil
}

func sameSummary(spec MeasurementSpec, summary Summary) bool {
	return sameFloatPointer(spec.Min, summary.Min) && sameFloatPointer(spec.P50, summary.P50) && sameFloatPointer(spec.P95, summary.P95) && sameFloatPointer(spec.Max, summary.Max)
}

func sameFloatPointer(left, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func cloneMeasurement(value Measurement) Measurement {
	clone := value
	clone.Spec.RawSamples = append([]float64{}, value.Spec.RawSamples...)
	clone.Spec.Min = cloneFloatPointer(value.Spec.Min)
	clone.Spec.P50 = cloneFloatPointer(value.Spec.P50)
	clone.Spec.P95 = cloneFloatPointer(value.Spec.P95)
	clone.Spec.Max = cloneFloatPointer(value.Spec.Max)
	clone.Spec.Baseline = cloneFloatPointer(value.Spec.Baseline)
	clone.Spec.Delta = cloneFloatPointer(value.Spec.Delta)
	clone.Spec.ExitCode = cloneIntPointer(value.Spec.ExitCode)
	return clone
}

func cloneReproducibility(value Reproducibility) Reproducibility {
	clone := value
	clone.ToolVersions = make(map[string]string, len(value.ToolVersions))
	for name, version := range value.ToolVersions {
		clone.ToolVersions[name] = version
	}
	return clone
}

func cloneFloatPointer(value *float64) *float64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
