package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/knowgyu/dev-control-room/internal/app"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/mcp"
	"github.com/knowgyu/dev-control-room/internal/scheduler"
)

const version = "0.5.0"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return runServe(nil, stdout, stderr)
	}
	switch args[0] {
	case "version":
		return runVersionTo(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stdout, stderr)
	case "project":
		return runProject(args[1:], stdout, stderr)
	case "proposal":
		return runProposal(args[1:], stdout, stderr)
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "action":
		return runAction(args[1:], stdout, stderr)
	case "finding":
		return runFinding(args[1:], stdout, stderr)
	case "cleanup":
		return runCleanup(args[1:], stdout, stderr)
	case "guidance":
		return runGuidance(args[1:], stdout, stderr)
	case "failure":
		return runFailure(args[1:], stdout, stderr)
	case "safeguard":
		return runSafeguard(args[1:], stdout, stderr)
	case "mcp":
		return runMCP(args[1:], stdout, stderr)
	case "event":
		return runEvent(args[1:], stdout, stderr)
	case "env":
		return runEnvironment(args[1:], stdout, stderr)
	case "agent":
		return runAgent(args[1:], stdout, stderr)
	case "schedule":
		return runSchedule(args[1:], stdout, stderr)
	default:
		return writeCLIErrorTo(stderr, contract.InvalidInput("unknown command: "+args[0]))
	}
}

func runVersion(args []string) int { return runVersionTo(args, os.Stdout, os.Stderr) }

func runVersionTo(args []string, stdout, stderr io.Writer) int {
	jsonOutput, remaining, err := parseJSONFlag(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	if len(remaining) != 0 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("version does not accept positional arguments"))
	}
	data := map[string]string{"version": version, "milestone": "post-mvp", "cli_schema": contract.EnvelopeSchema, "api_version": domain.APIVersion}
	if jsonOutput {
		return encodeSuccess(stdout, data)
	}
	_, _ = fmt.Fprintln(stdout, version)
	return int(contract.ExitSuccess)
}

func runServe(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	listen := flags.String("listen", "127.0.0.1:38471", "loopback address for the local control room")
	home := flags.String("home", defaultHome(), "local data directory")
	if err := flags.Parse(args); err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	service, err := app.New(*home, *listen)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go service.RunDoctor(ctx)
	server := &http.Server{Addr: *listen, Handler: service.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("Dev Control Room listening on http://%s", *listen)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return writeCLIErrorTo(stderr, err)
	}
	return int(contract.ExitSuccess)
}

func runProject(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("project requires list, show, add, update, remove, repository, worktree, discover, sync, export, import, or scan"))
	}
	subcommand := args[0]
	args = args[1:]
	if subcommand == "repository" {
		return runRepository(args, stdout, stderr)
	}
	if subcommand == "worktree" {
		return runWorktree(args, stdout, stderr)
	}
	if subcommand == "discover" {
		return runDiscover(args, stdout, stderr)
	}
	if subcommand == "sync" {
		return runProjectSync(args, stdout, stderr)
	}
	jsonOutput, args, err := parseJSONFlag(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	var home string
	args, home, err = parseHome(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	ctx := context.Background()
	switch subcommand {
	case "list":
		if len(args) != 0 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("project list takes no positional arguments"))
		}
		projects, err := service.Projects(ctx)
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		if jsonOutput {
			return encodeSuccess(stdout, projects)
		}
		for _, project := range projects {
			_, _ = fmt.Fprintf(stdout, "%s\t%s\t%d repositories\n", project.Metadata.ID, project.Metadata.Name, len(project.Spec.Repositories))
		}
		return int(contract.ExitSuccess)
	case "show":
		if len(args) != 1 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("project show requires an id"))
		}
		project, err := service.Project(ctx, args[0])
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, project, jsonOutput)
	case "add":
		return runProjectAdd(service, args, stdout, stderr, jsonOutput)
	case "update":
		return runProjectUpdate(service, args, stdout, stderr, jsonOutput)
	case "remove":
		if len(args) != 1 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("project remove requires an id"))
		}
		if err := service.RemoveProject(ctx, args[0]); err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, map[string]bool{"removed": true}, jsonOutput)
	case "export":
		if len(args) != 1 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("project export requires an id"))
		}
		data, err := service.ExportProject(ctx, args[0])
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		if jsonOutput {
			return encodeSuccess(stdout, json.RawMessage(data))
		}
		_, _ = stdout.Write(append(data, '\n'))
		return int(contract.ExitSuccess)
	case "import":
		return runProjectImport(service, args, stdout, stderr, jsonOutput)
	case "scan":
		if len(args) != 0 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("project scan takes no positional arguments"))
		}
		if err := service.RunScan(ctx, "manual"); err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, map[string]string{"status": "completed"}, jsonOutput)
	default:
		return writeCLIErrorTo(stderr, contract.InvalidInput("unknown project command: "+subcommand))
	}
}

