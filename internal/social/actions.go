package social

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/zuwance/agent-social-gateway/internal/storage"
	"github.com/zuwance/agent-social-gateway/internal/types"
)

type Actions struct {
	db     *storage.DB
	graph  *Graph
	logger *slog.Logger
}

func NewActions(db *storage.DB, logger *slog.Logger) *Actions {
	return &Actions{
		db:     db,
		graph:  NewGraph(db, logger),
		logger: logger.With("component", "social-actions"),
	}
}

func (a *Actions) Graph() *Graph { return a.graph }

func (a *Actions) Follow(follower, target types.AgentID) error {
	_, err := a.db.Exec(`INSERT OR IGNORE INTO social_relations (from_agent, to_agent, relation_type) VALUES (?, ?, ?)`,
		string(follower), string(target), string(types.RelFollow))
	if err != nil {
		return fmt.Errorf("follow: %w", err)
	}
	a.logger.Info("follow", "from", follower, "to", target)
	return nil
}

func (a *Actions) Unfollow(follower, target types.AgentID) error {
	_, err := a.db.Exec(`DELETE FROM social_relations WHERE from_agent = ? AND to_agent = ? AND relation_type = ?`,
		string(follower), string(target), string(types.RelFollow))
	if err != nil {
		return fmt.Errorf("unfollow: %w", err)
	}
	a.logger.Info("unfollow", "from", follower, "to", target)
	return nil
}

func (a *Actions) Like(agentID types.AgentID, messageID string) error {
	_, err := a.db.Exec(`INSERT OR IGNORE INTO likes (agent_id, message_id) VALUES (?, ?)`,
		string(agentID), messageID)
	if err != nil {
		return fmt.Errorf("like: %w", err)
	}
	a.logger.Info("like", "agent", agentID, "message", messageID)
	return nil
}

func (a *Actions) Unlike(agentID types.AgentID, messageID string) error {
	_, err := a.db.Exec(`DELETE FROM likes WHERE agent_id = ? AND message_id = ?`,
		string(agentID), messageID)
	return err
}

func (a *Actions) GetLikeCount(messageID string) (int, error) {
	var count int
	err := a.db.QueryRow(`SELECT COUNT(*) FROM likes WHERE message_id = ?`, messageID).Scan(&count)
	return count, err
}

func (a *Actions) Endorse(from types.AgentID, target types.AgentID, skillID string) error {
	meta := map[string]string{"skill_id": skillID}
	metaJSON, _ := json.Marshal(meta)

	_, err := a.db.Exec(`INSERT OR REPLACE INTO social_relations (from_agent, to_agent, relation_type, metadata_json) VALUES (?, ?, ?, ?)`,
		string(from), string(target), string(types.RelEndorse), string(metaJSON))
	if err != nil {
		return fmt.Errorf("endorse: %w", err)
	}

	a.updateReputation(target)
	a.logger.Info("endorse", "from", from, "to", target, "skill", skillID)
	return nil
}

func (a *Actions) RequestCollaboration(from, to types.AgentID, proposal map[string]interface{}) error {
	metaJSON, _ := json.Marshal(proposal)
	_, err := a.db.Exec(`INSERT INTO social_relations (from_agent, to_agent, relation_type, metadata_json) VALUES (?, ?, ?, ?)`,
		string(from), string(to), string(types.RelCollaborate), string(metaJSON))
	if err != nil {
		return fmt.Errorf("request collaboration: %w", err)
	}
	a.logger.Info("collaboration requested", "from", from, "to", to)
	return nil
}

func (a *Actions) updateReputation(agentID types.AgentID) {
	var count int
	a.db.QueryRow(`SELECT COUNT(*) FROM social_relations WHERE to_agent = ? AND relation_type = ?`,
		string(agentID), string(types.RelEndorse)).Scan(&count)

	// Simple reputation: normalized endorsement count (log scale)
	score := float64(count) / (float64(count) + 10.0) // sigmoid-like normalization
	a.db.Exec(`UPDATE agents SET reputation_score = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		score, string(agentID))
}

func (a *Actions) EnsureAgent(agentID types.AgentID, name string) error {
	_, err := a.db.Exec(`INSERT OR IGNORE INTO agents (id, name) VALUES (?, ?)`,
		string(agentID), name)
	return err
}

func (a *Actions) EnsureMessage(messageID string, fromAgent types.AgentID) {
	a.db.Exec(`INSERT OR IGNORE INTO messages (id, from_agent, content_json, message_type) VALUES (?, ?, '{}', 'reference')`,
		messageID, string(fromAgent))
}
