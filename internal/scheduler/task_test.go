package scheduler

import (
	"context"
	"strings"
	"testing"
)

func TestFakeSchedulerInstallUninstallStatusAndDryRun(t *testing.T) {
	adapter := &FakeAdapter{}
	operation, err := Plan(OperationInstall, `C:\Program Files\DevControlRoom\devroom.exe`, []string{"serve", "--home", `C:\Users\Fixture\AppData\Local\DevControlRoom`})
	if err != nil {
		t.Fatal(err)
	}
	if !operation.CatchUpDaily || operation.MultipleInstancePolicy != MultipleInstanceIgnore {
		t.Fatalf("unsafe scheduling defaults: %#v", operation)
	}
	if operation.Arguments[2] != `C:\Users\Fixture\AppData\Local\DevControlRoom` || strings.Contains(operation.Arguments[2], `"`) {
		t.Fatalf("Windows argument was interpolated or quoted: %#v", operation.Arguments)
	}
	result, err := adapter.Apply(context.Background(), operation)
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied || !result.Exists || !strings.Contains(result.Message, "not changed") {
		t.Fatalf("fake install changed state: %#v", result)
	}
	status, err := Plan(OperationStatus, operation.ExecutablePath, operation.Arguments)
	if err != nil {
		t.Fatal(err)
	}
	result, err = adapter.Apply(context.Background(), status)
	if err != nil || !result.Exists {
		t.Fatalf("status did not report planned install: %#v, %v", result, err)
	}
	dryRun, err := Plan(OperationDryRun, operation.ExecutablePath, operation.Arguments)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Apply(context.Background(), dryRun); err != nil {
		t.Fatal(err)
	}
	uninstall, err := Plan(OperationUninstall, operation.ExecutablePath, operation.Arguments)
	if err != nil {
		t.Fatal(err)
	}
	result, err = adapter.Apply(context.Background(), uninstall)
	if err != nil || result.Exists {
		t.Fatalf("fake uninstall did not clear planned state: %#v, %v", result, err)
	}
}

func TestSchedulerRejectsWSLPathsAndUnsafeInstancePolicy(t *testing.T) {
	if _, err := Plan(OperationInstall, `/mnt/c/Program Files/DevControlRoom/devroom.exe`, nil); err == nil {
		t.Fatal("WSL path was accepted")
	}
	operation := Operation{Kind: OperationInstall, TaskName: AppTaskName, ExecutablePath: `C:\devroom.exe`, CatchUpDaily: true, MultipleInstancePolicy: "parallel"}
	if err := Validate(operation); err == nil {
		t.Fatal("duplicate-instance policy was accepted")
	}
	operation.MultipleInstancePolicy = MultipleInstanceIgnore
	operation.Arguments = []string{"serve", "--home", "/mnt/c/DevControlRoom"}
	if err := Validate(operation); err == nil {
		t.Fatal("WSL scheduler argument was accepted")
	}
	for _, executable := range []string{`\\server\share\devroom.exe`, `C:\tools\..\devroom.exe`, `C:\tools\devroom.exe:payload`, `C:\tools\devroom.cmd`, `C:\tools\"devroom.exe`} {
		if _, err := Plan(OperationDryRun, executable, []string{"serve", "--home", `C:\DevControlRoom`}); err == nil {
			t.Fatalf("unsafe scheduler executable was accepted: %q", executable)
		}
	}
}

func TestSchedulerDailyCatchUpContractIsDeterministic(t *testing.T) {
	if DailyStartBoundary != "2000-01-01T03:00:00" {
		t.Fatalf("unexpected daily catch-up boundary: %q", DailyStartBoundary)
	}
}
