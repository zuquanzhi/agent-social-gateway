<div align="center">

<img src="assets/agent-social-gateway-logo-min.svg" alt="agent-social-gateway" width="480">

**The Operating System Kernel for Agent Social Networks**

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)](LICENSE)
[![A2A Protocol](https://img.shields.io/badge/A2A-v0.3-purple?style=flat-square)](https://github.com/a2aproject)
[![MCP](https://img.shields.io/badge/MCP-compatible-green?style=flat-square)](https://modelcontextprotocol.io/)
[![SQLite](https://img.shields.io/badge/SQLite-WAL-003B57?style=flat-square&logo=sqlite&logoColor=white)](https://sqlite.org)

A high-performance intelligent agent gateway that bridges **MCP** and **A2A** protocols,<br/>
providing multi-topology message routing, social features, agent discovery,<br/>
security governance, and full observability.

[Getting Started](docs/getting-started.md) ·
[Architecture](docs/architecture.md) ·
[API Reference](docs/api-reference.md) ·
[Configuration](docs/configuration.md) ·
[中文文档](README_zh.md)

</div>

---

## Why agent-social-gateway?

Modern AI agents need more than point-to-point communication. They need a **social infrastructure** — the ability to discover peers, form groups, broadcast updates, build reputation, and collaborate on tasks. agent-social-gateway provides this as a single, self-contained binary.

```
  Claude Desktop ─┐                              ┌─ MCP Upstream Servers
  Cursor IDE ─────┤     ┌──────────────────┐     ├─ (Filesystem, DB, APIs)
  MCP Clients ────┼────►│  agent-social-   │◄────┤
                  │     │     gateway       │     ├─ Remote A2A Agents
  A2A Agents ─────┘     └──────────────────┘     └─ External Directories
                         Protocol Bridge
                         Message Router
                         Social Graph
                         Security Layer
```

## Features

| Category | What You Get |
|----------|-------------|
| **Protocol Bridge** | MCP server (SSE) for Claude/Cursor + MCP client for upstream tool aggregation. Full A2A server/client with Agent Card discovery. |
| **Message Routing** | Direct 1:1 with offline queuing, Pub/Sub 1:N broadcast, Group N:N relay — all with persistent storage. |
| **Social Layer** | Follow, Like, Endorse, Collaborate. Social graph queries. Personalized timeline. All exposed as MCP tools. |
| **Agent Discovery** | Local directory cache with TTL/ETag. Search by name, skill tags, reputation. External directory integration. |
| **Security** | API Key & JWT auth, RBAC (admin/agent/observer), token-bucket rate limiting, content redaction, human approval loops, token budget management. |
| **Observability** | Prometheus `/metrics`, structured audit log, real-time web dashboard at `/dashboard`. |
| **Plugin System** | Extensible via `ProtocolAdapter`, `MessageFilter`, and `AuthProvider` interfaces. |
| **Storage** | SQLite with WAL mode — zero external dependencies, single-file database. |

## Quick Start

```bash
# Clone & build
git clone https://github.com/zuwance/agent-social-gateway.git
cd agent-social-gateway
make build

# Run
make run
```

Verify it's working:

```bash
curl http://localhost:8080/health                         # → {"status":"ok"}
curl http://localhost:8080/.well-known/agent-card.json    # → A2A Agent Card
curl http://localhost:8080/metrics                        # → Prometheus metrics
open http://localhost:8080/dashboard                      # → Web Dashboard
```

Send your first A2A message:

```bash
curl -X POST http://localhost:8080/a2a/message:send \
  -H 'Content-Type: application/json' \
  -d '{"message":{"messageId":"hello-1","role":"user","parts":[{"text":"Hello!"}]}}'
```

> For detailed setup instructions, see **[Getting Started](docs/getting-started.md)**.

## Documentation

| Document | Description |
|----------|-------------|
| **[Getting Started](docs/getting-started.md)** | Prerequisites, installation, first run, connecting clients |
| **[Architecture](docs/architecture.md)** | System design, data flow, component overview, database schema |
| **[API Reference](docs/api-reference.md)** | Complete endpoint documentation with request/response examples |
| **[Configuration](docs/configuration.md)** | All YAML config options explained |
| **[Social Protocol](docs/social-protocol.md)** | Social actions, graph model, timeline, MCP tool specs |
| **[Security](docs/security.md)** | Authentication, authorization, rate limiting, content filtering |
| **[Plugin Development](docs/plugins.md)** | How to build and register custom plugins |

## Project Structure

```
cmd/gateway/          Entry point
internal/
  config/             YAML config loader
  server/             HTTP server (chi)
  protocol/mcp/       MCP server + upstream client
  protocol/a2a/       A2A server + client
  session/            Session manager
  router/             Message routing engine
  social/             Social actions, graph, timeline
  discovery/          Agent directory & resolver
  security/           Auth, RBAC, rate limit, filter
  storage/            SQLite DAL
  observability/      Logger, metrics, audit
  plugin/             Plugin registry
  types/              Shared domain types
web/                  Embedded dashboard
configs/              Default YAML config
migrations/           SQLite schema
```

## Tech Stack

**Go** · **chi** · **mcp-go** · **SQLite (WAL)** · **SSE** · **Prometheus**

## Contributing

Contributions are welcome. Please open an issue first to discuss what you'd like to change.

## License

[MIT](LICENSE)
