package a2a

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/zuwance/agent-social-gateway/internal/social"
	"github.com/zuwance/agent-social-gateway/internal/storage"
	"github.com/zuwance/agent-social-gateway/internal/types"
)

// SocialExtensions implements A2A Social Extension Layers 1-5.
type SocialExtensions struct {
	db       *storage.DB
	actions  *social.Actions
	timeline *social.Timeline
	logger   *slog.Logger

	// Layer 5: per-agent feed subscribers
	feedSubs   map[string][]chan *types.SocialEvent
	feedSubsMu sync.RWMutex
}

func NewSocialExtensions(db *storage.DB, actions *social.Actions, timeline *social.Timeline, logger *slog.Logger) *SocialExtensions {
	return &SocialExtensions{
		db:       db,
		actions:  actions,
		timeline: timeline,
		logger:   logger.With("component", "a2a-social-ext"),
		feedSubs: make(map[string][]chan *types.SocialEvent),
	}
}

// RegisterRoutes adds A2A Social Extension endpoints.
func (se *SocialExtensions) RegisterRoutes(r chi.Router) {
	// Layer 1: Enhanced Agent Card (social profile enrichment)
	r.Get("/a2a/agents/{agentID}/card", se.handleSocialAgentCard)

	// Layer 2: Social Event Protocol
	r.Post("/a2a/social/event", se.handleSocialEvent)
	r.Get("/a2a/social/events", se.handleListSocialEvents)

	// Layer 3: Relationship-Aware Routing
	r.Post("/a2a/social/route", se.handleSocialRoute)

	// Layer 4: Conversation Context
	r.Post("/a2a/contexts", se.handleCreateContext)
	r.Get("/a2a/contexts/{contextID}", se.handleGetContext)
	r.Get("/a2a/contexts", se.handleListContexts)
	r.Patch("/a2a/contexts/{contextID}", se.handleUpdateContext)

	// Layer 5: Social Feed SSE
	r.Get("/a2a/agents/{agentID}/feed", se.handleFeedSubscribe)
}

// ─── Layer 1: Social Agent Card ─────────────────────────────────

