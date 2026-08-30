package measurement

import (
	"errors"
	"math"
	"testing"
)

func TestPercentile(t *testing.T) {
	tests := []struct {
		name       string
		samples    []float64
		percentile float64
		want       float64
	}{
		{name: "single sample", samples: []float64{42}, percentile: 95, want: 42},
		{name: "unsorted odd samples", samples: []float64{9, 1, 5}, percentile: 50, want: 5},
		{name: "even samples interpolate p50", samples: []float64{1, 2, 3, 4}, percentile: 50, want: 2.5},
		{name: "interpolated p95", samples: []float64{10, 20, 30, 40}, percentile: 95, want: 38.5},
		{name: "minimum", samples: []float64{-4, 0, 10}, percentile: 0, want: -4},
		{name: "maximum", samples: []float64{1, 2, 3}, percentile: 100, want: 3},
		{name: "duplicate samples", samples: []float64{2, 2, 2}, percentile: 95, want: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Percentile(test.samples, test.percentile)
			if err != nil {
				t.Fatalf("Percentile() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("Percentile() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestPercentileRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name       string
		samples    []float64
		percentile float64
		want       error
	}{
		{name: "empty samples", samples: []float64{}, percentile: 50, want: ErrNoSamples},
		{name: "nan sample", samples: []float64{1, math.NaN()}, percentile: 50, want: ErrInvalidSample},
		{name: "positive infinity", samples: []float64{1, math.Inf(1)}, percentile: 50, want: ErrInvalidSample},
		{name: "negative infinity", samples: []float64{1, math.Inf(-1)}, percentile: 50, want: ErrInvalidSample},
		{name: "too many samples", samples: make([]float64, MaxRawSamples+1), percentile: 50, want: ErrTooManySamples},
		{name: "percentile below range", samples: []float64{1}, percentile: -0.1, want: ErrInvalidPercentile},
		{name: "percentile above range", samples: []float64{1}, percentile: 100.1, want: ErrInvalidPercentile},
		{name: "nan percentile", samples: []float64{1}, percentile: math.NaN(), want: ErrInvalidPercentile},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Percentile(test.samples, test.percentile)
			if !errors.Is(err, test.want) {
				t.Fatalf("Percentile() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestSummarizePreservesInputAndReturnsAllStatistics(t *testing.T) {
	samples := []float64{4, 1, 8, 2}
	original := append([]float64{}, samples...)
	summary, err := Summarize(samples)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Min == nil || !closeEnough(*summary.Min, 1) || summary.P50 == nil || !closeEnough(*summary.P50, 3) || summary.P95 == nil || !closeEnough(*summary.P95, 7.4) || summary.Max == nil || !closeEnough(*summary.Max, 8) {
		t.Fatalf("summary = min=%v p50=%v p95=%v max=%v", summary.Min, summary.P50, summary.P95, summary.Max)
	}
	for index := range samples {
		if samples[index] != original[index] {
			t.Fatalf("Summarize reordered input: got %v, want %v", samples, original)
		}
	}
}

func closeEnough(got, want float64) bool {
	return math.Abs(got-want) < 1e-12
}
