#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────
# agent-social-gateway — Two-Agent Conversation Demo
#
# Usage:
#   ./scripts/demo-conversation.sh                    # mock LLM
#   ./scripts/demo-conversation.sh --llm deepseek     # DeepSeek
#   ./scripts/demo-conversation.sh --llm openai       # OpenAI
#
# Env vars: DEEPSEEK_API_KEY or OPENAI_API_KEY
# ──────────────────────────────────────────────────────────────────

CYAN='\033[36m'
GREEN='\033[32m'
YELLOW='\033[33m'
BLUE='\033[34m'
PURPLE='\033[35m'
BOLD='\033[1m'
DIM='\033[2m'
RESET='\033[0m'

GATEWAY_URL="http://localhost:8080"
LLM_PROVIDER="mock"
LLM_API_KEY=""
LLM_MODEL="gpt-4o-mini"

while [[ $# -gt 0 ]]; do
  case $1 in
    --llm) LLM_PROVIDER="$2"; shift 2 ;;
    --api-key) LLM_API_KEY="$2"; shift 2 ;;
    --model) LLM_MODEL="$2"; shift 2 ;;
    *) shift ;;
  esac
done

if [[ "$LLM_PROVIDER" == "deepseek" && -z "$LLM_API_KEY" ]]; then
  LLM_API_KEY="${DEEPSEEK_API_KEY:-}"
  if [[ -z "$LLM_API_KEY" ]]; then
    echo "Error: DEEPSEEK_API_KEY env var or --api-key required for DeepSeek mode"
    exit 1
  fi
fi
if [[ "$LLM_PROVIDER" == "openai" && -z "$LLM_API_KEY" ]]; then
  LLM_API_KEY="${OPENAI_API_KEY:-}"
  if [[ -z "$LLM_API_KEY" ]]; then
    echo "Error: OPENAI_API_KEY env var or --api-key required for OpenAI mode"
    exit 1
  fi
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

cleanup() {
  echo -e "\n${DIM}Cleaning up...${RESET}"
  kill $GATEWAY_PID $ALPHA_PID $BETA_PID 2>/dev/null || true
  wait $GATEWAY_PID $ALPHA_PID $BETA_PID 2>/dev/null || true
}
trap cleanup EXIT

echo -e "${BOLD}${CYAN}"
echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║                                                                ║"
echo "║   agent-social-gateway — Two-Agent Conversation Demo           ║"
echo "║                                                                ║"
echo "║   Alpha (Research) ←──→ Gateway ←──→ Beta (Code Review)       ║"
echo "║        :9001         A2A :8080          :9002                  ║"
echo "║                   LLM: $LLM_PROVIDER                                    ║"
echo "║                                                                ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo -e "${RESET}"

# Build
echo -e "${YELLOW}Building...${RESET}"
go build -o bin/agent-social-gateway ./cmd/gateway/
go build -o bin/agent ./cmd/agent/
echo -e "${GREEN}Build complete${RESET}"

# Start Gateway
echo -e "\n${BOLD}${YELLOW}── Starting Gateway ──${RESET}"
rm -f gateway.db gateway.db-wal gateway.db-shm
./bin/agent-social-gateway -config configs/gateway.yaml > /tmp/gateway.log 2>&1 &
GATEWAY_PID=$!
sleep 2
echo -e "  Gateway running (PID $GATEWAY_PID)"

# Start Agent Alpha
echo -e "\n${BOLD}${YELLOW}── Starting Agent Alpha (Research) ──${RESET}"
LLM_FLAGS="--llm $LLM_PROVIDER"
if [[ "$LLM_PROVIDER" == "openai" || "$LLM_PROVIDER" == "deepseek" ]]; then
  LLM_FLAGS="$LLM_FLAGS --llm-api-key $LLM_API_KEY --model $LLM_MODEL"
fi
./bin/agent \
  --id agent-alpha \
  --name "Alpha (Research Agent)" \
  --port 9001 \
  --gateway "$GATEWAY_URL" \
  --api-key alpha-key-001 \
  $LLM_FLAGS \
  --system "You are Alpha, a research-focused AI agent. You specialize in analyzing papers, finding patterns, and synthesizing knowledge. You are collaborating with Beta (a code review agent) on an AI safety project. Keep responses concise (2-3 sentences)." \
  > /tmp/agent-alpha.log 2>&1 &
ALPHA_PID=$!
sleep 1
echo -e "  ${BLUE}Agent Alpha${RESET} running on :9001 (PID $ALPHA_PID)"

# Start Agent Beta
echo -e "\n${BOLD}${YELLOW}── Starting Agent Beta (Code Review) ──${RESET}"
./bin/agent \
  --id agent-beta \
  --name "Beta (Code Review Agent)" \
  --port 9002 \
  --gateway "$GATEWAY_URL" \
  --api-key beta-key-001 \
  $LLM_FLAGS \
  --system "You are Beta, a code review and engineering AI agent. You specialize in reviewing code, finding bugs, and suggesting improvements. You are collaborating with Alpha (a research agent) on an AI safety project. Keep responses concise (2-3 sentences)." \
  > /tmp/agent-beta.log 2>&1 &
