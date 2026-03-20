package web

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/zuwance/agent-social-gateway/internal/observability"
	"github.com/zuwance/agent-social-gateway/internal/session"
	"github.com/zuwance/agent-social-gateway/internal/social"
)

type Dashboard struct {
	metrics    *observability.Metrics
	audit      *observability.AuditLogger
	sessionMgr *session.Manager
	socialAct  *social.Actions
	logger     *slog.Logger
}

func NewDashboard(
	metrics *observability.Metrics,
	audit *observability.AuditLogger,
	sessionMgr *session.Manager,
	socialAct *social.Actions,
	logger *slog.Logger,
) *Dashboard {
	return &Dashboard{
		metrics:    metrics,
		audit:      audit,
		sessionMgr: sessionMgr,
		socialAct:  socialAct,
		logger:     logger,
	}
}

func (d *Dashboard) RegisterRoutes(r chi.Router) {
	r.Route("/dashboard", func(r chi.Router) {
		r.Get("/", d.handleDashboardPage)
		r.Get("/api/metrics", d.metrics.JSONHandler())
		r.Get("/api/sessions", d.handleSessions)
		r.Get("/api/audit", d.handleAuditLog)
	})
}

func (d *Dashboard) handleDashboardPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashboardHTML))
}

func (d *Dashboard) handleSessions(w http.ResponseWriter, r *http.Request) {
	sessions := d.sessionMgr.ListActiveSessions()
	type sessionInfo struct {
		ID             string `json:"id"`
		AgentID        string `json:"agentId"`
		ConnectionType string `json:"connectionType"`
		Status         string `json:"status"`
		CreatedAt      string `json:"createdAt"`
		LastActiveAt   string `json:"lastActiveAt"`
	}
	var result []sessionInfo
	for _, s := range sessions {
		result = append(result, sessionInfo{
			ID:             s.ID,
			AgentID:        string(s.AgentID),
			ConnectionType: string(s.ConnectionType),
			Status:         s.Status,
			CreatedAt:      s.CreatedAt.String(),
			LastActiveAt:   s.LastActiveAt.String(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessions": result,
		"total":    len(result),
	})
}

func (d *Dashboard) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	entries, err := d.audit.Query(action, 100, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries": entries,
		"total":   len(entries),
	})
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Agent Social Gateway - Dashboard</title>
<style>
  :root { --bg: #0f172a; --card: #1e293b; --text: #e2e8f0; --accent: #38bdf8; --border: #334155; }
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: var(--bg); color: var(--text); padding: 2rem; }
  h1 { color: var(--accent); margin-bottom: 1.5rem; font-size: 1.5rem; }
  h2 { color: var(--accent); margin-bottom: 1rem; font-size: 1.1rem; }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1rem; margin-bottom: 2rem; }
  .card { background: var(--card); border: 1px solid var(--border); border-radius: 8px; padding: 1.25rem; }
  .card .label { font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; color: #94a3b8; }
  .card .value { font-size: 1.75rem; font-weight: bold; margin-top: 0.25rem; }
  table { width: 100%; border-collapse: collapse; margin-top: 0.5rem; }
  th, td { padding: 0.5rem 0.75rem; text-align: left; border-bottom: 1px solid var(--border); font-size: 0.85rem; }
  th { color: #94a3b8; font-weight: 600; }
  .section { margin-bottom: 2rem; }
  .refresh { color: var(--accent); cursor: pointer; font-size: 0.8rem; float: right; }
</style>
</head>
<body>
<h1>Agent Social Gateway</h1>
<div id="metrics" class="grid"></div>
<div class="section">
  <h2>Active Sessions <span class="refresh" onclick="loadSessions()">refresh</span></h2>
  <table><thead><tr><th>ID</th><th>Agent</th><th>Type</th><th>Status</th><th>Last Active</th></tr></thead><tbody id="sessions"></tbody></table>
</div>
<div class="section">
  <h2>Audit Log <span class="refresh" onclick="loadAudit()">refresh</span></h2>
  <table><thead><tr><th>Time</th><th>Action</th><th>From</th><th>To</th><th>Hash</th></tr></thead><tbody id="audit"></tbody></table>
</div>
<script>
async function loadMetrics() {
  const r = await fetch('/dashboard/api/metrics');
  const d = await r.json();
  const el = document.getElementById('metrics');
  el.innerHTML = Object.entries(d).filter(([k])=>k!=='counters').map(([k,v])=>
    '<div class="card"><div class="label">'+k.replace(/_/g,' ')+'</div><div class="value">'+(typeof v==='number'?v.toFixed(v%1?2:0):v)+'</div></div>'
  ).join('');
}
async function loadSessions() {
  const r = await fetch('/dashboard/api/sessions');
  const d = await r.json();
  const el = document.getElementById('sessions');
  el.innerHTML = (d.sessions||[]).map(s=>
    '<tr><td>'+s.id.slice(0,8)+'</td><td>'+s.agentId+'</td><td>'+s.connectionType+'</td><td>'+s.status+'</td><td>'+s.lastActiveAt+'</td></tr>'
  ).join('') || '<tr><td colspan="5" style="text-align:center;color:#64748b">No active sessions</td></tr>';
}
async function loadAudit() {
  const r = await fetch('/dashboard/api/audit');
  const d = await r.json();
  const el = document.getElementById('audit');
  el.innerHTML = (d.entries||[]).slice(0,50).map(e=>
    '<tr><td>'+new Date(e.timestamp).toLocaleString()+'</td><td>'+e.action+'</td><td>'+e.fromAgent+'</td><td>'+e.toAgent+'</td><td>'+e.payloadHash+'</td></tr>'
  ).join('') || '<tr><td colspan="5" style="text-align:center;color:#64748b">No audit entries</td></tr>';
}
loadMetrics(); loadSessions(); loadAudit();
setInterval(loadMetrics, 5000);
</script>
</body>
</html>`
