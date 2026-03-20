# Security

[English](#authentication) | [中文](#认证)

---

## Authentication

The gateway supports multiple authentication methods, all configurable via `security.auth` in `gateway.yaml`.

> By default, authentication is **disabled** for local development. **Enable it in production.**

### API Key

Send the key in the `X-API-Key` header:

```bash
curl -H 'X-API-Key: your-key-here' http://localhost:8080/a2a/tasks
```

Configure valid keys:

```yaml
security:
  auth:
    enabled: true
    api_keys:
      - "key-agent-alpha"
      - "key-agent-beta"
```

### Bearer Token (JWT)

Send a JWT in the `Authorization` header:

```bash
curl -H 'Authorization: Bearer eyJhbG...' http://localhost:8080/a2a/tasks
```

Configure JWT:

```yaml
security:
  auth:
    enabled: true
    jwt:
      enabled: true
      secret: "your-signing-secret"
      issuer: "your-issuer"
```

### Unauthenticated Endpoints

These endpoints are always accessible without authentication:

- `GET /health`
- `GET /.well-known/agent-card.json`

## Authorization (RBAC)

Three built-in roles with ascending privileges:

| Role | Permissions |
|------|------------|
| `observer` | `read:agents`, `read:messages` |
| `agent` | All of observer + `write:agents`, `write:messages`, `manage:groups`, `broadcast` |
| `admin` | All of agent + `admin` |

Role is extracted from the authentication token. API Key users default to `agent` role.

## Rate Limiting

Token-bucket rate limiting per agent identity (or IP if unauthenticated).

```yaml
security:
  rate_limit:
    enabled: true
    requests_per_min: 120
    broadcasts_per_min: 20
```

When the limit is exceeded, the gateway returns:

```
HTTP 429 Too Many Requests
Retry-After: 60
{"error": "rate limit exceeded"}
```

Buckets refill continuously (not in fixed windows). Inactive buckets are cleaned up every 10 minutes.

## Content Filtering

The built-in content filter automatically detects and redacts:

| Pattern | Example |
|---------|---------|
| API keys / tokens | `api_key: sk-abc123...` → `api_***...123` |
| AWS access keys | `AKIAIOSFODNN7EXAMPLE` → `AKIA***...MPLE` |
| OpenAI keys | `sk-xxxxxxxx...` → `sk-x***...xxxx` |
| Private keys | `-----BEGIN PRIVATE KEY-----` → `[REDACTED]` |

The filter can be used in two modes:
- **Detection**: `ContainsSensitiveData(text) → bool`
- **Redaction**: `Redact(text) → sanitized string`

Custom patterns can be added programmatically via `AddPattern(regex)`.

## Human Approval Loop

Certain sensitive operations can require manual admin approval before execution.

Default sensitive actions:
- `delete_agent`
- `admin_action`
- `bulk_broadcast`

When a sensitive action is triggered:
1. The request is queued in the approval system
2. A channel is returned to the caller for async waiting
3. An admin reviews pending approvals and approves or denies
4. The original request proceeds or is rejected

Configure custom sensitive actions via the `ApprovalQueue.SetSensitiveActions()` API.

## Token Budget Management

Control token consumption per session/task:

```yaml
security:
  token_budget:
    enabled: true
    max_tokens_per_task: 100000
    alert_threshold: 0.8
```

- When usage reaches the alert threshold (default 80%), a warning is logged
- When the budget is exceeded, further consumption is rejected with an error
- Budgets can be reset per session

---

# 中文

## 认证

网关支持多种认证方式，通过 `security.auth` 配置。

> 默认**禁用**认证，便于本地开发。**生产环境请务必启用**。

### API Key

在 `X-API-Key` 请求头中发送密钥：

```bash
curl -H 'X-API-Key: your-key' http://localhost:8080/a2a/tasks
```

### JWT

在 `Authorization` 请求头中发送 Bearer Token：

```bash
curl -H 'Authorization: Bearer eyJhbG...' http://localhost:8080/a2a/tasks
```

## 授权 (RBAC)

三级角色：

| 角色 | 权限 |
|------|------|
| `observer` | 只读智能体和消息 |
| `agent` | observer + 写智能体/消息、管理群组、广播 |
| `admin` | agent + 管理员操作 |

## 速率限制

每智能体独立的令牌桶，持续补充（非固定窗口）。超出返回 `429 Too Many Requests`。

## 内容安全

自动检测并脱敏 API 密钥、AWS 密钥、私钥等敏感信息。支持自定义正则模式。

## 人工审批

预定义的敏感操作（如删除智能体、批量广播）需管理员批准后才能执行。

## Token 预算

按会话/任务设置 Token 消耗上限。达到阈值时告警，超出时拒绝。
