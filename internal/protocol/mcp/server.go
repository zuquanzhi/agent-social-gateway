package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/zuwance/agent-social-gateway/internal/config"
)

type Server struct {
	mcpServer *mcpserver.MCPServer
	sseServer *mcpserver.SSEServer
	cfg       *config.MCPServerConfig
	logger    *slog.Logger
	mu        sync.RWMutex
}

func NewServer(cfg *config.MCPServerConfig, baseURL string, logger *slog.Logger) *Server {
	mcpSrv := mcpserver.NewMCPServer(
		cfg.Name,
		cfg.Version,
		mcpserver.WithToolCapabilities(true),
		mcpserver.WithResourceCapabilities(true, true),
		mcpserver.WithRecovery(),
	)

	sseSrv := mcpserver.NewSSEServer(mcpSrv,
		mcpserver.WithBaseURL(baseURL),
		mcpserver.WithSSEEndpoint(cfg.SSEEndpoint),
		mcpserver.WithMessageEndpoint(cfg.SSEEndpoint+"/message"),
		mcpserver.WithKeepAliveInterval(30),
	)

	return &Server{
		mcpServer: mcpSrv,
		sseServer: sseSrv,
		cfg:       cfg,
		logger:    logger,
	}
}

func (s *Server) MCPServer() *mcpserver.MCPServer {
	return s.mcpServer
}

func (s *Server) SSEHandler() http.Handler {
	return s.sseServer
}

func (s *Server) AddTool(name, description string, schema map[string]any, handler func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	opts := []mcp.ToolOption{mcp.WithDescription(description)}

	for propName, propVal := range schema {
		if propMap, ok := propVal.(map[string]any); ok {
			propType, _ := propMap["type"].(string)
			desc, _ := propMap["description"].(string)
			required, _ := propMap["required"].(bool)

			switch propType {
			case "string":
				propOpts := []mcp.PropertyOption{mcp.Description(desc)}
				if required {
					propOpts = append(propOpts, mcp.Required())
				}
				opts = append(opts, mcp.WithString(propName, propOpts...))
			case "number":
				propOpts := []mcp.PropertyOption{mcp.Description(desc)}
				if required {
					propOpts = append(propOpts, mcp.Required())
				}
				opts = append(opts, mcp.WithNumber(propName, propOpts...))
			case "boolean":
				propOpts := []mcp.PropertyOption{mcp.Description(desc)}
				if required {
					propOpts = append(propOpts, mcp.Required())
				}
				opts = append(opts, mcp.WithBoolean(propName, propOpts...))
			}
		}
	}

	tool := mcp.NewTool(name, opts...)
	s.mcpServer.AddTool(tool, handler)
	s.logger.Info("mcp tool registered", "name", name)
}

func (s *Server) AddResource(uri, name, description, mimeType string, handler func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resource := mcp.NewResource(uri, name,
		mcp.WithResourceDescription(description),
		mcp.WithMIMEType(mimeType),
	)
	s.mcpServer.AddResource(resource, handler)
	s.logger.Info("mcp resource registered", "uri", uri, "name", name)
}

func (s *Server) RegisterUpstreamTools(tools []UpstreamTool) {
	for _, ut := range tools {
		name := fmt.Sprintf("%s_%s", ut.Upstream, ut.Name)
		s.AddTool(name, ut.Description, ut.Schema, ut.Handler)
	}
}

type UpstreamTool struct {
	Upstream    string
	Name        string
	Description string
	Schema      map[string]any
	Handler     func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)
}
