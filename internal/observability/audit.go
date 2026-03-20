package observability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/zuwance/agent-social-gateway/internal/storage"
)

type AuditLogger struct {
	db     *storage.DB
	logger *slog.Logger
}

func NewAuditLogger(db *storage.DB, logger *slog.Logger) *AuditLogger {
	return &AuditLogger{
		db:     db,
		logger: logger.With("component", "audit"),
	}
}

type AuditEntry struct {
	ID          int64     `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	SessionID   string    `json:"sessionId,omitempty"`
	FromAgent   string    `json:"fromAgent,omitempty"`
	ToAgent     string    `json:"toAgent,omitempty"`
	Action      string    `json:"action"`
	PayloadHash string    `json:"payloadHash,omitempty"`
	Details     string    `json:"details,omitempty"`
}

func (a *AuditLogger) Log(sessionID, fromAgent, toAgent, action string, details interface{}) {
	var detailsJSON string
	var payloadHash string

	if details != nil {
		data, _ := json.Marshal(details)
		detailsJSON = string(data)
		hash := sha256.Sum256(data)
		payloadHash = hex.EncodeToString(hash[:8])
	}

	_, err := a.db.Exec(
		`INSERT INTO audit_log (session_id, from_agent, to_agent, action, payload_hash, details_json) VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID, fromAgent, toAgent, action, payloadHash, detailsJSON)
	if err != nil {
		a.logger.Warn("failed to write audit log", "error", err)
	}
}

func (a *AuditLogger) Query(action string, limit, offset int) ([]AuditEntry, error) {
	query := `SELECT id, timestamp, COALESCE(session_id,''), COALESCE(from_agent,''), COALESCE(to_agent,''), action, COALESCE(payload_hash,''), COALESCE(details_json,'')
		FROM audit_log`
	args := []interface{}{}

	if action != "" {
		query += ` WHERE action LIKE ?`
		args = append(args, "%"+action+"%")
	}
	query += ` ORDER BY timestamp DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := a.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.SessionID, &e.FromAgent, &e.ToAgent, &e.Action, &e.PayloadHash, &e.Details); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (a *AuditLogger) Count(action string) (int, error) {
	query := `SELECT COUNT(*) FROM audit_log`
	args := []interface{}{}
	if action != "" {
		query += ` WHERE action LIKE ?`
		args = append(args, "%"+action+"%")
	}
	var count int
	err := a.db.QueryRow(query, args...).Scan(&count)
	return count, err
}
