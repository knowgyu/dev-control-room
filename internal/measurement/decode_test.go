package measurement

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDecodeManifestUsesStrictBoundedContract(t *testing.T) {
	item, err := NewMeasurement(MeasurementInput{
		ID:         "quality-check",
		Name:       "quality.check",
		Category:   CategoryQuality,
		Status:     StatusPass,
		Provenance: ProvenanceMeasured,
		Unit:       "milliseconds",
		Samples:    []float64{1},
		CommandID:  "check",
		Required:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := NewRun(Reproducibility{
		RunID:               "dogfood-decode-test",
		Commit:              strings.Repeat("a", 40),
		Head:                strings.Repeat("a", 40),
		DirtyState:          DirtyClean,
		OS:                  "windows",
		Arch:                "amd64",
		ToolVersions:        map[string]string{"go": "go1.26.7"},
		ConfigurationDigest: SHA256Digest([]byte("decode-config")),
		StartedAt:           time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		EndedAt:             time.Date(2026, 9, 1, 0, 0, 1, 0, time.UTC),
	}, []Measurement{item})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeManifest(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Metadata.ID != run.Metadata.ID {
		t.Fatalf("decoded run id = %q, want %q", decoded.Metadata.ID, run.Metadata.ID)
	}

	unknownField := append(append([]byte{}, encoded[:len(encoded)-1]...), []byte(`,"extra":true}`)...)
	if _, err := DecodeManifest(bytes.NewReader(unknownField)); err == nil {
		t.Fatal("manifest with an unknown field was accepted")
	}
	if _, err := DecodeManifest(bytes.NewReader(bytes.Repeat([]byte("x"), MaxManifestBytes+1))); err == nil {
		t.Fatal("oversized manifest was accepted")
	}
}
