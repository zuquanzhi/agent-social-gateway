package social

import (
	"log/slog"

	"github.com/zuwance/agent-social-gateway/internal/storage"
	"github.com/zuwance/agent-social-gateway/internal/types"
)

type Timeline struct {
	db     *storage.DB
	logger *slog.Logger
}

func NewTimeline(db *storage.DB, logger *slog.Logger) *Timeline {
	return &Timeline{db: db, logger: logger.With("component", "timeline")}
}

func (t *Timeline) AddEvent(agentID types.AgentID, eventType string, sourceAgent types.AgentID, messageID string) error {
	var msgID interface{}
	if messageID != "" {
		msgID = messageID
	}
	_, err := t.db.Exec(
		`INSERT INTO timeline_events (agent_id, event_type, source_agent, message_id) VALUES (?, ?, ?, ?)`,
		string(agentID), eventType, string(sourceAgent), msgID)
	return err
}

func (t *Timeline) GetTimeline(agentID types.AgentID, cursor int64, limit int) ([]types.TimelineEvent, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT id, agent_id, event_type, source_agent, COALESCE(message_id, ''), created_at
		FROM timeline_events WHERE agent_id = ?`
	args := []interface{}{string(agentID)}

	if cursor > 0 {
		query += ` AND id < ?`
		args = append(args, cursor)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := t.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []types.TimelineEvent
	for rows.Next() {
		var e types.TimelineEvent
		var agentStr, sourceStr string
		if err := rows.Scan(&e.ID, &agentStr, &e.EventType, &sourceStr, &e.MessageID, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.AgentID = types.AgentID(agentStr)
		e.SourceAgent = types.AgentID(sourceStr)
		events = append(events, e)
	}
	return events, rows.Err()
}

// PopulateTimelineForFollowers generates timeline events for all followers of a source agent.
func (t *Timeline) PopulateTimelineForFollowers(graph *Graph, sourceAgent types.AgentID, eventType, messageID string) error {
	followers, err := graph.GetFollowers(sourceAgent)
	if err != nil {
		return err
	}

	for _, follower := range followers {
		if err := t.AddEvent(follower, eventType, sourceAgent, messageID); err != nil {
			t.logger.Warn("failed to add timeline event", "follower", follower, "error", err)
		}
	}
	return nil
}
