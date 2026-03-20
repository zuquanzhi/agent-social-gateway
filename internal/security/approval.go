package security

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/zuwance/agent-social-gateway/internal/storage"
	"github.com/zuwance/agent-social-gateway/internal/types"
)

type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalDenied   ApprovalStatus = "denied"
)

type ApprovalRequest struct {
	ID          string                 `json:"id"`
	AgentID     types.AgentID          `json:"agentId"`
	Action      string                 `json:"action"`
	Details     map[string]interface{} `json:"details"`
	Status      ApprovalStatus         `json:"status"`
	CreatedAt   time.Time              `json:"createdAt"`
	ResolvedAt  *time.Time             `json:"resolvedAt,omitempty"`
	ResolvedBy  string                 `json:"resolvedBy,omitempty"`
}

type ApprovalQueue struct {
	db       *storage.DB
	logger   *slog.Logger
	pending  map[string]*ApprovalRequest
	waiters  map[string]chan ApprovalStatus
	mu       sync.RWMutex
	actions  map[string]bool // actions requiring approval
}

func NewApprovalQueue(db *storage.DB, logger *slog.Logger) *ApprovalQueue {
	return &ApprovalQueue{
		db:      db,
		logger:  logger.With("component", "approval"),
		pending: make(map[string]*ApprovalRequest),
		waiters: make(map[string]chan ApprovalStatus),
		actions: map[string]bool{
			"delete_agent":   true,
			"admin_action":   true,
			"bulk_broadcast": true,
		},
	}
}

func (q *ApprovalQueue) SetSensitiveActions(actions []string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.actions = make(map[string]bool)
	for _, a := range actions {
		q.actions[a] = true
	}
}

func (q *ApprovalQueue) RequiresApproval(action string) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.actions[action]
}

func (q *ApprovalQueue) Submit(agentID types.AgentID, action string, details map[string]interface{}) (*ApprovalRequest, chan ApprovalStatus, error) {
	req := &ApprovalRequest{
		ID:        uuid.New().String(),
		AgentID:   agentID,
		Action:    action,
		Details:   details,
		Status:    ApprovalPending,
		CreatedAt: time.Now(),
	}

	detailsJSON, _ := json.Marshal(details)
	q.db.Exec(`INSERT INTO audit_log (session_id, from_agent, action, details_json) VALUES (?, ?, ?, ?)`,
		req.ID, string(agentID), "approval_requested:"+action, string(detailsJSON))

	ch := make(chan ApprovalStatus, 1)

	q.mu.Lock()
	q.pending[req.ID] = req
	q.waiters[req.ID] = ch
	q.mu.Unlock()

	q.logger.Info("approval requested", "id", req.ID, "agent", agentID, "action", action)
	return req, ch, nil
}

func (q *ApprovalQueue) Approve(reqID, approvedBy string) error {
	return q.resolve(reqID, ApprovalApproved, approvedBy)
}

func (q *ApprovalQueue) Deny(reqID, deniedBy string) error {
	return q.resolve(reqID, ApprovalDenied, deniedBy)
}

func (q *ApprovalQueue) resolve(reqID string, status ApprovalStatus, resolvedBy string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	req, ok := q.pending[reqID]
	if !ok {
		return ErrApprovalNotFound
	}

	now := time.Now()
	req.Status = status
	req.ResolvedAt = &now
	req.ResolvedBy = resolvedBy

	if ch, ok := q.waiters[reqID]; ok {
		ch <- status
		close(ch)
		delete(q.waiters, reqID)
	}
	delete(q.pending, reqID)

	q.logger.Info("approval resolved", "id", reqID, "status", status, "by", resolvedBy)
	return nil
}

func (q *ApprovalQueue) ListPending() []*ApprovalRequest {
	q.mu.RLock()
	defer q.mu.RUnlock()
	result := make([]*ApprovalRequest, 0, len(q.pending))
	for _, r := range q.pending {
		result = append(result, r)
	}
	return result
}
