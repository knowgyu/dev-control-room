package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/contract"
)

func TestPowerShellRunbookPlansTypedArgumentsAndPersistsReferences(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	repository := tempGitRepository(t, "runbook")
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Runbooks", Path: repository})
	if err != nil || service.RunScan(context.Background(), "manual") != nil {
		t.Fatalf("project setup = %v", err)
	}
	runbook, err := service.AddPowerShellRunbook(context.Background(), AddPowerShellRunbookInput{
		ID: "release", Name: "Fixture release", ScriptPath: filepath.Join(repository, "release.ps1"), Parameters: []string{"Environment", "Version"}, EnvironmentAllowlist: []string{"PATH", "FIXTURE_STAGE"}, TimeoutSeconds: 120,
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanPowerShellRunbook(context.Background(), PowerShellRunbookPlanInput{RunbookID: runbook.ID, ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Parameters: map[string]string{"version": "1.2.3", "environment": "stage"}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Spec.ActionType != "powershell.runbook" || !plan.Spec.ApprovalRequired || plan.Spec.Execution.Executable != "pwsh" || plan.Spec.Execution.TimeoutSeconds != 120 {
		t.Fatalf("runbook plan = %#v", plan.Spec)
	}
	arguments := strings.Join(plan.Spec.Execution.Arguments, " ")
	if !strings.Contains(arguments, "-Environment stage") || !strings.Contains(arguments, "-Version 1.2.3") || !strings.HasSuffix(plan.Spec.Execution.Arguments[2], "release.ps1") {
		t.Fatalf("typed runbook arguments = %#v", plan.Spec.Execution.Arguments)
	}
	if _, err := service.PlanPowerShellRunbook(context.Background(), PowerShellRunbookPlanInput{RunbookID: runbook.ID, ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Parameters: map[string]string{"TOKEN": "must-not-persist"}}); contract.Classify(err).Code != contract.ErrorInvalidInput {
		t.Fatalf("secret parameter error = %v", err)
	}
	items, err := service.Runbooks(context.Background())
	if err != nil || len(items) != 1 || items[0].ScriptPath != runbook.ScriptPath {
		t.Fatalf("runbooks = %#v, err = %v", items, err)
	}
}
