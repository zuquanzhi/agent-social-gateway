# Plugin Development

[English](#overview) | [中文](#概述-1)

---

## Overview

agent-social-gateway supports three plugin types for extending its capabilities without modifying core code.

## Plugin Interface

Every plugin must implement the base interface:

```go
type Plugin interface {
    Name() string
    Type() PluginType
    Init(config map[string]interface{}) error
    Close() error
}
```

## Plugin Types

### ProtocolAdapter

Add support for new transport protocols or protocol bindings.

```go
type ProtocolAdapter interface {
    Plugin
    HandleRequest(ctx context.Context, req []byte) ([]byte, error)
}
```

**Use cases:**
- WebSocket transport adapter
- gRPC binding for A2A
- Custom binary protocol support

### MessageFilter

Inspect and transform messages as they flow through the routing engine.

```go
type MessageFilter interface {
    Plugin
    Filter(ctx context.Context, msg []byte) ([]byte, bool, error)
    // Returns: (modified message, allow, error)
    // If allow is false, the message is dropped.
}
```

**Use cases:**
- Content moderation (profanity filter, toxicity detection)
- Message transformation (format conversion, language translation)
- Logging / analytics hooks
- Compliance filtering

### AuthProvider

Implement custom authentication mechanisms.

```go
type AuthProvider interface {
    Plugin
    Authenticate(ctx context.Context, credentials map[string]string) (agentID string, role string, err error)
}
```

**Use cases:**
- LDAP/Active Directory authentication
- Custom OAuth2 provider
- Certificate-based authentication
- Blockchain wallet authentication

## Registering Plugins

```go
registry := plugin.NewRegistry(logger)

// Register a plugin
myFilter := &MyContentFilter{}
registry.Register(myFilter)

// Initialize with config
registry.Init("my-filter", map[string]interface{}{
    "block_patterns": []string{"spam", "phishing"},
})

// Query plugins by type
filters := registry.GetMessageFilters()
for _, f := range filters {
    msg, allow, err := f.Filter(ctx, rawMessage)
    if !allow {
        // message blocked
    }
}

// Cleanup on shutdown
registry.CloseAll()
```

## Example: Content Moderation Filter

```go
package myplugin

import (
    "context"
    "strings"
    "github.com/zuwance/agent-social-gateway/internal/plugin"
)

type ModerationFilter struct {
    blockedWords []string
}

func (f *ModerationFilter) Name() string              { return "moderation-filter" }
func (f *ModerationFilter) Type() plugin.PluginType   { return plugin.TypeMessageFilter }

func (f *ModerationFilter) Init(config map[string]interface{}) error {
    if words, ok := config["blocked_words"].([]string); ok {
        f.blockedWords = words
    }
    return nil
}

func (f *ModerationFilter) Close() error { return nil }

func (f *ModerationFilter) Filter(ctx context.Context, msg []byte) ([]byte, bool, error) {
    text := string(msg)
    for _, word := range f.blockedWords {
        if strings.Contains(strings.ToLower(text), word) {
            return nil, false, nil // block the message
        }
    }
    return msg, true, nil // allow the message
}
```

## Registry API

| Method | Description |
|--------|-------------|
| `Register(plugin)` | Add plugin to registry |
| `Init(name, config)` | Initialize a specific plugin |
| `InitAll(configs)` | Initialize all plugins |
| `Get(name)` | Get plugin by name |
| `GetByType(type)` | Get all plugins of a type |
| `GetMessageFilters()` | Get all message filter plugins |
| `GetAuthProviders()` | Get all auth provider plugins |
| `List()` | List all registered plugins |
| `CloseAll()` | Shutdown all plugins |

---

# 中文

## 概述

agent-social-gateway 支持三种插件类型，无需修改核心代码即可扩展功能。

## 插件类型

### ProtocolAdapter（协议适配器）

为新的传输协议或协议绑定提供支持。

```go
type ProtocolAdapter interface {
    Plugin
    HandleRequest(ctx context.Context, req []byte) ([]byte, error)
}
```

**应用场景**：WebSocket 传输、gRPC 绑定、自定义二进制协议

### MessageFilter（消息过滤器）

在消息通过路由引擎时进行检查和转换。

```go
type MessageFilter interface {
    Plugin
    Filter(ctx context.Context, msg []byte) ([]byte, bool, error)
    // 返回：(修改后的消息, 是否放行, 错误)
}
```

**应用场景**：内容审核、消息转换、合规过滤

### AuthProvider（认证提供者）

实现自定义认证机制。

```go
type AuthProvider interface {
    Plugin
    Authenticate(ctx context.Context, credentials map[string]string) (agentID string, role string, err error)
}
```

**应用场景**：LDAP 认证、自定义 OAuth2、证书认证

## 注册插件

```go
registry := plugin.NewRegistry(logger)
registry.Register(&MyPlugin{})
registry.Init("my-plugin", config)
```

## 注册表 API

| 方法 | 说明 |
|------|------|
| `Register(plugin)` | 注册插件 |
| `Init(name, config)` | 初始化指定插件 |
| `GetMessageFilters()` | 获取所有消息过滤器 |
| `GetAuthProviders()` | 获取所有认证提供者 |
| `CloseAll()` | 关闭所有插件 |
