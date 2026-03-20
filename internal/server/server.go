package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/zuwance/agent-social-gateway/internal/config"
)

type Server struct {
	cfg    *config.Config
	router chi.Router
	http   *http.Server
	logger *slog.Logger
}

func New(cfg *config.Config, logger *slog.Logger) *Server {
	r := chi.NewRouter()

	s := &Server{
		cfg:    cfg,
		router: r,
		logger: logger,
	}

	return s
}

// SetupRoutes must be called after all middleware is registered via Router().Use().
func (s *Server) SetupRoutes() {
	s.router.Get("/health", s.handleHealth)
}

func (s *Server) Router() chi.Router {
	return s.router
}

func (s *Server) Start() error {
	addr := s.cfg.Server.Addr()
	s.http = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // SSE needs no write timeout
		IdleTimeout:  120 * time.Second,
	}

	s.logger.Info("server starting", "addr", addr)

	if s.cfg.Server.TLS.Enabled {
		return s.http.ListenAndServeTLS(
			s.cfg.Server.TLS.CertFile,
			s.cfg.Server.TLS.KeyFile,
		)
	}
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("server shutting down")
	return s.http.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"ok"}`)
}

func slogMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.Debug("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration_ms", time.Since(start).Milliseconds(),
				"bytes", ww.BytesWritten(),
			)
		})
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
