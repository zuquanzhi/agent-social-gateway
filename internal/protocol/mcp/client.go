package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/zuwance/agent-social-gateway/internal/config"
)

type UpstreamConnection struct {
	Name      string
	Config    config.MCPUpstreamConfig
	Client    *client.Client
	Tools     []mcp.Tool
	Connected bool
	LastCheck time.Time
}

type UpstreamManager struct {
	connections map[string]*UpstreamConnection
	mu          sync.RWMutex
	logger      *slog.Logger
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewUpstreamManager(logger *slog.Logger) *UpstreamManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &UpstreamManager{
		connections: make(map[string]*UpstreamConnection),
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (m *UpstreamManager) ConnectAll(upstreams []config.MCPUpstreamConfig) error {
	for _, upstream := range upstreams {
		if err := m.Connect(upstream); err != nil {
			m.logger.Warn("failed to connect upstream", "name", upstream.Name, "error", err)
			continue
		}
	}
	go m.healthCheckLoop()
	return nil
}

func (m *UpstreamManager) Connect(upstream config.MCPUpstreamConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var c *client.Client
	var err error

	switch upstream.Transport {
	case "sse":
		c, err = client.NewSSEMCPClient(upstream.URL)
		if err != nil {
			return fmt.Errorf("creating SSE client for %s: %w", upstream.Name, err)
		}
		startCtx, startCancel := context.WithTimeout(m.ctx, 10*time.Second)
		if startErr := c.Start(startCtx); startErr != nil {
			startCancel()
			return fmt.Errorf("starting SSE client for %s: %w", upstream.Name, startErr)
		}
		startCancel()
	case "stdio":
		c, err = client.NewStdioMCPClient(upstream.Command, nil, upstream.Args...)
		if err != nil {
			return fmt.Errorf("creating stdio client for %s: %w", upstream.Name, err)
		}
	default:
		return fmt.Errorf("unknown transport %q for %s", upstream.Transport, upstream.Name)
	}

	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "agent-social-gateway",
		Version: "0.1.0",
	}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		c.Close()
		return fmt.Errorf("initializing client for %s: %w", upstream.Name, err)
	}

	toolsResult, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		c.Close()
		return fmt.Errorf("listing tools for %s: %w", upstream.Name, err)
	}

	conn := &UpstreamConnection{
		Name:      upstream.Name,
		Config:    upstream,
		Client:    c,
		Tools:     toolsResult.Tools,
		Connected: true,
		LastCheck: time.Now(),
	}

	m.connections[upstream.Name] = conn
	m.logger.Info("upstream connected", "name", upstream.Name, "tools", len(toolsResult.Tools))
	return nil
}

func (m *UpstreamManager) GetAggregatedTools() []UpstreamTool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tools []UpstreamTool
	for _, conn := range m.connections {
		if !conn.Connected {
			continue
		}
		for _, tool := range conn.Tools {
			upstreamName := conn.Name
			handler := m.makeProxyHandler(upstreamName, tool.Name)
			tools = append(tools, UpstreamTool{
				Upstream:    upstreamName,
				Name:        tool.Name,
				Description: tool.Description,
				Schema:      nil,
				Handler:     handler,
			})
		}
	}
	return tools
}

func (m *UpstreamManager) makeProxyHandler(upstreamName, toolName string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m.mu.RLock()
		conn, ok := m.connections[upstreamName]
		m.mu.RUnlock()

		if !ok || !conn.Connected {
			return mcp.NewToolResultError(fmt.Sprintf("upstream %s not connected", upstreamName)), nil
		}

		proxyReq := mcp.CallToolRequest{}
		proxyReq.Method = "tools/call"
		proxyReq.Params.Name = toolName
		proxyReq.Params.Arguments = req.GetArguments()

		result, err := conn.Client.CallTool(ctx, proxyReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("upstream %s error: %v", upstreamName, err)), nil
		}
		return result, nil
	}
}

func (m *UpstreamManager) healthCheckLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkHealth()
		}
	}
}

func (m *UpstreamManager) checkHealth() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, conn := range m.connections {
		ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
		err := conn.Client.Ping(ctx)
		cancel()

		if err != nil {
			if conn.Connected {
				m.logger.Warn("upstream disconnected", "name", name, "error", err)
				conn.Connected = false
			}
			go m.tryReconnect(name, conn.Config)
		} else {
			conn.Connected = true
			conn.LastCheck = time.Now()
		}
	}
}

func (m *UpstreamManager) tryReconnect(name string, cfg config.MCPUpstreamConfig) {
	m.logger.Info("attempting reconnect", "name", name)
	if err := m.Connect(cfg); err != nil {
		m.logger.Warn("reconnect failed", "name", name, "error", err)
	}
}

func (m *UpstreamManager) Close() {
	m.cancel()

	m.mu.Lock()
	defer m.mu.Unlock()

	for name, conn := range m.connections {
		if err := conn.Client.Close(); err != nil {
			m.logger.Warn("error closing upstream", "name", name, "error", err)
		}
	}
}

func (m *UpstreamManager) GetConnection(name string) (*UpstreamConnection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conn, ok := m.connections[name]
	return conn, ok
}

func (m *UpstreamManager) ListConnections() []UpstreamConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []UpstreamConnection
	for _, conn := range m.connections {
		list = append(list, *conn)
	}
	return list
}
