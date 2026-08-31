package measurement

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// DecodeManifest decodes and validates one bounded v1 measurement manifest.
// Keeping decoding here gives HTTP import and offline preflight one canonical
// schema boundary.
func DecodeManifest(reader io.Reader) (Run, error) {
	if reader == nil {
		return Run{}, errors.New("measurement manifest reader is nil")
	}
	data, err := io.ReadAll(io.LimitReader(reader, MaxManifestBytes+1))
	if err != nil {
		return Run{}, err
	}
	if len(data) > MaxManifestBytes {
		return Run{}, errors.New("measurement manifest exceeds size limit")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var run Run
	if err := decoder.Decode(&run); err != nil {
		return Run{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Run{}, errors.New("measurement manifest contains multiple JSON values")
		}
		return Run{}, err
	}
	if err := run.Validate(); err != nil {
		return Run{}, err
	}
	return run, nil
}
