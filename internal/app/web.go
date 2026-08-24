package app

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

const folderPickerScript = `<script>(function(){const box=document.getElementById('repository-candidates'),path=document.getElementById('path'),headers={'Content-Type':'application/json','X-Control-Room-Token':document.querySelector('meta[name="control-room-token"]').content};const request=async(url,options)=>{const response=await fetch(url,options),body=await response.json();if(!response.ok||!body.ok)throw Error(body.error?.message||response.statusText);return body.data};const discover=async()=>{if(!path.value.trim()){box.dataset.discovered='false';box.textContent='Choose a folder first.';return}box.textContent='Finding Git repositories…';try{const items=await request('/api/projects/discover',{method:'POST',headers,body:JSON.stringify({path:path.value})});box.dataset.discovered='true';box.innerHTML=items.length?'<strong>'+items.length+' repositories found</strong>'+items.map(item=>'<label style="display:block;margin-top:7px"><input type="checkbox" data-repository-path value="'+esc(item.path)+'" checked> '+esc(item.name)+' <code>'+esc(item.path)+'</code></label>').join(''):'No Git repositories found below this folder.'}catch(error){box.dataset.discovered='false';box.textContent=error.message}};document.getElementById('pick-folder').onclick=async()=>{try{const result=await request('/api/folder-picker',{method:'POST',headers});if(result.path){path.value=result.path;await discover()}}catch(error){alert(error.message)}};document.getElementById('find-repositories').onclick=discover;document.getElementById('add-form').onsubmit=async event=>{event.preventDefault();const selected=[...box.querySelectorAll('input[data-repository-path]:checked')].map(input=>input.value);if(box.dataset.discovered==='true'&&!selected.length){alert('Select at least one repository.');return}try{const body={name:document.getElementById('name').value,path:path.value};if(selected.length)body.paths=selected;await request('/api/projects',{method:'POST',headers,body:JSON.stringify(body)});event.target.reset();box.dataset.discovered='false';box.textContent='Choose a folder to discover Git repositories below it.';setTimeout(()=>location.reload(),300)}catch(error){alert(error.message)}};function esc(value){return String(value??'').replace(/[&<>'"]/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[char]))}})();</script>`

const environmentSourceScript = `<script>(function(){const esc=value=>String(value??'').replace(/[&<>'"]/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[char]));const source=type=>type?.startsWith('tool.')?'Tool':type?.startsWith('agent_profile.')?'Agent profile':'Configuration';const refreshEnvironment=async()=>{const response=await fetch('/api/environment'),body=await response.json();if(!response.ok||!body.ok)return;const health=body.data,box=document.getElementById('environment');if(!box)return;box.innerHTML='<div class="'+(health.available?'ok':'warn')+'">'+(health.available?'All configured capabilities available':'Some capabilities are unavailable; see next actions below')+'</div>'+(health.findings||[]).map(item=>'<div class="finding"><strong>'+esc(item.severity)+' · '+esc(source(item.type))+' · '+esc(item.target||item.type)+'</strong><div>'+esc(item.summary)+'</div><div class="muted">Next: '+esc(item.recommendedNextAction)+'</div></div>').join('')||'<div class="ok">No environment findings</div>'};refreshEnvironment().catch(()=>{});setInterval(()=>refreshEnvironment().catch(()=>{}),15000)})();</script>`

