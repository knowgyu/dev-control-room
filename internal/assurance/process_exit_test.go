package assurance

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/environment"
)

// The test binary is also used as a tiny child process. Running this branch
// during init keeps the output limited to the adapter report and produces the
// same result + *exec.ExitError shape as ProcessRunner sees in production.
func init() {
	if os.Getenv("QUALITY_PROCESS_HELPER") != "1" {
		return
	}
	payloads := map[string]string{
		"ruff":   `[{"code":"E501","message":"line too long","filename":"main.py","location":{"row":4,"column":2}}]`,
		"pytest": `{"passed":1,"failed":1}`,
		"eslint": `[{"filePath":"src/app.ts","messages":[{"ruleId":"no-console","message":"Unexpected console statement","severity":2,"line":3,"column":4}]}]`,
		"vitest": `{"numTotalTests":2,"numPassedTests":1,"numFailedTests":1,"numPendingTests":0,"testResults":[]}`,
	}
	payload, ok := payloads[os.Getenv("QUALITY_PROCESS_HELPER_KIND")]
	if !ok {
		os.Exit(2)
	}
	_, _ = os.Stdout.WriteString(payload)
	os.Exit(1)
}

type helperQualityProcess struct {
	kind       string
	processErr *error
}

func (p *helperQualityProcess) Run(ctx context.Context, _ TypedCommand, directory string, timeout time.Duration, limit int) (QualityProcessOutput, error) {
	executable, err := os.Executable()
	if err != nil {
		return QualityProcessOutput{}, err
	}
	result, err := (environment.ProcessRunner{OutputLimit: limit}).RunInDirectory(
		ctx,
		executable,
		[]string{"-test.run=^$"},
		[]string{"QUALITY_PROCESS_HELPER=1", "QUALITY_PROCESS_HELPER_KIND=" + p.kind},
		directory,
		timeout,
	)
	if p.processErr != nil {
		*p.processErr = err
	}
	return QualityProcessOutput{Stdout: result.Stdout, Stderr: result.Stderr, ExitCode: result.ExitCode}, err
}

func assertExpectedProcessExit(t *testing.T, processErr error) {
	t.Helper()
	var exitErr *exec.ExitError
	if !errors.As(processErr, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("ProcessRunner error = %T %v, want *exec.ExitError with exit code 1", processErr, processErr)
	}
}