func runProjectSync(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("project sync requires plan or execute"))
	}
	subcommand := args[0]
	jsonOutput, args, err := parseJSONFlag(args[1:])
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	args, home, err := parseHome(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	ctx := context.Background()
	switch subcommand {
	case "plan":
		if len(args) != 1 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("project sync plan requires a project id"))
		}
		plan, err := service.RepositorySyncPlan(ctx, args[0])
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, plan, jsonOutput)
	case "execute":
		if len(args) < 2 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("project sync execute requires a project id and at least one plan id"))
		}
		result, err := service.ExecuteRepositorySync(ctx, app.ExecuteRepositorySyncInput{ProjectID: args[0], PlanIDs: args[1:], RequestID: fmt.Sprintf("cli-sync-%d", time.Now().UnixNano())})
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, result, jsonOutput)
	default:
		return writeCLIErrorTo(stderr, contract.InvalidInput("unknown project sync command: "+subcommand))
	}
}

func runDiscover(args []string, stdout, stderr io.Writer) int {
	jsonOutput, args, err := parseJSONFlag(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	args, home, err := parseHome(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	if len(args) != 3 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("project discover requires project, repository, and worktree ids"))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	item, err := service.Discover(context.Background(), args[0], args[1], args[2])
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	return emitObject(stdout, item, jsonOutput)
}

func runProposal(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("proposal requires list, show, apply, or reject"))
	}
	subcommand := args[0]
	jsonOutput, args, err := parseJSONFlag(args[1:])
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	args, home, err := parseHome(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	ctx := context.Background()
	switch subcommand {
	case "list":
		if len(args) < 2 || len(args) > 3 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("proposal list requires project and repository ids, plus optional worktree id"))
		}
		worktreeID := ""
		if len(args) == 3 {
			worktreeID = args[2]
		}
		items, err := service.Proposals(ctx, args[0], args[1], worktreeID)
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		if jsonOutput {
			return encodeSuccess(stdout, items)
		}
		for _, item := range items {
			_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", item.Metadata.ID, item.Spec.State, item.Spec.SourcePath, item.Spec.Command)
		}
		return int(contract.ExitSuccess)
	case "show", "apply", "reject":
		if len(args) != 1 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("proposal "+subcommand+" requires an id"))
		}
		var item domain.Proposal
		if subcommand == "show" {
			item, err = service.Proposal(ctx, args[0])
		} else if subcommand == "apply" {
			item, err = service.ApplyProposal(ctx, args[0])
		} else {
			item, err = service.RejectProposal(ctx, args[0])
		}
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, item, jsonOutput)
	default:
		return writeCLIErrorTo(stderr, contract.InvalidInput("unknown proposal command: "+subcommand))
	}
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("check requires list, show, create, apply, run, or runs"))
	}
	subcommand := args[0]
	jsonOutput, args, err := parseJSONFlag(args[1:])
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	args, home, err := parseHome(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	ctx := context.Background()
	switch subcommand {
	case "list":
		if len(args) != 2 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("check list requires project and repository ids"))
		}
		items, err := service.Checksets(ctx, args[0], args[1])
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, items, jsonOutput)
	case "show", "apply", "run", "runs":
		if len(args) != 1 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("check "+subcommand+" requires a checkset id"))
		}
		if subcommand == "show" {
			item, err := service.Checkset(ctx, args[0])
			if err != nil {
				return writeCLIErrorTo(stderr, err)
			}
			return emitObject(stdout, item, jsonOutput)
		}
		if subcommand == "apply" {
			item, err := service.ApplyCheckset(ctx, args[0])
			if err != nil {
				return writeCLIErrorTo(stderr, err)
			}
			return emitObject(stdout, item, jsonOutput)
		}
		if subcommand == "run" {
			item, err := service.RunCheckset(ctx, args[0])
			if err != nil {
				return writeCLIErrorTo(stderr, err)
			}
			if exit := emitObject(stdout, item, jsonOutput); exit != int(contract.ExitSuccess) {
				return exit
			}
			return checkRunExitCode(item.Spec.Status)
		}
		items, err := service.CheckRuns(ctx, args[0])
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, items, jsonOutput)
	case "create":
		if len(args) != 1 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("check create requires a JSON input file"))
		}
		data, err := os.ReadFile(args[0])
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		var input app.CreateChecksetInput
		if err := json.Unmarshal(data, &input); err != nil {
			return writeCLIErrorTo(stderr, contract.InvalidInput("invalid checkset JSON input"))
		}
		item, err := service.CreateCheckset(ctx, input)
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, item, jsonOutput)
	default:
		return writeCLIErrorTo(stderr, contract.InvalidInput("unknown check command: "+subcommand))
	}
}