const actionUIScript = `<script>(function(){const esc=value=>String(value??'').replace(/[&<>'"]/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[char]));const headers={'Content-Type':'application/json','X-Control-Room-Token':document.querySelector('meta[name="control-room-token"]').content};const request=async(url,options={})=>{const response=await fetch(url,options),body=await response.json();if(!response.ok||!body.ok)throw Error(body.error?.message||response.statusText);return body.data};const box=document.createElement('section');box.innerHTML='<h2>Actions</h2><div id="action-ui" class="muted">Loading…</div>';document.querySelector('main').insertBefore(box,document.getElementById('findings').parentElement);const render=async()=>{const[state,plans]=await Promise.all([request('/api/state'),request('/api/actions/plans')]),targets=[];(state.projects||[]).forEach(project=>(project.repos||[]).forEach(repo=>(repo.worktrees||[]).forEach(worktree=>targets.push({value:project.id+'|'+repo.id+'|'+worktree.metadata.id,label:project.name+' / '+repo.id+' / '+worktree.metadata.id}))));const details=await Promise.all(plans.map(async plan=>({plan,status:await request('/api/actions/plans/'+encodeURIComponent(plan.metadata.id)),runs:await request('/api/actions/plans/'+encodeURIComponent(plan.metadata.id)+'/runs')})));document.getElementById('action-ui').innerHTML='<div class="picker-row"><select id="action-target">'+(targets.length?targets.map(target=>'<option value="'+esc(target.value)+'">'+esc(target.label)+'</option>').join(''):'<option>No observed worktrees</option>')+'</select><button id="action-plan" class="primary" type="button" '+(targets.length?'':'disabled')+'>Plan repository refresh</button></div>'+(details.length?details.map(item=>{const status=item.status.admission,run=item.runs[0],button=status==='approval_required'?'<button data-action="approve" data-id="'+esc(item.plan.metadata.id)+'">Ask for approval</button>':status==='eligible'?'<button data-action="execute" data-id="'+esc(item.plan.metadata.id)+'">Execute</button>':'';return '<div class="finding"><strong>'+esc(item.plan.metadata.name)+' · '+esc(item.plan.metadata.id)+'</strong><div class="muted">'+esc(status)+' · '+esc(item.plan.spec.worktreeId)+(run?' · '+esc(run.spec.status):'')+'</div>'+button+' <button data-action="runs" data-id="'+esc(item.plan.metadata.id)+'">Review result</button><div id="action-result-'+esc(item.plan.metadata.id)+'" class="muted"></div></div>'}).join(''):'<div class="muted" style="margin-top:10px">No reviewed action plans yet.</div>')};box.addEventListener('click',async event=>{const button=event.target;if(button.id==='action-plan'){const value=document.getElementById('action-target').value.split('|');try{await request('/api/actions/plans',{method:'POST',headers,body:JSON.stringify({id:'plan-'+Date.now(),name:'Repository refresh',projectId:value[0],repositoryId:value[1],worktreeId:value[2],actionType:'repository.refresh'})});await render()}catch(error){alert(error.message)}return}const kind=button.dataset.action,id=button.dataset.id;if(!kind)return;try{if(kind==='approve')await request('/ui/actions/plans/'+encodeURIComponent(id)+'/approval',{method:'POST',headers,body:''});else if(kind==='execute')await request('/api/actions/plans/'+encodeURIComponent(id)+'/execute',{method:'POST',headers,body:JSON.stringify({holder:'ui',idempotencyKey:'ui-'+Date.now()})});else{const runs=await request('/api/actions/plans/'+encodeURIComponent(id)+'/runs');document.getElementById('action-result-'+id).textContent=runs.length?runs.map(run=>run.spec.status+' · '+(run.spec.stderr||run.spec.stdout||'no output')).join(' | '):'No result yet';return}await render()}catch(error){alert(error.message)}});render().catch(error=>{document.getElementById('action-ui').textContent=error.message})})();</script>`

const actionTrustScript = `<script>(function(){const headers={'Content-Type':'application/json','X-Control-Room-Token':document.querySelector('meta[name="control-room-token"]').content};const addTrustButtons=()=>document.querySelectorAll('#action-ui button[data-action]').forEach(button=>{const parent=button.parentElement;if(parent.querySelector('button[data-trust]'))return;const trust=document.createElement('button');trust.type='button';trust.dataset.trust=button.dataset.id;trust.textContent='Mark worktree execution-ready';parent.insertBefore(trust,button)});const observer=new MutationObserver(addTrustButtons);observer.observe(document.getElementById('action-ui'),{childList:true,subtree:true});document.getElementById('action-ui').addEventListener('click',async event=>{const button=event.target;if(!button.dataset.trust)return;button.disabled=true;try{await fetch('/ui/actions/plans/'+encodeURIComponent(button.dataset.trust)+'/worktree-trust',{method:'POST',headers,body:''});button.textContent='Worktree execution-ready'}catch(error){button.disabled=false;alert(error.message)}});addTrustButtons()})();</script>`