BETA_PID=$!
sleep 1
echo -e "  ${PURPLE}Agent Beta${RESET} running on :9002 (PID $BETA_PID)"

# Verify all services
echo -e "\n${BOLD}${YELLOW}── Verifying Services ──${RESET}"
curl -sf "$GATEWAY_URL/health" > /dev/null && echo -e "  Gateway health: ${GREEN}OK${RESET}" || echo -e "  Gateway health: FAIL"
curl -sf "http://localhost:9001/health" > /dev/null && echo -e "  Alpha health:   ${GREEN}OK${RESET}" || echo -e "  Alpha health: FAIL"
curl -sf "http://localhost:9002/health" > /dev/null && echo -e "  Beta health:    ${GREEN}OK${RESET}" || echo -e "  Beta health: FAIL"

# ── Multi-turn Conversation ──
echo -e "\n${BOLD}${CYAN}═══ Starting Multi-Turn Conversation ═══${RESET}"

send_message() {
  local from_port=$1
  local from_name=$2
  local from_color=$3
  local target=$4
  local message=$5
  local context_id=$6

  echo -e "\n  ${BOLD}${from_color}[${from_name}]${RESET} → ${target}"
  echo -e "  ${DIM}\"${message}\"${RESET}"

  response=$(curl -sf "http://localhost:${from_port}/chat" \
    -H "Content-Type: application/json" \
    -d "{\"target_agent\":\"${target}\",\"message\":\"${message}\",\"context_id\":\"${context_id}\"}" 2>/dev/null)

  # Extract reply text from the response
  reply=$(echo "$response" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    task = d.get('task', {})
    status = task.get('status', {})
    msg = status.get('message', {})
    parts = msg.get('parts', [])
    if parts:
        print(parts[0].get('text', '(no text)'))
    else:
        print('(no reply)')
except:
    print('(parse error)')
" 2>/dev/null)

  if [[ "$target" == "agent-beta" ]]; then
    echo -e "  ${BOLD}${PURPLE}[Beta replies]${RESET}"
  else
    echo -e "  ${BOLD}${BLUE}[Alpha replies]${RESET}"
  fi
  echo -e "  ${DIM}${reply}${RESET}"
}

CTX_ID="convo-$(date +%s)"

echo -e "\n${BOLD}${YELLOW}── Turn 1: Alpha initiates ──${RESET}"
send_message 9001 "Alpha" "$BLUE" "agent-beta" \
  "Hi Beta! I've been researching AI agent cooperation patterns. I found that message-passing architectures outperform shared-memory approaches by 40% in multi-agent scenarios. What's your take from the implementation side?" \
  "$CTX_ID"

sleep 1

echo -e "\n${BOLD}${YELLOW}── Turn 2: Beta responds to Alpha ──${RESET}"
send_message 9002 "Beta" "$PURPLE" "agent-alpha" \
  "Interesting findings! From the code review perspective, I've seen that message-passing systems are also easier to test and debug. The key challenge is designing robust serialization formats. Have you looked at any specific protocols?" \
  "$CTX_ID"

sleep 1

echo -e "\n${BOLD}${YELLOW}── Turn 3: Alpha follows up ──${RESET}"
send_message 9001 "Alpha" "$BLUE" "agent-beta" \
  "Yes! The A2A protocol looks very promising. It uses JSON-RPC with task lifecycle management. Could you review the protocol spec and suggest any improvements for error handling?" \
  "$CTX_ID"

sleep 1

echo -e "\n${BOLD}${YELLOW}── Turn 4: Beta's recommendation ──${RESET}"
send_message 9002 "Beta" "$PURPLE" "agent-alpha" \
  "I reviewed the A2A spec. The task state machine is well-designed, but I'd recommend adding exponential backoff for retries and circuit breakers for downstream calls. Should we write a joint proposal?" \
  "$CTX_ID"

# Show results
echo -e "\n${BOLD}${CYAN}═══ Conversation Summary ═══${RESET}"
echo -e "\n${BOLD}Gateway Tasks:${RESET}"
curl -sf "$GATEWAY_URL/a2a/tasks?pageSize=10" | python3 -c "
import sys, json
d = json.load(sys.stdin)
for t in d.get('tasks', []):
    tid = t['id'][:8]
    state = t.get('status', {}).get('state', '?')
    hist = t.get('history', [])
    first_text = ''
    if hist:
        parts = hist[0].get('parts', [])
        if parts:
            first_text = parts[0].get('text', '')[:60]
    print(f'  [{state}] {tid}  {first_text}...')
" 2>/dev/null

echo -e "\n${BOLD}Gateway Metrics:${RESET}"
curl -sf "$GATEWAY_URL/metrics/json" | python3 -c "
import sys, json
m = json.load(sys.stdin)
print(f'  Requests: {int(m[\"total_requests\"])}  Errors: {int(m[\"total_errors\"])}  Latency: {m[\"avg_latency_ms\"]:.1f}ms')
" 2>/dev/null

echo -e "\n${BOLD}${GREEN}╔══════════════════════════════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}${GREEN}║  ✓ Conversation complete — messages routed through gateway      ║${RESET}"
echo -e "${BOLD}${GREEN}╚══════════════════════════════════════════════════════════════════╝${RESET}"
echo ""
