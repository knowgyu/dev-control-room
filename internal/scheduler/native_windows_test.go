//go:build windows

package scheduler

import "testing"

func TestNormalizeDispatchExceptionUsesExceptionSCodeForNotFound(t *testing.T) {
	got := normalizeDispatchException(hresult(dispEException), -2147024894) // 0x80070002
	if !isNotFound(got) {
		t.Fatalf("normalized error was not not-found: 0x%08X", uint32(got))
	}
}

func TestNormalizeDispatchExceptionPreservesOtherErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		got  hresult
		want hresult
	}{
		{"other HRESULT", normalizeDispatchException(hresult(0x80004005), -2147024894), hresult(0x80004005)},
		{"zero scode", normalizeDispatchException(hresult(dispEException), 0), hresult(dispEException)},
		{"other exception scode", normalizeDispatchException(hresult(dispEException), -2147024891), hresult(0x80070005)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("unexpected normalization: got 0x%08X want 0x%08X", uint32(test.got), uint32(test.want))
			}
		})
	}
}
