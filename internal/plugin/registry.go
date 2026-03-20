package plugin

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

type PluginType string

const (
	TypeProtocolAdapter PluginType = "protocol_adapter"
	TypeMessageFilter   PluginType = "message_filter"
	TypeAuthProvider    PluginType = "auth_provider"
)

// Plugin is the interface all plugins must implement.
type Plugin interface {
	Name() string
	Type() PluginType
	Init(config map[string]interface{}) error
	Close() error
}

// ProtocolAdapter extends Protocol support (e.g., new transport or protocol binding).
type ProtocolAdapter interface {
	Plugin
	HandleRequest(ctx context.Context, req []byte) ([]byte, error)
}

// MessageFilter can inspect and transform messages flowing through the router.
type MessageFilter interface {
	Plugin
	Filter(ctx context.Context, msg []byte) ([]byte, bool, error) // returns (modified msg, allow, error)
}

// AuthProvider provides custom authentication logic.
type AuthProvider interface {
	Plugin
	Authenticate(ctx context.Context, credentials map[string]string) (agentID string, role string, err error)
}

type Registry struct {
	plugins  map[string]Plugin
	byType   map[PluginType][]Plugin
	mu       sync.RWMutex
	logger   *slog.Logger
}

func NewRegistry(logger *slog.Logger) *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
		byType:  make(map[PluginType][]Plugin),
		logger:  logger.With("component", "plugin-registry"),
	}
}

func (r *Registry) Register(p Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[p.Name()]; exists {
		return fmt.Errorf("plugin %q already registered", p.Name())
	}

	r.plugins[p.Name()] = p
	r.byType[p.Type()] = append(r.byType[p.Type()], p)
	r.logger.Info("plugin registered", "name", p.Name(), "type", p.Type())
	return nil
}

func (r *Registry) Init(name string, config map[string]interface{}) error {
	r.mu.RLock()
	p, ok := r.plugins[name]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	return p.Init(config)
}

func (r *Registry) InitAll(configs map[string]map[string]interface{}) error {
	for name, cfg := range configs {
		if err := r.Init(name, cfg); err != nil {
			return fmt.Errorf("initializing plugin %q: %w", name, err)
		}
	}
	return nil
}

func (r *Registry) Get(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

func (r *Registry) GetByType(t PluginType) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Plugin, len(r.byType[t]))
	copy(result, r.byType[t])
	return result
}

func (r *Registry) GetMessageFilters() []MessageFilter {
	plugins := r.GetByType(TypeMessageFilter)
	filters := make([]MessageFilter, 0, len(plugins))
	for _, p := range plugins {
		if f, ok := p.(MessageFilter); ok {
			filters = append(filters, f)
		}
	}
	return filters
}

func (r *Registry) GetAuthProviders() []AuthProvider {
	plugins := r.GetByType(TypeAuthProvider)
	providers := make([]AuthProvider, 0, len(plugins))
	for _, p := range plugins {
		if ap, ok := p.(AuthProvider); ok {
			providers = append(providers, ap)
		}
	}
	return providers
}

func (r *Registry) List() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		result = append(result, p)
	}
	return result
}

func (r *Registry) CloseAll() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for name, p := range r.plugins {
		if err := p.Close(); err != nil {
			r.logger.Warn("error closing plugin", "name", name, "error", err)
		}
	}
}
