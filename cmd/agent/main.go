package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type AgentConfig struct {
	ID           string
	Name         string
	Port         int
	GatewayURL   string
	APIKey       string
	LLMProvider  string
	LLMAPIKey    string
	LLMModel     string
	LLMBaseURL   string
	SystemPrompt string
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLMClient interface {
	Chat(ctx context.Context, messages []ChatMessage) (string, error)
}

type Agent struct {
	cfg     AgentConfig
	llm     LLMClient
	logger  *slog.Logger
	history map[string][]ChatMessage
	mu      sync.RWMutex
}

func main() {
	cfg := AgentConfig{}
	flag.StringVar(&cfg.ID, "id", "agent-1", "Agent ID")
	flag.StringVar(&cfg.Name, "name", "Agent", "Agent display name")
	flag.IntVar(&cfg.Port, "port", 9001, "HTTP server port")
	flag.StringVar(&cfg.GatewayURL, "gateway", "http://localhost:8080", "Gateway URL")
	flag.StringVar(&cfg.APIKey, "api-key", "", "API key for gateway authentication")
	flag.StringVar(&cfg.LLMProvider, "llm", "mock", "LLM provider: mock, openai")
	flag.StringVar(&cfg.LLMAPIKey, "llm-api-key", "", "LLM API key (or set OPENAI_API_KEY env)")
	flag.StringVar(&cfg.LLMModel, "model", "", "LLM model name")
	flag.StringVar(&cfg.LLMBaseURL, "llm-base-url", "", "LLM API base URL (for OpenAI-compatible APIs)")
	flag.StringVar(&cfg.SystemPrompt, "system", "", "System prompt for agent personality")
	flag.Parse()

	if cfg.LLMAPIKey == "" {
		switch cfg.LLMProvider {
		case "deepseek":
			cfg.LLMAPIKey = os.Getenv("DEEPSEEK_API_KEY")
		case "openai":
			cfg.LLMAPIKey = os.Getenv("OPENAI_API_KEY")
		}
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	agent := &Agent{
		cfg:     cfg,
		logger:  logger,
		history: make(map[string][]ChatMessage),
	}

	switch cfg.LLMProvider {
	case "deepseek":
		if cfg.LLMAPIKey == "" {
			logger.Error("DeepSeek API key required: use --llm-api-key or set DEEPSEEK_API_KEY")
			os.Exit(1)
		}
		baseURL := cfg.LLMBaseURL
		if baseURL == "" {
			baseURL = "https://api.deepseek.com/v1"
		}
		model := cfg.LLMModel
		if model == "" {
			model = "deepseek-chat"
		}
		agent.llm = &OpenAICompatClient{apiKey: cfg.LLMAPIKey, model: model, baseURL: baseURL}
		logger.Info("using DeepSeek LLM", "model", model, "base_url", baseURL)
	case "openai":
		if cfg.LLMAPIKey == "" {
			logger.Error("OpenAI API key required: use --llm-api-key or set OPENAI_API_KEY")
			os.Exit(1)
		}
		baseURL := cfg.LLMBaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		model := cfg.LLMModel
		if model == "" {
			model = "gpt-4o-mini"
		}
		agent.llm = &OpenAICompatClient{apiKey: cfg.LLMAPIKey, model: model, baseURL: baseURL}
		logger.Info("using OpenAI LLM", "model", model, "base_url", baseURL)
	default:
		agent.llm = &MockLLM{name: cfg.Name}
		logger.Info("using mock LLM (set --llm=deepseek or --llm=openai for real AI)")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", agent.handleAgentCard)
	mux.HandleFunc("/a2a/message:send", agent.handleMessage)
	mux.HandleFunc("/chat", agent.handleChat)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "agent": cfg.ID})
	})

	srv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.Port), Handler: mux}

	go func() {
		logger.Info("agent starting", "id", cfg.ID, "name", cfg.Name, "port", cfg.Port, "llm", cfg.LLMProvider)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down agent")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

// ─── HTTP Handlers ──────────────────────────────────────────────

func (a *Agent) handleAgentCard(w http.ResponseWriter, r *http.Request) {
	card := map[string]any{
		"name":        a.cfg.Name,
		"description": fmt.Sprintf("%s — LLM-powered agent connected to agent-social-gateway", a.cfg.Name),
		"version":     "1.0.0",
		"supportedInterfaces": []map[string]any{
			{"url": fmt.Sprintf("http://localhost:%d", a.cfg.Port), "protocolBinding": "JSONRPC", "protocolVersion": "0.3"},
		},
		"capabilities": map[string]any{"streaming": false},
		"skills": []map[string]any{
			{"id": a.cfg.ID + "-llm", "name": a.cfg.Name + " LLM", "tags": []string{a.cfg.LLMProvider}},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(card)
}

func (a *Agent) handleMessage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message struct {
			MessageID string `json:"messageId"`
			ContextID string `json:"contextId"`
			Role      string `json:"role"`
			Parts     []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"message"`
		Metadata map[string]any `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	var userText string
	for _, p := range req.Message.Parts {
		if p.Text != "" {
			userText = p.Text
			break
		}
	}

	contextID := req.Message.ContextID
	if contextID == "" {
		if req.Metadata != nil {
			if cid, ok := req.Metadata["context_id"].(string); ok {
				contextID = cid
			}
		}
	}
	if contextID == "" {
		contextID = "default"
	}

	a.logger.Info("received message", "from", req.Message.Role, "context", contextID, "text", truncate(userText, 60))

	a.mu.Lock()
	history := a.history[contextID]
	history = append(history, ChatMessage{Role: "user", Content: userText})
	a.mu.Unlock()

	messages := a.buildMessages(history)
	reply, err := a.llm.Chat(r.Context(), messages)
	if err != nil {
		a.logger.Error("LLM error", "error", err)
		reply = fmt.Sprintf("[%s] I encountered an error processing your message.", a.cfg.Name)
	}

	a.mu.Lock()
	a.history[contextID] = append(history, ChatMessage{Role: "assistant", Content: reply})
	a.mu.Unlock()

	a.logger.Info("generated reply", "context", contextID, "reply", truncate(reply, 60))

	task := map[string]any{
		"id":        fmt.Sprintf("task-%s-%d", a.cfg.ID, time.Now().UnixMilli()),
		"contextId": contextID,
		"status": map[string]any{
			"state":     "COMPLETED",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"message": map[string]any{
				"messageId": fmt.Sprintf("reply-%d", time.Now().UnixMilli()),
				"role":      "agent",
				"parts":     []map[string]any{{"text": reply}},
			},
		},
		"history": []map[string]any{
			{"role": "user", "parts": []map[string]any{{"text": userText}}},
			{"role": "agent", "parts": []map[string]any{{"text": reply}}},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"task": task})
}

// handleChat allows initiating a conversation with another agent through the gateway
func (a *Agent) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TargetAgent string `json:"target_agent"`
		Message     string `json:"message"`
		ContextID   string `json:"context_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	a.logger.Info("initiating chat", "target", req.TargetAgent, "message", truncate(req.Message, 60))

	body := map[string]any{
		"message": map[string]any{
			"messageId": fmt.Sprintf("%s-%d", a.cfg.ID, time.Now().UnixMilli()),
			"contextId": req.ContextID,
			"role":      "user",
			"parts":     []map[string]any{{"text": req.Message}},
		},
		"metadata": map[string]any{
			"target_agent": req.TargetAgent,
			"from_agent":   a.cfg.ID,
		},
	}
	data, _ := json.Marshal(body)

	httpReq, _ := http.NewRequestWithContext(r.Context(), "POST",
		strings.TrimSuffix(a.cfg.GatewayURL, "/")+"/a2a/message:send",
		bytes.NewReader(data))
	httpReq.Header.Set("Content-Type", "application/json")
	if a.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (a *Agent) buildMessages(history []ChatMessage) []ChatMessage {
	var msgs []ChatMessage
	if a.cfg.SystemPrompt != "" {
		msgs = append(msgs, ChatMessage{Role: "system", Content: a.cfg.SystemPrompt})
	}
	msgs = append(msgs, history...)
	return msgs
}

// ─── LLM Clients ────────────────────────────────────────────────

// OpenAICompatClient works with any OpenAI-compatible API (OpenAI, DeepSeek, etc.)
type OpenAICompatClient struct {
	apiKey  string
	model   string
	baseURL string
}

func (c *OpenAICompatClient) Chat(ctx context.Context, messages []ChatMessage) (string, error) {
	body := map[string]any{
		"model":    c.model,
		"messages": messages,
	}
	data, _ := json.Marshal(body)

	endpoint := strings.TrimSuffix(c.baseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM error %d: %s", resp.StatusCode, truncate(string(b), 200))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("LLM decode error: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}
	return result.Choices[0].Message.Content, nil
}

type MockLLM struct {
	name  string
	count int
	mu    sync.Mutex
}

func (m *MockLLM) Chat(ctx context.Context, messages []ChatMessage) (string, error) {
	m.mu.Lock()
	m.count++
	turn := m.count
	m.mu.Unlock()

	last := messages[len(messages)-1].Content

	responses := []string{
		"That's an interesting point. Let me analyze this further — I think we could approach it from a different angle.",
		"Great insight! I've been working on something similar. Let me share my findings with you.",
		"I agree with your analysis. Here's what I'd add based on my research...",
		"Fascinating! This connects to several patterns I've observed. Let me elaborate.",
		"Thank you for sharing. I've processed this and have some concrete suggestions.",
	}

	idx := (turn - 1) % len(responses)
	return fmt.Sprintf("[%s] %s\n\n(Regarding: %q)", m.name, responses[idx], truncate(last, 80)), nil
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n-3] + "..."
	}
	return s
}
