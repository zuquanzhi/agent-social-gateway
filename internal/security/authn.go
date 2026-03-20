package security

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/zuwance/agent-social-gateway/internal/config"
	"github.com/zuwance/agent-social-gateway/internal/types"
)

type contextKey string

const (
	AgentIDKey contextKey = "agent_id"
	RoleKey    contextKey = "role"
)

type Authenticator struct {
	cfg    *config.AuthConfig
	logger *slog.Logger
}

func NewAuthenticator(cfg *config.AuthConfig, logger *slog.Logger) *Authenticator {
	return &Authenticator{cfg: cfg, logger: logger.With("component", "authn")}
}

func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.cfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Skip auth for well-known and health endpoints
		if r.URL.Path == "/health" || r.URL.Path == "/.well-known/agent-card.json" {
			next.ServeHTTP(w, r)
			return
		}

		agentID, role, ok := a.authenticate(r)
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="agent-social-gateway"`)
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), AgentIDKey, agentID)
		ctx = context.WithValue(ctx, RoleKey, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Authenticator) authenticate(r *http.Request) (types.AgentID, string, bool) {
	// Try API key first
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		for _, key := range a.cfg.APIKeys {
			if apiKey == key {
				return types.AgentID("apikey-agent"), "agent", true
			}
		}
		return "", "", false
	}

	// Try Bearer token (JWT)
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		if a.cfg.JWT.Enabled {
			agentID, role, err := validateJWT(token, a.cfg.JWT.Secret)
			if err != nil {
				a.logger.Debug("jwt validation failed", "error", err)
				return "", "", false
			}
			return agentID, role, true
		}
		// Fallback: treat token as agent ID for simple setups
		return types.AgentID(token), "agent", true
	}

	return "", "", false
}

func GetAgentID(ctx context.Context) types.AgentID {
	v, _ := ctx.Value(AgentIDKey).(types.AgentID)
	return v
}

func GetRole(ctx context.Context) string {
	v, _ := ctx.Value(RoleKey).(string)
	return v
}

// Minimal JWT validation (for demo; production should use a proper JWT library)
func validateJWT(token, secret string) (types.AgentID, string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", "", ErrInvalidToken
	}
	// In production, decode and verify the JWT signature using the secret.
	// For now, extract a simple agent ID from the token.
	return types.AgentID("jwt-agent"), "agent", nil
}
