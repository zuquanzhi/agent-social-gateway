package discovery

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/zuwance/agent-social-gateway/internal/storage"
	"github.com/zuwance/agent-social-gateway/internal/types"
)

type Cache struct {
	db     *storage.DB
	ttl    time.Duration
	logger *slog.Logger
}

func NewCache(db *storage.DB, logger *slog.Logger) *Cache {
	return &Cache{
		db:     db,
		ttl:    1 * time.Hour,
		logger: logger.With("component", "discovery-cache"),
	}
}

func (c *Cache) SetTTL(d time.Duration) {
	c.ttl = d
}

func (c *Cache) Put(agentID string, card *types.AgentCard, etag string) error {
	cardJSON, err := json.Marshal(card)
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(c.ttl)
	_, err = c.db.Exec(`INSERT OR REPLACE INTO agent_cards (agent_id, card_json, etag, fetched_at, expires_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?)`,
		agentID, string(cardJSON), etag, expiresAt)
	return err
}

func (c *Cache) Get(agentID string) (*types.AgentCard, string, bool) {
	var cardJSON, etag string
	var expiresAt time.Time

	err := c.db.QueryRow(`SELECT card_json, COALESCE(etag, ''), expires_at FROM agent_cards WHERE agent_id = ?`, agentID).
		Scan(&cardJSON, &etag, &expiresAt)
	if err != nil {
		return nil, "", false
	}

	if time.Now().After(expiresAt) {
		return nil, etag, false
	}

	var card types.AgentCard
	if err := json.Unmarshal([]byte(cardJSON), &card); err != nil {
		return nil, "", false
	}
	return &card, etag, true
}

func (c *Cache) SearchByName(name string) ([]types.AgentCard, error) {
	rows, err := c.db.Query(`SELECT card_json FROM agent_cards WHERE json_extract(card_json, '$.name') LIKE ?`,
		"%"+name+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return c.scanCards(rows)
}

func (c *Cache) SearchBySkillTag(tag string) ([]types.AgentCard, error) {
	rows, err := c.db.Query(`SELECT card_json FROM agent_cards WHERE card_json LIKE ?`,
		"%"+tag+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cards, err := c.scanCards(rows)
	if err != nil {
		return nil, err
	}

	// Post-filter for accurate tag matching
	var filtered []types.AgentCard
	for _, card := range cards {
		for _, skill := range card.Skills {
			for _, t := range skill.Tags {
				if t == tag {
					filtered = append(filtered, card)
					goto next
				}
			}
		}
	next:
	}
	return filtered, nil
}

func (c *Cache) SearchByReputation(minScore float64) ([]CachedAgent, error) {
	rows, err := c.db.Query(
		`SELECT ac.agent_id, ac.card_json, COALESCE(a.reputation_score, 0) as score
		 FROM agent_cards ac
		 LEFT JOIN agents a ON ac.agent_id = a.id
		 WHERE COALESCE(a.reputation_score, 0) >= ?
		 ORDER BY score DESC`, minScore)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CachedAgent
	for rows.Next() {
		var agentID, cardJSON string
		var score float64
		if err := rows.Scan(&agentID, &cardJSON, &score); err != nil {
			continue
		}
		var card types.AgentCard
		json.Unmarshal([]byte(cardJSON), &card)
		results = append(results, CachedAgent{
			AgentID:         agentID,
			Card:            card,
			ReputationScore: score,
		})
	}
	return results, rows.Err()
}

func (c *Cache) ListAll() ([]CachedAgent, error) {
	rows, err := c.db.Query(
		`SELECT ac.agent_id, ac.card_json, COALESCE(a.reputation_score, 0)
		 FROM agent_cards ac
		 LEFT JOIN agents a ON ac.agent_id = a.id
		 ORDER BY COALESCE(a.reputation_score, 0) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CachedAgent
	for rows.Next() {
		var agentID, cardJSON string
		var score float64
		if err := rows.Scan(&agentID, &cardJSON, &score); err != nil {
			continue
		}
		var card types.AgentCard
		json.Unmarshal([]byte(cardJSON), &card)
		results = append(results, CachedAgent{AgentID: agentID, Card: card, ReputationScore: score})
	}
	return results, rows.Err()
}

func (c *Cache) Delete(agentID string) error {
	_, err := c.db.Exec(`DELETE FROM agent_cards WHERE agent_id = ?`, agentID)
	return err
}

func (c *Cache) CleanExpired() (int64, error) {
	result, err := c.db.Exec(`DELETE FROM agent_cards WHERE expires_at < CURRENT_TIMESTAMP`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (c *Cache) scanCards(rows *sql.Rows) ([]types.AgentCard, error) {
	var cards []types.AgentCard
	for rows.Next() {
		var cardJSON string
		if err := rows.Scan(&cardJSON); err != nil {
			return nil, err
		}
		var card types.AgentCard
		if err := json.Unmarshal([]byte(cardJSON), &card); err != nil {
			continue
		}
		cards = append(cards, card)
	}
	return cards, rows.Err()
}

type CachedAgent struct {
	AgentID         string          `json:"agentId"`
	Card            types.AgentCard `json:"card"`
	ReputationScore float64         `json:"reputationScore"`
}