func runAction(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("action requires plan, status, admit, or execute"))
	}
	command := args[0]
	jsonOutput, args, err := parseJSONFlag(args[1:])
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	args, home, err := parseHome(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	ctx := context.Background()
	switch command {
	case "plan":
		flags := flag.NewFlagSet("action plan", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		id := flags.String("id", "", "plan id")
		name := flags.String("name", "", "plan name")
		projectID := flags.String("project", "", "project id")
		repositoryID := flags.String("repository", "", "repository id")
		worktreeID := flags.String("worktree", "", "worktree id")
		actionType := flags.String("type", "", "reviewed action type")
		var inputs stringList
		flags.Var(&inputs, "input", "reviewed input as key=value")
		if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("action plan accepts only flags"))
		}
		values, err := inputs.Map()
		if err != nil {
			return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
		}
		plan, err := service.PlanAction(ctx, app.ActionPlanInput{ID: *id, Name: *name, ProjectID: *projectID, RepositoryID: *repositoryID, WorktreeID: *worktreeID, ActionType: *actionType, Inputs: values})
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, plan, jsonOutput)
	case "status":
		if len(args) != 1 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("action status requires a plan id"))
		}
		status, err := service.ActionStatus(ctx, args[0])
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, status, jsonOutput)
	case "runs":
		if len(args) != 1 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("action runs requires a plan id"))
		}
		runs, err := service.ActionRuns(ctx, args[0])
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, runs, jsonOutput)
	case "admit":
		if len(args) < 1 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("action admit requires a plan id"))
		}
		flags := flag.NewFlagSet("action admit", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		holder := flags.String("holder", "", "execution holder id")
		idempotencyKey := flags.String("idempotency-key", "", "unique admission request id")
		if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("action admit accepts only flags after the plan id"))
		}
		admission, err := service.AdmitAction(ctx, args[0], *holder, *idempotencyKey)
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, admission, jsonOutput)
	case "execute":
		if len(args) < 1 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("action execute requires a plan id"))
		}
		flags := flag.NewFlagSet("action execute", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		holder := flags.String("holder", "", "execution holder id")
		idempotencyKey := flags.String("idempotency-key", "", "unique execution request id")
		if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("action execute accepts only flags after the plan id"))
		}
		run, err := service.ExecuteAction(ctx, args[0], *holder, *idempotencyKey)
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, run, jsonOutput)
	default:
		return writeCLIErrorTo(stderr, contract.InvalidInput("unknown action command: "+command))
	}
}

