package a2a

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zuwance/agent-social-gateway/internal/types"
)

type Client struct {
	httpClient *http.Client
	logger     *slog.Logger
	agents     map[string]*RemoteAgent
	mu         sync.RWMutex
}

type RemoteAgent struct {
	BaseURL string
	Card    *types.AgentCard
	FetchedAt time.Time
}

func NewClient(logger *slog.Logger) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
		agents:     make(map[string]*RemoteAgent),
	}
}

func (c *Client) DiscoverAgent(ctx context.Context, baseURL string) (*types.AgentCard, error) {
	cardURL := strings.TrimSuffix(baseURL, "/") + "/.well-known/agent-card.json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cardURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching agent card: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent card fetch failed: status %d", resp.StatusCode)
	}

	var card types.AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("decoding agent card: %w", err)
	}

	c.mu.Lock()
	c.agents[baseURL] = &RemoteAgent{
		BaseURL:   baseURL,
		Card:      &card,
		FetchedAt: time.Now(),
	}
	c.mu.Unlock()

	c.logger.Info("agent discovered", "url", baseURL, "name", card.Name)
	return &card, nil
}

func (c *Client) SendMessage(ctx context.Context, baseURL string, msg *types.Message) (*SendMessageResponse, error) {
	endpoint := strings.TrimSuffix(baseURL, "/") + "/a2a/message:send"

	reqBody := SendMessageRequest{Message: *msg}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("A2A-Version", "0.3")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("send message failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var result SendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &result, nil
}

type StreamCallback func(eventType string, data json.RawMessage) error

func (c *Client) SendStreamingMessage(ctx context.Context, baseURL string, msg *types.Message, callback StreamCallback) error {
	endpoint := strings.TrimSuffix(baseURL, "/") + "/a2a/message:stream"

	reqBody := SendMessageRequest{Message: *msg}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("A2A-Version", "0.3")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("streaming request: %w", err)
	}
	defer resp.Body.Close()

	return c.readSSEStream(resp.Body, callback)
}

func (c *Client) GetTask(ctx context.Context, baseURL, taskID string) (*types.Task, error) {
	endpoint := fmt.Sprintf("%s/a2a/tasks/%s", strings.TrimSuffix(baseURL, "/"), taskID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("A2A-Version", "0.3")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get task failed: status %d", resp.StatusCode)
	}

	var task types.Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (c *Client) CancelTask(ctx context.Context, baseURL, taskID string) (*types.Task, error) {
	endpoint := fmt.Sprintf("%s/a2a/tasks/%s:cancel", strings.TrimSuffix(baseURL, "/"), taskID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("A2A-Version", "0.3")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var task types.Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (c *Client) SubscribeToTask(ctx context.Context, baseURL, taskID string, callback StreamCallback) error {
	endpoint := fmt.Sprintf("%s/a2a/tasks/%s:subscribe", strings.TrimSuffix(baseURL, "/"), taskID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("A2A-Version", "0.3")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.readSSEStream(resp.Body, callback)
}

func (c *Client) readSSEStream(body io.Reader, callback StreamCallback) error {
	scanner := bufio.NewScanner(body)
	var eventType string
	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if eventType != "" && len(dataLines) > 0 {
				data := json.RawMessage(strings.Join(dataLines, "\n"))
				if err := callback(eventType, data); err != nil {
					return err
				}
			}
			eventType = ""
			dataLines = nil
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		}
	}

	return scanner.Err()
}

func (c *Client) GetRemoteAgent(baseURL string) (*RemoteAgent, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	agent, ok := c.agents[baseURL]
	return agent, ok
}

func (c *Client) ListRemoteAgents() []*RemoteAgent {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var agents []*RemoteAgent
	for _, a := range c.agents {
		agents = append(agents, a)
	}
	return agents
}
