package assurance

import (
	"context"
	"errors"
	"testing"
)

func TestCodexPackageAndNPMLauncherUseTypedNodeArgv(t *testing.T) {
	entry, err := ValidateCodexPackage([]byte(`{"name":"@openai/codex","bin":{"codex":"bin/codex.js"}}`))
	if err != nil || entry != "bin/codex.js" {
		t.Fatalf("entry = %q, %v", entry, err)
	}
	command, err := BuildCodexNPMCommand(`C:\Program Files\nodejs\node.exe`, `C:\Users\fixture\AppData\Roaming\npm\node_modules\@openai\codex`, []byte(`{"name":"@openai/codex","bin":"bin/codex.js"}`), []string{"--json"})
	if err != nil {
		t.Fatal(err)
	}
	if command.Executable != `C:\Program Files\nodejs\node.exe` || command.Arguments[1] != "--json" {
		t.Fatalf("unexpected typed command: %#v", command)
	}
	for _, value := range append([]string{command.Executable}, command.Arguments...) {
		if value == "cmd.exe" || value == "cmd" {
			t.Fatal("cmd.exe must not be invoked")
		}
	}
}

func TestCodexPackageRejectsUntrustedBin(t *testing.T) {
	for _, raw := range []string{`{"name":"other","bin":"bin/codex.js"}`, `{"name":"@openai/codex","bin":"../codex.js"}`, `{"name":"@openai/codex","bin":"bin/run.cmd"}`} {
		if _, err := ValidateCodexPackage([]byte(raw)); err == nil {
			t.Fatalf("accepted untrusted package %s", raw)
		}
	}
}

func TestFakeProviderFailureMatrixNeverStoresTranscript(t *testing.T) {
	for _, scenario := range []FakeScenario{FakeSuccess, FakeMalformedOutput, FakeTimeout, FakeCancelled, FakeAuthFailure, FakeApprovalPrompt, FakeMissingUsage, FakeNestedLaunch, FakeProviderFailure} {
		result := (FakeAdapter{Provider: "fake", Scenario: scenario}).Run(context.Background(), RunRequest{})
		if result.RawTranscript {
			t.Fatalf("scenario %s stored raw transcript", scenario)
		}
		if scenario == FakeSuccess && result.State != "succeeded" {
			t.Fatalf("success state = %s", result.State)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := (FakeAdapter{Provider: "fake", Scenario: FakeSuccess}).Run(ctx, RunRequest{})
	if result.FailureCode != "provider.cancelled" {
		t.Fatalf("cancel failure = %s", result.FailureCode)
	}
	if errors.Is(ctx.Err(), context.Canceled) == false {
		t.Fatal("cancel context was not cancelled")
	}
}
