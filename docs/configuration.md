# Configuration

[English](#config-file) | [中文](#配置文件)

---

## Config File

Default path: `configs/gateway.yaml`

Override with the `-config` flag:

```bash
./bin/agent-social-gateway -config /path/to/custom.yaml
```

## Full Reference

### server

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `server.host` | string | `0.0.0.0` | Bind address |
| `server.port` | int | `8080` | Listen port |
| `server.tls.enabled` | bool | `false` | Enable TLS |
| `server.tls.cert_file` | string | — | Path to TLS certificate |
| `server.tls.key_file` | string | — | Path to TLS private key |

### storage

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `storage.dsn` | string | `gateway.db` | SQLite database file path |
| `storage.migrations_path` | string | `migrations` | SQL migration files directory |

### a2a

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `a2a.enabled` | bool | `true` | Enable A2A protocol server |
| `a2a.agent.name` | string | — | Agent display name |
| `a2a.agent.description` | string | — | Agent description |
| `a2a.agent.version` | string | — | Agent version (semver) |
| `a2a.agent.url` | string | — | Agent public URL |
| `a2a.agent.protocol_version` | string | `0.3` | A2A protocol version |
| `a2a.agent.documentation_url` | string | — | Link to agent docs |
| `a2a.agent.default_input_modes` | []string | `["text/plain"]` | Accepted input MIME types |
| `a2a.agent.default_output_modes` | []string | `["text/plain"]` | Output MIME types |
| `a2a.agent.provider.url` | string | — | Provider website |
| `a2a.agent.provider.organization` | string | — | Provider org name |
| `a2a.agent.skills` | []object | — | List of agent skills |
| `a2a.agent.skills[].id` | string | — | Skill identifier |
| `a2a.agent.skills[].name` | string | — | Skill display name |
| `a2a.agent.skills[].description` | string | — | Skill description |
| `a2a.agent.skills[].tags` | []string | — | Search tags |
| `a2a.agent.capabilities.streaming` | bool | `false` | SSE streaming support |
| `a2a.agent.capabilities.push_notifications` | bool | `false` | Push notification support |
| `a2a.agent.capabilities.extended_agent_card` | bool | `false` | Extended card support |

### mcp

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `mcp.server.enabled` | bool | `true` | Enable MCP server |
| `mcp.server.name` | string | — | MCP server name |
| `mcp.server.version` | string | — | MCP server version |
| `mcp.server.sse_endpoint` | string | `/mcp/sse` | SSE endpoint path |
| `mcp.upstream` | []object | `[]` | Upstream MCP servers |
| `mcp.upstream[].name` | string | — | Upstream name (used as tool prefix) |
| `mcp.upstream[].url` | string | — | Upstream URL (for SSE transport) |
| `mcp.upstream[].transport` | string | — | `sse` or `stdio` |
| `mcp.upstream[].command` | string | — | Command for stdio transport |
| `mcp.upstream[].args` | []string | — | Arguments for stdio command |

### security

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `security.auth.enabled` | bool | `false` | Enable authentication |
| `security.auth.api_keys` | []string | `[]` | Valid API keys (use `X-API-Key` header) |
| `security.auth.jwt.enabled` | bool | `false` | Enable JWT validation |
| `security.auth.jwt.secret` | string | — | JWT signing secret |
| `security.auth.jwt.issuer` | string | — | Expected JWT issuer |
| `security.rate_limit.enabled` | bool | `true` | Enable rate limiting |
| `security.rate_limit.requests_per_min` | int | `60` | Max requests per minute per agent |
| `security.rate_limit.broadcasts_per_min` | int | `10` | Max broadcasts per minute per agent |
| `security.token_budget.enabled` | bool | `false` | Enable token budget tracking |
| `security.token_budget.max_tokens_per_task` | int | `100000` | Max tokens per task |
| `security.token_budget.alert_threshold` | float | `0.8` | Alert at this usage ratio |

### agents

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `agents` | []object | `[]` | List of registered agents for message forwarding |
| `agents[].id` | string | — | Agent identifier (e.g. `agent-alpha`) |
| `agents[].name` | string | — | Agent display name |
| `agents[].url` | string | — | Agent's HTTP base URL |
| `agents[].api_key` | string | — | API key for authenticating forwarded messages |

Example:

```yaml
agents:
  - id: agent-alpha
    name: "Research Agent"
    url: "http://localhost:9001"
    api_key: "alpha-key-001"
  - id: agent-beta
    name: "Code Agent"
    url: "http://localhost:9002"
    api_key: "beta-key-001"
```

When a `SendMessage` request includes `metadata.target_agent`, the gateway looks up the target from this registry and forwards the message with the configured API key.

### log

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `log.level` | string | `info` | `debug`, `info`, `warn`, `error` |
| `log.format` | string | `json` | `json` or `text` |

## Example: Production Config

```yaml
server:
  host: "0.0.0.0"
  port: 443
  tls:
    enabled: true
    cert_file: "/etc/ssl/certs/gateway.crt"
    key_file: "/etc/ssl/private/gateway.key"

storage:
  dsn: "/var/lib/gateway/data.db"

a2a:
  enabled: true
  agent:
    name: "Production Gateway"
    version: "1.0.0"
    url: "https://gateway.example.com"

security:
  auth:
    enabled: true
    api_keys: ["${API_KEY_1}", "${API_KEY_2}"]
    jwt:
      enabled: true
      secret: "${JWT_SECRET}"
  rate_limit:
    enabled: true
    requests_per_min: 60
  token_budget:
    enabled: true
    max_tokens_per_task: 50000
    alert_threshold: 0.8

log:
  level: "warn"
  format: "json"
```

---

# 中文

## 配置文件

默认路径：`configs/gateway.yaml`

通过 `-config` 标志覆盖：

```bash
./bin/agent-social-gateway -config /path/to/custom.yaml
```

## 完整参考

### server（服务器）

| 键 | 类型 | 默认值 | 说明 |
|----|------|--------|------|
| `server.host` | string | `0.0.0.0` | 监听地址 |
| `server.port` | int | `8080` | 监听端口 |
| `server.tls.enabled` | bool | `false` | 启用 TLS |

### storage（存储）

| 键 | 类型 | 默认值 | 说明 |
|----|------|--------|------|
| `storage.dsn` | string | `gateway.db` | SQLite 数据库文件路径 |
| `storage.migrations_path` | string | `migrations` | 迁移文件目录 |

### a2a（A2A 协议）

| 键 | 说明 |
|----|------|
| `a2a.enabled` | 是否启用 A2A 服务器 |
| `a2a.agent.name` | 智能体显示名称 |
| `a2a.agent.url` | 智能体公开 URL |
| `a2a.agent.skills` | 技能列表（id、name、description、tags） |
| `a2a.agent.capabilities.*` | 流式、推送通知、扩展名片能力开关 |

### mcp（MCP 协议）

| 键 | 说明 |
|----|------|
| `mcp.server.enabled` | 是否启用 MCP 服务器 |
| `mcp.server.sse_endpoint` | SSE 端点路径 |
| `mcp.upstream` | 上游 MCP 服务器列表（name、url、transport） |

### security（安全）

| 键 | 说明 |
|----|------|
| `security.auth.enabled` | 是否启用认证 |
| `security.auth.api_keys` | 有效 API Key 列表 |
| `security.auth.jwt.enabled` | 是否启用 JWT |
| `security.rate_limit.requests_per_min` | 每分钟最大请求数 |
| `security.token_budget.max_tokens_per_task` | 每任务最大 Token 数 |

### agents（智能体注册）

| 键 | 类型 | 说明 |
|----|------|------|
| `agents` | []object | 注册的智能体列表，用于消息转发 |
| `agents[].id` | string | 智能体标识符 |
| `agents[].name` | string | 显示名称 |
| `agents[].url` | string | HTTP 基础 URL |
| `agents[].api_key` | string | 转发消息时的认证 API Key |

### log（日志）

| 键 | 说明 |
|----|------|
| `log.level` | 日志级别：`debug` / `info` / `warn` / `error` |
| `log.format` | 日志格式：`json` / `text` |