func checkRunExitCode(status domain.CheckRunStatus) int {
	switch status {
	case domain.CheckPassed, domain.CheckSkipped:
		return int(contract.ExitSuccess)
	case domain.CheckFailed:
		return int(contract.ExitCheckFailed)
	case domain.CheckUnavailable, domain.CheckTimedOut:
		return int(contract.ExitUnavailable)
	case domain.CheckCancelled:
		return int(contract.ExitExecutionError)
	default:
		return int(contract.ExitExecutionError)
	}
}

func runWorktree(args []string, stdout, stderr io.Writer) int {
	jsonOutput, args, err := parseJSONFlag(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	args, home, err := parseHome(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	if len(args) < 3 || (args[0] != "list" && args[0] != "show") {
		return writeCLIErrorTo(stderr, contract.InvalidInput("project worktree requires list or show with project and repository ids"))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	if args[0] == "show" {
		if len(args) != 4 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("project worktree show requires project, repository, and worktree ids"))
		}
		item, err := service.Worktree(context.Background(), args[1], args[2], args[3])
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, item, jsonOutput)
	}
	if len(args) != 3 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("project worktree list requires project and repository ids"))
	}
	items, err := service.Worktrees(context.Background(), args[1], args[2])
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	if jsonOutput {
		return encodeSuccess(stdout, items)
	}
	for _, item := range items {
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", item.Metadata.ID, item.Spec.CanonicalPath, item.Spec.Branch, item.Spec.Trust)
	}
	return int(contract.ExitSuccess)
}

func runProjectAdd(service *app.App, args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	flags := flag.NewFlagSet("project add", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	name := flags.String("name", "", "project name")
	path := flags.String("path", "", "registered repository path")
	if err := flags.Parse(args); err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	project, err := service.AddProject(context.Background(), app.AddProjectInput{Name: *name, Path: *path})
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	return emitObject(stdout, project, jsonOutput)
}

func runProjectUpdate(service *app.App, args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if len(args) < 1 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("project update requires an id"))
	}
	flags := flag.NewFlagSet("project update", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	name := flags.String("name", "", "project name")
	if err := flags.Parse(args[1:]); err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	project, err := service.UpdateProject(context.Background(), args[0], app.UpdateProjectInput{Name: *name})
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	return emitObject(stdout, project, jsonOutput)
}

func runProjectImport(service *app.App, args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	flags := flag.NewFlagSet("project import", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	input := flags.String("input", "", "project export path")
	if err := flags.Parse(args); err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	if *input == "" {
		return writeCLIErrorTo(stderr, contract.InvalidInput("project import requires --input"))
	}
	data, err := os.ReadFile(*input)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput("project import input could not be read"))
	}
	project, err := service.ImportProject(context.Background(), data)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	return emitObject(stdout, project, jsonOutput)
}

func runRepository(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("project repository requires list, add, update, or remove"))
	}
	subcommand := args[0]
	args = args[1:]
	jsonOutput, args, err := parseJSONFlag(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	args, home, err := parseHome(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	ctx := context.Background()
	switch subcommand {
	case "list":
		if len(args) != 1 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("repository list requires a project id"))
		}
		repositories, err := service.Repositories(ctx, args[0])
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		if jsonOutput {
			return encodeSuccess(stdout, repositories)
		}
		for _, repository := range repositories {
			_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\n", repository.Metadata.ID, repository.Metadata.Name, repository.Spec.Path)
		}
		return int(contract.ExitSuccess)
	case "add":
		flags := flag.NewFlagSet("project repository add", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		projectID := flags.String("project", "", "project id")
		id := flags.String("id", "", "repository id")
		name := flags.String("name", "", "repository name")
		path := flags.String("path", "", "registered repository path")
		if err := flags.Parse(args); err != nil {
			return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
		}
		repository, err := service.AddRepository(ctx, app.AddRepositoryInput{ProjectID: *projectID, ID: *id, Name: *name, Path: *path})
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, repository, jsonOutput)
	case "update":
		if len(args) < 2 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("repository update requires project and repository ids"))
		}
		flags := flag.NewFlagSet("project repository update", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		name := flags.String("name", "", "repository name")
		path := flags.String("path", "", "registered repository path")
		if err := flags.Parse(args[2:]); err != nil {
			return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
		}
		repository, err := service.UpdateRepository(ctx, args[0], args[1], app.UpdateRepositoryInput{Name: *name, Path: *path})
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, repository, jsonOutput)
	case "remove":
		if len(args) != 2 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("repository remove requires project and repository ids"))
		}
		if err := service.RemoveRepository(ctx, args[0], args[1]); err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, map[string]bool{"removed": true}, jsonOutput)
	default:
		return writeCLIErrorTo(stderr, contract.InvalidInput("unknown repository command: "+subcommand))
	}
}