const cleanupQueueScript = `<script>(function(){const esc=value=>String(value??'').replace(/[&<>'"]/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[char]));const box=document.createElement('section');box.innerHTML='<h2>Cleanup queue</h2><div class="muted">Read-only assessment. Every candidate stays blocked until configured merge evidence and a fresh policy review exist.</div><div id="cleanup-queue" class="muted" style="margin-top:10px">Loading…</div>';document.querySelector('main').insertBefore(box,document.getElementById('findings').parentElement);fetch('/api/cleanup/candidates').then(response=>response.json()).then(body=>{if(!body.ok)throw Error(body.error?.message||'cleanup queue unavailable');const items=body.data||[];document.getElementById('cleanup-queue').innerHTML=items.length?'<table><thead><tr><th>Target</th><th>Branch / HEAD</th><th>State</th><th>Why blocked</th></tr></thead><tbody>'+items.map(item=>'<tr><td><code>'+esc(item.spec.worktreeId)+'</code><br><span class="muted">'+esc(item.spec.canonicalPath)+'</span></td><td>'+esc(item.spec.branch||'detached')+'<br><code>'+esc(item.spec.head||'unavailable')+'</code></td><td class="warn">'+esc(item.spec.decision)+'</td><td>'+item.spec.reasons.map(esc).join('<br>')+'</td></tr>').join('')+'</tbody></table>':'No observed worktrees';}).catch(error=>{document.getElementById('cleanup-queue').textContent=error.message})})();</script>`

const guidanceHandoffScript = `<script>(function(){const esc=value=>String(value??'').replace(/[&<>'"]/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[char]));const headers={'Content-Type':'application/json','X-Control-Room-Token':document.querySelector('meta[name="control-room-token"]').content};const request=async(url,options={})=>{const response=await fetch(url,options),body=await response.json();if(!response.ok||!body.ok)throw Error(body.error?.message||response.statusText);return body.data};const box=document.createElement('section');box.innerHTML='<h2>Guidance Doctor</h2><div class="muted">Handoff preview keeps transcriptIncluded=false.</div><div id="guidance-ui" class="muted">Loading…</div>';document.querySelector('main').insertBefore(box,document.getElementById('findings').parentElement);const render=async()=>{const[state,profiles]=await Promise.all([request('/api/state'),request('/api/agent-profiles')]),targets=[];(state.projects||[]).forEach(project=>(project.repos||[]).forEach(repo=>(repo.worktrees||[]).forEach(worktree=>targets.push({value:project.id+'|'+repo.id+'|'+worktree.metadata.id,label:project.name+' / '+repo.id+' / '+worktree.metadata.id}))));const targetOptions=targets.length?targets.map(target=>'<option value="'+esc(target.value)+'">'+esc(target.label)+'</option>').join(''):'<option value="">No observed worktrees</option>';const profileOptions=profiles.length?profiles.map(profile=>'<option value="'+esc(profile.metadata.id)+'">'+esc(profile.metadata.name)+'</option>').join(''):'<option value="">No agent profiles</option>';document.getElementById('guidance-ui').innerHTML='<div class="picker-row"><select id="guidance-target">'+targetOptions+'</select><select id="handoff-profile">'+profileOptions+'</select><button id="guidance-check" type="button" '+(targets.length?'':'disabled')+'>Run Guidance Doctor</button><button id="handoff-preview" type="button" '+(targets.length&&profiles.length?'':'disabled')+'>Preview handoff</button></div><pre id="guidance-result" class="muted"></pre>'};box.addEventListener('click',async event=>{const button=event.target;if(button.id!=='guidance-check'&&button.id!=='handoff-preview')return;const target=document.getElementById('guidance-target').value.split('|');try{let result;if(button.id==='guidance-check')result=await request('/api/projects/'+encodeURIComponent(target[0])+'/repositories/'+encodeURIComponent(target[1])+'/worktrees/'+encodeURIComponent(target[2])+'/guidance');else result=await request('/api/handoffs/preview',{method:'POST',headers,body:JSON.stringify({profileId:document.getElementById('handoff-profile').value,projectId:target[0],repositoryId:target[1],worktreeId:target[2]})});document.getElementById('guidance-result').textContent=JSON.stringify(result,null,2)}catch(error){document.getElementById('guidance-result').textContent=error.message}});render().catch(error=>{document.getElementById('guidance-ui').textContent=error.message})})();</script>`

