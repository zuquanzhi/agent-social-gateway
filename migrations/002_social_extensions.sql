-- A2A Social Extensions: conversation contexts + social events log

-- conversation_contexts: persistent multi-agent conversation rooms (Layer 4)
CREATE TABLE IF NOT EXISTS conversation_contexts (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL DEFAULT 'chat',
    topic TEXT,
    participants_json TEXT NOT NULL DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'active',
    message_count INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_convo_status ON conversation_contexts(status);
CREATE INDEX IF NOT EXISTS idx_convo_updated ON conversation_contexts(updated_at DESC);

-- social_events: lightweight event log for non-task social interactions (Layer 2)
CREATE TABLE IF NOT EXISTS social_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type TEXT NOT NULL,
    from_agent TEXT NOT NULL,
    to_agent TEXT,
    skill TEXT,
    data_json TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sevt_type ON social_events(event_type);
CREATE INDEX IF NOT EXISTS idx_sevt_from ON social_events(from_agent);
CREATE INDEX IF NOT EXISTS idx_sevt_to ON social_events(to_agent);
CREATE INDEX IF NOT EXISTS idx_sevt_time ON social_events(created_at DESC);

-- Add tags column to agents table for social profile
ALTER TABLE agents ADD COLUMN tags_json TEXT DEFAULT '[]';
ALTER TABLE agents ADD COLUMN trust_level TEXT DEFAULT 'unverified';
