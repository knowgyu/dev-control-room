package app

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/knowgyu/dev-control-room/internal/contract"
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
		writeEnvelope(response, http.StatusOK, contract.Success(service.Snapshot(request.Context())))
	})
	mux.HandleFunc("GET /api/events", func(response http.ResponseWriter, request *http.Request) {
		events, err := service.Events(request.Context(), 100)
		if err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(events))
	})
	mux.HandleFunc("POST /api/scan", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := service.QueueScan(request.Context(), "manual"); err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusAccepted, contract.Success(map[string]string{"status": "queued"}))
	}))
	mux.HandleFunc("POST /api/projects", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(response, request.Body, 32<<10)).Decode(&input); err != nil {
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
	mux.HandleFunc("DELETE /api/projects/{id}", protected(mutationToken, listen, func(response http.ResponseWriter, request *http.Request) {
		if err := service.RemoveProject(request.Context(), request.PathValue("id")); err != nil {
			writeServiceError(response, err)
			return
		}
		writeEnvelope(response, http.StatusOK, contract.Success(map[string]bool{"removed": true}))
	}))
	return requestLog(mux)
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

var _ ApplicationService = (*App)(nil)

const indexHTML = `<!doctype html>
<html lang="ko">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <meta name="control-room-token" content="__MUTATION_TOKEN__">
  <title>Dev Control Room</title>
  <style>
    :root { color-scheme: dark; font-family: Inter, "Segoe UI", sans-serif; background:#0b0d10; color:#e7eaee; }
    * { box-sizing:border-box; }
    body { margin:0; }
    header { display:flex; justify-content:space-between; align-items:center; padding:20px 28px; border-bottom:1px solid #232832; }
    h1 { margin:0; font-size:20px; }
    main { max-width:1180px; margin:0 auto; padding:28px; }
    .grid { display:grid; grid-template-columns:repeat(4,1fr); gap:12px; margin-bottom:22px; }
    .card, section { background:#12161c; border:1px solid #242b35; border-radius:12px; }
    .card { padding:18px; }
    .metric { font-size:28px; font-weight:700; margin-top:6px; }
    .muted { color:#98a2b3; font-size:13px; }
    section { margin-top:16px; padding:18px; }
    section h2 { margin:0 0 14px; font-size:16px; }
    button, input { border-radius:8px; border:1px solid #303846; background:#181d25; color:#e7eaee; padding:9px 12px; }
    button { cursor:pointer; }
    button.primary { background:#2667ff; border-color:#2667ff; }
    form { display:flex; gap:8px; flex-wrap:wrap; }
    input { min-width:220px; flex:1; }
    table { width:100%; border-collapse:collapse; }
    th, td { text-align:left; padding:11px 8px; border-bottom:1px solid #242b35; font-size:13px; }
    .ok { color:#62d49a; } .warn { color:#ffbe5c; } .bad { color:#ff7185; }
    .empty { padding:24px 0; color:#98a2b3; }
    code { color:#b8c7e8; }
    @media(max-width:760px){ .grid{grid-template-columns:repeat(2,1fr)} main{padding:16px} }
  </style>
</head>
<body>
  <header><div><h1>Dev Control Room</h1><div class="muted">Local-only engineering control plane</div></div><button id="scan" class="primary">Scan now</button></header>
  <main>
    <div class="grid">
      <div class="card"><div class="muted">Projects</div><div id="m-projects" class="metric">0</div></div>
      <div class="card"><div class="muted">Repositories</div><div id="m-repos" class="metric">0</div></div>
      <div class="card"><div class="muted">Needs attention</div><div id="m-attention" class="metric">0</div></div>
      <div class="card"><div class="muted">Last scan</div><div id="m-scan" class="metric" style="font-size:15px">Never</div></div>
    </div>

    <section>
      <h2>Add project</h2>
      <form id="add-form"><input id="name" placeholder="Name, e.g. Backend"><input id="path" placeholder="Windows repository path, e.g. C:\\work\\backend" required><button class="primary">Add</button></form>
      <div class="muted" style="margin-top:9px">Only paths added here are scanned. The prototype expects the selected path itself to be a Git worktree.</div>
    </section>

    <section><h2>Repository health</h2><div id="repos" class="empty">No projects registered.</div></section>
    <section><h2>Recent activity</h2><div id="events" class="empty">No events yet.</div></section>
  </main>
<script>
const token = document.querySelector('meta[name="control-room-token"]').content;
const headers = {'Content-Type':'application/json','X-Control-Room-Token':token};
const escapeHTML = value => String(value ?? '').replace(/[&<>'"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]));

async function request(path, options={}) {
  const response = await fetch(path, options);
  const body = await response.json();
  if (!response.ok || !body.ok) throw new Error(body.error?.message || response.statusText);
  return body.data;
}

async function refresh() {
  const [state, events] = await Promise.all([request('/api/state'), request('/api/events')]);
  const projects = state.projects || [];
  const repos = projects.flatMap(p => (p.repos || []).map(r => ({...r, project:p.name, id:p.id})));
  const attention = repos.filter(r => r.error || r.dirty || r.behind > 0).length;
  document.getElementById('m-projects').textContent = projects.length;
  document.getElementById('m-repos').textContent = repos.length;
  document.getElementById('m-attention').textContent = attention;
  document.getElementById('m-scan').textContent = state.generated_at ? new Date(state.generated_at).toLocaleString() : 'Never';
  const repoRows = repos.map(r => '<tr><td>' + escapeHTML(r.project) + '</td><td><code>' + escapeHTML(r.path) + '</code><div class="muted">' + escapeHTML(r.origin) + '</div></td><td>' + escapeHTML(r.branch || '-') + '</td><td class="' + (r.error ? 'bad' : r.dirty ? 'warn' : 'ok') + '">' + escapeHTML(r.error || (r.dirty ? 'Dirty' : 'Clean')) + '</td><td>' + (r.ahead || 0) + ' ahead / ' + (r.behind || 0) + ' behind</td><td><button onclick="removeProject(\'' + escapeHTML(r.id) + '\')">Remove</button></td></tr>').join('');
  document.getElementById('repos').innerHTML = repos.length ? '<table><thead><tr><th>Project</th><th>Repository</th><th>Branch</th><th>Status</th><th>Upstream</th><th></th></tr></thead><tbody>' + repoRows + '</tbody></table>' : '<div class="empty">No projects registered.</div>';
  const eventRows = events.slice().reverse().map(e => '<tr><td>' + new Date(e.spec.occurredAt).toLocaleString() + '</td><td>' + escapeHTML(e.spec.type) + '</td><td>' + escapeHTML(e.spec.summary) + '</td></tr>').join('');
  document.getElementById('events').innerHTML = events.length ? '<table><tbody>' + eventRows + '</tbody></table>' : '<div class="empty">No events yet.</div>';
}

document.getElementById('scan').onclick = async () => { await request('/api/scan',{method:'POST',headers}); setTimeout(refresh,600); };
document.getElementById('add-form').onsubmit = async event => { event.preventDefault(); try { await request('/api/projects',{method:'POST',headers,body:JSON.stringify({name:document.getElementById('name').value,path:document.getElementById('path').value})}); event.target.reset(); setTimeout(refresh,600); } catch(error) { alert(error.message); } };
async function removeProject(id) { if (!confirm('Remove this project from Control Room? Repository files will not be changed.')) return; await request('/api/projects/'+encodeURIComponent(id),{method:'DELETE',headers}); setTimeout(refresh,300); }
refresh(); setInterval(refresh,15000);
</script>
</body>
</html>`