func runFinding(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("finding requires list, show, or acknowledge"))
	}
	subcommand := args[0]
	args = args[1:]
	jsonOutput, args, err := parseJSONFlag(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	args, home, err := parseHome(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	flags := flag.NewFlagSet("finding", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	projectID := flags.String("project", "", "project id filter")
	repositoryID := flags.String("repository", "", "repository id filter")
	if subcommand == "list" {
		if err := flags.Parse(args); err != nil {
			return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
		}
		service, err := openCLIService(home)
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		defer service.Close()
		findings, err := service.Findings(context.Background(), *projectID, *repositoryID)
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		if jsonOutput {
			return encodeSuccess(stdout, findings)
		}
		for _, finding := range findings {
			_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", finding.Metadata.ID, finding.Spec.Severity, finding.Spec.State, finding.Spec.Summary)
		}
		return int(contract.ExitSuccess)
	}
	if len(args) != 1 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("finding command requires an id"))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	if subcommand == "show" {
		finding, err := service.Finding(context.Background(), args[0])
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, finding, jsonOutput)
	}
	if subcommand == "acknowledge" {
		if err := service.AcknowledgeFinding(context.Background(), args[0]); err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, map[string]string{"status": "acknowledged"}, jsonOutput)
	}
	return writeCLIErrorTo(stderr, contract.InvalidInput("unknown finding command: "+subcommand))
}

