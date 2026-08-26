package assurance

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/masking"
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
	for _, raw := range []string{`{"name":"other","bin":"bin/codex.js"}`, `{"name":"@openai/codex","bin":"../codex.js"}`, `{"name":"@openai/codex","bin":"bin/../codex.js"}`, `{"name":"@openai/codex","bin":"lib/codex.js"}`, `{"name":"@openai/codex","bin":"BIN\\CODEX.JS"}`, `{"name":"@openai/codex","bin":"bin/run.cmd"}`, `{"name":"@openai/codex","bin":"C:\\tmp\\codex.js"}`} {
		if _, err := ValidateCodexPackage([]byte(raw)); err == nil {
			t.Fatalf("accepted untrusted package %s", raw)
		}
	}
}

func TestCodexResolverUsesAdjacentPackageAndNeverTrustsNativeOrBatchLauncher(t *testing.T) {
	lookups := map[string]string{
		"codex.ps1": `C:\Users\fixture\AppData\Roaming\npm\codex.ps1`,
		"node":      `C:\Program Files\nodejs\node.exe`,
	}
	resolver := CodexResolver{
		LookPath: func(name string) (string, error) {
			if value, ok := lookups[name]; ok {
				return value, nil
			}
			return "", errors.New("not found")
		},
		ReadFile: func(name string) ([]byte, error) {
			switch name {
			case `C:\Users\fixture\AppData\Roaming\npm\node_modules\@openai\codex\package.json`:
				return []byte(`{"name":"@openai/codex","bin":{"codex":"bin/codex.js"}}`), nil
			case `C:\Users\fixture\AppData\Roaming\npm\node_modules\@openai\codex\bin\codex.js`:
				return []byte("export {}"), nil
			default:
				t.Fatalf("unexpected read path %q", name)
				return nil, errors.New("unexpected path")
			}
		},
		StatFile: func(name string) error {
			if name != `C:\Users\fixture\AppData\Roaming\npm\node_modules\@openai\codex\bin\codex.js` {
				t.Fatalf("unexpected entry path %q", name)
			}
			return nil
		},
	}
	status := resolver.Resolve()
	if status.State != ProviderReady || !status.LaunchTrusted || !status.ProfileReady || len(status.ResolvedCommand) != 2 {
		t.Fatalf("resolver status = %#v", status)
	}
	if status.ResolvedCommand[0] != lookups["node"] || status.ResolvedCommand[1] != `C:\Users\fixture\AppData\Roaming\npm\node_modules\@openai\codex\bin\codex.js` {
		t.Fatalf("resolved command = %#v", status.ResolvedCommand)
	}

	for _, launcher := range []string{`C:\Tools\codex.exe`, `C:\Tools\codex.bat`} {
		status := (CodexResolver{LookPath: func(name string) (string, error) {
			if name == "codex.cmd" {
				return launcher, nil
			}
			return "", errors.New("not found")
		}}).Resolve()
		if status.State == ProviderReady || status.LaunchTrusted || status.ProfileReady || status.ReasonCode != "provider.untrusted_launcher" {
			t.Fatalf("launcher %q was trusted: %#v", launcher, status)
		}
	}
}

func TestCodexResolverProbesCmdThenPs1WithoutBareCodex(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		cmdPath     string
		wantLookups []string
	}{
		{name: "cmd wins", cmdPath: `C:\Users\fixture\AppData\Roaming\npm\codex.cmd`, wantLookups: []string{"codex.cmd", "node"}},
		{name: "ps1 fallback", wantLookups: []string{"codex.cmd", "codex.ps1", "node"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			lookups := []string{}
			resolver := CodexResolver{
				LookPath: func(name string) (string, error) {
					lookups = append(lookups, name)
					switch name {
					case "codex.cmd":
						if testCase.cmdPath != "" {
							return testCase.cmdPath, nil
						}
					case "codex.ps1":
						return `C:\Users\fixture\AppData\Roaming\npm\codex.ps1`, nil
					case "node":
						return `C:\Program Files\nodejs\node.exe`, nil
					default:
						t.Fatalf("bare or unexpected lookup %q", name)
					}
					return "", errors.New("not found")
				},
				ReadFile: func(name string) ([]byte, error) {
					if strings.HasSuffix(name, `\package.json`) {
						return []byte(`{"name":"@openai/codex","bin":"bin/codex.js"}`), nil
					}
					return []byte("export {}"), nil
				},
				StatFile: func(string) error { return nil },
			}
			status := resolver.Resolve()
			if status.State != ProviderReady {
				t.Fatalf("status=%#v", status)
			}
			if len(lookups) != len(testCase.wantLookups) {
				t.Fatalf("lookups=%#v want=%#v", lookups, testCase.wantLookups)
			}
			for index := range lookups {
				if lookups[index] != testCase.wantLookups[index] {
					t.Fatalf("lookups=%#v want=%#v", lookups, testCase.wantLookups)
				}
			}
		})
	}
}

