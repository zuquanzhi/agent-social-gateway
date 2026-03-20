package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/zuwance/agent-social-gateway/internal/types"
)

type Resolver struct {
	cache      *Cache
	httpClient *http.Client
	logger     *slog.Logger
	extDirs    []string // external directory service URLs
}

func NewResolver(cache *Cache, logger *slog.Logger) *Resolver {
	return &Resolver{
		cache:      cache,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger.With("component", "resolver"),
	}
}

func (r *Resolver) SetExternalDirectories(urls []string) {
	r.extDirs = urls
}

func (r *Resolver) Resolve(ctx context.Context, agentURL string) (*types.AgentCard, error) {
	// Check cache first
	if card, _, ok := r.cache.Get(agentURL); ok {
		return card, nil
	}

	// Fetch from /.well-known/agent-card.json
	card, etag, err := r.fetchAgentCard(ctx, agentURL)
	if err != nil {
		// Try external directories
		for _, dir := range r.extDirs {
			card, err = r.queryExternalDirectory(ctx, dir, agentURL)
			if err == nil {
				r.cache.Put(agentURL, card, "")
				return card, nil
			}
		}
		return nil, fmt.Errorf("failed to resolve agent %s: %w", agentURL, err)
	}

	r.cache.Put(agentURL, card, etag)
	return card, nil
}

func (r *Resolver) fetchAgentCard(ctx context.Context, baseURL string) (*types.AgentCard, string, error) {
	cardURL := strings.TrimSuffix(baseURL, "/") + "/.well-known/agent-card.json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cardURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/json")

	// Check if we have an etag for conditional request
	if _, etag, _ := r.cache.Get(baseURL); etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		if card, etag, ok := r.cache.Get(baseURL); ok {
			return card, etag, nil
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("status %d", resp.StatusCode)
	}

	var card types.AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, "", err
	}

	etag := resp.Header.Get("ETag")
	return &card, etag, nil
}

func (r *Resolver) queryExternalDirectory(ctx context.Context, dirURL, agentURL string) (*types.AgentCard, error) {
	endpoint := fmt.Sprintf("%s/agents?url=%s", strings.TrimSuffix(dirURL, "/"), agentURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("directory returned status %d", resp.StatusCode)
	}

	var card types.AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, err
	}
	return &card, nil
}
