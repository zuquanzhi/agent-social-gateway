package router

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/zuwance/agent-social-gateway/internal/session"
	"github.com/zuwance/agent-social-gateway/internal/storage"
	"github.com/zuwance/agent-social-gateway/internal/types"
)

type GroupRouter struct {
	db      *storage.DB
	session *session.Manager
	logger  *slog.Logger
}

func NewGroupRouter(db *storage.DB, sessionMgr *session.Manager, logger *slog.Logger) *GroupRouter {
	return &GroupRouter{
		db:      db,
		session: sessionMgr,
		logger:  logger.With("router", "group"),
	}
}

func (g *GroupRouter) CreateGroup(name, description string, createdBy types.AgentID) (*types.Group, error) {
	group := &types.Group{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		CreatedBy:   createdBy,
	}

	_, err := g.db.Exec(`INSERT INTO groups (id, name, description, created_by) VALUES (?, ?, ?, ?)`,
		group.ID, group.Name, group.Description, string(group.CreatedBy))
	if err != nil {
		return nil, fmt.Errorf("creating group: %w", err)
	}

	if err := g.Join(group.ID, createdBy, "admin"); err != nil {
		return nil, fmt.Errorf("joining as creator: %w", err)
	}

	g.logger.Info("group created", "group_id", group.ID, "name", name, "by", createdBy)
	return group, nil
}

func (g *GroupRouter) Join(groupID string, agentID types.AgentID, role string) error {
	if role == "" {
		role = "member"
	}
	_, err := g.db.Exec(`INSERT OR IGNORE INTO group_members (group_id, agent_id, role) VALUES (?, ?, ?)`,
		groupID, string(agentID), role)
	if err != nil {
		return fmt.Errorf("joining group: %w", err)
	}
	g.logger.Info("agent joined group", "group_id", groupID, "agent_id", agentID)
	return nil
}

func (g *GroupRouter) Leave(groupID string, agentID types.AgentID) error {
	_, err := g.db.Exec(`DELETE FROM group_members WHERE group_id = ? AND agent_id = ?`,
		groupID, string(agentID))
	return err
}

func (g *GroupRouter) GetMembers(groupID string) ([]types.GroupMember, error) {
	rows, err := g.db.Query(`SELECT group_id, agent_id, role, joined_at FROM group_members WHERE group_id = ?`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []types.GroupMember
	for rows.Next() {
		var m types.GroupMember
		var agentID string
		if err := rows.Scan(&m.GroupID, &agentID, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		m.AgentID = types.AgentID(agentID)
		members = append(members, m)
	}
	return members, rows.Err()
}

func (g *GroupRouter) GetGroup(groupID string) (*types.Group, error) {
	var group types.Group
	var createdBy string
	err := g.db.QueryRow(`SELECT id, name, description, created_by, created_at FROM groups WHERE id = ?`, groupID).
		Scan(&group.ID, &group.Name, &group.Description, &createdBy, &group.CreatedAt)
	if err != nil {
		return nil, err
	}
	group.CreatedBy = types.AgentID(createdBy)
	return &group, nil
}

func (g *GroupRouter) ListGroups() ([]types.Group, error) {
	rows, err := g.db.Query(`SELECT id, name, description, created_by, created_at FROM groups ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []types.Group
	for rows.Next() {
		var group types.Group
		var createdBy string
		if err := rows.Scan(&group.ID, &group.Name, &group.Description, &createdBy, &group.CreatedAt); err != nil {
			return nil, err
		}
		group.CreatedBy = types.AgentID(createdBy)
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (g *GroupRouter) Relay(ctx context.Context, from types.AgentID, groupID string, msg *types.Message) error {
	members, err := g.GetMembers(groupID)
	if err != nil {
		return fmt.Errorf("getting group members: %w", err)
	}

	data, err := json.Marshal(map[string]interface{}{
		"type":     "group_message",
		"from":     string(from),
		"group_id": groupID,
		"message":  msg,
	})
	if err != nil {
		return fmt.Errorf("marshaling group message: %w", err)
	}

	var delivered, queued int
	for _, m := range members {
		if m.AgentID == from {
			continue
		}

		if g.session.IsAgentOnline(m.AgentID) {
			if err := g.session.SendToAgent(ctx, m.AgentID, data); err == nil {
				delivered++
				continue
			}
		}
		g.db.Exec(`INSERT INTO pending_messages (target_agent, message_json) VALUES (?, ?)`,
			string(m.AgentID), string(data))
		queued++
	}

	g.logger.Debug("group message relayed", "group_id", groupID, "from", from, "delivered", delivered, "queued", queued)
	return nil
}

func (g *GroupRouter) DeleteGroup(groupID string) error {
	_, err := g.db.Exec(`DELETE FROM groups WHERE id = ?`, groupID)
	return err
}
