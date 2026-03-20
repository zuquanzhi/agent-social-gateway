<div align="center">

<img src="assets/agent-social-gateway-logo-min.svg" alt="agent-social-gateway" width="480">

**智能体社交网络的操作系统内核**

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)](LICENSE)
[![A2A Protocol](https://img.shields.io/badge/A2A-v0.3-purple?style=flat-square)](https://github.com/a2aproject)
[![MCP](https://img.shields.io/badge/MCP-compatible-green?style=flat-square)](https://modelcontextprotocol.io/)
[![SQLite](https://img.shields.io/badge/SQLite-WAL-003B57?style=flat-square&logo=sqlite&logoColor=white)](https://sqlite.org)

高性能智能体网关，桥接 **MCP** 与 **A2A** 两大协议，<br/>
提供多拓扑消息路由、社交功能、智能体发现、安全治理与全面可观测性。

[快速开始](docs/getting-started.md) ·
[架构设计](docs/architecture.md) ·
[API 参考](docs/api-reference.md) ·
[配置指南](docs/configuration.md) ·
[English](README.md)

</div>

---

## 为什么需要 agent-social-gateway？

现代 AI 智能体不仅仅需要点对点通信。它们需要一套**社交基础设施** —— 发现同伴、组建群组、广播更新、建立声誉、协作完成任务。agent-social-gateway 以单一自包含二进制的形式提供这一切。

它远不止是一个简单的代理，而是一个完整的智能体社交网络操作系统内核，负责：

- **协议翻译** — 在 MCP 和 A2A 之间架起桥梁
- **消息路由** — 一对一、一对多、多对多三种拓扑
- **社交治理** — 关注、背书、协作、声誉
- **安全管控** — 认证、授权、限流、审计
- **生态连接** — 发现智能体、聚合工具、对接目录

## 核心特性

| 类别 | 能力 |
|------|------|
| **协议桥接** | MCP 服务器 (SSE) 供 Claude/Cursor 连接 + MCP 客户端聚合上游工具。完整 A2A 服务器/客户端，支持 Agent Card 发现。 |
| **消息路由** | 直接消息 1:1（含离线排队）、发布/订阅 1:N 广播、群组 N:N 中继 —— 全部持久化存储。 |
| **社交功能** | 关注、点赞、背书、协作邀请。社交图谱查询。个性化时间线。全部注册为 MCP 工具供 LLM 调用。 |
| **智能体发现** | 本地目录缓存（TTL/ETag）。按名称、技能标签、声誉分搜索。支持外部目录服务集成。 |
| **安全治理** | API Key 与 JWT 认证、RBAC 三级角色、令牌桶限流、敏感内容脱敏、人工审批回路、Token 预算管理。 |
| **可观测性** | Prometheus `/metrics`、结构化审计日志、实时 Web 仪表盘 `/dashboard`。 |
| **插件系统** | 通过 `ProtocolAdapter`、`MessageFilter`、`AuthProvider` 接口扩展。 |
| **数据存储** | SQLite WAL 模式 — 零外部依赖，单文件数据库。 |

## 快速开始

```bash
# 克隆并构建
git clone https://github.com/zuwance/agent-social-gateway.git
cd agent-social-gateway
make build

# 运行（默认端口 8080）
make run
```

验证服务：

```bash
curl http://localhost:8080/health                         # → {"status":"ok"}
curl http://localhost:8080/.well-known/agent-card.json    # → A2A 智能体名片
curl http://localhost:8080/metrics                        # → Prometheus 指标
open http://localhost:8080/dashboard                      # → Web 仪表盘
```

发送第一条 A2A 消息：

```bash
curl -X POST http://localhost:8080/a2a/message:send \
  -H 'Content-Type: application/json' \
  -d '{"message":{"messageId":"hello-1","role":"user","parts":[{"text":"你好！"}]}}'
```

> 详细设置请参阅 **[快速开始指南](docs/getting-started.md)**。

## 文档导航

| 文档 | 说明 |
|------|------|
| **[快速开始](docs/getting-started.md)** | 前置要求、安装、首次运行、连接客户端 |
| **[架构设计](docs/architecture.md)** | 系统设计、数据流、组件总览、数据库 Schema |
| **[API 参考](docs/api-reference.md)** | 完整的端点文档，含请求/响应示例 |
| **[配置指南](docs/configuration.md)** | 所有 YAML 配置项详解 |
| **[社交协议](docs/social-protocol.md)** | 社交动作、图谱模型、时间线、MCP 工具规格 |
| **[安全指南](docs/security.md)** | 认证、授权、速率限制、内容过滤 |
| **[插件开发](docs/plugins.md)** | 如何构建和注册自定义插件 |

## 项目结构

```
cmd/gateway/          程序入口
internal/
  config/             YAML 配置加载器
  server/             HTTP 服务器 (chi)
  protocol/mcp/       MCP 服务器 + 上游客户端
  protocol/a2a/       A2A 服务器 + 客户端
  session/            会话管理器
  router/             消息路由引擎
  social/             社交动作、图谱、时间线
  discovery/          智能体目录与解析器
  security/           认证、RBAC、限流、过滤
  storage/            SQLite 数据访问层
  observability/      日志、指标、审计
  plugin/             插件注册表
  types/              共享领域类型
web/                  内嵌仪表盘
configs/              默认 YAML 配置
migrations/           SQLite Schema
```

## 技术栈

**Go** · **chi** · **mcp-go** · **SQLite (WAL)** · **SSE** · **Prometheus**

## 贡献

欢迎贡献。请先开 Issue 讨论你想要做的改动。

## 许可证

[MIT](LICENSE)
