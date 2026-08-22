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
	"github.com/knowgyu/dev-control-room/internal/scheduler"
)

const version = "0.3.0-milestone-2"

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
	case "finding":
		return runFinding(args[1:], stdout, stderr)
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
	data := map[string]string{"version": version, "milestone": "2", "cli_schema": contract.EnvelopeSchema, "api_version": domain.APIVersion}
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
		return writeCLIErrorTo(stderr, contract.InvalidInput("project requires list, show, add, update, remove, repository, export, import, or scan"))
	}
	subcommand := args[0]
	args = args[1:]
	if subcommand == "repository" {
		return runRepository(args, stdout, stderr)
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
	if len(args) < 2 || args[0] != "profile" {
		return writeCLIErrorTo(stderr, contract.InvalidInput("agent requires profile list, show, add, update, or remove"))
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
	allowlist := flags.String("env", "", "comma-separated allowed environment variable names")
	if err := flags.Parse(args); err != nil {
		return writeCLIErrorTo(stderr, contract.InvalidInput(err.Error()))
	}
	probes := splitCSV(*probe)
	allowed := splitCSV(*allowlist)
	mode := domain.AgentLaunchMode(*launchMode)
	dataBoundary := domain.AgentDataBoundary(*boundary)
	ctx := context.Background()
	if command == "add" {
		profile, err := service.AddAgentProfile(ctx, app.AddAgentProfileInput{ID: *id, Name: *name, Command: *profileCommand, VersionProbe: probes, TimeoutSeconds: *timeout, EnvironmentAllowlist: allowed, LaunchMode: mode, DataBoundary: dataBoundary})
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
	profile, err := service.UpdateAgentProfile(ctx, *id, app.UpdateAgentProfileInput{Name: *name, Command: *profileCommand, VersionProbe: probes, TimeoutSeconds: *timeout, EnvironmentAllowlist: allowed, LaunchMode: mode, DataBoundary: dataBoundary})
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
