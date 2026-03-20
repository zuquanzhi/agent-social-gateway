package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/zuwance/agent-social-gateway/internal/storage"
	"github.com/zuwance/agent-social-gateway/internal/types"
)

type Manager struct {
	sessions map[string]*ActiveSession
	mu       sync.RWMutex
	db       *storage.DB
	logger   *slog.Logger
	ctx      context.Context
	cancel   context.CancelFunc
	timeout  time.Duration
}

type ActiveSession struct {
	types.Session
	Context   *SessionContext
	Transport Transport
}

type Transport interface {
	Send(ctx context.Context, data []byte) error
	Close() error
	Type() types.ConnectionType
}

func NewManager(db *storage.DB, logger *slog.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		sessions: make(map[string]*ActiveSession),
		db:       db,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
		timeout:  5 * time.Minute,
	}
	go m.cleanupLoop()
	return m
}

func (m *Manager) CreateSession(agentID types.AgentID, connType types.ConnectionType, transport Transport) (*ActiveSession, error) {
	sessionID := uuid.New().String()
	now := time.Now()

	session := &ActiveSession{
		Session: types.Session{
			ID:             sessionID,
			AgentID:        agentID,
			ConnectionType: connType,
			Status:         "active",
			CreatedAt:      now,
			LastActiveAt:   now,
		},
		Context:   NewSessionContext(),
		Transport: transport,
	}

	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	metaJSON, _ := json.Marshal(session.Metadata)
	m.db.Exec(`INSERT INTO sessions (id, agent_id, connection_type, status, metadata_json, created_at, last_active_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sessionID, string(agentID), string(connType), "active", string(metaJSON), now, now)

	m.logger.Info("session created", "session_id", sessionID, "agent_id", agentID, "type", connType)
	return session, nil
}

func (m *Manager) GetSession(sessionID string) (*ActiveSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[sessionID]
	return s, ok
}

func (m *Manager) GetSessionsByAgent(agentID types.AgentID) []*ActiveSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*ActiveSession
	for _, s := range m.sessions {
		if s.AgentID == agentID && s.Status == "active" {
			result = append(result, s)
		}
	}
	return result
}

func (m *Manager) TouchSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[sessionID]; ok {
		s.LastActiveAt = time.Now()
		m.db.Exec(`UPDATE sessions SET last_active_at = ? WHERE id = ?`, s.LastActiveAt, sessionID)
	}
}

func (m *Manager) CloseSession(sessionID string) error {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	if ok {
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if session.Transport != nil {
		session.Transport.Close()
	}
	session.Status = "closed"

	m.db.Exec(`UPDATE sessions SET status = 'closed' WHERE id = ?`, sessionID)
	m.logger.Info("session closed", "session_id", sessionID, "agent_id", session.AgentID)
	return nil
}

func (m *Manager) IsAgentOnline(agentID types.AgentID) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.sessions {
		if s.AgentID == agentID && s.Status == "active" {
			return true
		}
	}
	return false
}

func (m *Manager) SendToAgent(ctx context.Context, agentID types.AgentID, data []byte) error {
	sessions := m.GetSessionsByAgent(agentID)
	if len(sessions) == 0 {
		return fmt.Errorf("agent %s not online", agentID)
	}

	var lastErr error
	for _, s := range sessions {
		if s.Transport != nil {
			if err := s.Transport.Send(ctx, data); err != nil {
				lastErr = err
				m.logger.Warn("send failed", "session_id", s.ID, "error", err)
			}
		}
	}
	return lastErr
}

func (m *Manager) ListActiveSessions() []*ActiveSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*ActiveSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.cleanup()
		}
	}
}

func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-m.timeout)
	var toRemove []string

	for id, s := range m.sessions {
		if s.LastActiveAt.Before(cutoff) {
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		session := m.sessions[id]
		if session.Transport != nil {
			session.Transport.Close()
		}
		delete(m.sessions, id)
		m.db.Exec(`UPDATE sessions SET status = 'timeout' WHERE id = ?`, id)
		m.logger.Info("session timed out", "session_id", id, "agent_id", session.AgentID)
	}
}

func (m *Manager) Close() {
	m.cancel()

	m.mu.Lock()
	defer m.mu.Unlock()

	for id, s := range m.sessions {
		if s.Transport != nil {
			s.Transport.Close()
		}
		m.db.Exec(`UPDATE sessions SET status = 'closed' WHERE id = ?`, id)
	}
	m.sessions = make(map[string]*ActiveSession)
}