func (se *SocialExtensions) handleSocialAgentCard(w http.ResponseWriter, r *http.Request) {
	agentID := types.AgentID(chi.URLParam(r, "agentID"))
	profile := se.BuildSocialProfile(agentID)

	card := map[string]any{
		"agentId":       string(agentID),
		"socialProfile": profile,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(card)
}

// BuildSocialProfile constructs a SocialProfile from DB data for inclusion in Agent Cards.
func (se *SocialExtensions) BuildSocialProfile(agentID types.AgentID) *types.SocialProfile {
	graph := se.actions.Graph()
	followers, _ := graph.GetFollowerCount(agentID)
	following, _ := graph.GetFollowingCount(agentID)
	reputation, _ := graph.GetReputationScore(agentID)

	endorsements := make(map[string]int)
	rels, _ := graph.GetEndorsements(agentID)
	for _, rel := range rels {
		if rel.Metadata != nil {
			if skill, ok := rel.Metadata["skill_id"].(string); ok {
				endorsements[skill]++
			}
		}
	}

	var tags []string
	var trustLevel, joinedAt string
	row := se.db.QueryRow(`SELECT COALESCE(tags_json,'[]'), COALESCE(trust_level,'unverified'), created_at FROM agents WHERE id = ?`, string(agentID))
	var tagsJSON string
	if err := row.Scan(&tagsJSON, &trustLevel, &joinedAt); err != nil {
		trustLevel = "unverified"
	}
	json.Unmarshal([]byte(tagsJSON), &tags)

	return &types.SocialProfile{
		Followers:    followers,
		Following:    following,
		Reputation:   reputation,
		TrustLevel:   trustLevel,
		Tags:         tags,
		Endorsements: endorsements,
		JoinedAt:     joinedAt,
	}
}

// ─── Layer 2: Social Event Protocol ─────────────────────────────

func (se *SocialExtensions) handleSocialEvent(w http.ResponseWriter, r *http.Request) {
	var evt types.SocialEvent
	if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
		writeJSONError(w, http.StatusBadRequest, -32700, "parse error", err.Error())
		return
	}

	if evt.Timestamp == "" {
		evt.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	if err := se.ProcessSocialEvent(&evt); err != nil {
		writeJSONError(w, http.StatusInternalServerError, -32603, "event processing failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "accepted", "event": evt})
}

// ProcessSocialEvent executes a social event: applies the action, persists the event, notifies feed subscribers.
func (se *SocialExtensions) ProcessSocialEvent(evt *types.SocialEvent) error {
	from := types.AgentID(evt.From)
	to := types.AgentID(evt.To)

	se.actions.EnsureAgent(from, evt.From)
	if evt.To != "" {
		se.actions.EnsureAgent(to, evt.To)
	}

	switch evt.Type {
	case types.SocialEventFollow:
		if err := se.actions.Follow(from, to); err != nil {
			return err
		}
		se.timeline.AddEvent(to, "new_follower", from, "")
	case types.SocialEventUnfollow:
		if err := se.actions.Unfollow(from, to); err != nil {
			return err
		}
	case types.SocialEventLike:
		msgID := ""
		if evt.Data != nil {
			if m, ok := evt.Data["message_id"].(string); ok {
				msgID = m
			}
		}
		if msgID != "" {
			se.actions.EnsureMessage(msgID, from)
			if err := se.actions.Like(from, msgID); err != nil {
				return err
			}
		}
	case types.SocialEventUnlike:
		msgID := ""
		if evt.Data != nil {
			if m, ok := evt.Data["message_id"].(string); ok {
				msgID = m
			}
		}
		if msgID != "" {
			se.actions.Unlike(from, msgID)
		}
	case types.SocialEventEndorse:
		if err := se.actions.Endorse(from, to, evt.Skill); err != nil {
			return err
		}
		se.timeline.AddEvent(to, "endorsed", from, evt.Skill)
	case types.SocialEventCollabReq:
		proposal := map[string]interface{}{}
		if evt.Data != nil {
			proposal = evt.Data
		}
		if err := se.actions.RequestCollaboration(from, to, proposal); err != nil {
			return err
		}
		se.timeline.AddEvent(to, "collab_request", from, "")
	}

	// Persist to social_events log
	dataJSON, _ := json.Marshal(evt.Data)
	se.db.Exec(`INSERT INTO social_events (event_type, from_agent, to_agent, skill, data_json) VALUES (?,?,?,?,?)`,
		evt.Type, evt.From, evt.To, evt.Skill, string(dataJSON))

	se.logger.Info("social event processed", "type", evt.Type, "from", evt.From, "to", evt.To)

	// Layer 5: notify feed subscribers of affected agents
	se.notifyFeed(evt.To, evt)
	se.notifyFeed(evt.From, evt)

	return nil
}

func (se *SocialExtensions) handleListSocialEvents(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent")
	eventType := r.URL.Query().Get("type")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 200 {
		limit = v
	}

	query := `SELECT id, event_type, from_agent, COALESCE(to_agent,''), COALESCE(skill,''), COALESCE(data_json,'{}'), created_at FROM social_events WHERE 1=1`
	var args []interface{}

	if agentID != "" {
		query += ` AND (from_agent = ? OR to_agent = ?)`
		args = append(args, agentID, agentID)
	}
	if eventType != "" {
		query += ` AND event_type = ?`
		args = append(args, eventType)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := se.db.Query(query, args...)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, -32603, "query failed", err.Error())
		return
	}
	defer rows.Close()

	var events []map[string]any
	for rows.Next() {
		var id int64
		var evtType, from, to, skill, dataJSON, createdAt string
		if err := rows.Scan(&id, &evtType, &from, &to, &skill, &dataJSON, &createdAt); err != nil {
			continue
		}
		evt := map[string]any{
			"id":        id,
			"type":      evtType,
			"from":      from,
			"to":        to,
			"timestamp": createdAt,
		}
		if skill != "" {
			evt["skill"] = skill
		}
		var data map[string]any
		if json.Unmarshal([]byte(dataJSON), &data) == nil && len(data) > 0 {
			evt["data"] = data
		}
		events = append(events, evt)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"events": events, "count": len(events)})
}

// ─── Layer 3: Relationship-Aware Routing ────────────────────────

func (se *SocialExtensions) handleSocialRoute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From     string               `json:"from"`
		Message  types.Message        `json:"message"`
		Routing  types.RoutingStrategy `json:"routing"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, -32700, "parse error", err.Error())
		return
	}

	targets, err := se.ResolveTargets(types.AgentID(req.From), &req.Routing)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, -32603, "resolve targets failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"strategy": req.Routing.Strategy,
		"from":     req.From,
		"targets":  targets,
		"count":    len(targets),
	})
}

// ResolveTargets returns the list of agents that should receive a message according to the routing strategy.
func (se *SocialExtensions) ResolveTargets(from types.AgentID, rs *types.RoutingStrategy) ([]string, error) {
	graph := se.actions.Graph()
	excludeSet := make(map[string]bool)
	for _, e := range rs.Exclude {
		excludeSet[e] = true
	}

	var raw []types.AgentID
	var err error

	switch rs.Strategy {
	case types.RouteFollowers:
		raw, err = graph.GetFollowers(from)
	case types.RouteMutualFollow:
		raw, err = graph.GetMutualFollows(from)
	case types.RouteTrustCircle:
		raw, err = graph.GetFollowers(from)
		if err != nil {
			return nil, err
		}
		var filtered []types.AgentID
		for _, id := range raw {
			rep, _ := graph.GetReputationScore(id)
			if rep >= rs.TrustMinimum {
				filtered = append(filtered, id)
			}
		}
		raw = filtered
	default:
		return nil, fmt.Errorf("unknown routing strategy: %s", rs.Strategy)
	}

	if err != nil {
		return nil, err
	}

	var targets []string
	for _, id := range raw {
		s := string(id)
		if !excludeSet[s] && s != string(from) {
			targets = append(targets, s)
		}
	}
	return targets, nil
}

// ─── Layer 4: Conversation Context ──────────────────────────────

func (se *SocialExtensions) handleCreateContext(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type         string   `json:"type"`
		Topic        string   `json:"topic"`
		Participants []string `json:"participants"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, -32700, "parse error", err.Error())
		return
	}

	ctx := &types.ConversationContext{
		ID:           uuid.New().String(),
		Type:         req.Type,
		Topic:        req.Topic,
		Participants: req.Participants,
		Status:       types.ConvoStatusActive,
		MessageCount: 0,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if ctx.Type == "" {
		ctx.Type = "chat"
	}

	pJSON, _ := json.Marshal(ctx.Participants)
	_, err := se.db.Exec(
		`INSERT INTO conversation_contexts (id, type, topic, participants_json, status, message_count, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)`,
		ctx.ID, ctx.Type, ctx.Topic, string(pJSON), ctx.Status, ctx.MessageCount, ctx.CreatedAt, ctx.UpdatedAt,
	)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, -32603, "create context failed", err.Error())
		return
	}

	se.logger.Info("conversation context created", "id", ctx.ID, "topic", ctx.Topic, "participants", len(ctx.Participants))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ctx)
}

