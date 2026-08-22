package contract

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestSuccessEnvelopeIsStable(t *testing.T) {
	payload, err := json.Marshal(Success(map[string]string{"status": "ok"}))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema":"devroom/cli/v1","ok":true,"data":{"status":"ok"},"meta":{}}`
	if string(payload) != want {
		t.Fatalf("unexpected envelope JSON: %s", payload)
	}
}

func TestErrorCodesMapToDistinctExitCodes(t *testing.T) {
	tests := []struct {
		code ErrorCode
		want ExitCode
	}{
		{ErrorInvalidInput, ExitInvalidInput},
		{ErrorCheckFailed, ExitCheckFailed},
		{ErrorPolicyDenied, ExitPolicyDenied},
		{ErrorExecutionFailed, ExitExecutionError},
		{ErrorUnavailable, ExitUnavailable},
		{ErrorNotFound, ExitNotFound},
		{ErrorConflict, ExitConflict},
		{ErrorForbidden, ExitForbidden},
	}
	for _, test := range tests {
		if got := test.code.ExitCode(); got != test.want {
			t.Errorf("%s: got %d, want %d", test.code, got, test.want)
		}
	}
}

func TestClassifyHidesUnknownAndInternalErrorMessages(t *testing.T) {
	secret := "internal SQL path: C:\\private\\token=secret-canary"
	for _, err := range []error{errors.New(secret), CodedError{Code: ErrorInternal, Message: secret}} {
		classified := Classify(err)
		if classified.Code != ErrorInternal || classified.Message != "internal error" || classified.Details != nil {
			t.Fatalf("internal error was not sanitized: %#v", classified)
		}
		payload := FromError[map[string]any](err)
		data, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if string(data) == "" || contains(string(data), secret) || contains(string(data), "private") {
			t.Fatalf("internal details leaked into envelope: %s", data)
		}
	}
}

func TestClassifyPreservesOnlyReviewedPublicCode(t *testing.T) {
	classified := Classify(CodedError{Code: ErrorInvalidInput, Message: "path is required", Details: map[string]any{"raw": "secret"}})
	if classified.Code != ErrorInvalidInput || classified.Message != "path is required" || classified.Details != nil {
		t.Fatalf("unexpected public classification: %#v", classified)
	}
}

func contains(value, needle string) bool {
	for index := 0; index+len(needle) <= len(value); index++ {
		if value[index:index+len(needle)] == needle {
			return true
		}
	}
	return false
}
