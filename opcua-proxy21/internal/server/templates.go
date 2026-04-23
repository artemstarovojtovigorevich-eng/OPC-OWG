package server

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>OPC UA Proxy Admin</title>
    <style>
        :root {
            --bg-primary: #1a1a2e;
            --bg-secondary: #16213e;
            --bg-tertiary: #0f3460;
            --text-primary: #e8e8e8;
            --text-secondary: #a0a0a0;
            --accent: #e94560;
            --accent-hover: #ff6b6b;
            --border: #2a2a4a;
            --success: #4ade80;
            --warning: #fbbf24;
            --danger: #ef4444;
        }
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: var(--bg-primary);
            color: var(--text-primary);
            min-height: 100vh;
            padding: 2rem;
        }
        .container { max-width: 1200px; margin: 0 auto; }
        header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 2rem;
            padding-bottom: 1rem;
            border-bottom: 1px solid var(--border);
        }
        h1 { font-size: 1.5rem; font-weight: 600; }
        .status-badge {
            padding: 0.5rem 1rem;
            border-radius: 4px;
            font-size: 0.875rem;
            font-weight: 500;
            background: var(--bg-tertiary);
        }
        .status-badge.configured { background: var(--warning); color: #000; }
        .status-badge.discovering { background: var(--accent); color: #fff; }
        .status-badge.running { background: var(--success); color: #000; }
        .status-badge.waiting_config { background: var(--bg-tertiary); color: var(--text-secondary); }
        .status-badge.error { background: var(--danger); color: #fff; }
        .card {
            background: var(--bg-secondary);
            border-radius: 8px;
            padding: 1.5rem;
            margin-bottom: 1.5rem;
        }
        .card h2 {
            font-size: 1.125rem;
            margin-bottom: 1rem;
            color: var(--text-secondary);
        }
        .form-group { display: flex; gap: 1rem; align-items: flex-end; }
        .form-group label { display: flex; flex-direction: column; gap: 0.5rem; }
        .form-group label span { font-size: 0.875rem; color: var(--text-secondary); }
        input[type="text"] {
            background: var(--bg-primary);
            border: 1px solid var(--border);
            color: var(--text-primary);
            padding: 0.75rem 1rem;
            border-radius: 4px;
            font-size: 1rem;
            width: 200px;
        }
        input:focus { outline: none; border-color: var(--accent); }
        button {
            background: var(--accent);
            color: #fff;
            border: none;
            padding: 0.75rem 1.5rem;
            border-radius: 4px;
            font-size: 1rem;
            cursor: pointer;
            transition: background 0.2s;
        }
        button:hover { background: var(--accent-hover); }
        button:disabled { opacity: 0.5; cursor: not-allowed; }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1rem; margin-bottom: 1.5rem; }
        .stat { background: var(--bg-secondary); padding: 1rem; border-radius: 8px; }
        .stat-value { font-size: 2rem; font-weight: 600; }
        .stat-label { color: var(--text-secondary); font-size: 0.875rem; }
        table { width: 100%; border-collapse: collapse; }
        th, td { text-align: left; padding: 1rem; border-bottom: 1px solid var(--border); }
        th { color: var(--text-secondary); font-weight: 500; font-size: 0.875rem; }
        .checkbox { width: 20px; height: 20px; cursor: pointer; }
        .btn-small { padding: 0.5rem 1rem; font-size: 0.875rem; }
        .btn-delete { background: var(--danger); }
        .btn-delete:hover { background: #dc2626; }
        .node-id { font-family: monospace; color: var(--accent); }
        .empty-state { text-align: center; padding: 3rem; color: var(--text-secondary); }
        #loadingOverlay {
            position: fixed; top: 0; left: 0; right: 0; bottom: 0;
            background: rgba(0,0,0,0.8);
            display: none;
            justify-content: center; align-items: center; z-index: 100;
        }
        #loadingOverlay.active { display: flex; }
        .spinner {
            width: 50px; height: 50px;
            border: 3px solid var(--border);
            border-top-color: var(--accent);
            border-radius: 50%;
            animation: spin 1s linear infinite;
        }
        @keyframes spin { to { transform: rotate(360deg); } }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>OPC UA Proxy Admin</h1>
            <span class="status-badge {{.State}}" id="statusBadge">{{.State}}</span>
        </header>
        <div class="stats">
            <div class="stat">
                <div class="stat-value" id="nodeCount">{{len .Nodes}}</div>
                <div class="stat-label">Nodes Configured</div>
            </div>
            <div class="stat">
                <div class="stat-value" id="namespaceValue">{{.Namespace}}</div>
                <div class="stat-label">Namespace</div>
            </div>
        </div>
        <div class="card">
            <h2>Configuration</h2>
            <div class="form-group">
                <label>
                    <span>Namespace</span>
                    <input type="text" id="namespaceInput" value="{{.Namespace}}" placeholder="e.g., 3">
                </label>
                <button id="configBtn" onclick="saveConfig()">Save & Start Discovery</button>
            </div>
        </div>
        <div class="card">
            <h2>Nodes</h2>
            <table id="nodesTable">
                <thead>
                    <tr>
                        <th>Enabled</th>
                        <th>Node ID</th>
                        <th>Name</th>
                        <th>Data Type</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody id="nodesBody">
{{range .Nodes}}
                    <tr data-id="{{.ID}}">
                        <td><input type="checkbox" class="checkbox" {{if .Enabled}}checked{{end}} onchange="toggleNode({{.ID}}, this.checked)"></td>
                        <td class="node-id">{{.NodeID}}</td>
                        <td>{{.Name}}</td>
                        <td>{{.DataType}}</td>
                        <td><button class="btn-small btn-delete" onclick="deleteNode({{.ID}})">Delete</button></td>
                    </tr>
{{else}}
                    <tr><td colspan="5" class="empty-state">No nodes configured. Enter namespace and click "Save & Start Discovery"</td></tr>
{{end}}
                </tbody>
            </table>
        </div>
    </div>
    <div id="loadingOverlay">
        <div class="spinner"></div>
    </div>
    <script>
        async function saveConfig() {
            const namespace = document.getElementById('namespaceInput').value;
            if (!namespace) { alert('Please enter a namespace'); return; }
            document.getElementById('loadingOverlay').classList.add('active');
            try {
                const resp = await fetch('/api/settings', {
                    method: 'POST',
                    body: 'namespace=' + encodeURIComponent(namespace),
                    headers: {'Content-Type': 'application/x-www-form-urlencoded'}
                });
                if (resp.ok) { window.location.reload(); }
                else { alert('Failed to save config'); }
            } catch (e) { alert('Error: ' + e); }
            document.getElementById('loadingOverlay').classList.remove('active');
        }
        async function toggleNode(id, enabled) {
            try {
                await fetch('/api/node/enable', {
                    method: 'POST',
                    body: 'id=' + id + '&enabled=' + enabled,
                    headers: {'Content-Type': 'application/x-www-form-urlencoded'}
                });
            } catch (e) { alert('Error: ' + e); }
        }
        async function deleteNode(id) {
            if (!confirm('Delete this node?')) return;
            try {
                await fetch('/api/node/delete', {
                    method: 'POST',
                    body: 'id=' + id,
                    headers: {'Content-Type': 'application/x-www-form-urlencoded'}
                });
                window.location.reload();
            } catch (e) { alert('Error: ' + e); }
        }
        const status = '{{.State}}';
        async function checkState() {
            try {
                const resp = await fetch('/api/state');
                const data = await resp.json();
                document.getElementById('statusBadge').textContent = data.status;
                document.getElementById('statusBadge').className = 'status-badge ' + data.status;
                document.getElementById('nodeCount').textContent = data.nodeCount;
                document.getElementById('namespaceValue').textContent = data.namespace || '';
                if (data.status === 'running') { window.location.reload(); }
            } catch (e) { console.error('Failed to check state:', e); }
        }
        setInterval(checkState, 3000);
        if (status === 'discovering') { checkState(); }
    </script>
</body>
</html>`