// Package measurement contains bounded, reproducible dogfood measurement
// contracts and deterministic summary helpers. It has no I/O or runtime
// instrumentation dependencies.
package measurement

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

const MaxRawSamples = 128

var (
	ErrNoSamples         = errors.New("measurement has no samples")
	ErrTooManySamples    = errors.New("measurement has too many samples")
	ErrInvalidSample     = errors.New("measurement sample is invalid")
	ErrInvalidPercentile = errors.New("measurement percentile is invalid")
)

// Summary contains deterministic descriptive statistics for a bounded sample
// set. The pointers distinguish an unavailable summary from a measured zero.
type Summary struct {
	Min *float64 `json:"min"`
	P50 *float64 `json:"p50"`
	P95 *float64 `json:"p95"`
	Max *float64 `json:"max"`
}

// Summarize returns min, p50, p95, and max using linear interpolation between
// adjacent sorted samples. The input is copied and is never reordered.
func Summarize(samples []float64) (Summary, error) {
	sorted, err := sortedSamples(samples)
	if err != nil {
		return Summary{}, err
	}
	return Summary{
		Min: floatPointer(sorted[0]),
		P50: floatPointer(percentileFromSorted(sorted, 50)),
		P95: floatPointer(percentileFromSorted(sorted, 95)),
		Max: floatPointer(sorted[len(sorted)-1]),
	}, nil
}

// Percentile returns a percentile in the inclusive range [0, 100] using
// linear interpolation between adjacent sorted samples.
func Percentile(samples []float64, percentile float64) (float64, error) {
	if math.IsNaN(percentile) || math.IsInf(percentile, 0) || percentile < 0 || percentile > 100 {
		return 0, fmt.Errorf("%w: %v", ErrInvalidPercentile, percentile)
	}
	sorted, err := sortedSamples(samples)
	if err != nil {
		return 0, err
	}
	return percentileFromSorted(sorted, percentile), nil
}

func sortedSamples(samples []float64) ([]float64, error) {
	if len(samples) == 0 {
		return nil, ErrNoSamples
	}
	if len(samples) > MaxRawSamples {
		return nil, fmt.Errorf("%w: got %d, maximum %d", ErrTooManySamples, len(samples), MaxRawSamples)
	}
	sorted := append([]float64{}, samples...)
	for index, sample := range sorted {
		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return nil, fmt.Errorf("%w at index %d", ErrInvalidSample, index)
		}
	}
	sort.Float64s(sorted)
	return sorted, nil
}

func percentileFromSorted(sorted []float64, percentile float64) float64 {
	position := percentile / 100 * float64(len(sorted)-1)
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))
	if lower == upper {
		return sorted[lower]
	}
	fraction := position - float64(lower)
	return sorted[lower] + (sorted[upper]-sorted[lower])*fraction
}

func floatPointer(value float64) *float64 {
	return &value
}
