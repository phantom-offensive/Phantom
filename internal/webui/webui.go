package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/phantom-c2/phantom/internal/protocol"
	"github.com/phantom-c2/phantom/internal/server"
	"github.com/phantom-c2/phantom/internal/util"
)

// WebUI serves a browser-based dashboard for Phantom C2.
// It runs alongside the CLI — both can be used simultaneously.
type WebUI struct {
	server   *server.Server
	bindAddr string
}

// New creates a new WebUI instance.
func New(srv *server.Server, bindAddr string) *WebUI {
	return &WebUI{server: srv, bindAddr: bindAddr}
}

// Start launches the web UI HTTP server.
func (w *WebUI) Start() error {
	mux := http.NewServeMux()

	// Dashboard page
	mux.HandleFunc("/", w.handleDashboard)

	// API endpoints
	mux.HandleFunc("/api/agents", w.handleAPIAgents)
	mux.HandleFunc("/api/listeners", w.handleAPIListeners)
	mux.HandleFunc("/api/tasks", w.handleAPITasks)
	mux.HandleFunc("/api/events", w.handleAPIEvents)

	httpServer := &http.Server{
		Addr:         w.bindAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return httpServer.ListenAndServe()
}

// ──────── API Handlers ────────

func (w *WebUI) handleAPIAgents(rw http.ResponseWriter, r *http.Request) {
	agents, _ := w.server.AgentMgr.List()
	w.server.AgentMgr.RefreshStatuses()

	type agentResp struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		OS       string `json:"os"`
		Hostname string `json:"hostname"`
		Username string `json:"username"`
		IP       string `json:"ip"`
		Sleep    string `json:"sleep"`
		LastSeen string `json:"last_seen"`
		Status   string `json:"status"`
	}

	var resp []agentResp
	for _, a := range agents {
		resp = append(resp, agentResp{
			ID:       util.ShortID(a.ID),
			Name:     a.Name,
			OS:       a.OS,
			Hostname: a.Hostname,
			Username: a.Username,
			IP:       a.ExternalIP,
			Sleep:    fmt.Sprintf("%ds/%d%%", a.Sleep, a.Jitter),
			LastSeen: util.TimeAgo(a.LastSeen),
			Status:   a.Status,
		})
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(resp)
}

func (w *WebUI) handleAPIListeners(rw http.ResponseWriter, r *http.Request) {
	listeners := w.server.ListenerMgr.List()

	type listenerResp struct {
		Name   string `json:"name"`
		Type   string `json:"type"`
		Bind   string `json:"bind"`
		Status string `json:"status"`
	}

	var resp []listenerResp
	for _, l := range listeners {
		status := "stopped"
		if l.IsRunning() {
			status = "running"
		}
		resp = append(resp, listenerResp{
			Name:   l.Name,
			Type:   strings.ToUpper(l.Type),
			Bind:   l.BindAddr,
			Status: status,
		})
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(resp)
}

func (w *WebUI) handleAPITasks(rw http.ResponseWriter, r *http.Request) {
	agents, _ := w.server.AgentMgr.List()

	type taskResp struct {
		ID      string `json:"id"`
		Agent   string `json:"agent"`
		Type    string `json:"type"`
		Args    string `json:"args"`
		Status  string `json:"status"`
		Created string `json:"created"`
		Output  string `json:"output"`
	}

	var resp []taskResp
	for _, a := range agents {
		tasks, _ := w.server.TaskDisp.GetTaskHistory(a.ID)
		for _, t := range tasks {
			output := ""
			result, _ := w.server.TaskDisp.GetResult(t.ID)
			if result != nil && len(result.Output) > 0 {
				output = string(result.Output)
				if len(output) > 500 {
					output = output[:500] + "..."
				}
			}

			resp = append(resp, taskResp{
				ID:      util.ShortID(t.ID),
				Agent:   a.Name,
				Type:    protocol.TaskTypeName(uint8(t.Type)),
				Args:    strings.Join(t.Args, " "),
				Status:  protocol.StatusName(uint8(t.Status)),
				Created: util.TimeAgo(t.CreatedAt),
				Output:  output,
			})
		}
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(resp)
}

func (w *WebUI) handleAPIEvents(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(w.server.EventLog)
}

// ──────── Dashboard HTML ────────

func (w *WebUI) handleDashboard(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.Write([]byte(dashboardHTML))
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Phantom C2 — Dashboard</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { background: #0f172a; color: #e2e8f0; font-family: 'Segoe UI', system-ui, sans-serif; }

  .header { background: #1e293b; padding: 20px 30px; border-bottom: 1px solid #334155; display: flex; align-items: center; justify-content: space-between; }
  .header h1 { color: #a78bfa; font-size: 24px; }
  .header h1 span { color: #64748b; font-size: 14px; font-weight: normal; }
  .status-dot { width: 10px; height: 10px; background: #4ade80; border-radius: 50%; display: inline-block; margin-right: 8px; }

  .container { max-width: 1400px; margin: 0 auto; padding: 20px; }

  .stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 24px; }
  .stat-card { background: #1e293b; border: 1px solid #334155; border-radius: 12px; padding: 20px; }
  .stat-card .label { color: #94a3b8; font-size: 13px; text-transform: uppercase; letter-spacing: 1px; }
  .stat-card .value { color: #f8fafc; font-size: 36px; font-weight: bold; margin-top: 4px; }
  .stat-card .value.green { color: #4ade80; }
  .stat-card .value.purple { color: #a78bfa; }
  .stat-card .value.yellow { color: #fbbf24; }
  .stat-card .value.blue { color: #60a5fa; }

  .panel { background: #1e293b; border: 1px solid #334155; border-radius: 12px; margin-bottom: 20px; }
  .panel-header { padding: 16px 20px; border-bottom: 1px solid #334155; display: flex; justify-content: space-between; align-items: center; }
  .panel-header h2 { font-size: 16px; color: #a78bfa; }
  .panel-body { padding: 0; }

  table { width: 100%; border-collapse: collapse; }
  th { background: #0f172a; padding: 12px 16px; text-align: left; font-size: 12px; text-transform: uppercase; letter-spacing: 1px; color: #94a3b8; }
  td { padding: 12px 16px; border-top: 1px solid #1a2332; font-size: 14px; }
  tr:hover td { background: #172033; }

  .badge { display: inline-block; padding: 2px 10px; border-radius: 12px; font-size: 12px; font-weight: 600; }
  .badge-active { background: #065f46; color: #6ee7b7; }
  .badge-dormant { background: #78350f; color: #fcd34d; }
  .badge-dead { background: #7f1d1d; color: #fca5a5; }
  .badge-running { background: #065f46; color: #6ee7b7; }
  .badge-stopped { background: #7f1d1d; color: #fca5a5; }
  .badge-complete { background: #1e3a5f; color: #93c5fd; }
  .badge-pending { background: #78350f; color: #fcd34d; }
  .badge-sent { background: #4a1d96; color: #c4b5fd; }

  .os-icon { font-size: 16px; margin-right: 6px; }
  .refresh-btn { background: #334155; border: none; color: #94a3b8; padding: 6px 14px; border-radius: 6px; cursor: pointer; font-size: 12px; }
  .refresh-btn:hover { background: #475569; color: #e2e8f0; }

  .output-cell { max-width: 400px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: monospace; font-size: 12px; color: #94a3b8; }

  @media (max-width: 768px) { .stats { grid-template-columns: repeat(2, 1fr); } }
</style>
</head>
<body>

<div class="header">
  <h1>
    <span class="status-dot"></span>
    Phantom C2 <span>— Dashboard</span>
  </h1>
  <button class="refresh-btn" onclick="refreshAll()">↻ Refresh</button>
</div>

<div class="container">
  <div class="stats">
    <div class="stat-card"><div class="label">Total Agents</div><div class="value green" id="stat-agents">-</div></div>
    <div class="stat-card"><div class="label">Active Listeners</div><div class="value purple" id="stat-listeners">-</div></div>
    <div class="stat-card"><div class="label">Tasks Completed</div><div class="value blue" id="stat-tasks">-</div></div>
    <div class="stat-card"><div class="label">Events</div><div class="value yellow" id="stat-events">-</div></div>
  </div>

  <div class="panel">
    <div class="panel-header"><h2>Agents</h2></div>
    <div class="panel-body"><table>
      <thead><tr><th>Name</th><th>OS</th><th>Hostname</th><th>User</th><th>IP</th><th>Sleep</th><th>Last Seen</th><th>Status</th></tr></thead>
      <tbody id="agents-table"></tbody>
    </table></div>
  </div>

  <div class="panel">
    <div class="panel-header"><h2>Listeners</h2></div>
    <div class="panel-body"><table>
      <thead><tr><th>Name</th><th>Type</th><th>Bind Address</th><th>Status</th></tr></thead>
      <tbody id="listeners-table"></tbody>
    </table></div>
  </div>

  <div class="panel">
    <div class="panel-header"><h2>Recent Tasks</h2></div>
    <div class="panel-body"><table>
      <thead><tr><th>ID</th><th>Agent</th><th>Type</th><th>Command</th><th>Status</th><th>Time</th><th>Output</th></tr></thead>
      <tbody id="tasks-table"></tbody>
    </table></div>
  </div>
</div>

<script>
function badge(status) {
  const cls = 'badge-' + status.toLowerCase();
  return '<span class="badge ' + cls + '">' + status + '</span>';
}
function osIcon(os) {
  return os === 'windows' ? '<span class="os-icon">🪟</span>' : '<span class="os-icon">🐧</span>';
}

async function fetchJSON(url) {
  const r = await fetch(url);
  return r.json();
}

async function refreshAgents() {
  const agents = await fetchJSON('/api/agents') || [];
  document.getElementById('stat-agents').textContent = agents.length;
  const tbody = document.getElementById('agents-table');
  tbody.innerHTML = agents.map(a =>
    '<tr><td><strong>' + a.name + '</strong></td><td>' + osIcon(a.os) + a.os + '</td><td>' + a.hostname +
    '</td><td>' + a.username + '</td><td>' + a.ip + '</td><td>' + a.sleep + '</td><td>' + a.last_seen +
    '</td><td>' + badge(a.status) + '</td></tr>'
  ).join('');
}

async function refreshListeners() {
  const listeners = await fetchJSON('/api/listeners') || [];
  document.getElementById('stat-listeners').textContent = listeners.length;
  const tbody = document.getElementById('listeners-table');
  tbody.innerHTML = listeners.map(l =>
    '<tr><td>' + l.name + '</td><td>' + l.type + '</td><td>' + l.bind + '</td><td>' + badge(l.status) + '</td></tr>'
  ).join('');
}

async function refreshTasks() {
  const tasks = await fetchJSON('/api/tasks') || [];
  document.getElementById('stat-tasks').textContent = tasks.filter(t => t.status === 'complete').length;
  const tbody = document.getElementById('tasks-table');
  tbody.innerHTML = tasks.slice(0, 50).map(t =>
    '<tr><td>' + t.id + '</td><td>' + t.agent + '</td><td>' + t.type + '</td><td><code>' + (t.args || '-') +
    '</code></td><td>' + badge(t.status) + '</td><td>' + t.created + '</td><td class="output-cell">' +
    (t.output || '-') + '</td></tr>'
  ).join('');
}

async function refreshEvents() {
  const events = await fetchJSON('/api/events') || [];
  document.getElementById('stat-events').textContent = events.length;
}

function refreshAll() {
  refreshAgents();
  refreshListeners();
  refreshTasks();
  refreshEvents();
}

refreshAll();
setInterval(refreshAll, 5000);
</script>
</body>
</html>`
