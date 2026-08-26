package assurance

import (
	"bytes"
	"context"
	"errors"
	"os"
	"runtime"
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
		StatFile: func(name string) (os.FileMode, error) {
			if name != `C:\Program Files\nodejs\node.exe` && name != `C:\Users\fixture\AppData\Roaming\npm\node_modules\@openai\codex\bin\codex.js` {
				t.Fatalf("unexpected entry path %q", name)
			}
			return 0o600, nil
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
				StatFile: func(string) (os.FileMode, error) { return 0o600, nil },
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
		StatFile: func(name string) (os.FileMode, error) {
			if name == `C:\Program Files\nodejs\node.exe` {
				return 0o600, nil
			}
			return 0, errors.New("missing")
		},
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
		StatFile: func(string) (os.FileMode, error) { return 0o600, nil },
	}).Resolve()
	if status.State == ProviderReady || status.LaunchTrusted || status.ReasonCode != "provider.package_entry_unreadable" {
		t.Fatalf("unreadable package entry was trusted: %#v", status)
	}
}

func TestCodexResolverRequiresRegularNonSymlinkNodeAndScript(t *testing.T) {
	const (
		nodePath   = `C:\Program Files\nodejs\node.exe`
		scriptPath = `C:\Users\fixture\AppData\Roaming\npm\node_modules\@openai\codex\bin\codex.js`
	)
	regular := os.FileMode(0o600)
	for _, testCase := range []struct {
		name       string
		nodeMode   os.FileMode
		nodeErr    error
		scriptMode os.FileMode
		scriptErr  error
		wantReason string
	}{
		{name: "missing node", nodeErr: errors.New("missing"), scriptMode: regular, wantReason: "provider.node_missing"},
		{name: "node symlink", nodeMode: os.ModeSymlink | 0o777, scriptMode: regular, wantReason: "provider.node_not_regular"},
		{name: "node directory", nodeMode: os.ModeDir | 0o700, scriptMode: regular, wantReason: "provider.node_not_regular"},
		{name: "missing script", nodeMode: regular, scriptErr: errors.New("missing"), wantReason: "provider.package_entry_missing"},
		{name: "script symlink", nodeMode: regular, scriptMode: os.ModeSymlink | 0o777, wantReason: "provider.package_entry_not_regular"},
		{name: "script directory", nodeMode: regular, scriptMode: os.ModeDir | 0o700, wantReason: "provider.package_entry_not_regular"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			resolver := CodexResolver{
				LookPath: func(name string) (string, error) {
					switch name {
					case "codex.cmd":
						return `C:\Users\fixture\AppData\Roaming\npm\codex.cmd`, nil
					case "node":
						return nodePath, nil
					default:
						return "", errors.New("not found")
					}
				},
				ReadFile: func(name string) ([]byte, error) {
					if strings.HasSuffix(name, `\package.json`) {
						return []byte(`{"name":"@openai/codex","bin":"bin/codex.js"}`), nil
					}
					if name != scriptPath {
						t.Fatalf("unexpected read path %q", name)
					}
					return []byte("export {}"), nil
				},
				StatFile: func(name string) (os.FileMode, error) {
					switch name {
					case nodePath:
						return testCase.nodeMode, testCase.nodeErr
					case scriptPath:
						return testCase.scriptMode, testCase.scriptErr
					default:
						t.Fatalf("unexpected stat path %q", name)
						return 0, errors.New("unexpected path")
					}
				},
			}
			status := resolver.Resolve()
			if status.State == ProviderReady || status.LaunchTrusted || status.ProfileReady || status.ReasonCode != testCase.wantReason {
				t.Fatalf("unsafe file was trusted: %#v", status)
			}
		})
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
		if _, err := BuildCodexInvocationCommand(base, "fixture-model", CodexInvocationOptions{Worktree: `C:\fixture`, SchemaPath: `C:\app\runtime\codex\output-schema.json`, Prompt: "inspect"}); err == nil {
			t.Fatalf("accepted untrusted resolved command %#v", resolved)
		}
	}
}