func (se *SocialExtensions) handleGetContext(w http.ResponseWriter, r *http.Request) {
	contextID := chi.URLParam(r, "contextID")
	ctx, err := se.GetConversationContext(contextID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, -32001, "context not found", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ctx)
}

func (se *SocialExtensions) handleListContexts(w http.ResponseWriter, r *http.Request) {
	participant := r.URL.Query().Get("participant")
	status := r.URL.Query().Get("status")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
		limit = v
	}

	query := `SELECT id, type, COALESCE(topic,''), participants_json, status, message_count, created_at, updated_at FROM conversation_contexts WHERE 1=1`
	var args []interface{}

	if participant != "" {
		query += ` AND participants_json LIKE ?`
		args = append(args, "%"+participant+"%")
	}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := se.db.Query(query, args...)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, -32603, "query failed", err.Error())
		return
	}
	defer rows.Close()

	var contexts []*types.ConversationContext
	for rows.Next() {
		c := &types.ConversationContext{}
		var pJSON string
		if err := rows.Scan(&c.ID, &c.Type, &c.Topic, &pJSON, &c.Status, &c.MessageCount, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(pJSON), &c.Participants)
		contexts = append(contexts, c)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"contexts": contexts, "count": len(contexts)})
}

func (se *SocialExtensions) handleUpdateContext(w http.ResponseWriter, r *http.Request) {
	contextID := chi.URLParam(r, "contextID")

	var req struct {
		Status       *string  `json:"status,omitempty"`
		Topic        *string  `json:"topic,omitempty"`
		Participants []string `json:"participants,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, -32700, "parse error", err.Error())
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if req.Status != nil {
		se.db.Exec(`UPDATE conversation_contexts SET status = ?, updated_at = ? WHERE id = ?`, *req.Status, now, contextID)
	}
	if req.Topic != nil {
		se.db.Exec(`UPDATE conversation_contexts SET topic = ?, updated_at = ? WHERE id = ?`, *req.Topic, now, contextID)
	}
	if req.Participants != nil {
		pJSON, _ := json.Marshal(req.Participants)
		se.db.Exec(`UPDATE conversation_contexts SET participants_json = ?, updated_at = ? WHERE id = ?`, string(pJSON), now, contextID)
	}

	ctx, err := se.GetConversationContext(contextID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, -32001, "context not found", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ctx)
}

func (se *SocialExtensions) GetConversationContext(contextID string) (*types.ConversationContext, error) {
	c := &types.ConversationContext{}
	var pJSON string
	err := se.db.QueryRow(
		`SELECT id, type, COALESCE(topic,''), participants_json, status, message_count, created_at, updated_at FROM conversation_contexts WHERE id = ?`,
		contextID,
	).Scan(&c.ID, &c.Type, &c.Topic, &pJSON, &c.Status, &c.MessageCount, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(pJSON), &c.Participants)
	return c, nil
}

// IncrementContextMessageCount is called by the A2A server after forwarding a message in a conversation.
func (se *SocialExtensions) IncrementContextMessageCount(contextID string) {
	se.db.Exec(`UPDATE conversation_contexts SET message_count = message_count + 1, updated_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), contextID)
}

