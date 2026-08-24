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