const safeguardScript = `<script>(function(){const esc=value=>String(value??'').replace(/[&<>'"]/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[char]));const box=document.createElement('section');box.innerHTML='<h2>Repeated failures</h2><div class="muted">Proposals stay in shadow mode until a human reviews masked evidence.</div><div id="safeguards" class="muted">Loading…</div>';document.querySelector('main').insertBefore(box,document.getElementById('findings').parentElement);fetch('/api/safeguards/proposals').then(response=>response.json()).then(body=>{if(!body.ok)throw Error(body.error?.message||'safeguards unavailable');const items=body.data||[];document.getElementById('safeguards').innerHTML=items.length?items.map(item=>'<div class="finding"><strong>'+esc(item.category)+' · '+esc(item.mode)+'</strong><div>'+esc(item.summary)+'</div><div class="muted">'+esc(item.occurrenceCount)+' occurrences · '+esc(item.recommendedNextAction)+'</div></div>').join(''):'No repeated-failure safeguard proposals';}).catch(error=>{document.getElementById('safeguards').textContent=error.message})})();</script>`

func newHTTPHandler(service ApplicationService, listen, mutationToken string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		page := strings.ReplaceAll(indexHTML, "__MUTATION_TOKEN__", mutationToken)
		page = strings.Replace(page, "</body>", folderPickerScript+environmentSourceScript+actionUIScript+actionTrustScript+cleanupQueueScript+guidanceHandoffScript+safeguardScript+"</body>", 1)
		_, _ = strings.NewReader(page).WriteTo(response)
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
		items, err := service.SafeguardProposals(request.Context(), limit)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(items))
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
			ModelArgumentTemplate string                   `json:"modelArgumentTemplate"`
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

