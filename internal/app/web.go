package app

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

func newHTTPHandler(service ApplicationService, listen, mutationToken string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = strings.NewReader(strings.ReplaceAll(indexHTML, "__MUTATION_TOKEN__", mutationToken)).WriteTo(response)
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
	mux.HandleFunc("POST /api/scan", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := service.QueueScan(request.Context(), "manual"); err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusAccepted, contract.Success(map[string]string{"status": "queued"}))
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
			Name string `json:"name"`
			Path string `json:"path"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		project, err := service.AddProject(request.Context(), AddProjectInput{Name: input.Name, Path: input.Path})
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(project))
	}))
	mux.HandleFunc("POST /api/agent-profiles", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			ID, Name, Command    string
			VersionProbe         []string                 `json:"versionProbe"`
			TimeoutSeconds       int                      `json:"timeoutSeconds"`
			EnvironmentAllowlist []string                 `json:"environmentAllowlist"`
			LaunchMode           domain.AgentLaunchMode   `json:"launchMode"`
			DataBoundary         domain.AgentDataBoundary `json:"dataBoundary"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		profile, err := service.AddAgentProfile(request.Context(), AddAgentProfileInput{ID: input.ID, Name: input.Name, Command: input.Command, VersionProbe: input.VersionProbe, TimeoutSeconds: input.TimeoutSeconds, EnvironmentAllowlist: input.EnvironmentAllowlist, LaunchMode: input.LaunchMode, DataBoundary: input.DataBoundary})
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusCreated, contract.Success(profile))
	}))
	mux.HandleFunc("PUT /api/agent-profiles/{profileID}", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Name, Command        string
			VersionProbe         []string                 `json:"versionProbe"`
			TimeoutSeconds       int                      `json:"timeoutSeconds"`
			EnvironmentAllowlist []string                 `json:"environmentAllowlist"`
			LaunchMode           domain.AgentLaunchMode   `json:"launchMode"`
			DataBoundary         domain.AgentDataBoundary `json:"dataBoundary"`
		}
		if err := decodeBody(response, request, &input); err != nil {
			writeServiceError(response, contract.InvalidInput("invalid JSON body"))
			return
		}
		profile, err := service.UpdateAgentProfile(request.Context(), request.PathValue("profileID"), UpdateAgentProfileInput{Name: input.Name, Command: input.Command, VersionProbe: input.VersionProbe, TimeoutSeconds: input.TimeoutSeconds, EnvironmentAllowlist: input.EnvironmentAllowlist, LaunchMode: input.LaunchMode, DataBoundary: input.DataBoundary})
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

