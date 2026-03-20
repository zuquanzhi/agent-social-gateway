package router

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/zuwance/agent-social-gateway/internal/session"
	"github.com/zuwance/agent-social-gateway/internal/storage"
	"github.com/zuwance/agent-social-gateway/internal/types"
)

type MessageRouter interface {
	RouteDirectMessage(ctx context.Context, from, to types.AgentID, msg *types.Message) error
	RouteBroadcast(ctx context.Context, from types.AgentID, topic string, msg *types.Message) error
	RouteGroupMessage(ctx context.Context, from types.AgentID, groupID string, msg *types.Message) error
}

type Router struct {
	direct  *DirectRouter
	pubsub  *PubSubRouter
	group   *GroupRouter
	db      *storage.DB
	session *session.Manager
	logger  *slog.Logger
}

func New(db *storage.DB, sessionMgr *session.Manager, logger *slog.Logger) *Router {
	r := &Router{
		db:      db,
		session: sessionMgr,
		logger:  logger,
	}
	r.direct = NewDirectRouter(db, sessionMgr, logger)
	r.pubsub = NewPubSubRouter(db, sessionMgr, logger)
	r.group = NewGroupRouter(db, sessionMgr, logger)
	return r
}

func (r *Router) RouteDirectMessage(ctx context.Context, from, to types.AgentID, msg *types.Message) error {
	if err := r.persistMessage(from, to, "", "", msg, types.MsgDirect); err != nil {
		r.logger.Warn("failed to persist direct message", "error", err)
	}
	return r.direct.Send(ctx, from, to, msg)
}

func (r *Router) RouteBroadcast(ctx context.Context, from types.AgentID, topic string, msg *types.Message) error {
	if err := r.persistMessage(from, "", "", topic, msg, types.MsgBroadcast); err != nil {
		r.logger.Warn("failed to persist broadcast", "error", err)
	}
	return r.pubsub.Publish(ctx, from, topic, msg)
}

func (r *Router) RouteGroupMessage(ctx context.Context, from types.AgentID, groupID string, msg *types.Message) error {
	if err := r.persistMessage(from, "", groupID, "", msg, types.MsgGroup); err != nil {
		r.logger.Warn("failed to persist group message", "error", err)
	}
	return r.group.Relay(ctx, from, groupID, msg)
}

func (r *Router) Direct() *DirectRouter   { return r.direct }
func (r *Router) PubSub() *PubSubRouter   { return r.pubsub }
func (r *Router) Group() *GroupRouter      { return r.group }

func (r *Router) persistMessage(from types.AgentID, to types.AgentID, groupID, topic string, msg *types.Message, msgType types.MessageType) error {
	id := msg.MessageID
	if id == "" {
		id = uuid.New().String()
	}
	contentJSON, _ := json.Marshal(msg)
	_, err := r.db.Exec(`INSERT INTO messages (id, from_agent, to_agent, group_id, topic, content_json, message_type) VALUES (?,?,?,?,?,?,?)`,
		id, string(from), string(to), groupID, topic, string(contentJSON), string(msgType))
	return err
}