func runCleanup(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "list" {
		return writeCLIErrorTo(stderr, contract.InvalidInput("cleanup requires list"))
	}
	jsonOutput, args, err := parseJSONFlag(args[1:])
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	args, home, err := parseHome(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	flags := flag.NewFlagSet("cleanup list", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	projectID := flags.String("project", "", "project id filter")
	if err := flags.Parse(args); err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	items, err := service.CleanupCandidates(context.Background(), *projectID)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	if jsonOutput {
		return encodeSuccess(stdout, items)
	}
	for _, item := range items {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", item.Metadata.ID, item.Spec.Decision, item.Spec.Branch, strings.Join(item.Spec.Reasons, "; "))
	}
	return int(contract.ExitSuccess)
}

func runGuidance(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "check" {
		return writeCLIErrorTo(stderr, contract.InvalidInput("guidance requires check"))
	}
	jsonOutput, args, err := parseJSONFlag(args[1:])
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	args, home, err := parseHome(args)
	if err != nil || len(args) != 3 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("guidance check requires project, repository, and worktree ids"))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	report, err := service.Guidance(context.Background(), args[0], args[1], args[2])
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	if jsonOutput {
		return encodeSuccess(stdout, report)
	}
	for _, finding := range report.Findings {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\n", finding.Severity, finding.Code, finding.Summary)
	}
	if len(report.Findings) == 0 {
		_, _ = fmt.Fprintln(stdout, "guidance healthy")
	}
	return int(contract.ExitSuccess)
}

func runFailure(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "list" {
		return writeCLIErrorTo(stderr, contract.InvalidInput("failure requires list"))
	}
	jsonOutput, args, err := parseJSONFlag(args[1:])
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	args, home, err := parseHome(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	flags := flag.NewFlagSet("failure list", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	limit := flags.Int("limit", 100, "maximum fingerprints")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("failure list accepts only --limit"))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	items, err := service.FailureFingerprints(context.Background(), *limit)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	if jsonOutput {
		return encodeSuccess(stdout, items)
	}
	for _, item := range items {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\t%d\n", item.Spec.Fingerprint, item.Spec.Category, item.Spec.OccurrenceCount)
	}
	return int(contract.ExitSuccess)
}

func runSafeguard(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "list" {
		return writeCLIErrorTo(stderr, contract.InvalidInput("safeguard requires list"))
	}
	jsonOutput, args, err := parseJSONFlag(args[1:])
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	args, home, err := parseHome(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	flags := flag.NewFlagSet("safeguard list", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	limit := flags.Int("limit", 100, "maximum fingerprints to inspect")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("safeguard list accepts only --limit"))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	items, err := service.Safeguards(context.Background(), *limit)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	return emitObject(stdout, items, jsonOutput)
}

func runMCP(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "serve" {
		return writeCLIErrorTo(stderr, contract.InvalidInput("mcp requires serve"))
	}
	args, home, err := parseHome(args[1:])
	if err != nil || len(args) != 0 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("mcp serve accepts only --home"))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	if err := mcp.Serve(context.Background(), os.Stdin, stdout, service); err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	return int(contract.ExitSuccess)
}

func runEvent(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "list" {
		return writeCLIErrorTo(stderr, contract.InvalidInput("event requires list"))
	}
	args = args[1:]
	jsonOutput, args, err := parseJSONFlag(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	args, home, err := parseHome(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	flags := flag.NewFlagSet("event list", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	limit := flags.Int("limit", 100, "maximum events")
	if err := flags.Parse(args); err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	events, err := service.Events(context.Background(), *limit)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	if jsonOutput {
		return encodeSuccess(stdout, events)
	}
	for _, event := range events {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\n", event.Spec.OccurredAt.Format(time.RFC3339), event.Spec.EventType, event.Spec.Summary)
	}
	return int(contract.ExitSuccess)
}

func runEnvironment(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "doctor" && args[0] != "status") {
		return writeCLIErrorTo(stderr, contract.InvalidInput("env requires doctor or status"))
	}
	command := args[0]
	jsonOutput, args, err := parseJSONFlag(args[1:])
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	args, home, err := parseHome(args)
	if err != nil || len(args) != 0 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("env command accepts only --json and --home"))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	health, err := service.EnvironmentHealth(context.Background(), command == "doctor")
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	if jsonOutput {
		if code := encodeSuccess(stdout, health); code != int(contract.ExitSuccess) {
			return code
		}
	} else {
		for _, finding := range health.Findings {
			_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", finding.Severity, finding.Type, finding.Summary, finding.RecommendedNextAction)
		}
		if len(health.Findings) == 0 {
			_, _ = fmt.Fprintln(stdout, "environment healthy")
		}
	}
	if !health.Available {
		return int(contract.ExitUnavailable)
	}
	return int(contract.ExitSuccess)
}

func runAgent(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("agent requires profile or handoff"))
	}
	if args[0] == "handoff" {
		return runAgentHandoff(args[1:], stdout, stderr)
	}
	if args[0] != "profile" {
		return writeCLIErrorTo(stderr, contract.InvalidInput("agent requires profile or handoff"))
	}
	command := args[1]
	jsonOutput, args, err := parseJSONFlag(args[2:])
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	args, home, err := parseHome(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	ctx := context.Background()
	switch command {
	case "list":
		if len(args) != 0 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("agent profile list takes no positional arguments"))
		}
		profiles, err := service.AgentProfiles(ctx)
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		if jsonOutput {
			return encodeSuccess(stdout, profiles)
		}
		for _, profile := range profiles {
			_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", profile.Metadata.ID, profile.Metadata.Name, profile.Spec.LaunchMode, profile.Spec.Command)
		}
		return int(contract.ExitSuccess)
	case "show":
		if len(args) != 1 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("agent profile show requires an id"))
		}
		profile, err := service.AgentProfile(ctx, args[0])
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, profile, jsonOutput)
	case "remove":
		if len(args) != 1 {
			return writeCLIErrorTo(stderr, contract.InvalidInput("agent profile remove requires an id"))
		}
		if err := service.RemoveAgentProfile(ctx, args[0]); err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, map[string]bool{"removed": true}, jsonOutput)
	case "add", "update":
		return runAgentProfileMutation(service, command, args, stdout, stderr, jsonOutput)
	default:
		return writeCLIErrorTo(stderr, contract.InvalidInput("unknown agent profile command: "+command))
	}
}