func TestBuildCodexInvocationCommandRejectsUnsafeInvocationInputs(t *testing.T) {
	status := ProviderStatus{Provider: "codex", State: ProviderReady, CommandFound: true, LaunchTrusted: true, ProfileReady: true, ResolvedCommand: []string{`C:\Program Files\nodejs\node.exe`, `C:\Users\fixture\node_modules\@openai\codex\bin\codex.js`}}
	base := CodexInvocationOptions{Worktree: `C:\fixture`, SchemaPath: `C:\app\runtime\codex\output-schema.json`, Prompt: "inspect"}
	if _, err := BuildCodexInvocationCommand(status, "--ignore-rules", base); err == nil {
		t.Fatal("accepted a model that could be parsed as a control flag")
	}
	base.Worktree = `relative\fixture`
	if _, err := BuildCodexInvocationCommand(status, "fixture", base); err == nil {
		t.Fatal("accepted a relative worktree")
	}
	base = CodexInvocationOptions{Worktree: `C:\fixture`, SchemaPath: `C:\tmp\schema.json`, Prompt: "inspect"}
	if _, err := BuildCodexInvocationCommand(status, "fixture", base); err == nil {
		t.Fatal("accepted an arbitrary schema path")
	}
	base = CodexInvocationOptions{Worktree: `C:\fixture`, SchemaPath: `C:\app\runtime\codex\output-schema.json`, Prompt: "inspect\nfixture"}
	if _, err := BuildCodexInvocationCommand(status, "fixture", base); err == nil {
		t.Fatal("accepted a multiline prompt")
	}
	command, err := BuildCodexInvocationCommand(status, "fixture", CodexInvocationOptions{
		Worktree:   `C:\fixture`,
		SchemaPath: `C:\app\runtime\codex\output-schema.json`,
		Prompt:     "--help",
	})
	if err != nil {
		t.Fatal(err)
	}
	separatorIndex := len(command.Arguments) - 2
	if command.Arguments[separatorIndex] != "--" || command.Arguments[separatorIndex+1] != "--help" {
		t.Fatalf("leading-hyphen prompt was not separated: %#v", command.Arguments)
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
	command, err := BuildCodexInvocationCommand(execution.Resolver(), "fixture model", CodexInvocationOptions{Worktree: `C:\worktree`, SchemaPath: `C:\app\runtime\codex\output-schema.json`, Prompt: "inspect fixture"})
	if err != nil {
		t.Fatal(err)
	}
	result := execution.Runner(context.Background(), RunRequest{Provider: "codex", Model: "fixture model", Command: command}, masking.New(nil, nil))
	if result.State != "succeeded" || captured.Command.Executable != `C:\Program Files\nodejs\node.exe` {
		t.Fatalf("result=%#v command=%#v", result, captured.Command)
	}
	want := []string{`C:\Users\fixture\node_modules\@openai\codex\bin\codex.js`, "exec", "--json", "--sandbox", "read-only", "--ephemeral", "--ignore-user-config", "--ignore-rules", "--cd", `C:\worktree`, "--output-schema", `C:\app\runtime\codex\output-schema.json`, "--model", "fixture model", "--", "inspect fixture"}
	if len(captured.Command.Arguments) != len(want) {
		t.Fatalf("arguments = %#v", captured.Command.Arguments)
	}
	for index := range want {
		if captured.Command.Arguments[index] != want[index] {
			t.Fatalf("arguments = %#v, want %#v", captured.Command.Arguments, want)
		}
	}
}

func TestValidateCodexInvocationCommandRequiresCompleteFixedArgv(t *testing.T) {
	status := ProviderStatus{Provider: "codex", State: ProviderReady, CommandFound: true, LaunchTrusted: true, ProfileReady: true, ResolvedCommand: []string{`C:\Program Files\nodejs\node.exe`, `C:\Users\fixture\node_modules\@openai\codex\bin\codex.js`}}
	command, err := BuildCodexInvocationCommand(status, "fixture-model", CodexInvocationOptions{
		Worktree:   `C:\fixture`,
		SchemaPath: `C:\app\runtime\codex\output-schema.json`,
		Prompt:     "inspect fixture",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCodexInvocationCommand(command); err != nil {
		t.Fatalf("valid command rejected: %v", err)
	}

	mutate := func(change func([]string) []string) TypedCommand {
		args := append([]string(nil), command.Arguments...)
		return TypedCommand{Executable: command.Executable, Arguments: change(args)}
	}
	for _, testCase := range []struct {
		name    string
		command TypedCommand
	}{
		{name: "missing separator", command: mutate(func(args []string) []string {
			return append(args[:codexPromptSeparatorIndex+2], args[codexPromptSeparatorIndex+3:]...)
		})},
		{name: "wrong separator", command: mutate(func(args []string) []string {
			args[codexPromptSeparatorIndex+2] = "--help"
			return args
		})},
		{name: "extra argument", command: mutate(func(args []string) []string {
			return append(args, "unexpected")
		})},
		{name: "control reordered", command: mutate(func(args []string) []string {
			args[1] = "--"
			return args
		})},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if err := ValidateCodexInvocationCommand(testCase.command); err == nil {
				t.Fatalf("accepted malformed fixed argv: %#v", testCase.command.Arguments)
			}
		})
	}
}

func TestCodexPromptValidationAndPrivateFixedSchema(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		prompt string
		valid  bool
	}{
		{name: "trims", prompt: "  inspect fixture  ", valid: true},
		{name: "empty", prompt: " \t ", valid: false},
		{name: "nul", prompt: "inspect\x00fixture", valid: false},
		{name: "cr", prompt: "inspect\rfixture", valid: false},
		{name: "lf", prompt: "inspect\nfixture", valid: false},
		{name: "byte bound", prompt: strings.Repeat("가", (CodexPromptMaxBytes/3)+1), valid: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ValidateCodexPrompt(testCase.prompt)
			if testCase.valid {
				if err != nil || got != "inspect fixture" {
					t.Fatalf("prompt = %q, err = %v", got, err)
				}
			} else if err == nil || got != "" {
				t.Fatalf("invalid prompt accepted: %q, err=%v", got, err)
			}
		})
	}

	home := t.TempDir()
	path, err := WriteCodexOutputSchema(home)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(content, CodexOutputSchema()) {
		t.Fatalf("schema content = %q, err=%v", content, err)
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0) {
		t.Fatalf("schema permissions/type = %v, err=%v", info.Mode(), err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyCodexOutputSchema(path); err == nil {
		t.Fatal("accepted a schema-shaped path with non-fixed content")
	}
}

func TestRunTypedRejectsWorktreeMismatchAndUncontrolledSchemaBeforeProcess(t *testing.T) {
	home := t.TempDir()
	schemaPath, err := WriteCodexOutputSchema(home)
	if err != nil {
		t.Fatal(err)
	}
	worktree := t.TempDir()
	command, err := BuildCodexInvocationCommand(ProviderStatus{Provider: "codex", State: ProviderReady, CommandFound: true, LaunchTrusted: true, ProfileReady: true, ResolvedCommand: []string{`C:\Program Files\nodejs\node.exe`, `C:\Users\fixture\node_modules\@openai\codex\bin\codex.js`}}, "fixture", CodexInvocationOptions{Worktree: worktree, SchemaPath: schemaPath, Prompt: "inspect"})
	if err != nil {
		t.Fatal(err)
	}
	result := RunTyped(context.Background(), RunRequest{Provider: "codex", Command: command, Worktree: t.TempDir()}, masking.New(nil, nil))
	if result.FailureCode != "provider.invalid_command" {
		t.Fatalf("worktree mismatch result = %#v", result)
	}
	if err := os.WriteFile(schemaPath, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	result = RunTyped(context.Background(), RunRequest{Provider: "codex", Command: command, Worktree: worktree}, masking.New(nil, nil))
	if result.FailureCode != "provider.invalid_command" {
		t.Fatalf("tampered schema result = %#v", result)
	}
}

func TestParseCodexOutputMasksAndRejectsUnsupportedShape(t *testing.T) {
	masker := masking.New(nil, []string{"AUTHORIZATION"})
	fixture := strings.Join([]string{
		`{"type":"thread.started","thread_id":"thread-fixture"}`,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"{\"summary\":\"Authorization: Bearer fixture-token\",\"findings\":[\"safe finding\"],\"nextAction\":\"review\"}"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":2,"output_tokens":5,"total_tokens":15}}`,
	}, "\n")
	structured, usage, err := ParseCodexOutputWithUsage(fixture, masker)
	if err != nil {
		t.Fatal(err)
	}
	message, _ := structured["summary"].(string)
	if !strings.Contains(message, masking.Replacement) || strings.Contains(message, "fixture-token") {
		t.Fatalf("structured=%#v", structured)
	}
	if _, ok := structured["result"]; ok {
		t.Fatalf("structured result must be reduced schema data: %#v", structured)
	}
	if usage["input"] != 10 || usage["cached"] != 2 || usage["output"] != 5 || usage["total"] != 15 {
		t.Fatalf("usage=%#v", usage)
	}
	for _, output := range []string{"", "not json", "[1,2]", `{"type":"turn.started"}`, `{"type":"unknown"}`} {
		if _, err := ParseCodexOutput(output, masker); err == nil {
			t.Fatalf("accepted unsupported Codex output %q", output)
		}
	}
	for _, output := range []string{
		`{"type":"turn.completed","result":{"summary":"ok","findings":[],"nextAction":"review","extra":"reject"}}`,
		`{"type":"turn.completed","result":{"summary":"ok","findings":[1],"nextAction":"review"}}`,
		`{"type":"turn.completed","result":"{not schema json}"}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"plain text"}}
{"type":"turn.completed"}`,
	} {
		if _, err := ParseCodexOutput(output, masker); err == nil {
			t.Fatalf("accepted malformed schema output %q", output)
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
