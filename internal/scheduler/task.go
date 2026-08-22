// Package scheduler defines the typed Windows Task Scheduler boundary.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

type OperationKind string

const (
	OperationInstall   OperationKind = "install"
	OperationUninstall OperationKind = "uninstall"
	OperationStatus    OperationKind = "status"
	OperationDryRun    OperationKind = "dry-run"
)

const (
	AppTaskName            = "DevControlRoom.Startup"
	MultipleInstanceIgnore = "ignore_new"

	// Windows Task Scheduler COM vtable offsets. Keep these constants separate
	// from the raw calls so they are reviewable without executing COM.
	taskServiceGetFolderSlot             = 7
	taskServiceNewTaskSlot               = 9
	taskServiceConnectSlot               = 10
	taskFolderRegisterTaskDefinitionSlot = 17
	// DailyStartBoundary is intentionally fixed so the task definition is
	// inspectable and does not depend on the process's current time zone.
	DailyStartBoundary = "2000-01-01T03:00:00"
)

type Operation struct {
	Kind                   OperationKind `json:"kind"`
	TaskName               string        `json:"taskName"`
	ExecutablePath         string        `json:"executablePath"`
	Arguments              []string      `json:"arguments,omitempty"`
	CatchUpDaily           bool          `json:"catchUpDaily"`
	MultipleInstancePolicy string        `json:"multipleInstancePolicy"`
}

type Result struct {
	Operation Operation `json:"operation"`
	Applied   bool      `json:"applied"`
	Exists    bool      `json:"exists"`
	Message   string    `json:"message"`
}

type Adapter interface {
	Apply(context.Context, Operation) (Result, error)
}

type FakeAdapter struct {
	mu     sync.Mutex
	Last   *Result
	Exists bool
}

// Restore preserves the fake scheduler's state across a local restart. Native
// adapters always query Task Scheduler instead of trusting this cached value.
func (a *FakeAdapter) Restore(exists bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Exists = exists
}

func (a *FakeAdapter) Apply(ctx context.Context, operation Operation) (Result, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := Validate(operation); err != nil {
		return Result{}, err
	}
	result := Result{Operation: operation, Exists: a.Exists, Message: "dry-run only; Windows Task Scheduler was not changed"}
	switch operation.Kind {
	case OperationInstall:
		result.Exists = true
	case OperationUninstall:
		result.Exists = false
	case OperationStatus, OperationDryRun:
		result.Exists = a.Exists
	default:
		return Result{}, errors.New("unsupported scheduler operation")
	}
	result.Applied = false
	a.Exists = result.Exists
	a.Last = &result
	return result, nil
}

func Plan(kind OperationKind, executable string, args []string) (Operation, error) {
	operation := Operation{Kind: kind, TaskName: AppTaskName, ExecutablePath: executable, Arguments: append([]string(nil), args...), CatchUpDaily: true, MultipleInstancePolicy: MultipleInstanceIgnore}
	if err := Validate(operation); err != nil {
		return Operation{}, err
	}
	return operation, nil
}

func Validate(operation Operation) error {
	switch operation.Kind {
	case OperationInstall, OperationUninstall, OperationStatus, OperationDryRun:
	default:
		return errors.New("unsupported scheduler operation")
	}
	if operation.TaskName != AppTaskName {
		return errors.New("scheduler task name is not application-owned")
	}
	if !isLocalAbsoluteWindowsPath(operation.ExecutablePath) {
		return errors.New("scheduler executable must be an absolute Windows path")
	}
	if !strings.EqualFold(filepath.Ext(operation.ExecutablePath), ".exe") {
		return errors.New("scheduler executable must be a Windows executable")
	}
	if hasUnsafeWindowsPathSegment(operation.ExecutablePath) {
		return errors.New("scheduler executable path must be canonical")
	}
	if filepath.Clean(operation.ExecutablePath) != operation.ExecutablePath && runtime.GOOS == "windows" {
		return errors.New("scheduler executable path must be canonical")
	}
	if operation.MultipleInstancePolicy != MultipleInstanceIgnore {
		return errors.New("scheduler must ignore duplicate instances")
	}
	if !operation.CatchUpDaily {
		return errors.New("scheduler must enable daily catch-up")
	}
	if len(operation.Arguments) != 3 || operation.Arguments[0] != "serve" || operation.Arguments[1] != "--home" || !isLocalAbsoluteWindowsPath(operation.Arguments[2]) || hasUnsafeWindowsPathSegment(operation.Arguments[2]) {
		return errors.New("scheduler arguments must be the typed serve and Windows home arguments")
	}
	for _, argument := range operation.Arguments {
		if strings.ContainsRune(argument, '\x00') {
			return errors.New("scheduler argument contains a null byte")
		}
		if strings.HasPrefix(filepath.ToSlash(filepath.Clean(argument)), "/mnt/") {
			return errors.New("scheduler arguments may not contain implicit WSL paths")
		}
	}
	return nil
}

var drivePath = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

func isLocalAbsoluteWindowsPath(value string) bool {
	value = strings.TrimSpace(value)
	return drivePath.MatchString(value) && !strings.Contains(value[2:], ":")
}

func hasUnsafeWindowsPathSegment(value string) bool {
	if strings.TrimSpace(value) != value || strings.ContainsRune(value, '\x00') || strings.ContainsRune(value, '"') {
		return true
	}
	segments := strings.Split(strings.ReplaceAll(value[3:], "/", `\`), `\`)
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." || strings.HasSuffix(segment, " ") || strings.HasSuffix(segment, ".") {
			return true
		}
	}
	return false
}

func Format(operation Operation) string {
	return fmt.Sprintf("%s %s (%s)", operation.Kind, operation.TaskName, operation.ExecutablePath)
}