// ─── Layer 5: Social Feed SSE ───────────────────────────────────

func (se *SocialExtensions) handleFeedSubscribe(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, -32603, "streaming not supported", nil)
		return
	}

	ch := make(chan *types.SocialEvent, 32)
	se.addFeedSub(agentID, ch)
	defer se.removeFeedSub(agentID, ch)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Send initial keepalive
	fmt.Fprintf(w, ": connected to feed for %s\n\n", agentID)
	flusher.Flush()

	se.logger.Info("feed subscriber connected", "agent", agentID)

	ctx := r.Context()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			se.logger.Info("feed subscriber disconnected", "agent", agentID)
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "event: social\ndata: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func (se *SocialExtensions) addFeedSub(agentID string, ch chan *types.SocialEvent) {
	se.feedSubsMu.Lock()
	defer se.feedSubsMu.Unlock()
	se.feedSubs[agentID] = append(se.feedSubs[agentID], ch)
}

func (se *SocialExtensions) removeFeedSub(agentID string, ch chan *types.SocialEvent) {
	se.feedSubsMu.Lock()
	defer se.feedSubsMu.Unlock()
	subs := se.feedSubs[agentID]
	for i, sub := range subs {
		if sub == ch {
			se.feedSubs[agentID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}
	if len(se.feedSubs[agentID]) == 0 {
		delete(se.feedSubs, agentID)
	}
}

func (se *SocialExtensions) notifyFeed(agentID string, evt *types.SocialEvent) {
	if agentID == "" {
		return
	}
	se.feedSubsMu.RLock()
	defer se.feedSubsMu.RUnlock()
	for _, ch := range se.feedSubs[agentID] {
		select {
		case ch <- evt:
		default:
		}
	}
}