func TestCodexResolverFailsClosedWhenPackageEntryIsMissing(t *testing.T) {
	status := (CodexResolver{
		LookPath: func(name string) (string, error) {
			switch name {
			case "codex.cmd":
				return `C:\Tools\codex.cmd`, nil
			case "node":
				return `C:\Program Files\nodejs\node.exe`, nil
			default:
				return "", errors.New("not found")
			}
		},
		ReadFile: func(string) ([]byte, error) {
			return []byte(`{"name":"@openai/codex","bin":"bin/codex.js"}`), nil
		},
		StatFile: func(string) error { return errors.New("missing") },
	}).Resolve()
	if status.State == ProviderReady || status.LaunchTrusted || status.ReasonCode != "provider.package_entry_missing" {
		t.Fatalf("missing package entry was trusted: %#v", status)
	}
}

func TestCodexResolverFailsClosedWhenPackageEntryCannotBeRead(t *testing.T) {
	status := (CodexResolver{
		LookPath: func(name string) (string, error) {
			switch name {
			case "codex.ps1":
				return `C:\Tools\codex.ps1`, nil
			case "node":
				return `C:\Program Files\nodejs\node.exe`, nil
			default:
				return "", errors.New("not found")
			}
		},
		ReadFile: func(name string) ([]byte, error) {
			if strings.HasSuffix(name, `\package.json`) {
				return []byte(`{"name":"@openai/codex","bin":"bin/codex.js"}`), nil
			}
			return nil, errors.New("unreadable")
		},
		StatFile: func(string) error { return nil },
	}).Resolve()
	if status.State == ProviderReady || status.LaunchTrusted || status.ReasonCode != "provider.package_entry_unreadable" {
		t.Fatalf("unreadable package entry was trusted: %#v", status)
	}
}

func TestBuildCodexInvocationCommandRejectsUntrustedResolvedCommand(t *testing.T) {
	base := ProviderStatus{Provider: "codex", State: ProviderReady, CommandFound: true, LaunchTrusted: true, ProfileReady: true}
	for _, resolved := range [][]string{
		{`C:\Windows\System32\cmd.exe`, `C:\Tools\codex.js`},
		{`C:\Program Files\nodejs\node.exe`, `C:\Tools\codex.cmd`},
		{`C:\Program Files\nodejs\node.exe`, `C:\Tools\..\codex.js`},
		{`C:\Program Files\nodejs\node.exe`},
	} {
		base.ResolvedCommand = resolved
		if _, err := BuildCodexInvocationCommand(base, "fixture-model"); err == nil {
			t.Fatalf("accepted untrusted resolved command %#v", resolved)
		}
	}
}

func TestCodexExecutionContextSeamPreservesTypedArgv(t *testing.T) {
	var captured RunRequest
	ctx := WithCodexExecution(context.Background(), CodexExecution{
		Resolver: func() ProviderStatus {
			return ProviderStatus{Provider: "codex", State: ProviderReady, CommandFound: true, LaunchTrusted: true, ProfileReady: true, ResolvedCommand: []string{`C:\Program Files\nodejs\node.exe`, `C:\Users\fixture\node_modules\@openai\codex\bin\codex.js`}}
		},
		Runner: func(_ context.Context, request RunRequest, _ *masking.Masker) RunResult {
			captured = request
			return RunResult{State: "succeeded", Structured: map[string]any{"ok": true}, RawTranscript: true}
		},
	})
	execution := CodexExecutionFromContext(ctx)
	command, err := BuildCodexInvocationCommand(execution.Resolver(), "fixture model")
	if err != nil {
		t.Fatal(err)
	}
	result := execution.Runner(context.Background(), RunRequest{Provider: "codex", Model: "fixture model", Command: command}, masking.New(nil, nil))
	if result.State != "succeeded" || captured.Command.Executable != `C:\Program Files\nodejs\node.exe` {
		t.Fatalf("result=%#v command=%#v", result, captured.Command)
	}
	want := []string{`C:\Users\fixture\node_modules\@openai\codex\bin\codex.js`, "exec", "--json", "--model", "fixture model"}
	if len(captured.Command.Arguments) != len(want) {
		t.Fatalf("arguments = %#v", captured.Command.Arguments)
	}
	for index := range want {
		if captured.Command.Arguments[index] != want[index] {
			t.Fatalf("arguments = %#v, want %#v", captured.Command.Arguments, want)
		}
	}
}

func TestParseCodexOutputMasksAndRejectsUnsupportedShape(t *testing.T) {
	masker := masking.New(nil, []string{"AUTHORIZATION"})
	fixture := strings.Join([]string{
		`{"type":"thread.started","thread_id":"thread-fixture"}`,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"Authorization: Bearer fixture-token"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":2,"output_tokens":5,"total_tokens":15}}`,
	}, "\n")
	structured, usage, err := ParseCodexOutputWithUsage(fixture, masker)
	if err != nil {
		t.Fatal(err)
	}
	message, _ := structured["result"].(string)
	if !strings.Contains(message, masking.Replacement) || strings.Contains(message, "fixture-token") {
		t.Fatalf("structured=%#v", structured)
	}
	if usage["input"] != 10 || usage["cached"] != 2 || usage["output"] != 5 || usage["total"] != 15 {
		t.Fatalf("usage=%#v", usage)
	}
	for _, output := range []string{"", "not json", "[1,2]", `{"type":"turn.started"}`, `{"type":"unknown"}`} {
		if _, err := ParseCodexOutput(output, masker); err == nil {
			t.Fatalf("accepted unsupported Codex output %q", output)
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
