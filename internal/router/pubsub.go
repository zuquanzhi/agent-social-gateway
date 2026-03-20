package router

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/zuwance/agent-social-gateway/internal/session"
	"github.com/zuwance/agent-social-gateway/internal/storage"
	"github.com/zuwance/agent-social-gateway/internal/types"
)

type PubSubRouter struct {
	db          *storage.DB
	session     *session.Manager
	logger      *slog.Logger
	subscribers map[string]map[types.AgentID]bool // topic -> agent set (in-memory cache)
	mu          sync.RWMutex
}

func NewPubSubRouter(db *storage.DB, sessionMgr *session.Manager, logger *slog.Logger) *PubSubRouter {
	ps := &PubSubRouter{
		db:          db,
		session:     sessionMgr,
		logger:      logger.With("router", "pubsub"),
		subscribers: make(map[string]map[types.AgentID]bool),
	}
	ps.loadSubscriptions()
	return ps
}

func (ps *PubSubRouter) Subscribe(agentID types.AgentID, topic string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.subscribers[topic] == nil {
		ps.subscribers[topic] = make(map[types.AgentID]bool)
	}
	ps.subscribers[topic][agentID] = true

	_, err := ps.db.Exec(`INSERT OR IGNORE INTO subscriptions (agent_id, topic) VALUES (?, ?)`,
		string(agentID), topic)
	if err != nil {
		return fmt.Errorf("persisting subscription: %w", err)
	}

	ps.logger.Info("subscribed", "agent_id", agentID, "topic", topic)
	return nil
}

func (ps *PubSubRouter) Unsubscribe(agentID types.AgentID, topic string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if subs, ok := ps.subscribers[topic]; ok {
		delete(subs, agentID)
		if len(subs) == 0 {
			delete(ps.subscribers, topic)
		}
	}

	_, err := ps.db.Exec(`DELETE FROM subscriptions WHERE agent_id = ? AND topic = ?`,
		string(agentID), topic)
	return err
}

func (ps *PubSubRouter) Publish(ctx context.Context, from types.AgentID, topic string, msg *types.Message) error {
	ps.mu.RLock()
	subs := ps.subscribers[topic]
	agents := make([]types.AgentID, 0, len(subs))
	for agentID := range subs {
		if agentID != from {
			agents = append(agents, agentID)
		}
	}
	ps.mu.RUnlock()

	data, err := json.Marshal(map[string]interface{}{
		"type":    "broadcast",
		"from":    string(from),
		"topic":   topic,
		"message": msg,
	})
	if err != nil {
		return fmt.Errorf("marshaling broadcast: %w", err)
	}

	var delivered, queued int
	for _, agentID := range agents {
		if ps.session.IsAgentOnline(agentID) {
			if err := ps.session.SendToAgent(ctx, agentID, data); err == nil {
				delivered++
				continue
			}
		}
		ps.queueForLater(agentID, data)
		queued++
	}

	ps.logger.Debug("broadcast published", "topic", topic, "from", from, "delivered", delivered, "queued", queued)
	return nil
}

func (ps *PubSubRouter) GetSubscribers(topic string) []types.AgentID {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var agents []types.AgentID
	for agentID := range ps.subscribers[topic] {
		agents = append(agents, agentID)
	}
	return agents
}

func (ps *PubSubRouter) GetTopics() []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	topics := make([]string, 0, len(ps.subscribers))
	for t := range ps.subscribers {
		topics = append(topics, t)
	}
	return topics
}

func (ps *PubSubRouter) loadSubscriptions() {
	rows, err := ps.db.Query(`SELECT agent_id, topic FROM subscriptions`)
	if err != nil {
		ps.logger.Warn("failed to load subscriptions", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var agentID, topic string
		if err := rows.Scan(&agentID, &topic); err != nil {
			continue
		}
		if ps.subscribers[topic] == nil {
			ps.subscribers[topic] = make(map[types.AgentID]bool)
		}
		ps.subscribers[topic][types.AgentID(agentID)] = true
	}
}

func (ps *PubSubRouter) queueForLater(target types.AgentID, data []byte) {
	ps.db.Exec(`INSERT INTO pending_messages (target_agent, message_json) VALUES (?, ?)`,
		string(target), string(data))
}
