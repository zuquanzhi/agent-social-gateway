package router

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/zuwance/agent-social-gateway/internal/session"
	"github.com/zuwance/agent-social-gateway/internal/storage"
	"github.com/zuwance/agent-social-gateway/internal/types"
)

type DirectRouter struct {
	db      *storage.DB
	session *session.Manager
	logger  *slog.Logger
}

func NewDirectRouter(db *storage.DB, sessionMgr *session.Manager, logger *slog.Logger) *DirectRouter {
	return &DirectRouter{
		db:      db,
		session: sessionMgr,
		logger:  logger.With("router", "direct"),
	}
}

func (d *DirectRouter) Send(ctx context.Context, from, to types.AgentID, msg *types.Message) error {
	data, err := json.Marshal(map[string]interface{}{
		"type":    "direct_message",
		"from":    string(from),
		"message": msg,
	})
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}

	if d.session.IsAgentOnline(to) {
		if err := d.session.SendToAgent(ctx, to, data); err != nil {
			d.logger.Warn("delivery failed, queuing", "to", to, "error", err)
			return d.queueForLater(to, data)
		}
		d.logger.Debug("message delivered", "from", from, "to", to)
		return nil
	}

	d.logger.Debug("agent offline, queuing", "to", to)
	return d.queueForLater(to, data)
}

func (d *DirectRouter) queueForLater(target types.AgentID, data []byte) error {
	_, err := d.db.Exec(`INSERT INTO pending_messages (target_agent, message_json) VALUES (?, ?)`,
		string(target), string(data))
	return err
}

func (d *DirectRouter) DeliverPending(ctx context.Context, agentID types.AgentID) (int, error) {
	rows, err := d.db.Query(`SELECT id, message_json FROM pending_messages WHERE target_agent = ? ORDER BY created_at ASC`, string(agentID))
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var delivered int
	var toDelete []int64

	for rows.Next() {
		var id int64
		var msgJSON string
		if err := rows.Scan(&id, &msgJSON); err != nil {
			continue
		}

		if err := d.session.SendToAgent(ctx, agentID, []byte(msgJSON)); err != nil {
			break
		}
		toDelete = append(toDelete, id)
		delivered++
	}

	for _, id := range toDelete {
		d.db.Exec(`DELETE FROM pending_messages WHERE id = ?`, id)
	}

	if delivered > 0 {
		d.logger.Info("pending messages delivered", "agent_id", agentID, "count", delivered)
	}
	return delivered, nil
}