const indexHTML = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="control-room-token" content="__MUTATION_TOKEN__"><title>Dev Control Room</title>
<style>:root{color-scheme:dark;font-family:Inter,"Segoe UI",sans-serif;background:#0b0d10;color:#e7eaee}*{box-sizing:border-box}body{margin:0}header{display:flex;justify-content:space-between;align-items:center;padding:20px 28px;border-bottom:1px solid #232832}h1{margin:0;font-size:20px}main{max-width:1220px;margin:0 auto;padding:28px}.grid{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin-bottom:22px}.card,section{background:#12161c;border:1px solid #242b35;border-radius:12px}.card{padding:18px}.metric{font-size:28px;font-weight:700;margin-top:6px}.muted{color:#98a2b3;font-size:13px}section{margin-top:16px;padding:18px}section h2{margin:0 0 14px;font-size:16px}button,input{border-radius:8px;border:1px solid #303846;background:#181d25;color:#e7eaee;padding:9px 12px}button{cursor:pointer}button.primary{background:#2667ff;border-color:#2667ff}form{display:flex;gap:8px;flex-wrap:wrap}.picker-row{display:flex;gap:8px;flex-wrap:wrap;width:100%}.picker-row input{min-width:280px}input{min-width:220px;flex:1}table{width:100%;border-collapse:collapse}th,td{text-align:left;padding:11px 8px;border-bottom:1px solid #242b35;font-size:13px}.ok{color:#62d49a}.warn{color:#ffbe5c}.bad{color:#ff7185}.finding{padding:10px 0;border-bottom:1px solid #242b35}.finding:last-child{border:0}@media(max-width:760px){.grid{grid-template-columns:repeat(2,1fr)}main{padding:16px}.picker-row{display:grid}.picker-row button{width:100%}}</style></head>
<body><header><div><h1>Dev Control Room</h1><div class="muted">Local-only project health and evidence</div></div><button id="scan" class="primary">Scan now</button></header><main>
<div class="grid"><div class="card"><div class="muted">Projects</div><div id="m-projects" class="metric">0</div></div><div class="card"><div class="muted">Repositories</div><div id="m-repos" class="metric">0</div></div><div class="card"><div class="muted">Open findings</div><div id="m-findings" class="metric">0</div></div><div class="card"><div class="muted">Last scan</div><div id="m-scan" class="metric" style="font-size:15px">Never</div></div></div>
<section><h2>Register project</h2><form id="add-form"><input id="name" placeholder="Project name"><div class="picker-row"><input id="path" placeholder="Choose a parent folder or Git repository" required><button id="pick-folder" type="button">Choose folder</button><button id="find-repositories" type="button">Find repositories below</button></div><div id="repository-candidates" class="muted" style="width:100%;margin-top:9px">Choose a folder to discover Git repositories below it.</div><button class="primary" type="submit">Register selected</button></form><div class="muted" style="margin-top:9px">Discovery is read-only. Only selected repository paths are registered and passed to the bounded Git collector.</div></section>
<section><h2>Projects and repositories</h2><div id="projects" class="muted">Loading…</div></section><section><h2>Pre-PR checksets</h2><div id="checksets" class="muted">Loading…</div></section><section><h2>Environment Health</h2><button id="env-doctor" type="button">Run environment doctor</button><div id="environment" class="muted">Loading…</div></section><section><h2>Findings and evidence</h2><div id="findings" class="muted">Loading…</div></section><section><h2>Recent activity</h2><div id="events" class="muted">Loading…</div></section></main>
<script>const token=document.querySelector('meta[name="control-room-token"]').content;const headers={'Content-Type':'application/json','X-Control-Room-Token':token};const esc=v=>String(v??'').replace(/[&<>'"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]));async function req(p,o={}){const r=await fetch(p,o),b=await r.json();if(!r.ok||!b.ok)throw Error(b.error?.message||r.statusText);return b.data}const checksetRow=(c,r)=>'<div class="finding"><strong>'+esc(r.project)+' · '+esc(r.id)+' · '+esc(c.metadata.name)+'</strong><div class="muted">'+esc(c.spec.state)+' · '+esc(c.spec.worktreeId)+' · '+esc(c.spec.head)+'</div><div>'+c.spec.steps.map(s=>esc(s.name)).join(', ')+'</div><button data-checkset="apply" data-id="'+esc(c.metadata.id)+'" '+(c.spec.state==='draft'?'':'disabled')+'>Apply</button> <button data-checkset="run" data-id="'+esc(c.metadata.id)+'" '+(c.spec.state==='applied'?'':'disabled')+'>Run</button> <button data-checkset="results" data-id="'+esc(c.metadata.id)+'">Review results</button><div id="runs-'+esc(c.metadata.id)+'" class="muted"></div></div>';async function refresh(){const[s,f,e,h]=await Promise.all([req('/api/state'),req('/api/findings'),req('/api/events'),req('/api/environment')]);const ps=s.projects||[],rs=ps.flatMap(p=>(p.repos||[]).map(r=>({...r,project:p.name,projectID:p.id}))),open=f.filter(x=>x.spec.state==='open'||x.spec.state==='acknowledged'),checks=await Promise.all(rs.map(async r=>({r,checksets:await req('/api/projects/'+encodeURIComponent(r.projectID)+'/repositories/'+encodeURIComponent(r.id)+'/checksets'),proposals:await req('/api/projects/'+encodeURIComponent(r.projectID)+'/repositories/'+encodeURIComponent(r.id)+'/proposals')})));document.getElementById('m-projects').textContent=ps.length;document.getElementById('m-repos').textContent=rs.length;document.getElementById('m-findings').textContent=open.length;document.getElementById('m-scan').textContent=s.generated_at?new Date(s.generated_at).toLocaleString():'Never';document.getElementById('projects').innerHTML=ps.length?'<table><thead><tr><th>Project</th><th>Repository</th><th>Branch</th><th>Status</th><th>Remote</th></tr></thead><tbody>'+rs.map(r=>'<tr><td>'+esc(r.project)+'<br><code>'+esc(r.projectID)+'</code></td><td><code>'+esc(r.id)+'</code><br><code>'+esc(r.path)+'</code></td><td>'+esc(r.branch||'detached')+'</td><td class="'+(r.error||r.unsafe_cleanup?'bad':r.dirty||r.behind?'warn':'ok')+'">'+esc(r.error||((r.dirty?'dirty ':'')+(r.behind?'behind':'clean'))||'clean')+'</td><td>'+esc(r.origin||'missing')+'</td></tr><tr><td></td><td colspan=4><details><summary>'+esc(String((r.worktrees||[]).length))+' worktrees</summary>'+((r.worktrees||[]).map(w=>'<div><code>'+esc(w.metadata.id)+'</code> · '+esc(w.spec.canonicalPath)+' · '+esc(w.spec.head||'unavailable')+' · '+esc(w.spec.branch||'detached')+' · '+esc(w.spec.trust)+' · '+esc(w.spec.dirty?'dirty':'clean')+' · '+esc(w.spec.untracked?'untracked':'tracked')+' · '+esc(w.spec.upstream||'no upstream')+' '+esc(String(w.spec.ahead||0))+'/'+esc(String(w.spec.behind||0))+' · '+esc(w.spec.locked?'locked':'unlocked')+' · '+esc(w.spec.prunable?'prunable':'present')+' · '+esc(w.spec.tombstonedAt?'tombstoned':(w.spec.error||'current'))+'</div>').join('')||'No worktree details')+'</details></td></tr>').join('')+'</tbody></table>':'No registered projects';document.getElementById('checksets').innerHTML=checks.map(({r,checksets,proposals})=>checksets.map(c=>checksetRow(c,r)).join('')+proposals.filter(p=>p.spec.state==='applied'&&p.spec.typedCommand).map(p=>'<div class="finding"><strong>'+esc(r.project)+' · '+esc(r.id)+'</strong><div>'+esc(p.metadata.name)+'</div><button data-checkset="create" data-id="'+esc(p.metadata.id)+'">Create Checkset</button></div>').join('')).join('')||'No applied discovery proposals or checksets';document.getElementById('environment').innerHTML='<div class="'+(h.available?'ok':'warn')+'">'+(h.available?'All configured capabilities available':'Some capabilities are unavailable; see next actions below')+'</div>'+(h.findings||[]).map(x=>'<div class="finding"><strong>'+esc(x.severity)+' · '+esc(x.target||x.type)+'</strong><div>'+esc(x.summary)+'</div><div class="muted">Next: '+esc(x.recommendedNextAction)+'</div></div>').join('')||'<div class="ok">No environment findings</div>';document.getElementById('findings').innerHTML=open.length?open.map(x=>'<div class="finding"><strong>'+esc(x.spec.severity)+' · '+esc(x.spec.type)+'</strong><div>'+esc(x.spec.summary)+'</div><div class="muted">Evidence: '+esc((x.spec.evidenceRefs||[]).join(', ')||'none')+' · Next: '+esc(x.spec.recommendedNextAction)+'</div></div>').join(''):'No open findings';document.getElementById('events').innerHTML=e.length?'<table><tbody>'+e.slice().reverse().map(x=>'<tr><td>'+new Date(x.spec.occurredAt).toLocaleString()+'</td><td>'+esc(x.spec.type)+'</td><td>'+esc(x.spec.summary)+'</td></tr>').join('')+'</tbody></table>':'No events yet'}document.getElementById('scan').onclick=async()=>{await req('/api/scan',{method:'POST',headers});setTimeout(refresh,500)};document.getElementById('env-doctor').onclick=async()=>{const b=document.getElementById('env-doctor');b.disabled=true;try{await req('/api/environment/doctor',{method:'POST',headers});await refresh()}catch(e){alert(e.message)}finally{b.disabled=false}};document.getElementById('add-form').onsubmit=async ev=>{ev.preventDefault();try{await req('/api/projects',{method:'POST',headers,body:JSON.stringify({name:document.getElementById('name').value,path:document.getElementById('path').value})});ev.target.reset();setTimeout(refresh,300)}catch(e){alert(e.message)}};document.getElementById('checksets').onclick=async ev=>{const b=ev.target,a=b.dataset.checkset,id=b.dataset.id;if(!a)return;try{if(a==='create'){const p=await req('/api/proposals/'+encodeURIComponent(id));await req('/api/checksets',{method:'POST',headers,body:JSON.stringify({id:'checks-'+id,name:p.metadata.name,proposalId:id,steps:[{id:'check',name:p.metadata.name,command:p.spec.typedCommand}]})})}else if(a==='apply')await req('/api/checksets/'+encodeURIComponent(id)+'/apply',{method:'POST',headers});else if(a==='run')await req('/api/checksets/'+encodeURIComponent(id)+'/run',{method:'POST',headers});else{const runs=await req('/api/checksets/'+encodeURIComponent(id)+'/runs');document.getElementById('runs-'+id).innerHTML=runs.length?runs.slice().reverse().map(r=>'<div>'+esc(r.spec.status)+' · '+esc(r.spec.completedAt||r.spec.startedAt)+' · '+r.spec.steps.map(s=>esc(s.stepId)+': '+esc(s.status)).join(', ')+'</div>').join(''):'No runs yet';return}await refresh()}catch(e){alert(e.message)}};refresh().catch(e=>document.getElementById('projects').textContent=e.message);setInterval(refresh,15000);</script></body></html>`
