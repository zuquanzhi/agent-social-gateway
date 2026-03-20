package security

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/zuwance/agent-social-gateway/internal/config"
)

type BudgetManager struct {
	cfg     *config.TokenBudgetConfig
	budgets map[string]*Budget
	mu      sync.RWMutex
	logger  *slog.Logger
}

type Budget struct {
	SessionID   string  `json:"sessionId"`
	MaxTokens   int     `json:"maxTokens"`
	UsedTokens  int     `json:"usedTokens"`
	AlertFired  bool    `json:"alertFired"`
}

func (b *Budget) Remaining() int {
	return b.MaxTokens - b.UsedTokens
}

func (b *Budget) UsageRatio() float64 {
	if b.MaxTokens == 0 {
		return 0
	}
	return float64(b.UsedTokens) / float64(b.MaxTokens)
}

func NewBudgetManager(cfg *config.TokenBudgetConfig, logger *slog.Logger) *BudgetManager {
	return &BudgetManager{
		cfg:     cfg,
		budgets: make(map[string]*Budget),
		logger:  logger.With("component", "budget"),
	}
}

func (bm *BudgetManager) GetOrCreate(sessionID string) *Budget {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if b, ok := bm.budgets[sessionID]; ok {
		return b
	}

	b := &Budget{
		SessionID: sessionID,
		MaxTokens: bm.cfg.MaxTokensPerTask,
	}
	bm.budgets[sessionID] = b
	return b
}

func (bm *BudgetManager) Consume(sessionID string, tokens int) error {
	if !bm.cfg.Enabled {
		return nil
	}

	b := bm.GetOrCreate(sessionID)
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if b.UsedTokens+tokens > b.MaxTokens {
		return fmt.Errorf("token budget exceeded for session %s: used %d + %d > max %d",
			sessionID, b.UsedTokens, tokens, b.MaxTokens)
	}

	b.UsedTokens += tokens

	if !b.AlertFired && b.UsageRatio() >= bm.cfg.AlertThreshold {
		b.AlertFired = true
		bm.logger.Warn("token budget alert",
			"session_id", sessionID,
			"used", b.UsedTokens,
			"max", b.MaxTokens,
			"ratio", b.UsageRatio(),
		)
	}

	return nil
}

func (bm *BudgetManager) GetUsage(sessionID string) *Budget {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.budgets[sessionID]
}

func (bm *BudgetManager) Reset(sessionID string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	delete(bm.budgets, sessionID)
}
