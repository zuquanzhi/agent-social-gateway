package session

import "sync"

// SessionContext provides an isolated key-value store per session,
// ensuring no cross-session data leakage.
type SessionContext struct {
	store map[string]interface{}
	mu    sync.RWMutex
}

func NewSessionContext() *SessionContext {
	return &SessionContext{
		store: make(map[string]interface{}),
	}
}

func (c *SessionContext) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = value
}

func (c *SessionContext) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.store[key]
	return v, ok
}

func (c *SessionContext) GetString(key string) string {
	v, ok := c.Get(key)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func (c *SessionContext) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, key)
}

func (c *SessionContext) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = make(map[string]interface{})
}

func (c *SessionContext) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]string, 0, len(c.store))
	for k := range c.store {
		keys = append(keys, k)
	}
	return keys
}

func (c *SessionContext) Snapshot() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	snap := make(map[string]interface{}, len(c.store))
	for k, v := range c.store {
		snap[k] = v
	}
	return snap
}
