package masking

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestMaskingCoversExactEncodedHeadersAndVariables(t *testing.T) {
	secret := "s3cr3t/value+with space"
	masker := New([]string{secret}, []string{"JENKINS_TOKEN"})
	input := strings.Join([]string{
		"exact=" + secret,
		"url=https://example.test/callback?token=s3cr3t%2Fvalue%2Bwith+space",
		"Authorization: Bearer bearer-canary",
		"X-API-Key: header-canary",
		"JENKINS_TOKEN=variable-canary",
	}, "\n")
	masked := masker.Mask(input)
	for _, secretValue := range []string{secret, "s3cr3t%2Fvalue%2Bwith+space", "bearer-canary", "header-canary", "variable-canary"} {
		if strings.Contains(masked, secretValue) {
			t.Fatalf("secret %q survived masking: %s", secretValue, masked)
		}
	}
	if !strings.Contains(masked, Replacement) {
		t.Fatalf("expected redaction marker in %s", masked)
	}
}

func TestMaskingCoversTokenShapesAndCredentialURL(t *testing.T) {
	masker := New(nil, nil)
	input := "https://user:password@example.test AKIA1234567890ABCDEF eyJhbGciOiJub25l.aGVhZGVy.cGF5bG9hZA ghp_123456789012345678901234"
	masked := masker.Mask(input)
	for _, secretValue := range []string{"password", "AKIA1234567890ABCDEF", "eyJhbGciOiJub25l.aGVhZGVy.cGF5bG9hZA", "ghp_123456789012345678901234"} {
		if strings.Contains(masked, secretValue) {
			t.Fatalf("token-shaped value %q survived masking: %s", secretValue, masked)
		}
	}
}

func TestStreamMaskerHoldsSplitSecretUntilFlush(t *testing.T) {
	secret := "split-secret-canary"
	stream := New([]string{secret}, nil).Stream()
	if got := stream.Write("prefix split-secret"); got != "" {
		t.Fatalf("split secret prefix was emitted: %q", got)
	}
	got := stream.Write("-canary suffix") + stream.Flush()
	if strings.Contains(got, secret) {
		t.Fatalf("split secret survived stream masking: %q", got)
	}
	if !strings.Contains(got, Replacement) {
		t.Fatalf("expected split secret replacement: %q", got)
	}
}

func TestStreamMaskerMasksEverySecretSplitBoundary(t *testing.T) {
	secret := "boundary-secret-canary"
	for split := 0; split <= len(secret); split++ {
		stream := New([]string{secret}, nil).Stream()
		output := stream.Write("prefix:" + secret[:split])
		output += stream.Write(secret[split:] + ":suffix")
		output += stream.Flush()
		if strings.Contains(output, secret) || !strings.Contains(output, Replacement) {
			t.Fatalf("split %d leaked or failed to replace secret: %q", split, output)
		}
		if !utf8.ValidString(output) {
			t.Fatalf("split %d emitted invalid UTF-8: %q", split, output)
		}
	}
}

func TestStreamMaskerRetainsHeaderBoundaryAcrossChunks(t *testing.T) {
	secret := "header-boundary-secret"
	masker := New([]string{secret}, nil)
	stream := masker.Stream()
	output := stream.Write(strings.Repeat("safe ", 20) + "Authorization: Bear")
	output += stream.Write("er " + secret)
	output += stream.Flush()
	if strings.Contains(output, secret) || strings.Contains(output, "Authorization: Bearer "+secret) {
		t.Fatalf("header boundary leaked secret: %q", output)
	}
	if !strings.Contains(output, Replacement) {
		t.Fatalf("header boundary was not redacted: %q", output)
	}
}

func TestSeparateOutputStreamsCannotLeakSplitSecret(t *testing.T) {
	secret := "stdout-stderr-secret"
	masker := New([]string{secret}, nil)
	stdout := masker.Stream()
	stderr := masker.Stream()
	combined := stdout.Write("stdout prefix " + secret[:7])
	combined += stderr.Write("stderr prefix " + secret[7:])
	combined += stdout.Flush()
	combined += stderr.Flush()
	if strings.Contains(combined, secret) {
		t.Fatalf("split output stream leaked secret: %q", combined)
	}
}

func TestStreamMaskerPreservesUTF8AtHoldBoundary(t *testing.T) {
	stream := New([]string{"unicode-secret"}, nil).Stream()
	output := stream.Write(strings.Repeat("한", 40)+"unicode-secret") + stream.Flush()
	if !utf8.ValidString(output) || strings.Contains(output, "unicode-secret") {
		t.Fatalf("UTF-8 or secret boundary failed: %q", output)
	}
}

func TestMaskValueRecursesThroughEventData(t *testing.T) {
	masker := New([]string{"event-secret-canary"}, nil)
	value := map[string]any{"stdout": "event-secret-canary", "nested": []any{"safe", "event-secret-canary"}}
	masked, ok := masker.MaskValue(value).(map[string]any)
	if !ok || masked["stdout"] != Replacement {
		t.Fatalf("unexpected masked map: %#v", masked)
	}
}