func runAgentHandoff(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "preview" && args[0] != "launch") {
		return writeCLIErrorTo(stderr, contract.InvalidInput("agent handoff requires preview or launch"))
	}
	command := args[0]
	jsonOutput, args, err := parseJSONFlag(args[1:])
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	args, home, err := parseHome(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	flags := flag.NewFlagSet("agent handoff "+command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	profile := flags.String("profile", "", "agent profile id")
	model := flags.String("model", "", "optional selected model metadata")
	project := flags.String("project", "", "project id")
	repository := flags.String("repository", "", "repository id")
	worktree := flags.String("worktree", "", "worktree id")
	digest := flags.String("preview-digest", "", "digest returned by handoff preview")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("agent handoff accepts only flags"))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	input := app.HandoffInput{ProfileID: *profile, ProjectID: *project, RepositoryID: *repository, WorktreeID: *worktree, Model: *model}
	if command == "preview" {
		preview, err := service.PrepareHandoff(context.Background(), input)
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, preview, jsonOutput)
	}
	launch, err := service.LaunchHandoff(context.Background(), app.HandoffLaunchInput{HandoffInput: input, PreviewDigest: *digest})
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	return emitObject(stdout, launch, jsonOutput)
}

func runAgentProfileMutation(service *app.App, command string, args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	positionalID := ""
	if command == "update" && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		positionalID, args = args[0], args[1:]
	}
	flags := flag.NewFlagSet("agent profile "+command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	id := flags.String("id", "", "profile id")
	name := flags.String("name", "", "profile name")
	profileCommand := flags.String("command", "", "executable or PowerShell command name")
	launchMode := flags.String("launch-mode", "", "direct or powershell_profile")
	boundary := flags.String("data-boundary", "", "local or enterprise")
	probe := flags.String("version-probe", "", "comma-separated version probe arguments")
	timeout := flags.Int("timeout", 0, "probe timeout in seconds")
	modelArguments := flags.String("model-args", "", "optional model argument template")
	allowlist := flags.String("env", "", "comma-separated allowed environment variable names")
	if err := flags.Parse(args); err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	var updatedModelArguments *string
	flags.Visit(func(item *flag.Flag) {
		if item.Name == "model-args" {
			updatedModelArguments = modelArguments
		}
	})
	probes := splitCSV(*probe)
	allowed := splitCSV(*allowlist)
	mode := domain.AgentLaunchMode(*launchMode)
	dataBoundary := domain.AgentDataBoundary(*boundary)
	ctx := context.Background()
	if command == "add" {
		profile, err := service.AddAgentProfile(ctx, app.AddAgentProfileInput{ID: *id, Name: *name, Command: *profileCommand, VersionProbe: probes, TimeoutSeconds: *timeout, ModelArgumentTemplate: *modelArguments, EnvironmentAllowlist: allowed, LaunchMode: mode, DataBoundary: dataBoundary})
		if err != nil {
			return writeCLIErrorTo(stderr, err)
		}
		return emitObject(stdout, profile, jsonOutput)
	}
	if *id == "" {
		*id = positionalID
	}
	if *id == "" {
		return writeCLIErrorTo(stderr, contract.InvalidInput("agent profile update requires --id"))
	}
	profile, err := service.UpdateAgentProfile(ctx, *id, app.UpdateAgentProfileInput{Name: *name, Command: *profileCommand, VersionProbe: probes, TimeoutSeconds: *timeout, ModelArgumentTemplate: updatedModelArguments, EnvironmentAllowlist: allowed, LaunchMode: mode, DataBoundary: dataBoundary})
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	return emitObject(stdout, profile, jsonOutput)
}

