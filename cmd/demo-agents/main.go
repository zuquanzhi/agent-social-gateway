package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

const gatewayURL = "http://localhost:8080"

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	purple = "\033[35m"
	cyan   = "\033[36m"
)

type Agent struct {
	ID        string
	Name      string
	Port      int
	Color     string
	mcpClient *client.Client
	srv       *http.Server
}

func main() {
	banner()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	alpha := &Agent{ID: "agent-alpha", Name: "Alpha (Research Agent)", Port: 9001, Color: blue}
	beta := &Agent{ID: "agent-beta", Name: "Beta (Code Agent)", Port: 9002, Color: purple}

	// ── Step 1: Start agent HTTP servers ──
	step(1, "Starting Agent HTTP Servers")
	alpha.startServer()
	beta.startServer()
	time.Sleep(300 * time.Millisecond)
	alpha.log("HTTP server on http://localhost:%d  (agent card + webhook)", alpha.Port)
	beta.log("HTTP server on http://localhost:%d  (agent card + webhook)", beta.Port)

	// ── Step 2: Connect to Gateway via MCP SSE ──
	step(2, "Connecting to Gateway via MCP SSE Protocol")
	if err := alpha.connectMCP(ctx); err != nil {
		alpha.log("MCP: %v (using REST API fallback)", err)
	} else {
		alpha.log("MCP session established ✓")
	}
	time.Sleep(1 * time.Second)
	if err := beta.connectMCP(ctx); err != nil {
		beta.log("MCP: %v (using REST API fallback)", err)
	} else {
		beta.log("MCP session established ✓")
	}

	// ── Step 3: List MCP tools ──
	step(3, "Discovering MCP Tools via SSE")
	mcpAgent := alpha
	if alpha.mcpClient == nil {
		mcpAgent = beta
	}
	if mcpAgent.mcpClient != nil {
		mcpAgent.listTools(ctx)
	} else {
		fmt.Printf("  %s(MCP SSE not available — tools accessible via REST)%s\n", dim, reset)
	}

	// ── Step 4: Register via discovery ──
	step(4, "Registering Agents via Discovery Resolve")
	resolveAgent(alpha)
	resolveAgent(beta)

	// ── Step 5: Social – mutual follow ──
	step(5, "Social: Mutual Follow")
	alpha.socialPost("/api/v1/social/follow", map[string]any{
		"follower_id": alpha.ID, "target_id": beta.ID,
	})
	beta.socialPost("/api/v1/social/follow", map[string]any{
		"follower_id": beta.ID, "target_id": alpha.ID,
	})

	// ── Step 6: A2A message exchange ──
	step(6, "A2A: Message Exchange Through Gateway")
	msgA := alpha.sendA2A("Hi Beta! I'm researching AI agent cooperation patterns — can you review my code samples?")
	msgB := beta.sendA2A("Sure Alpha! I've analyzed 3 critical sections. Sending detailed feedback now.")
	msgC := alpha.sendA2A("Excellent! Let's merge our findings into a joint report.")

	// ── Step 7: Social – like messages ──
	step(7, "Social: Like Messages")
	beta.socialPost("/api/v1/social/like", map[string]any{
		"agent_id": beta.ID, "message_id": msgA,
	})
	alpha.socialPost("/api/v1/social/like", map[string]any{
		"agent_id": alpha.ID, "message_id": msgB,
	})
	beta.socialPost("/api/v1/social/like", map[string]any{
		"agent_id": beta.ID, "message_id": msgC,
	})

	// ── Step 8: Social – endorse skills ──
	step(8, "Social: Endorse Skills")
	beta.socialPost("/api/v1/social/endorse", map[string]any{
		"from_agent_id": beta.ID, "target_agent_id": alpha.ID, "skill_id": "research-methodology",
	})
	alpha.socialPost("/api/v1/social/endorse", map[string]any{
		"from_agent_id": alpha.ID, "target_agent_id": beta.ID, "skill_id": "code-review",
	})

	// ── Step 9: Social – request collaboration ──
	step(9, "Social: Request Collaboration")
	alpha.socialPost("/api/v1/social/collaborate", map[string]any{
		"from_agent_id": alpha.ID, "target_agent_id": beta.ID,
		"proposal": map[string]any{"task": "joint-paper", "title": "AI Agent Cooperation Patterns", "deadline": "2026-04-15"},
	})

	// ── Step 10: Query social graph ──
	step(10, "Query Social Graph")
	alpha.socialGet("/api/v1/social/graph/" + alpha.ID)
	beta.socialGet("/api/v1/social/graph/" + beta.ID)

	// ── Step 11: Query followers & following ──
	step(11, "Query Followers & Following")
	alpha.socialGet("/api/v1/social/followers/" + alpha.ID)
	alpha.socialGet("/api/v1/social/following/" + alpha.ID)

	// ── Step 12: Query timelines ──
	step(12, "Query Agent Timelines")
	alpha.socialGet("/api/v1/social/timeline/" + alpha.ID)
	beta.socialGet("/api/v1/social/timeline/" + beta.ID)

	// ── Step 13: A2A – list all tasks ──
	step(13, "A2A: List All Tasks in Gateway")
	listTasks()

	// ── Step 14: Gateway observability ──
	step(14, "Gateway Metrics & Audit")
	showMetrics()
	showAudit()

	// ── Cleanup ──
	fmt.Println()
	if alpha.mcpClient != nil {
		alpha.mcpClient.Close()
	}
	if beta.mcpClient != nil {
		beta.mcpClient.Close()
	}
	alpha.srv.Shutdown(ctx)
	beta.srv.Shutdown(ctx)

	fmt.Printf("\n%s%s╔══════════════════════════════════════════════════════════════════╗%s\n", bold, green, reset)
	fmt.Printf("%s%s║  ✓ Demo complete — two agents successfully communicated         ║%s\n", bold, green, reset)
	fmt.Printf("%s%s║    via MCP SSE + A2A protocol + Social REST API through gateway  ║%s\n", bold, green, reset)
	fmt.Printf("%s%s╚══════════════════════════════════════════════════════════════════╝%s\n\n", bold, green, reset)
}

