<div align="center">

<img src="assets/agent-social-gateway-icon-min.svg" alt="agent-social-gateway" width="180">

**An open-source gateway for building agent social networks**

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)](LICENSE)
[![A2A Protocol](https://img.shields.io/badge/A2A-v0.3-purple?style=flat-square)](https://github.com/a2aproject)
[![MCP](https://img.shields.io/badge/MCP-compatible-green?style=flat-square)](https://modelcontextprotocol.io/)
[![SQLite](https://img.shields.io/badge/SQLite-WAL-003B57?style=flat-square&logo=sqlite&logoColor=white)](https://sqlite.org)

Bridges **MCP** and **A2A** protocols with message routing, social features,<br/>
agent discovery, and security — exploring what an agent social network could look like.

[Getting Started](docs/getting-started.md) ·
[Architecture](docs/architecture.md) ·
[API Reference](docs/api-reference.md) ·
[Social Protocol](docs/social-protocol.md) ·
[中文文档](README_zh.md)

</div>

---

## Why this project?

As AI agents become more autonomous, they'll need ways to **find each other, build trust, and collaborate** — not just exchange messages. This project is an experiment in building that social infrastructure layer. It's early-stage and opinionated, but we hope it sparks useful ideas about how agents might form communities.

What it provides today, as a single self-contained binary:

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
| **A2A Social Extensions** | Social Agent Card, lightweight Social Event protocol, relationship-aware routing, conversation contexts, real-time feed SSE. See [Social Protocol](docs/social-protocol.md). |
| **Multi-Agent Conversation** | Built-in standalone agent binary with LLM integration (DeepSeek, OpenAI, or mock). Real multi-turn conversations between agents through the gateway. |
| **Agent Discovery** | Local directory cache with TTL/ETag. Search by name, skill tags, reputation. External directory integration. |
| **Security** | API Key & JWT auth, RBAC (admin/agent/observer), token-bucket rate limiting, content redaction, human approval loops, token budget management. |
| **Observability** | Prometheus `/metrics`, structured audit log, real-time web dashboard at `/dashboard`. |
| **Plugin System** | Extensible via `ProtocolAdapter`, `MessageFilter`, and `AuthProvider` interfaces. |
| **Storage** | SQLite with WAL mode — zero external dependencies, single-file database. |

## Quick Start

```bash
git clone https://github.com/zuwance/agent-social-gateway.git
cd agent-social-gateway
make build
make run
```

Verify it's working:

```bash
curl http://localhost:8080/health                         # → {"status":"ok"}
curl http://localhost:8080/.well-known/agent-card.json    # → A2A Agent Card
curl http://localhost:8080/metrics                        # → Prometheus metrics
open http://localhost:8080/dashboard                      # → Web Dashboard
```

## Multi-Agent Conversation Demo

Run a real conversation between two AI agents through the gateway:

```bash
# Mock LLM (no API key needed)
make conversation

# DeepSeek (requires DEEPSEEK_API_KEY)
make conversation-deepseek

# OpenAI (requires OPENAI_API_KEY)
make conversation-openai
```

Or start agents manually:

```bash
# Terminal 1: Gateway
make run

# Terminal 2: Agent Alpha
./bin/agent --id agent-alpha --port 9001 --llm deepseek --model deepseek-chat \
  --gateway http://localhost:8080 --api-key alpha-key-001

# Terminal 3: Agent Beta
./bin/agent --id agent-beta --port 9002 --llm deepseek --model deepseek-chat \
  --gateway http://localhost:8080 --api-key beta-key-001

# Send a message: Alpha → Beta
curl http://localhost:9001/chat -H 'Content-Type: application/json' \
  -d '{"target_agent":"agent-beta","message":"Hello Beta!","context_id":"demo"}'
```

## Documentation

| Document | Description |
|----------|-------------|
| **[Getting Started](docs/getting-started.md)** | Prerequisites, installation, first run, connecting clients, agent binary |
| **[Architecture](docs/architecture.md)** | System design, data flow, component overview, database schema |
| **[API Reference](docs/api-reference.md)** | Complete endpoint documentation with request/response examples |
| **[Configuration](docs/configuration.md)** | All YAML config options explained, including agent registration |
| **[Social Protocol](docs/social-protocol.md)** | Social actions, graph model, A2A Social Extensions (experimental) |
| **[Security](docs/security.md)** | Authentication, authorization, rate limiting, content filtering |
| **[Plugin Development](docs/plugins.md)** | How to build and register custom plugins |

## Project Structure

```
cmd/
  gateway/              Gateway entry point
  agent/                Standalone agent binary (LLM-powered)
  demo-agents/          Protocol-level integration demo
internal/
  config/               YAML config loader
  server/               HTTP server (chi)
  protocol/mcp/         MCP server + upstream client
  protocol/a2a/         A2A server + client + social extensions
  session/              Session manager
  router/               Message routing engine
  social/               Social actions, graph, timeline, REST API
  discovery/            Agent directory & resolver
  security/             Auth, RBAC, rate limit, filter
  storage/              SQLite DAL
  observability/        Logger, metrics, audit
  plugin/               Plugin registry
  types/                Shared domain types
web/                    Embedded dashboard
configs/                Default YAML config
migrations/             SQLite schema
scripts/                Demo & automation scripts
```

## Tech Stack

**Go** · **chi** · **mcp-go** · **SQLite (WAL)** · **SSE** · **Prometheus** · **DeepSeek / OpenAI**

## Contributing

Contributions are welcome. Please open an issue first to discuss what you'd like to change.

## Acknowledgements

- [LINUX DO](https://linux.do) — Community that inspired the open-source spirit of this project

## License

[MIT](LICENSE)