func runSchedule(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("schedule requires install, uninstall, status, or dry-run"))
	}
	command := args[0]
	jsonOutput, args, err := parseJSONFlag(args[1:])
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	args, home, err := parseHome(args)
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	flags := flag.NewFlagSet("schedule "+command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	executable := flags.String("executable", "", "absolute Windows executable path")
	if err := flags.Parse(args); err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	if flags.NArg() != 0 {
		return writeCLIErrorTo(stderr, contract.InvalidInput("schedule takes no positional arguments"))
	}
	kind := scheduler.OperationKind(command)
	if command == "dry-run" {
		kind = scheduler.OperationDryRun
	}
	if command != string(scheduler.OperationInstall) && command != string(scheduler.OperationUninstall) && command != string(scheduler.OperationStatus) && command != string(scheduler.OperationDryRun) {
		return writeCLIErrorTo(stderr, contract.InvalidInput("unknown schedule command"))
	}
	if *executable == "" {
		*executable, err = os.Executable()
		if err != nil {
			return writeCLIErrorTo(stderr, contract.InvalidInput("schedule requires --executable"))
		}
	}
	operation, err := scheduler.Plan(kind, *executable, []string{"serve", "--home", home})
	if err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	service, err := openCLIService(home)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	defer service.Close()
	result, err := service.Schedule(context.Background(), operation)
	if err != nil {
		return writeCLIErrorTo(stderr, err)
	}
	return emitObject(stdout, result, jsonOutput)
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			result = append(result, value)
		}
	}
	return result
}

type stringList []string

func (values *stringList) String() string { return strings.Join(*values, ",") }

func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func (values stringList) Map() (map[string]string, error) {
	result := make(map[string]string, len(values))
	for _, value := range values {
		key, item, ok := strings.Cut(value, "=")
		if !ok || strings.TrimSpace(key) == "" || strings.TrimSpace(item) == "" {
			return nil, errors.New("action input must be key=value")
		}
		if _, exists := result[key]; exists {
			return nil, errors.New("action input keys must be unique")
		}
		result[key] = item
	}
	return result, nil
}

func openCLIService(home string) (*app.App, error) { return app.New(home, "127.0.0.1:0") }

func parseJSONFlag(args []string) (bool, []string, error) {
	jsonOutput := false
	remaining := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		remaining = append(remaining, arg)
	}
	return jsonOutput, remaining, nil
}

func parseHome(args []string) ([]string, string, error) {
	home := defaultHome()
	remaining := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		if args[index] == "--home" {
			if index+1 >= len(args) {
				return nil, "", errors.New("--home requires a value")
			}
			home = args[index+1]
			index++
			continue
		}
		if strings.HasPrefix(args[index], "--home=") {
			home = strings.TrimPrefix(args[index], "--home=")
			continue
		}
		remaining = append(remaining, args[index])
	}
	return remaining, home, nil
}

func emitObject[T any](stdout io.Writer, value T, jsonOutput bool) int {
	if jsonOutput {
		return encodeSuccess(stdout, value)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return int(contract.ExitInternal)
	}
	_, _ = stdout.Write(append(data, '\n'))
	return int(contract.ExitSuccess)
}

func encodeSuccess[T any](stdout io.Writer, value T) int {
	if err := json.NewEncoder(stdout).Encode(contract.Success(value)); err != nil {
		return int(contract.ExitInternal)
	}
	return int(contract.ExitSuccess)
}

func writeCLIError(err error) int { return writeCLIErrorTo(os.Stderr, err) }

func writeCLIErrorTo(stderr io.Writer, err error) int {
	classified := contract.Classify(err)
	payload := contract.Failure[map[string]any](classified.Code, classified.Message, classified.Details)
	if encodeErr := json.NewEncoder(stderr).Encode(payload); encodeErr != nil {
		_, _ = fmt.Fprintln(stderr, classified.Message)
	}
	return int(classified.Code.ExitCode())
}

func defaultHome() string {
	if value := os.Getenv("DEV_CONTROL_ROOM_HOME"); value != "" {
		return value
	}
	if value := os.Getenv("LOCALAPPDATA"); value != "" {
		return filepath.Join(value, "DevControlRoom")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "data")
	}
	return filepath.Join(dir, "DevControlRoom")
}

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)
	log.SetPrefix("dev-control-room: ")
}
