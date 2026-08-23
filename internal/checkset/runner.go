// Package checkset executes reviewed argv check steps. It never invokes a
// shell and keeps the child environment deliberately allowlisted.
package checkset

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/environment"
	"github.com/knowgyu/dev-control-room/internal/masking"
)

const outputLimit = 64 << 10

func Run(ctx context.Context, directory string, command domain.CheckCommand, masker *masking.Masker) domain.CheckStepRun {
	result := domain.CheckStepRun{Status: domain.CheckUnavailable}
	if err := ctx.Err(); err != nil {
		result.Status = domain.CheckCancelled
		return result
	}
	environmentValues := childEnvironment(command.Environment)
	environmentValues = append(environmentValues, "TEMP="+directory)
	process, err := (environment.ProcessRunner{OutputLimit: outputLimit}).Run(ctx, command.Executable, command.Arguments, environmentValues, time.Duration(command.TimeoutSeconds)*time.Second)
	result.Stdout, result.Stderr = maskOutput(masker, command.Environment, process.Stdout), maskOutput(masker, command.Environment, process.Stderr)
	result.ExitCode = process.ExitCode
	if err == nil {
		result.Status = domain.CheckPassed
		return result
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "timed out") {
		result.Status = domain.CheckTimedOut
		return result
	}
	if ctx.Err() != nil {
		result.Status = domain.CheckCancelled
		return result
	}
	if process.ExitCode != 0 {
		result.Status = domain.CheckFailed
		return result
	}
	result.Stderr = strings.TrimSpace(result.Stderr + "\n" + mask(masker, fmt.Sprintf("process unavailable: %v", err)))
	return result
}

func childEnvironment(names []string) []string {
	values := []string{}
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok {
			values = append(values, name+"="+value)
		}
	}
	return values
}
func maskOutput(fallback *masking.Masker, names []string, value string) string {
	value = mask(fallback, value)
	secrets := []string{}
	for _, name := range names {
		if secret, ok := os.LookupEnv(name); ok {
			secrets = append(secrets, secret)
		}
	}
	return masking.New(secrets, nil).Mask(value)
}

func mask(masker *masking.Masker, value string) string {
	if masker == nil {
		return value
	}
	return masker.Mask(value)
}
