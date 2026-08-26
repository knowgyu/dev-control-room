package app

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

func newHTTPHandler(service ApplicationService, listen, mutationToken string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(response http.ResponseWriter, _ *http.Request) {
		writeUI(response, mutationToken)
	})
	mux.HandleFunc("GET /ui/app.css", func(response http.ResponseWriter, _ *http.Request) {
		writeUIAsset(response, "text/css", uiAppCSS)
	})
	mux.HandleFunc("GET /ui/app.js", func(response http.ResponseWriter, _ *http.Request) {
		writeUIAsset(response, "text/javascript", uiAppJS)
	})
	mux.HandleFunc("GET /api/health", func(response http.ResponseWriter, request *http.Request) {
		writeEnvelope(response, http.StatusOK, contract.Success(service.Health(request.Context())))
	})
	mux.HandleFunc("GET /api/state", func(response http.ResponseWriter, request *http.Request) {
		state, err := service.Snapshot(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(state))
	})
	mux.HandleFunc("GET /api/integrations", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.Integrations(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("GET /api/runbooks", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.Runbooks(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("GET /api/external-work-groups", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.ExternalWorkGroups(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("POST /api/external-work-groups", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input ExternalWorkGroupConfig
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.AddExternalWorkGroup(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(item))
	}))
	mux.HandleFunc("PUT /api/external-work-groups/{groupID}", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input ExternalWorkGroupConfig
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.UpdateExternalWorkGroup(request.Context(), request.PathValue("groupID"), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	}))
	mux.HandleFunc("DELETE /api/external-work-groups/{groupID}", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := service.RemoveExternalWorkGroup(request.Context(), request.PathValue("groupID")); err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(map[string]bool{"removed": true}))
	}))
	mux.HandleFunc("POST /api/external-work-groups/{groupID}/plan", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input ExternalWorkPlanInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		input.GroupID = request.PathValue("groupID")
		plan, err := service.PlanExternalWork(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(plan))
	}))
	mux.HandleFunc("GET /api/external-work-plans/{planID}", func(response http.ResponseWriter, request *http.Request) {
		plan, err := service.ExternalWorkPlan(request.Context(), request.PathValue("planID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(plan))
	})
	mux.HandleFunc("GET /api/external-work-plans/{planID}/result", func(response http.ResponseWriter, request *http.Request) {
		result, err := service.ExternalWorkResult(request.Context(), request.PathValue("planID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(result))
	})
	mux.HandleFunc("POST /api/external-work-plans/{planID}/execute", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Holder         string `json:"holder"`
			IdempotencyKey string `json:"idempotencyKey"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		result, err := service.ExecuteExternalWork(request.Context(), request.PathValue("planID"), input.Holder, input.IdempotencyKey)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(result))
	}))
	mux.HandleFunc("POST /api/releases/{groupID}/plan", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input ReleasePlanInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		input.GroupID = request.PathValue("groupID")
		plan, err := service.PlanRelease(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(plan))
	}))
	mux.HandleFunc("GET /api/release-plans/{planID}", func(response http.ResponseWriter, request *http.Request) {
		plan, err := service.ReleasePlan(request.Context(), request.PathValue("planID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(plan))
	})
	mux.HandleFunc("GET /api/release-plans/{planID}/result", func(response http.ResponseWriter, request *http.Request) {
		result, err := service.ReleaseResult(request.Context(), request.PathValue("planID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(result))
	})
	mux.HandleFunc("POST /api/release-plans/{planID}/execute", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Holder         string `json:"holder"`
			IdempotencyKey string `json:"idempotencyKey"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		result, err := service.ExecuteRelease(request.Context(), request.PathValue("planID"), input.Holder, input.IdempotencyKey)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(result))
	}))
	mux.HandleFunc("POST /api/runbooks", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input AddPowerShellRunbookInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.AddPowerShellRunbook(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(item))
	}))
	mux.HandleFunc("PUT /api/runbooks/{runbookID}", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input UpdatePowerShellRunbookInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.UpdatePowerShellRunbook(request.Context(), request.PathValue("runbookID"), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	}))
	mux.HandleFunc("DELETE /api/runbooks/{runbookID}", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := service.RemovePowerShellRunbook(request.Context(), request.PathValue("runbookID")); err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(map[string]bool{"removed": true}))
	}))
	mux.HandleFunc("POST /api/runbooks/{runbookID}/plan", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input PowerShellRunbookPlanInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		input.RunbookID = request.PathValue("runbookID")
		plan, err := service.PlanPowerShellRunbook(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(plan))
	}))
	mux.HandleFunc("POST /api/integrations", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input AddIntegrationInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.AddIntegration(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(item))
	}))
	mux.HandleFunc("PUT /api/integrations/{integrationID}", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input UpdateIntegrationInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.UpdateIntegration(request.Context(), request.PathValue("integrationID"), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	}))
	mux.HandleFunc("DELETE /api/integrations/{integrationID}", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := service.RemoveIntegration(request.Context(), request.PathValue("integrationID")); err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(map[string]bool{"removed": true}))
	}))
	mux.HandleFunc("POST /api/integrations/{integrationID}/check", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := requireEmptyBody(request); err != nil {
			writeServiceError(response, contract.InvalidInput("integration check accepts an empty body only"))
			return
		}
		health, err := service.CheckIntegration(request.Context(), request.PathValue("integrationID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(health))
	}))
	mux.HandleFunc("POST /api/integrations/{integrationID}/github/latest-run", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := requireEmptyBody(request); err != nil {
			writeServiceError(response, contract.InvalidInput("GitHub latest run accepts an empty body only"))
			return
		}
		run, err := service.GitHubLatestRun(request.Context(), request.PathValue("integrationID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(run))
	}))
	mux.HandleFunc("POST /api/integrations/{integrationID}/jenkins/latest-build", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := requireEmptyBody(request); err != nil {
			writeServiceError(response, contract.InvalidInput("Jenkins latest build accepts an empty body only"))
			return
		}
		build, err := service.JenkinsLatestBuild(request.Context(), request.PathValue("integrationID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(build))
	}))
	mux.HandleFunc("POST /api/integrations/{integrationID}/kubernetes/status", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := requireEmptyBody(request); err != nil {
			writeServiceError(response, contract.InvalidInput("Kubernetes status accepts an empty body only"))
			return
		}
		status, err := service.KubernetesStatus(request.Context(), request.PathValue("integrationID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(status))
	}))
	mux.HandleFunc("POST /api/integrations/{integrationID}/kubernetes/logs", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := requireEmptyBody(request); err != nil {
			writeServiceError(response, contract.InvalidInput("Kubernetes logs accepts an empty body only"))
			return
		}
		logs, err := service.KubernetesLogs(request.Context(), request.PathValue("integrationID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(logs))
	}))
	mux.HandleFunc("GET /api/projects", func(response http.ResponseWriter, request *http.Request) {
		projects, err := service.Projects(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(projects))
	})
	mux.HandleFunc("GET /api/projects/{projectID}", func(response http.ResponseWriter, request *http.Request) {
		project, err := service.Project(request.Context(), request.PathValue("projectID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(project))
	})
	mux.HandleFunc("GET /api/projects/{projectID}/repositories", func(response http.ResponseWriter, request *http.Request) {
		repositories, err := service.Repositories(request.Context(), request.PathValue("projectID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(repositories))
	})
	mux.HandleFunc("GET /api/projects/{projectID}/repositories/{repositoryID}", func(response http.ResponseWriter, request *http.Request) {
		repository, err := service.Repository(request.Context(), request.PathValue("projectID"), request.PathValue("repositoryID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(repository))
	})
	mux.HandleFunc("GET /api/projects/{projectID}/repositories/{repositoryID}/worktrees", func(response http.ResponseWriter, request *http.Request) {
		worktrees, err := service.Worktrees(request.Context(), request.PathValue("projectID"), request.PathValue("repositoryID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(worktrees))
	})
	mux.HandleFunc("GET /api/projects/{projectID}/repositories/{repositoryID}/worktrees/{worktreeID}", func(response http.ResponseWriter, request *http.Request) {
		item, err := service.Worktree(request.Context(), request.PathValue("projectID"), request.PathValue("repositoryID"), request.PathValue("worktreeID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	})
	mux.HandleFunc("GET /api/projects/{projectID}/repositories/{repositoryID}/proposals", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.Proposals(request.Context(), request.PathValue("projectID"), request.PathValue("repositoryID"), request.URL.Query().Get("worktree_id"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("GET /api/proposals/{proposalID}", func(response http.ResponseWriter, request *http.Request) {
		item, err := service.Proposal(request.Context(), request.PathValue("proposalID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	})
	mux.HandleFunc("GET /api/projects/{projectID}/repositories/{repositoryID}/checksets", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.Checksets(request.Context(), request.PathValue("projectID"), request.PathValue("repositoryID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("GET /api/checksets/{checksetID}", func(response http.ResponseWriter, request *http.Request) {
		item, err := service.Checkset(request.Context(), request.PathValue("checksetID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	})
	mux.HandleFunc("GET /api/checksets/{checksetID}/runs", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.CheckRuns(request.Context(), request.PathValue("checksetID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("GET /api/projects/{projectID}/export", func(response http.ResponseWriter, request *http.Request) {
		data, err := service.ExportProject(request.Context(), request.PathValue("projectID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(json.RawMessage(data)))
	})
	mux.HandleFunc("GET /api/findings", func(response http.ResponseWriter, request *http.Request) {
		findings, err := service.Findings(request.Context(), request.URL.Query().Get("project_id"), request.URL.Query().Get("repository_id"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(findings))
	})
	mux.HandleFunc("GET /api/findings/{findingID}", func(response http.ResponseWriter, request *http.Request) {
		finding, err := service.Finding(request.Context(), request.PathValue("findingID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(finding))
	})
	mux.HandleFunc("GET /api/events", func(response http.ResponseWriter, request *http.Request) {
		limit := 100
		if value, err := strconv.Atoi(request.URL.Query().Get("limit")); err == nil && value > 0 && value <= 1000 {
			limit = value
		}
		events, err := service.Events(request.Context(), limit)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(events))
	})
	mux.HandleFunc("GET /api/cleanup/candidates", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.CleanupCandidates(request.Context(), request.URL.Query().Get("project_id"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("POST /api/cleanup/{candidateID}/plan", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input CleanupPlanInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		input.CandidateID = request.PathValue("candidateID")
		plan, err := service.PlanCleanup(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(plan))
	}))
	mux.HandleFunc("GET /api/cleanup/plans/{planID}", func(response http.ResponseWriter, request *http.Request) {
		plan, err := service.CleanupPlan(request.Context(), request.PathValue("planID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(plan))
	})
	mux.HandleFunc("GET /api/cleanup/plans/{planID}/result", func(response http.ResponseWriter, request *http.Request) {
		result, err := service.CleanupResult(request.Context(), request.PathValue("planID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(result))
	})
	mux.HandleFunc("POST /api/cleanup/plans/{planID}/execute", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Holder         string `json:"holder"`
			IdempotencyKey string `json:"idempotencyKey"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		result, err := service.ExecuteCleanup(request.Context(), request.PathValue("planID"), input.Holder, input.IdempotencyKey)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(result))
	}))
	mux.HandleFunc("GET /api/projects/{projectID}/repositories/{repositoryID}/worktrees/{worktreeID}/guidance", func(response http.ResponseWriter, request *http.Request) {
		report, err := service.Guidance(request.Context(), request.PathValue("projectID"), request.PathValue("repositoryID"), request.PathValue("worktreeID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(report))
	})
	mux.HandleFunc("POST /api/handoffs/preview", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input HandoffInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		preview, err := service.PrepareHandoff(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(preview))
	}))
	mux.HandleFunc("POST /api/handoffs/launch", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input HandoffLaunchInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		launch, err := service.LaunchHandoff(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(launch))
	}))
	mux.HandleFunc("GET /api/environment", func(response http.ResponseWriter, request *http.Request) {
		health, err := service.EnvironmentHealth(request.Context(), false)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(health))
	})
	mux.HandleFunc("POST /api/environment/doctor", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		health, err := service.EnvironmentHealth(request.Context(), true)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(health))
	}))
	mux.HandleFunc("GET /api/agent-profiles", func(response http.ResponseWriter, request *http.Request) {
		profiles, err := service.AgentProfiles(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(profiles))
	})
	mux.HandleFunc("GET /api/agent-profiles/{profileID}", func(response http.ResponseWriter, request *http.Request) {
		profile, err := service.AgentProfile(request.Context(), request.PathValue("profileID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(profile))
	})
	mux.HandleFunc("GET /api/actions/plans/{planID}", func(response http.ResponseWriter, request *http.Request) {
		status, err := service.ActionStatus(request.Context(), request.PathValue("planID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(status))
	})
	mux.HandleFunc("GET /api/actions/plans", func(response http.ResponseWriter, request *http.Request) {
		plans, err := service.ActionPlans(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(plans))
	})
	mux.HandleFunc("GET /api/actions/plans/{planID}/runs", func(response http.ResponseWriter, request *http.Request) {
		runs, err := service.ActionRuns(request.Context(), request.PathValue("planID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(runs))
	})
	mux.HandleFunc("GET /api/failures/fingerprints", func(response http.ResponseWriter, request *http.Request) {
		limit := 100
		if value, err := strconv.Atoi(request.URL.Query().Get("limit")); err == nil && value >= 0 && value <= 1000 {
			limit = value
		}
		items, err := service.FailureFingerprints(request.Context(), limit)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("GET /api/safeguards/proposals", func(response http.ResponseWriter, request *http.Request) {
		limit := 100
		if value, err := strconv.Atoi(request.URL.Query().Get("limit")); err == nil && value >= 0 && value <= 1000 {
			limit = value
		}
		items, err := service.Safeguards(request.Context(), limit)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		proposals := make([]domain.SafeguardRule, 0, len(items))
		for _, item := range items {
			if item.Spec.State == domain.SafeguardProposal {
				proposals = append(proposals, item)
			}
		}
		writeEnvelope(response, http.StatusOK, contract.Success(proposals))
	})
	mux.HandleFunc("GET /api/safeguards/rules", func(response http.ResponseWriter, request *http.Request) {
		limit := 100
		if value, err := strconv.Atoi(request.URL.Query().Get("limit")); err == nil && value >= 0 && value <= 1000 {
			limit = value
		}
		items, err := service.Safeguards(request.Context(), limit)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("GET /api/safeguards/rules/{ruleID}", func(response http.ResponseWriter, request *http.Request) {
		rule, err := service.Safeguard(request.Context(), request.PathValue("ruleID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(rule))
	})
	// This is intentionally a UI ceremony route, not an automation API. The
	// native prompt derives all decision data from the persisted plan.
	mux.HandleFunc("POST /ui/actions/plans/{planID}/approval", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := requireEmptyBody(request); err != nil {
			writeServiceError(response, contract.InvalidInput("approval ceremony accepts an empty body only"))
			return
		}
		result, err := service.StartHumanApprovalCeremony(request.Context(), request.PathValue("planID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(result))
	}))
	mux.HandleFunc("POST /ui/safeguards/{ruleID}/shadow", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Owner string `json:"owner"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		rule, err := service.ReviewSafeguard(request.Context(), request.PathValue("ruleID"), input.Owner)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(rule))
	}))
	mux.HandleFunc("POST /ui/safeguards/{ruleID}/feedback", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Feedback domain.SafeguardFeedback `json:"feedback"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		rule, err := service.FeedbackSafeguard(request.Context(), request.PathValue("ruleID"), input.Feedback)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(rule))
	}))
	mux.HandleFunc("POST /ui/safeguards/{ruleID}/activate", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := requireEmptyBody(request); err != nil {
			writeServiceError(response, contract.InvalidInput("safeguard activation accepts an empty body only"))
			return
		}
		rule, err := service.ActivateSafeguard(request.Context(), request.PathValue("ruleID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(rule))
	}))
	mux.HandleFunc("POST /ui/safeguards/{ruleID}/rollback", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := requireEmptyBody(request); err != nil {
			writeServiceError(response, contract.InvalidInput("safeguard rollback accepts an empty body only"))
			return
		}
		rule, err := service.RollbackSafeguard(request.Context(), request.PathValue("ruleID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(rule))
	}))
	mux.HandleFunc("POST /ui/safeguards/{ruleID}/retire", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := requireEmptyBody(request); err != nil {
			writeServiceError(response, contract.InvalidInput("safeguard retirement accepts an empty body only"))
			return
		}
		rule, err := service.RetireSafeguard(request.Context(), request.PathValue("ruleID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(rule))
	}))
	mux.HandleFunc("POST /api/scan", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := service.QueueScan(request.Context(), "manual"); err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusAccepted, contract.Success(map[string]string{"status": "queued"}))
	}))
	mux.HandleFunc("POST /api/folder-picker", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		path, err := service.PickDirectory(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(map[string]any{"selected": path != "", "path": path}))
	}))
	mux.HandleFunc("POST /api/projects/discover", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Path string `json:"path"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		items, err := service.DiscoverRepositories(request.Context(), input.Path)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	}))
	mux.HandleFunc("POST /api/actions/plans", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input ActionPlanInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		plan, err := service.PlanAction(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(plan))
	}))
	mux.HandleFunc("POST /api/projects/{projectID}/repository-sync/plan", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := requireEmptyBody(request); err != nil {
			writeServiceError(response, contract.InvalidInput("repository sync planning accepts an empty body only"))
			return
		}
		plan, err := service.RepositorySyncPlan(request.Context(), request.PathValue("projectID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(plan))
	}))
	mux.HandleFunc("POST /api/projects/{projectID}/repository-sync/execute", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			PlanIDs   []string `json:"planIds"`
			RequestID string   `json:"requestId"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		result, err := service.ExecuteRepositorySync(request.Context(), ExecuteRepositorySyncInput{ProjectID: request.PathValue("projectID"), PlanIDs: input.PlanIDs, RequestID: input.RequestID})
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(result))
	}))
	mux.HandleFunc("POST /api/actions/plans/{planID}/admit", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Holder         string `json:"holder"`
			IdempotencyKey string `json:"idempotencyKey"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		admission, err := service.AdmitAction(request.Context(), request.PathValue("planID"), input.Holder, input.IdempotencyKey)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(admission))
	}))
	mux.HandleFunc("POST /api/actions/plans/{planID}/execute", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Holder         string `json:"holder"`
			IdempotencyKey string `json:"idempotencyKey"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		run, err := service.ExecuteAction(request.Context(), request.PathValue("planID"), input.Holder, input.IdempotencyKey)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(run))
	}))
	mux.HandleFunc("POST /ui/actions/plans/{planID}/worktree-trust", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := requireEmptyBody(request); err != nil {
			writeServiceError(response, contract.InvalidInput("Worktree execution trust accepts an empty body only"))
			return
		}
		trust, err := service.TrustActionWorktree(request.Context(), request.PathValue("planID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(trust))
	}))
	mux.HandleFunc("POST /api/projects/{projectID}/repositories/{repositoryID}/worktrees/{worktreeID}/discover", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		item, err := service.Discover(request.Context(), request.PathValue("projectID"), request.PathValue("repositoryID"), request.PathValue("worktreeID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(item))
	}))
	mux.HandleFunc("POST /api/proposals/{proposalID}/apply", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		item, err := service.ApplyProposal(request.Context(), request.PathValue("proposalID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	}))
	mux.HandleFunc("POST /api/proposals/{proposalID}/reject", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		item, err := service.RejectProposal(request.Context(), request.PathValue("proposalID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	}))
	mux.HandleFunc("POST /api/checksets", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input CreateChecksetInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.CreateCheckset(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(item))
	}))
	mux.HandleFunc("POST /api/checksets/{checksetID}/apply", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		item, err := service.ApplyCheckset(request.Context(), request.PathValue("checksetID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	}))
	mux.HandleFunc("POST /api/checksets/{checksetID}/run", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		item, err := service.RunCheckset(request.Context(), request.PathValue("checksetID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	}))
	mux.HandleFunc("POST /api/projects", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Name  string   `json:"name"`
			Path  string   `json:"path"`
			Paths []string `json:"paths"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		var project domain.Project
		var err error
		if len(input.Paths) > 0 {
			project, err = service.AddProjectTree(request.Context(), AddProjectTreeInput{Name: input.Name, Root: input.Path, Paths: input.Paths})
		} else {
			project, err = service.AddProject(request.Context(), AddProjectInput{Name: input.Name, Path: input.Path})
		}
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(project))
	}))
	mux.HandleFunc("POST /api/agent-profiles", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			ID, Name, Command     string
			VersionProbe          []string                 `json:"versionProbe"`
			TimeoutSeconds        int                      `json:"timeoutSeconds"`
			ModelArgumentTemplate string                   `json:"modelArgumentTemplate"`
			EnvironmentAllowlist  []string                 `json:"environmentAllowlist"`
			LaunchMode            domain.AgentLaunchMode   `json:"launchMode"`
			DataBoundary          domain.AgentDataBoundary `json:"dataBoundary"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		profile, err := service.AddAgentProfile(request.Context(), AddAgentProfileInput{ID: input.ID, Name: input.Name, Command: input.Command, VersionProbe: input.VersionProbe, TimeoutSeconds: input.TimeoutSeconds, ModelArgumentTemplate: input.ModelArgumentTemplate, EnvironmentAllowlist: input.EnvironmentAllowlist, LaunchMode: input.LaunchMode, DataBoundary: input.DataBoundary})
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(profile))
	}))
	mux.HandleFunc("PUT /api/agent-profiles/{profileID}", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Name, Command         string
			VersionProbe          []string                 `json:"versionProbe"`
			TimeoutSeconds        int                      `json:"timeoutSeconds"`
			ModelArgumentTemplate *string                  `json:"modelArgumentTemplate"`
			EnvironmentAllowlist  []string                 `json:"environmentAllowlist"`
			LaunchMode            domain.AgentLaunchMode   `json:"launchMode"`
			DataBoundary          domain.AgentDataBoundary `json:"dataBoundary"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		profile, err := service.UpdateAgentProfile(request.Context(), request.PathValue("profileID"), UpdateAgentProfileInput{Name: input.Name, Command: input.Command, VersionProbe: input.VersionProbe, TimeoutSeconds: input.TimeoutSeconds, ModelArgumentTemplate: input.ModelArgumentTemplate, EnvironmentAllowlist: input.EnvironmentAllowlist, LaunchMode: input.LaunchMode, DataBoundary: input.DataBoundary})
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(profile))
	}))
	mux.HandleFunc("DELETE /api/agent-profiles/{profileID}", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := service.RemoveAgentProfile(request.Context(), request.PathValue("profileID")); err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(map[string]bool{"removed": true}))
	}))
	mux.HandleFunc("POST /api/projects/import", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		data, err := readBody(response, request)
		if err != nil {
			writeServiceError(response, contract.InvalidInput("invalid project export"))
			return
		}
		project, err := service.ImportProject(request.Context(), data)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(project))
	}))
	mux.HandleFunc("POST /api/projects/{projectID}/repositories", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Path string `json:"path"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		repository, err := service.AddRepository(request.Context(), AddRepositoryInput{ProjectID: request.PathValue("projectID"), ID: input.ID, Name: input.Name, Path: input.Path})
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(repository))
	}))
	mux.HandleFunc("PUT /api/projects/{projectID}", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Name string `json:"name"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		project, err := service.UpdateProject(request.Context(), request.PathValue("projectID"), UpdateProjectInput{Name: input.Name})
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(project))
	}))
	mux.HandleFunc("PUT /api/projects/{projectID}/repositories/{repositoryID}", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		repository, err := service.UpdateRepository(request.Context(), request.PathValue("projectID"), request.PathValue("repositoryID"), UpdateRepositoryInput{Name: input.Name, Path: input.Path})
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(repository))
	}))
	mux.HandleFunc("DELETE /api/projects/{projectID}/repositories/{repositoryID}", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := service.RemoveRepository(request.Context(), request.PathValue("projectID"), request.PathValue("repositoryID")); err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(map[string]bool{"removed": true}))
	}))
	mux.HandleFunc("DELETE /api/projects/{projectID}", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := service.RemoveProject(request.Context(), request.PathValue("projectID")); err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(map[string]bool{"removed": true}))
	}))
	mux.HandleFunc("POST /api/findings/{findingID}/acknowledge", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := service.AcknowledgeFinding(request.Context(), request.PathValue("findingID")); err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(map[string]string{"status": "acknowledged"}))
	}))
	mux.HandleFunc("GET /api/assurance/providers", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.ProviderStatuses(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("GET /api/assurance/approval-scopes", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.UnattendedApprovalScopes(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("GET /api/assurance/approval-scopes/{scopeID}", func(response http.ResponseWriter, request *http.Request) {
		item, err := service.UnattendedApprovalScope(request.Context(), request.PathValue("scopeID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	})
	mux.HandleFunc("GET /api/assurance/action-plans/{planID}/approval-scope", func(response http.ResponseWriter, request *http.Request) {
		item, err := service.CheckUnattendedApprovalScope(request.Context(), request.PathValue("planID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	})
	mux.HandleFunc("POST /api/assurance/approval-scopes", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input UnattendedApprovalScopeInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.CreateUnattendedApprovalScope(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(item))
	}))
	mux.HandleFunc("POST /api/assurance/approval-scopes/{scopeID}/approve", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := requireEmptyBody(request); err != nil {
			writeServiceError(response, contract.InvalidInput("approval scope approval accepts an empty body only"))
			return
		}
		item, err := service.ApproveUnattendedApprovalScope(request.Context(), request.PathValue("scopeID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	}))
	mux.HandleFunc("POST /api/assurance/approval-scopes/{scopeID}/revoke", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := requireEmptyBody(request); err != nil {
			writeServiceError(response, contract.InvalidInput("approval scope revocation accepts an empty body only"))
			return
		}
		item, err := service.RevokeUnattendedApprovalScope(request.Context(), request.PathValue("scopeID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	}))
	mux.HandleFunc("GET /api/assurance/sessions", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.AssuranceSessions(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("GET /api/assurance/sessions/{sessionID}", func(response http.ResponseWriter, request *http.Request) {
		item, err := service.AssuranceSession(request.Context(), request.PathValue("sessionID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	})
	mux.HandleFunc("GET /api/assurance/sessions/{sessionID}/questions", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.AssuranceQuestions(request.Context(), request.PathValue("sessionID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("GET /api/assurance/sessions/{sessionID}/specs", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.AssuranceSpecs(request.Context(), request.PathValue("sessionID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("GET /api/assurance/sessions/{sessionID}/proposals", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.AssuranceProposals(request.Context(), request.PathValue("sessionID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("POST /api/assurance/sessions", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input AssuranceSessionInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.CreateAssuranceSession(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(item))
	}))
	mux.HandleFunc("POST /api/assurance/sessions/{sessionID}/questions/{questionID}/answer", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Answer string `json:"answer"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.AnswerAssuranceQuestion(request.Context(), request.PathValue("sessionID"), request.PathValue("questionID"), input.Answer)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	}))
	mux.HandleFunc("POST /api/assurance/questions", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input AssuranceQuestionInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.CreateAssuranceQuestion(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(item))
	}))
	mux.HandleFunc("POST /api/assurance/specs", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input AssuranceSpecInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.CreateAssuranceSpec(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(item))
	}))
	mux.HandleFunc("POST /api/assurance/proposals", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input AssuranceProposalInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.CreateAssuranceProposal(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(item))
	}))
	mux.HandleFunc("POST /api/assurance/proposals/{proposalID}/review", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Decision string `json:"decision"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.ReviewAssuranceProposal(request.Context(), request.PathValue("proposalID"), input.Decision)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	}))
	mux.HandleFunc("GET /api/assurance/baselines", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.PRCIBaselines(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("POST /api/assurance/baselines", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input BaselineInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.CreatePRCIBaseline(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(item))
	}))
	mux.HandleFunc("GET /api/assurance/campaigns", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.QualityCampaigns(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("POST /api/assurance/campaigns", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input QualityCampaignInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.CreateQualityCampaign(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(item))
	}))
	mux.HandleFunc("GET /api/assurance/runs", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.QualityRuns(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("GET /api/assurance/invocations", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.AgentInvocations(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("POST /api/assurance/invocations", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input AgentInvocationInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.RunAgentInvocation(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(item))
	}))
	mux.HandleFunc("POST /api/assurance/runs", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input QualityRunInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.RunQuality(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(item))
	}))
	mux.HandleFunc("GET /api/assurance/artifacts", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.AssuranceArtifacts(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("GET /api/assurance/artifacts/{artifactID}", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.AssuranceArtifacts(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		for _, item := range items {
			if item.Metadata.ID == request.PathValue("artifactID") {
				// The detail endpoint is safe to share; local storage paths stay
				// inside the operator-only artifact listing and never enter trace
				// or report responses.
				writeEnvelope(response, http.StatusOK, contract.Success(assuranceArtifactRef(item)))
				return
			}
		}
		writeServiceError(response, contract.NotFound("assurance artifact not found"))
	})
	mux.HandleFunc("POST /api/assurance/artifacts/export", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			IDs         []string `json:"ids"`
			Destination string   `json:"destination"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.ExportAssuranceArtifacts(request.Context(), input.IDs, input.Destination)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	}))
	mux.HandleFunc("POST /api/assurance/artifacts/{artifactID}/delete", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Confirmation string `json:"confirmation"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.DeleteAssuranceArtifact(request.Context(), request.PathValue("artifactID"), input.Confirmation)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	}))
	mux.HandleFunc("POST /api/assurance/artifacts/{artifactID}/retention", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Retention string `json:"retention"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.SetAssuranceArtifactRetention(request.Context(), request.PathValue("artifactID"), input.Retention)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	}))
	mux.HandleFunc("POST /api/assurance/artifacts/{artifactID}/restore", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := requireEmptyBody(request); err != nil {
			writeServiceError(response, contract.InvalidInput("restore request body must be empty"))
			return
		}
		item, err := service.RestoreAssuranceArtifact(request.Context(), request.PathValue("artifactID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	}))
	mux.HandleFunc("GET /api/assurance/effects", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.AssuranceEffects(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("POST /api/assurance/effects", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input EffectInput
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.CreateEffect(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(item))
	}))
	mux.HandleFunc("GET /api/assurance/pricing", func(response http.ResponseWriter, request *http.Request) {
		items, err := service.PricingSnapshots(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
	})
	mux.HandleFunc("GET /api/assurance/dashboard", func(response http.ResponseWriter, request *http.Request) {
		item, err := service.AssuranceDashboard(request.Context(), request.URL.Query().Get("provider"), request.URL.Query().Get("model"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	})
	mux.HandleFunc("GET /api/assurance/impact", func(response http.ResponseWriter, request *http.Request) {
		days, err := assuranceImpactDays(request.URL.Query().Get("days"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		item, err := service.AssuranceImpact(request.Context(), AssuranceImpactQuery{Provider: request.URL.Query().Get("provider"), Model: request.URL.Query().Get("model"), ProjectID: request.URL.Query().Get("project"), Days: days})
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	})
	mux.HandleFunc("GET /api/assurance/traces/{effectID}", func(response http.ResponseWriter, request *http.Request) {
		item, err := service.AssuranceTrace(request.Context(), request.PathValue("effectID"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	})
	mux.HandleFunc("GET /api/assurance/artifacts/storage", func(response http.ResponseWriter, request *http.Request) {
		item, err := service.AssuranceArtifactStorage(request.Context())
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(item))
	})
	mux.HandleFunc("GET /api/assurance/impact/export", func(response http.ResponseWriter, request *http.Request) {
		days, err := assuranceImpactDays(request.URL.Query().Get("days"))
		if err != nil {
			writeServiceError(response, err)
			return
		}
		export, err := service.ExportAssuranceReport(request.Context(), AssuranceReportQuery{Format: request.URL.Query().Get("format"), Provider: request.URL.Query().Get("provider"), Model: request.URL.Query().Get("model"), ProjectID: request.URL.Query().Get("project"), Days: days})
		if err != nil {
			writeServiceError(response, err)
			return
		}
		response.Header().Set("Content-Type", export.ContentType)
		response.Header().Set("Content-Disposition", `attachment; filename="`+export.Filename+`"`)
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(export.Body)
	})
	mux.HandleFunc("POST /api/assurance/pricing", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input domain.ProviderPricingSnapshot
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		item, err := service.SavePricingSnapshot(request.Context(), input)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(item))
	}))
	return requestLog(mux)
}

func requireEmptyBody(request *http.Request) error {
	if request.Body == nil {
		return nil
	}
	var byteValue [1]byte
	_, err := request.Body.Read(byteValue[:])
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return io.ErrUnexpectedEOF
	}
	return err
}

func assuranceImpactDays(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days < 1 || days > maxImpactPeriodDays {
		return 0, contract.InvalidInput("impact days must be between 1 and 365")
	}
	return days, nil
}

func decodeBody(response http.ResponseWriter, request *http.Request, target any) error {
	return json.NewDecoder(http.MaxBytesReader(response, request.Body, 64<<10)).Decode(target)
}

func readBody(response http.ResponseWriter, request *http.Request) ([]byte, error) {
	var raw json.RawMessage
	if err := decodeBody(response, request, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func protected(token, listen string, next http.HandlerFunc) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Control-Room-Token") != token {
			writeServiceError(response, contract.Forbidden("missing or invalid local mutation token"))
			return
		}
		origin := request.Header.Get("Origin")
		if origin != "" && origin != "http://"+listen {
			writeServiceError(response, contract.Forbidden("cross-origin mutation rejected"))
			return
		}
		next(response, request)
	}
}

func writeEnvelope[T any](response http.ResponseWriter, status int, value contract.Envelope[T]) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func writeServiceError(response http.ResponseWriter, err error) {
	classified := contract.Classify(err)
	status := http.StatusInternalServerError
	switch classified.Code {
	case contract.ErrorInvalidInput:
		status = http.StatusBadRequest
	case contract.ErrorNotFound:
		status = http.StatusNotFound
	case contract.ErrorConflict:
		status = http.StatusConflict
	case contract.ErrorForbidden, contract.ErrorPolicyDenied:
		status = http.StatusForbidden
	case contract.ErrorUnavailable:
		status = http.StatusServiceUnavailable
	}
	writeEnvelope(response, status, contract.Failure[map[string]any](classified.Code, classified.Message, classified.Details))
}
