package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/zuwance/agent-social-gateway/internal/config"
	"github.com/zuwance/agent-social-gateway/internal/types"
)

type Server struct {
	cfg       *config.A2AConfig
	card      *types.AgentCard
	taskStore *TaskStore
	logger    *slog.Logger
	agents    map[string]*config.AgentRegistration

	subscribers   map[string][]chan *StreamEvent
	subscribersMu sync.RWMutex
}

type StreamEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	ID      interface{}     `json:"id"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func NewServer(cfg *config.A2AConfig, taskStore *TaskStore, agents []config.AgentRegistration, logger *slog.Logger) *Server {
	card := buildAgentCard(cfg)
	agentMap := make(map[string]*config.AgentRegistration)
	for i := range agents {
		agentMap[agents[i].ID] = &agents[i]
	}
	return &Server{
		cfg:         cfg,
		card:        card,
		taskStore:   taskStore,
		logger:      logger,
		agents:      agentMap,
		subscribers: make(map[string][]chan *StreamEvent),
	}
}

func buildAgentCard(cfg *config.A2AConfig) *types.AgentCard {
	card := &types.AgentCard{
		Name:        cfg.Agent.Name,
		Description: cfg.Agent.Description,
		Version:     cfg.Agent.Version,
		SupportedInterfaces: []types.AgentInterface{
			{
				URL:             cfg.Agent.URL,
				ProtocolBinding: "JSONRPC",
				ProtocolVersion: cfg.Agent.ProtocolVersion,
			},
		},
		Capabilities: types.AgentCapabilities{
			Streaming:         cfg.Agent.Capabilities.Streaming,
			PushNotifications: cfg.Agent.Capabilities.PushNotifications,
			ExtendedAgentCard: cfg.Agent.Capabilities.ExtendedAgentCard,
		},
		DefaultInputModes:  cfg.Agent.DefaultInputModes,
		DefaultOutputModes: cfg.Agent.DefaultOutputModes,
	}

	if cfg.Agent.Provider != nil {
		card.Provider = &types.AgentProvider{
			URL:          cfg.Agent.Provider.URL,
			Organization: cfg.Agent.Provider.Organization,
		}
	}

	if cfg.Agent.DocumentationURL != "" {
		card.DocumentationURL = cfg.Agent.DocumentationURL
	}

	for _, s := range cfg.Agent.Skills {
		card.Skills = append(card.Skills, types.AgentSkill{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			Tags:        s.Tags,
			Examples:    s.Examples,
		})
	}

	return card
}

func (s *Server) RegisterRoutes(r chi.Router) {
	r.Get("/.well-known/agent-card.json", s.handleAgentCard)
	r.Get("/a2a/extendedAgentCard", s.handleExtendedAgentCard)

	r.Post("/a2a/message:send", s.handleSendMessage)
	r.Post("/a2a/message:stream", s.handleSendStreamingMessage)
	r.Get("/a2a/tasks/{taskID}", s.handleGetTask)
	r.Get("/a2a/tasks", s.handleListTasks)
	r.Post("/a2a/tasks/{taskID}:cancel", s.handleCancelTask)
	r.Get("/a2a/tasks/{taskID}:subscribe", s.handleSubscribeToTask)

	r.Post("/a2a/tasks/{taskID}/pushNotificationConfigs", s.handleCreatePushConfig)
	r.Get("/a2a/tasks/{taskID}/pushNotificationConfigs/{configID}", s.handleGetPushConfig)
	r.Get("/a2a/tasks/{taskID}/pushNotificationConfigs", s.handleListPushConfigs)
	r.Delete("/a2a/tasks/{taskID}/pushNotificationConfigs/{configID}", s.handleDeletePushConfig)

	// JSON-RPC endpoint (alternative single-endpoint binding)
	r.Post("/a2a/rpc", s.handleJSONRPC)
}

func (s *Server) handleAgentCard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	json.NewEncoder(w).Encode(s.card)
}

func (s *Server) handleExtendedAgentCard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.card)
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, -32700, "parse error", err.Error())
		return
	}

	if req.Message.MessageID == "" {
		req.Message.MessageID = uuid.New().String()
	}

	contextID := req.Message.ContextID
	if contextID == "" {
		contextID = uuid.New().String()
	}

	// Check for target_agent in metadata — if present, forward the message
	var targetAgentID string
	if req.Metadata != nil {
		if t, ok := req.Metadata["target_agent"].(string); ok && t != "" {
			targetAgentID = t
		}
	}

	if targetAgentID != "" {
		s.forwardMessage(w, r, &req, targetAgentID, contextID)
		return
	}

	// No target — process locally (gateway itself is the agent)
	s.processLocally(w, &req, contextID)
}

func (s *Server) forwardMessage(w http.ResponseWriter, r *http.Request, req *SendMessageRequest, targetID, contextID string) {
	agent, ok := s.agents[targetID]
	if !ok {
		writeJSONError(w, http.StatusNotFound, -32001, "agent not found",
			fmt.Sprintf("target agent %q is not registered", targetID))
		return
	}

	// Validate sender API key if security is needed
	senderKey := r.Header.Get("Authorization")
	if senderKey != "" {
		senderKey = strings.TrimPrefix(senderKey, "Bearer ")
	}

	taskID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	task := &types.Task{
		ID:        taskID,
		ContextID: contextID,
		Status:    types.TaskStatus{State: types.TaskStateSubmitted, Timestamp: now},
		History:   []types.Message{req.Message},
	}
	if err := s.taskStore.Create(task); err != nil {
		writeJSONError(w, http.StatusInternalServerError, -32603, "internal error", err.Error())
		return
	}

	s.logger.Info("forwarding message", "task_id", taskID, "target", targetID, "url", agent.URL)

	task.Status.State = types.TaskStateWorking
	task.Status.Timestamp = time.Now().UTC().Format(time.RFC3339)
	s.taskStore.Update(task)

	// Forward to target agent
	forwardReq := SendMessageRequest{
		Message:       req.Message,
		Configuration: req.Configuration,
		Metadata: map[string]interface{}{
			"forwarded_by": s.cfg.Agent.Name,
			"context_id":   contextID,
			"task_id":      taskID,
		},
	}
	forwardBody, _ := json.Marshal(forwardReq)

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	endpoint := strings.TrimSuffix(agent.URL, "/") + "/a2a/message:send"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(forwardBody))
	if err != nil {
		task.Status.State = types.TaskStateFailed
		task.Status.Timestamp = time.Now().UTC().Format(time.RFC3339)
		s.taskStore.Update(task)
		writeJSONError(w, http.StatusBadGateway, -32603, "forward failed", err.Error())
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("A2A-Version", "0.3")
	if agent.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+agent.APIKey)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		task.Status.State = types.TaskStateFailed
		task.Status.Timestamp = time.Now().UTC().Format(time.RFC3339)
		s.taskStore.Update(task)
		writeJSONError(w, http.StatusBadGateway, -32603, "agent unreachable", err.Error())
		return
	}
	defer resp.Body.Close()

	var agentResp SendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&agentResp); err != nil {
		task.Status.State = types.TaskStateFailed
		s.taskStore.Update(task)
		writeJSONError(w, http.StatusBadGateway, -32603, "bad agent response", err.Error())
		return
	}

	// Merge agent response into gateway task
	task.Status.State = types.TaskStateCompleted
	task.Status.Timestamp = time.Now().UTC().Format(time.RFC3339)
	if agentResp.Task != nil && agentResp.Task.Status.Message != nil {
		task.Status.Message = agentResp.Task.Status.Message
		task.History = append(task.History, *agentResp.Task.Status.Message)
	}
	s.taskStore.Update(task)

	s.logger.Info("message forwarded", "task_id", taskID, "target", targetID, "status", "completed")
	s.notifySubscribers(taskID, &StreamEvent{Type: "task", Payload: task})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SendMessageResponse{Task: task})
}

func (s *Server) processLocally(w http.ResponseWriter, req *SendMessageRequest, contextID string) {
	taskID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	task := &types.Task{
		ID:        taskID,
		ContextID: contextID,
		Status:    types.TaskStatus{State: types.TaskStateSubmitted, Timestamp: now},
		History:   []types.Message{req.Message},
	}
	if err := s.taskStore.Create(task); err != nil {
		writeJSONError(w, http.StatusInternalServerError, -32603, "internal error", err.Error())
		return
	}
	s.logger.Info("a2a task created", "task_id", taskID, "context_id", contextID)

	task.Status.State = types.TaskStateCompleted
	task.Status.Timestamp = time.Now().UTC().Format(time.RFC3339)
	task.Status.Message = &types.Message{
		MessageID: uuid.New().String(),
		Role:      "agent",
		Parts:     []types.Part{{Text: fmt.Sprintf("Message received and processed by %s", s.cfg.Agent.Name)}},
	}
	s.taskStore.Update(task)
	s.notifySubscribers(taskID, &StreamEvent{Type: "task", Payload: task})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SendMessageResponse{Task: task})
}

func (s *Server) handleSendStreamingMessage(w http.ResponseWriter, r *http.Request) {
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, -32700, "parse error", err.Error())
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, -32603, "streaming not supported", nil)
		return
	}

	contextID := req.Message.ContextID
	if contextID == "" {
		contextID = uuid.New().String()
	}

	taskID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	task := &types.Task{
		ID:        taskID,
		ContextID: contextID,
		Status:    types.TaskStatus{State: types.TaskStateSubmitted, Timestamp: now},
		History:   []types.Message{req.Message},
	}
	s.taskStore.Create(task)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	writeSSEEvent(w, "task", task)
	flusher.Flush()

	task.Status.State = types.TaskStateWorking
	task.Status.Timestamp = time.Now().UTC().Format(time.RFC3339)
	s.taskStore.Update(task)

	writeSSEEvent(w, "statusUpdate", map[string]interface{}{
		"taskId":    taskID,
		"contextId": contextID,
		"status":    task.Status,
	})
	flusher.Flush()

	task.Status.State = types.TaskStateCompleted
	task.Status.Timestamp = time.Now().UTC().Format(time.RFC3339)
	task.Status.Message = &types.Message{
		MessageID: uuid.New().String(),
		Role:      "agent",
		Parts:     []types.Part{{Text: fmt.Sprintf("Processed by %s", s.cfg.Agent.Name)}},
	}
	s.taskStore.Update(task)

	writeSSEEvent(w, "statusUpdate", map[string]interface{}{
		"taskId":    taskID,
		"contextId": contextID,
		"status":    task.Status,
	})
	flusher.Flush()
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	task, err := s.taskStore.Get(taskID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, -32001, "task not found", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	pageSize := 50
	if ps := r.URL.Query().Get("pageSize"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 100 {
			pageSize = v
		}
	}
	offset := 0
	if pt := r.URL.Query().Get("pageToken"); pt != "" {
		if v, err := strconv.Atoi(pt); err == nil {
			offset = v
		}
	}

	contextID := r.URL.Query().Get("contextId")
	var tasks []*types.Task
	var total int
	var err error

	if contextID != "" {
		tasks, err = s.taskStore.ListByContext(contextID, pageSize, offset)
		total = len(tasks)
	} else {
		tasks, total, err = s.taskStore.List(pageSize, offset)
	}

	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, -32603, "internal error", err.Error())
		return
	}

	nextToken := ""
	if offset+pageSize < total {
		nextToken = strconv.Itoa(offset + pageSize)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks":         tasks,
		"nextPageToken": nextToken,
		"pageSize":      pageSize,
		"totalSize":     total,
	})
}

func (s *Server) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")

	if err := s.taskStore.Cancel(taskID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, -32603, "cancel failed", err.Error())
		return
	}

	task, err := s.taskStore.Get(taskID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, -32001, "task not found", err.Error())
		return
	}

	s.notifySubscribers(taskID, &StreamEvent{Type: "statusUpdate", Payload: task.Status})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (s *Server) handleSubscribeToTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")

	task, err := s.taskStore.Get(taskID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, -32001, "task not found", err.Error())
		return
	}

	if task.Status.State.IsTerminal() {
		writeJSONError(w, http.StatusConflict, -32002, "unsupported operation", "task is in terminal state")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, -32603, "streaming not supported", nil)
		return
	}

	ch := make(chan *StreamEvent, 16)
	s.addSubscriber(taskID, ch)
	defer s.removeSubscriber(taskID, ch)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			writeSSEEvent(w, event.Type, event.Payload)
			flusher.Flush()
		}
	}
}

func (s *Server) handleCreatePushConfig(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	var cfg PushNotificationConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSONError(w, http.StatusBadRequest, -32700, "parse error", err.Error())
		return
	}
	cfg.TaskID = taskID
	if cfg.ID == "" {
		cfg.ID = uuid.New().String()
	}

	if err := s.taskStore.CreatePushConfig(&cfg); err != nil {
		writeJSONError(w, http.StatusInternalServerError, -32603, "internal error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cfg)
}

func (s *Server) handleGetPushConfig(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	configID := chi.URLParam(r, "configID")

	cfg, err := s.taskStore.GetPushConfig(taskID, configID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, -32001, "config not found", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}

func (s *Server) handleListPushConfigs(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")

	configs, err := s.taskStore.ListPushConfigs(taskID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, -32603, "internal error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"configs": configs})
}

func (s *Server) handleDeletePushConfig(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	configID := chi.URLParam(r, "configID")

	if err := s.taskStore.DeletePushConfig(taskID, configID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, -32603, "internal error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONRPCError(w, nil, -32700, "parse error", err.Error())
		return
	}

	switch req.Method {
	case "SendMessage":
		r.Body = io.NopCloser(bytes.NewReader(req.Params))
		s.handleSendMessage(w, r)
	case "GetTask":
		var params struct {
			ID string `json:"id"`
		}
		json.Unmarshal(req.Params, &params)
		r = setURLParam(r, "taskID", params.ID)
		s.handleGetTask(w, r)
	case "CancelTask":
		var params struct {
			ID string `json:"id"`
		}
		json.Unmarshal(req.Params, &params)
		r = setURLParam(r, "taskID", params.ID)
		s.handleCancelTask(w, r)
	default:
		writeJSONRPCError(w, req.ID, -32601, "method not found", req.Method)
	}
}

func (s *Server) addSubscriber(taskID string, ch chan *StreamEvent) {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()
	s.subscribers[taskID] = append(s.subscribers[taskID], ch)
}

func (s *Server) removeSubscriber(taskID string, ch chan *StreamEvent) {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()
	subs := s.subscribers[taskID]
	for i, sub := range subs {
		if sub == ch {
			s.subscribers[taskID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}
	if len(s.subscribers[taskID]) == 0 {
		delete(s.subscribers, taskID)
	}
}

func (s *Server) notifySubscribers(taskID string, event *StreamEvent) {
	s.subscribersMu.RLock()
	defer s.subscribersMu.RUnlock()
	for _, ch := range s.subscribers[taskID] {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *Server) GetAgentCard() *types.AgentCard {
	return s.card
}

// Request/response types
type SendMessageRequest struct {
	Message       types.Message              `json:"message"`
	Configuration *SendMessageConfiguration  `json:"configuration,omitempty"`
	Metadata      map[string]interface{}     `json:"metadata,omitempty"`
}

type SendMessageConfiguration struct {
	AcceptedOutputModes []string `json:"acceptedOutputModes,omitempty"`
	HistoryLength       *int     `json:"historyLength,omitempty"`
	ReturnImmediately   bool     `json:"returnImmediately,omitempty"`
}

type SendMessageResponse struct {
	Task    *types.Task    `json:"task,omitempty"`
	Message *types.Message `json:"message,omitempty"`
}

// Helpers

func writeJSONError(w http.ResponseWriter, status, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(JSONRPCResponse{
		JSONRPC: "2.0",
		Error:   &JSONRPCError{Code: code, Message: message, Data: data},
	})
}

func writeJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JSONRPCResponse{
		JSONRPC: "2.0",
		Error:   &JSONRPCError{Code: code, Message: message, Data: data},
		ID:      id,
	})
}

func writeSSEEvent(w http.ResponseWriter, eventType string, data interface{}) {
	payload, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, payload)
}

func setURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	ctx := r.Context()
	// Merge with existing route context if available
	if existing := chi.RouteContext(ctx); existing != nil {
		existing.URLParams.Add(key, value)
		return r
	}
	return r.WithContext(r.Context())
}

// GetVersion returns the A2A version header from the request.
func GetVersion(r *http.Request) string {
	v := r.Header.Get("A2A-Version")
	if v == "" {
		return "0.3"
	}
	return strings.TrimSpace(v)
}
