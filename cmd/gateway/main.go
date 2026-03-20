package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/zuwance/agent-social-gateway/internal/config"
	"github.com/zuwance/agent-social-gateway/internal/discovery"
	"github.com/zuwance/agent-social-gateway/internal/observability"
	"github.com/zuwance/agent-social-gateway/internal/plugin"
	"github.com/zuwance/agent-social-gateway/internal/protocol/a2a"
	mcpproto "github.com/zuwance/agent-social-gateway/internal/protocol/mcp"
	"github.com/zuwance/agent-social-gateway/internal/security"
	"github.com/zuwance/agent-social-gateway/internal/server"
	"github.com/zuwance/agent-social-gateway/internal/session"
	"github.com/zuwance/agent-social-gateway/internal/social"
	"github.com/zuwance/agent-social-gateway/internal/storage"
	"github.com/zuwance/agent-social-gateway/web"
)

func main() {
	configPath := flag.String("config", "configs/gateway.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger := observability.NewLogger(cfg.Log.Level, cfg.Log.Format)
	slog.SetDefault(logger)

	logger.Info("starting agent-social-gateway",
		"version", cfg.A2A.Agent.Version,
		"config", *configPath,
	)

	db, err := storage.New(cfg.Storage.DSN, logger)
	if err != nil {
		logger.Error("failed to connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.RunMigrations(cfg.Storage.MigrationsPath); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	srv := server.New(cfg, logger)
	registerAll(srv, cfg, db, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	select {
	case err := <-errCh:
		if err != nil {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown error", "error", err)
		}
	}

	logger.Info("agent-social-gateway stopped")
}

func registerAll(srv *server.Server, cfg *config.Config, db *storage.DB, logger *slog.Logger) {
	r := srv.Router()
	metrics := observability.NewMetrics()
	auditLog := observability.NewAuditLogger(db, logger)

	// All middleware MUST be registered before any routes (chi requirement)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(sseAwareTimeout(60 * time.Second))
	r.Use(metrics.MetricsMiddleware)

	authn := security.NewAuthenticator(&cfg.Security.Auth, logger)
	rateLimiter := security.NewRateLimiter(&cfg.Security.RateLimit)
	r.Use(authn.Middleware)
	r.Use(rateLimiter.Middleware)

	// Now register routes (health, etc.)
	srv.SetupRoutes()

	// Session manager (Phase 4)
	sessionMgr := session.NewManager(db, logger.With("component", "session"))

	// Metrics endpoint (Phase 9)
	r.Get("/metrics", metrics.PrometheusHandler())
	r.Get("/metrics/json", metrics.JSONHandler())

	// MCP Server + Client (Phase 2)
	var mcpSrv *mcpproto.Server
	if cfg.MCP.Server.Enabled {
		host := cfg.Server.Host
		if host == "" || host == "0.0.0.0" {
			host = "localhost"
		}
		baseURL := fmt.Sprintf("http://%s:%d", host, cfg.Server.Port)
		mcpSrv = mcpproto.NewServer(&cfg.MCP.Server, baseURL, logger.With("component", "mcp-server"))
		r.Handle(cfg.MCP.Server.SSEEndpoint+"/*", mcpSrv.SSEHandler())
		r.Handle(cfg.MCP.Server.SSEEndpoint, mcpSrv.SSEHandler())

		upstreamMgr := mcpproto.NewUpstreamManager(logger.With("component", "mcp-upstream"))
		if len(cfg.MCP.Upstream) > 0 {
			if err := upstreamMgr.ConnectAll(cfg.MCP.Upstream); err != nil {
				logger.Warn("some upstream connections failed", "error", err)
			}
			mcpSrv.RegisterUpstreamTools(upstreamMgr.GetAggregatedTools())
		}
		logger.Info("mcp server registered", "endpoint", cfg.MCP.Server.SSEEndpoint)
	}

	// Social features (Phase 6)
	socialActions := social.NewActions(db, logger)
	timeline := social.NewTimeline(db, logger)
	if mcpSrv != nil {
		social.RegisterMCPTools(mcpSrv, socialActions, timeline, logger)
	}
	socialAPI := social.NewAPI(socialActions, timeline, logger)
	socialAPI.RegisterRoutes(r)
	logger.Info("social api registered")

	// A2A Server (Phase 3) + Social Extensions (Layers 1-5)
	if cfg.A2A.Enabled {
		taskStore := a2a.NewTaskStore(db)
		a2aSrv := a2a.NewServer(&cfg.A2A, taskStore, cfg.Agents, logger.With("component", "a2a-server"))
		a2aSrv.RegisterRoutes(r)

		socialExt := a2a.NewSocialExtensions(db, socialActions, timeline, logger)
		a2aSrv.SetSocialExtensions(socialExt)
		socialExt.RegisterRoutes(r)

		logger.Info("a2a server registered", "agent", cfg.A2A.Agent.Name, "social_extensions", true)
	}

	// Discovery service (Phase 7)
	dirCache := discovery.NewCache(db, logger)
	resolver := discovery.NewResolver(dirCache, logger)
	discoveryAPI := discovery.NewAPI(dirCache, resolver, logger)
	discoveryAPI.RegisterRoutes(r)
	logger.Info("discovery api registered")

	// Web Dashboard (Phase 9)
	dashboard := web.NewDashboard(metrics, auditLog, sessionMgr, socialActions, logger)
	dashboard.RegisterRoutes(r)
	logger.Info("dashboard registered", "path", "/dashboard")

	// Plugin registry (Phase 10)
	pluginReg := plugin.NewRegistry(logger)
	_ = pluginReg
	logger.Info("plugin registry initialized")

	// Log audit entry for startup
	auditLog.Log("", "system", "", "gateway_started", map[string]string{
		"version": cfg.A2A.Agent.Version,
	})
}

func sseAwareTimeout(timeout time.Duration) func(http.Handler) http.Handler {
	timeoutMW := middleware.Timeout(timeout)
	return func(next http.Handler) http.Handler {
		wrapped := timeoutMW(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/mcp/sse") || strings.HasSuffix(r.URL.Path, "/feed") {
				next.ServeHTTP(w, r)
				return
			}
			wrapped.ServeHTTP(w, r)
		})
	}
}
