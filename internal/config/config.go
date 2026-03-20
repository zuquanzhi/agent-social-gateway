package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig        `yaml:"server"`
	Storage  StorageConfig       `yaml:"storage"`
	A2A      A2AConfig           `yaml:"a2a"`
	MCP      MCPConfig           `yaml:"mcp"`
	Security SecurityConfig      `yaml:"security"`
	Log      LogConfig           `yaml:"log"`
	Agents   []AgentRegistration `yaml:"agents"`
}

type AgentRegistration struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	URL    string `yaml:"url"`
	APIKey string `yaml:"api_key"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	TLS  struct {
		Enabled  bool   `yaml:"enabled"`
		CertFile string `yaml:"cert_file"`
		KeyFile  string `yaml:"key_file"`
	} `yaml:"tls"`
}

func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

type StorageConfig struct {
	DSN            string `yaml:"dsn"`
	MigrationsPath string `yaml:"migrations_path"`
}

type A2AConfig struct {
	Agent   A2AAgentConfig   `yaml:"agent"`
	Enabled bool             `yaml:"enabled"`
}

type A2AAgentConfig struct {
	Name               string             `yaml:"name"`
	Description        string             `yaml:"description"`
	Version            string             `yaml:"version"`
	URL                string             `yaml:"url"`
	DocumentationURL   string             `yaml:"documentation_url"`
	ProtocolVersion    string             `yaml:"protocol_version"`
	DefaultInputModes  []string           `yaml:"default_input_modes"`
	DefaultOutputModes []string           `yaml:"default_output_modes"`
	Provider           *A2AProviderConfig `yaml:"provider"`
	Skills             []A2ASkillConfig   `yaml:"skills"`
	Capabilities       A2ACapConfig       `yaml:"capabilities"`
}

type A2AProviderConfig struct {
	URL          string `yaml:"url"`
	Organization string `yaml:"organization"`
}

type A2ASkillConfig struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
	Examples    []string `yaml:"examples"`
}

type A2ACapConfig struct {
	Streaming         bool `yaml:"streaming"`
	PushNotifications bool `yaml:"push_notifications"`
	ExtendedAgentCard bool `yaml:"extended_agent_card"`
}

type MCPConfig struct {
	Server   MCPServerConfig    `yaml:"server"`
	Upstream []MCPUpstreamConfig `yaml:"upstream"`
}

type MCPServerConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	SSEEndpoint string `yaml:"sse_endpoint"`
}

type MCPUpstreamConfig struct {
	Name      string `yaml:"name"`
	URL       string `yaml:"url"`
	Transport string `yaml:"transport"` // "sse" or "stdio"
	Command   string `yaml:"command"`   // for stdio transport
	Args      []string `yaml:"args"`
}

type SecurityConfig struct {
	Auth       AuthConfig       `yaml:"auth"`
	RateLimit  RateLimitConfig  `yaml:"rate_limit"`
	TokenBudget TokenBudgetConfig `yaml:"token_budget"`
}

type AuthConfig struct {
	Enabled  bool             `yaml:"enabled"`
	APIKeys  []string         `yaml:"api_keys"`
	JWT      JWTConfig        `yaml:"jwt"`
}

type JWTConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Secret    string `yaml:"secret"`
	Issuer    string `yaml:"issuer"`
}

type RateLimitConfig struct {
	Enabled       bool `yaml:"enabled"`
	RequestsPerMin int  `yaml:"requests_per_min"`
	BroadcastsPerMin int `yaml:"broadcasts_per_min"`
}

type TokenBudgetConfig struct {
	Enabled          bool `yaml:"enabled"`
	MaxTokensPerTask int  `yaml:"max_tokens_per_task"`
	AlertThreshold   float64 `yaml:"alert_threshold"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"` // "json" or "text"
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	setDefaults(cfg)
	return cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Storage.DSN == "" {
		cfg.Storage.DSN = "gateway.db"
	}
	if cfg.Storage.MigrationsPath == "" {
		cfg.Storage.MigrationsPath = "migrations"
	}
	if cfg.A2A.Agent.ProtocolVersion == "" {
		cfg.A2A.Agent.ProtocolVersion = "0.3"
	}
	if len(cfg.A2A.Agent.DefaultInputModes) == 0 {
		cfg.A2A.Agent.DefaultInputModes = []string{"text/plain"}
	}
	if len(cfg.A2A.Agent.DefaultOutputModes) == 0 {
		cfg.A2A.Agent.DefaultOutputModes = []string{"text/plain"}
	}
	if cfg.MCP.Server.SSEEndpoint == "" {
		cfg.MCP.Server.SSEEndpoint = "/mcp/sse"
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}
	if cfg.Security.RateLimit.RequestsPerMin == 0 {
		cfg.Security.RateLimit.RequestsPerMin = 60
	}
	if cfg.Security.RateLimit.BroadcastsPerMin == 0 {
		cfg.Security.RateLimit.BroadcastsPerMin = 10
	}
}