const indexHTML = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="control-room-token" content="__MUTATION_TOKEN__"><title>Dev Control Room</title>
<style>:root{color-scheme:dark;font-family:Inter,"Segoe UI",sans-serif;background:#0b0d10;color:#e7eaee}*{box-sizing:border-box}body{margin:0}header{display:flex;justify-content:space-between;align-items:center;padding:20px 28px;border-bottom:1px solid #232832}h1{margin:0;font-size:20px}main{max-width:1220px;margin:0 auto;padding:28px}.grid{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin-bottom:22px}.card,section{background:#12161c;border:1px solid #242b35;border-radius:12px}.card{padding:18px}.metric{font-size:28px;font-weight:700;margin-top:6px}.muted{color:#98a2b3;font-size:13px}section{margin-top:16px;padding:18px}section h2{margin:0 0 14px;font-size:16px}button,input{border-radius:8px;border:1px solid #303846;background:#181d25;color:#e7eaee;padding:9px 12px}button{cursor:pointer}button.primary{background:#2667ff;border-color:#2667ff}form{display:flex;gap:8px;flex-wrap:wrap}input{min-width:220px;flex:1}table{width:100%;border-collapse:collapse}th,td{text-align:left;padding:11px 8px;border-bottom:1px solid #242b35;font-size:13px}.ok{color:#62d49a}.warn{color:#ffbe5c}.bad{color:#ff7185}.finding{padding:10px 0;border-bottom:1px solid #242b35}.finding:last-child{border:0}@media(max-width:760px){.grid{grid-template-columns:repeat(2,1fr)}main{padding:16px}}</style></head>
<body><header><div><h1>Dev Control Room</h1><div class="muted">Local-only project health and evidence</div></div><button id="scan" class="primary">Scan now</button></header><main>
<div class="grid"><div class="card"><div class="muted">Projects</div><div id="m-projects" class="metric">0</div></div><div class="card"><div class="muted">Repositories</div><div id="m-repos" class="metric">0</div></div><div class="card"><div class="muted">Open findings</div><div id="m-findings" class="metric">0</div></div><div class="card"><div class="muted">Last scan</div><div id="m-scan" class="metric" style="font-size:15px">Never</div></div></div>
<section><h2>Register project</h2><form id="add-form"><input id="name" placeholder="Project name"><input id="path" placeholder="Registered Git repository path" required><button class="primary">Add</button></form><div class="muted" style="margin-top:9px">Only paths registered here are ever passed to the bounded Git collector.</div></section>
<section><h2>Projects and repositories</h2><div id="projects" class="muted">Loading…</div></section><section><h2>Environment Health</h2><button id="env-doctor" type="button">Run environment doctor</button><div id="environment" class="muted">Loading…</div></section><section><h2>Findings and evidence</h2><div id="findings" class="muted">Loading…</div></section><section><h2>Recent activity</h2><div id="events" class="muted">Loading…</div></section></main>
<script>const token=document.querySelector('meta[name="control-room-token"]').content;const headers={'Content-Type':'application/json','X-Control-Room-Token':token};const esc=v=>String(v??'').replace(/[&<>'"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]));async function req(p,o={}){const r=await fetch(p,o),b=await r.json();if(!r.ok||!b.ok)throw Error(b.error?.message||r.statusText);return b.data}async function refresh(){const[s,f,e,h]=await Promise.all([req('/api/state'),req('/api/findings'),req('/api/events'),req('/api/environment')]);const ps=s.projects||[],rs=ps.flatMap(p=>(p.repos||[]).map(r=>({...r,project:p.name,projectID:p.id}))),open=f.filter(x=>x.spec.state==='open'||x.spec.state==='acknowledged');document.getElementById('m-projects').textContent=ps.length;document.getElementById('m-repos').textContent=rs.length;document.getElementById('m-findings').textContent=open.length;document.getElementById('m-scan').textContent=s.generated_at?new Date(s.generated_at).toLocaleString():'Never';document.getElementById('projects').innerHTML=ps.length?'<table><thead><tr><th>Project</th><th>Repository</th><th>Branch</th><th>Status</th><th>Remote</th></tr></thead><tbody>'+rs.map(r=>'<tr><td>'+esc(r.project)+'<br><code>'+esc(r.projectID)+'</code></td><td><code>'+esc(r.id)+'</code><br><code>'+esc(r.path)+'</code></td><td>'+esc(r.branch||'detached')+'</td><td class="'+(r.error||r.unsafe_cleanup?'bad':r.dirty||r.behind?'warn':'ok')+'">'+esc(r.error||((r.dirty?'dirty ':'')+(r.behind?'behind':'clean'))||'clean')+'</td><td>'+esc(r.origin||'missing')+'</td></tr><tr><td></td><td colspan=4><details><summary>'+esc(String((r.worktrees||[]).length))+' worktrees</summary>'+((r.worktrees||[]).map(w=>'<div><code>'+esc(w.metadata.id)+'</code> · '+esc(w.spec.canonicalPath)+' · '+esc(w.spec.head||'unavailable')+' · '+esc(w.spec.branch||'detached')+' · '+esc(w.spec.trust)+' · '+esc(w.spec.dirty?'dirty':'clean')+' · '+esc(w.spec.untracked?'untracked':'tracked')+' · '+esc(w.spec.upstream||'no upstream')+' '+esc(String(w.spec.ahead||0))+'/'+esc(String(w.spec.behind||0))+' · '+esc(w.spec.locked?'locked':'unlocked')+' · '+esc(w.spec.prunable?'prunable':'present')+' · '+esc(w.spec.tombstonedAt?'tombstoned':(w.spec.error||'current'))+'</div>').join('')||'No worktree details')+'</details></td></tr>').join('')+'</tbody></table>':'No registered projects';document.getElementById('environment').innerHTML='<div class="'+(h.available?'ok':'warn')+'">'+(h.available?'All configured capabilities available':'Some capabilities are unavailable; see next actions below')+'</div>'+(h.findings||[]).map(x=>'<div class="finding"><strong>'+esc(x.severity)+' · '+esc(x.target||x.type)+'</strong><div>'+esc(x.summary)+'</div><div class="muted">Next: '+esc(x.recommendedNextAction)+'</div></div>').join('')||'<div class="ok">No environment findings</div>';document.getElementById('findings').innerHTML=open.length?open.map(x=>'<div class="finding"><strong>'+esc(x.spec.severity)+' · '+esc(x.spec.type)+'</strong><div>'+esc(x.spec.summary)+'</div><div class="muted">Evidence: '+esc((x.spec.evidenceRefs||[]).join(', ')||'none')+' · Next: '+esc(x.spec.recommendedNextAction)+'</div></div>').join(''):'No open findings';document.getElementById('events').innerHTML=e.length?'<table><tbody>'+e.slice().reverse().map(x=>'<tr><td>'+new Date(x.spec.occurredAt).toLocaleString()+'</td><td>'+esc(x.spec.type)+'</td><td>'+esc(x.spec.summary)+'</td></tr>').join('')+'</tbody></table>':'No events yet'}document.getElementById('scan').onclick=async()=>{await req('/api/scan',{method:'POST',headers});setTimeout(refresh,500)};document.getElementById('env-doctor').onclick=async()=>{const b=document.getElementById('env-doctor');b.disabled=true;try{await req('/api/environment/doctor',{method:'POST',headers});await refresh()}catch(e){alert(e.message)}finally{b.disabled=false}};document.getElementById('add-form').onsubmit=async ev=>{ev.preventDefault();try{await req('/api/projects',{method:'POST',headers,body:JSON.stringify({name:document.getElementById('name').value,path:document.getElementById('path').value})});ev.target.reset();setTimeout(refresh,300)}catch(e){alert(e.message)}};refresh().catch(e=>document.getElementById('projects').textContent=e.message);setInterval(refresh,15000);</script></body></html>`