// ─── Agent Methods ──────────────────────────────────────────────

func (a *Agent) startServer() {
	mux := http.NewServeMux()

	card := map[string]any{
		"name":        a.Name,
		"description": fmt.Sprintf("%s — connected to agent-social-gateway", a.Name),
		"version":     "1.0.0",
		"supportedInterfaces": []map[string]any{
			{"url": fmt.Sprintf("http://localhost:%d", a.Port), "protocolBinding": "JSONRPC", "protocolVersion": "0.3"},
		},
		"capabilities": map[string]any{"streaming": true},
		"skills": []map[string]any{
			{"id": a.ID + "-skill", "name": a.Name + " Primary Skill", "tags": []string{"demo"}},
		},
	}

	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(card)
	})

	mux.HandleFunc("/a2a/message:send", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		a.log("  Received callback: %s", truncate(string(body), 100))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"received"}`)
	})

	a.srv = &http.Server{Addr: fmt.Sprintf(":%d", a.Port), Handler: mux}
	go a.srv.ListenAndServe()
}

func (a *Agent) connectMCP(ctx context.Context) error {
	c, err := client.NewSSEMCPClient(gatewayURL + "/mcp/sse")
	if err != nil {
		return fmt.Errorf("creating SSE client: %w", err)
	}

	// Use background context for SSE stream — it must stay alive for the entire session.
	// Start() has a built-in 30s timeout for initial endpoint discovery.
	if err := c.Start(context.Background()); err != nil {
		return fmt.Errorf("starting SSE: %w", err)
	}

	initCtx, initCancel := context.WithTimeout(ctx, 10*time.Second)
	defer initCancel()

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: a.ID, Version: "1.0.0"}

	if _, err := c.Initialize(initCtx, initReq); err != nil {
		c.Close()
		return fmt.Errorf("MCP init: %w", err)
	}

	a.mcpClient = c
	return nil
}

func (a *Agent) listTools(ctx context.Context) {
	toolCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := a.mcpClient.ListTools(toolCtx, mcp.ListToolsRequest{})
	if err != nil {
		a.log("ListTools error: %v", err)
		return
	}
	a.log("Discovered %d tools:", len(result.Tools))
	for _, t := range result.Tools {
		fmt.Printf("    %s•%s %-35s %s%s%s\n", cyan, reset, t.Name, dim, t.Description, reset)
	}
}

func (a *Agent) socialPost(path string, body map[string]any) {
	data, _ := json.Marshal(body)
	resp, err := http.Post(gatewayURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		a.log("✗ POST %s: %v", path, err)
		return
	}
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	a.log("→ %s", formatResult(result))
}

func (a *Agent) socialGet(path string) {
	resp, err := http.Get(gatewayURL + path)
	if err != nil {
		a.log("✗ GET %s: %v", path, err)
		return
	}
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	a.log("→ %s", formatResult(result))
}

func (a *Agent) sendA2A(text string) string {
	msgID := fmt.Sprintf("%s-%d", a.ID, time.Now().UnixMilli())
	body := map[string]any{
		"message": map[string]any{
			"messageId": msgID,
			"role":      "user",
			"parts":     []map[string]any{{"text": text}},
		},
	}
	data, _ := json.Marshal(body)

	resp, err := http.Post(gatewayURL+"/a2a/message:send", "application/json", bytes.NewReader(data))
	if err != nil {
		a.log("✗ A2A error: %v", err)
		return msgID
	}
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	if task, ok := result["task"].(map[string]any); ok {
		taskID, _ := task["id"].(string)
		status, _ := task["status"].(map[string]any)
		state, _ := status["state"].(string)
		a.log("A2A → task %s [%s]", taskID[:8], state)
	}
	fmt.Printf("    %s\"%s\"%s\n", dim, truncate(text, 80), reset)
	return msgID
}

func (a *Agent) log(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("  %s%s[%s]%s %s\n", bold, a.Color, a.ID, reset, msg)
}

// ─── Gateway Queries ────────────────────────────────────────────

func resolveAgent(a *Agent) {
	url := fmt.Sprintf("http://localhost:%d", a.Port)
	body, _ := json.Marshal(map[string]string{"url": url})
	resp, err := http.Post(gatewayURL+"/api/v1/discover/resolve", "application/json", bytes.NewReader(body))
	if err != nil {
		a.log("✗ Resolve error: %v", err)
		return
	}
	defer resp.Body.Close()

	var card map[string]any
	json.NewDecoder(resp.Body).Decode(&card)
	name, _ := card["name"].(string)
	a.log("Registered in discovery: %s", name)
}

func listTasks() {
	resp, err := http.Get(gatewayURL + "/a2a/tasks?pageSize=10")
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	total, _ := result["totalSize"].(float64)
	fmt.Printf("  %sTotal tasks: %.0f%s\n", bold, total, reset)

	if tasks, ok := result["tasks"].([]any); ok {
		for _, t := range tasks {
			task, _ := t.(map[string]any)
			taskID, _ := task["id"].(string)
			ctxID, _ := task["contextId"].(string)
			status, _ := task["status"].(map[string]any)
			state, _ := status["state"].(string)

			msgText := ""
			if hist, ok := task["history"].([]any); ok && len(hist) > 0 {
				if msg, ok := hist[0].(map[string]any); ok {
					if parts, ok := msg["parts"].([]any); ok && len(parts) > 0 {
						if p, ok := parts[0].(map[string]any); ok {
							msgText, _ = p["text"].(string)
						}
					}
				}
			}

			sc := green
			if state != "COMPLETED" {
				sc = yellow
			}
			fmt.Printf("  %s[%s%s%s]%s %s ctx=%s  %s%s%s\n",
				dim, sc, state, reset+dim, reset, taskID[:8], ctxID[:8], dim, truncate(msgText, 50), reset)
		}
	}
}

func showMetrics() {
	resp, err := http.Get(gatewayURL + "/metrics/json")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)

	fmt.Printf("  %s%sGateway Metrics%s\n", bold, cyan, reset)
	fmt.Printf("    %-22s %.0fs\n", "Uptime:", m["uptime_seconds"])
	fmt.Printf("    %-22s %.0f\n", "Total Requests:", m["total_requests"])
	fmt.Printf("    %-22s %.0f\n", "Total Errors:", m["total_errors"])
	fmt.Printf("    %-22s %.0f\n", "Active Connections:", m["active_connections"])
	fmt.Printf("    %-22s %.2fms\n", "Avg Latency:", m["avg_latency_ms"])
}

func showAudit() {
	resp, err := http.Get(gatewayURL + "/dashboard/api/audit")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	if entries, ok := result["entries"].([]any); ok {
		fmt.Printf("  %s%sAudit Log (%d entries)%s\n", bold, cyan, len(entries), reset)
		for _, e := range entries {
			entry, _ := e.(map[string]any)
			action, _ := entry["action"].(string)
			from, _ := entry["fromAgent"].(string)
			ts, _ := entry["timestamp"].(string)
			fmt.Printf("    %s%-22s%s %-28s %s\n", dim, ts, reset, action, from)
		}
	}
}

// ─── Helpers ────────────────────────────────────────────────────

func formatResult(m map[string]any) string {
	parts := make([]string, 0, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case string:
			parts = append(parts, fmt.Sprintf("%s=%s", k, val))
		case float64:
			if val == float64(int(val)) {
				parts = append(parts, fmt.Sprintf("%s=%.0f", k, val))
			} else {
				parts = append(parts, fmt.Sprintf("%s=%.4f", k, val))
			}
		case []any:
			items := make([]string, 0, len(val))
			for _, item := range val {
				if s, ok := item.(string); ok {
					items = append(items, s)
				}
			}
			if len(items) > 0 {
				parts = append(parts, fmt.Sprintf("%s=[%s]", k, strings.Join(items, ", ")))
			} else {
				parts = append(parts, fmt.Sprintf("%s=(%d items)", k, len(val)))
			}
		default:
			data, _ := json.Marshal(v)
			parts = append(parts, fmt.Sprintf("%s=%s", k, truncate(string(data), 60)))
		}
	}
	return strings.Join(parts, "  ")
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n-3] + "..."
	}
	return s
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s%sFATAL: %s%s\n", bold, red, fmt.Sprintf(format, args...), reset)
	os.Exit(1)
}

func banner() {
	fmt.Printf(`
%s%s╔══════════════════════════════════════════════════════════════════╗
║                                                                  ║
║   agent-social-gateway — Two-Agent Integration Demo              ║
║                                                                  ║
║   Agent Alpha (Research) ←──→ Gateway ←──→ Agent Beta (Code)     ║
║          :9001           MCP+A2A :8080           :9002            ║
║                                                                  ║
╚══════════════════════════════════════════════════════════════════╝%s

`, bold, cyan, reset)
}

func step(n int, title string) {
	fmt.Printf("\n%s%s── Step %d: %s ──%s\n", bold, yellow, n, title, reset)
}
