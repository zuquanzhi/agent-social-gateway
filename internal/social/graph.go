package social

import (
	"log/slog"

	"github.com/zuwance/agent-social-gateway/internal/storage"
	"github.com/zuwance/agent-social-gateway/internal/types"
)

type Graph struct {
	db     *storage.DB
	logger *slog.Logger
}

func NewGraph(db *storage.DB, logger *slog.Logger) *Graph {
	return &Graph{db: db, logger: logger.With("component", "social-graph")}
}

func (g *Graph) GetFollowers(agentID types.AgentID) ([]types.AgentID, error) {
	return g.queryRelatedAgents(
		`SELECT from_agent FROM social_relations WHERE to_agent = ? AND relation_type = ?`,
		string(agentID), string(types.RelFollow),
	)
}

func (g *Graph) GetFollowing(agentID types.AgentID) ([]types.AgentID, error) {
	return g.queryRelatedAgents(
		`SELECT to_agent FROM social_relations WHERE from_agent = ? AND relation_type = ?`,
		string(agentID), string(types.RelFollow),
	)
}

func (g *Graph) GetMutualFollows(agentID types.AgentID) ([]types.AgentID, error) {
	return g.queryRelatedAgents(
		`SELECT r1.to_agent FROM social_relations r1
		 INNER JOIN social_relations r2 ON r1.to_agent = r2.from_agent AND r2.to_agent = r1.from_agent
		 WHERE r1.from_agent = ? AND r1.relation_type = ? AND r2.relation_type = ?`,
		string(agentID), string(types.RelFollow), string(types.RelFollow),
	)
}

func (g *Graph) GetEndorsements(agentID types.AgentID) ([]types.SocialRelation, error) {
	return g.queryRelations(
		`SELECT id, from_agent, to_agent, relation_type, metadata_json, created_at FROM social_relations WHERE to_agent = ? AND relation_type = ?`,
		string(agentID), string(types.RelEndorse),
	)
}

func (g *Graph) GetEndorsementsBySkill(agentID types.AgentID, skillID string) (int, error) {
	var count int
	err := g.db.QueryRow(
		`SELECT COUNT(*) FROM social_relations WHERE to_agent = ? AND relation_type = ? AND json_extract(metadata_json, '$.skill_id') = ?`,
		string(agentID), string(types.RelEndorse), skillID).Scan(&count)
	return count, err
}

func (g *Graph) GetCollaborationRequests(agentID types.AgentID) ([]types.SocialRelation, error) {
	return g.queryRelations(
		`SELECT id, from_agent, to_agent, relation_type, metadata_json, created_at FROM social_relations WHERE to_agent = ? AND relation_type = ?`,
		string(agentID), string(types.RelCollaborate),
	)
}

func (g *Graph) IsFollowing(follower, target types.AgentID) (bool, error) {
	var count int
	err := g.db.QueryRow(
		`SELECT COUNT(*) FROM social_relations WHERE from_agent = ? AND to_agent = ? AND relation_type = ?`,
		string(follower), string(target), string(types.RelFollow)).Scan(&count)
	return count > 0, err
}

func (g *Graph) GetFollowerCount(agentID types.AgentID) (int, error) {
	var count int
	err := g.db.QueryRow(
		`SELECT COUNT(*) FROM social_relations WHERE to_agent = ? AND relation_type = ?`,
		string(agentID), string(types.RelFollow)).Scan(&count)
	return count, err
}

func (g *Graph) GetFollowingCount(agentID types.AgentID) (int, error) {
	var count int
	err := g.db.QueryRow(
		`SELECT COUNT(*) FROM social_relations WHERE from_agent = ? AND relation_type = ?`,
		string(agentID), string(types.RelFollow)).Scan(&count)
	return count, err
}

func (g *Graph) GetReputationScore(agentID types.AgentID) (float64, error) {
	var score float64
	err := g.db.QueryRow(`SELECT COALESCE(reputation_score, 0) FROM agents WHERE id = ?`,
		string(agentID)).Scan(&score)
	return score, err
}

func (g *Graph) queryRelatedAgents(query string, args ...interface{}) ([]types.AgentID, error) {
	rows, err := g.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []types.AgentID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		agents = append(agents, types.AgentID(id))
	}
	return agents, rows.Err()
}

func (g *Graph) queryRelations(query string, args ...interface{}) ([]types.SocialRelation, error) {
	rows, err := g.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relations []types.SocialRelation
	for rows.Next() {
		var r types.SocialRelation
		var from, to, relType string
		var metaJSON *string
		if err := rows.Scan(&r.ID, &from, &to, &relType, &metaJSON, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.FromAgent = types.AgentID(from)
		r.ToAgent = types.AgentID(to)
		r.RelationType = types.RelationType(relType)
		relations = append(relations, r)
	}
	return relations, rows.Err()
}
