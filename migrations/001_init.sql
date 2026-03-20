-- agents: registered agent metadata
CREATE TABLE IF NOT EXISTS agents (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    card_json TEXT,
    reputation_score REAL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- sessions: active connections
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    agent_id TEXT REFERENCES agents(id),
    connection_type TEXT,
    status TEXT DEFAULT 'active',
    metadata_json TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_active_at DATETIME
);

-- tasks: A2A tasks
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    context_id TEXT,
    state TEXT NOT NULL,
    status_message_json TEXT,
    artifacts_json TEXT,
    history_json TEXT,
    metadata_json TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tasks_context ON tasks(context_id);
CREATE INDEX IF NOT EXISTS idx_tasks_state ON tasks(state);

-- social_relations: follow/endorse/collaborate edges
CREATE TABLE IF NOT EXISTS social_relations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_agent TEXT REFERENCES agents(id),
    to_agent TEXT REFERENCES agents(id),
    relation_type TEXT,
    metadata_json TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_social_from ON social_relations(from_agent, relation_type);
CREATE INDEX IF NOT EXISTS idx_social_to ON social_relations(to_agent, relation_type);
CREATE UNIQUE INDEX IF NOT EXISTS idx_social_unique ON social_relations(from_agent, to_agent, relation_type);

-- groups
CREATE TABLE IF NOT EXISTS groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    created_by TEXT REFERENCES agents(id),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- group_members
CREATE TABLE IF NOT EXISTS group_members (
    group_id TEXT REFERENCES groups(id) ON DELETE CASCADE,
    agent_id TEXT REFERENCES agents(id),
    role TEXT DEFAULT 'member',
    joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, agent_id)
);

-- subscriptions: pub/sub topics
CREATE TABLE IF NOT EXISTS subscriptions (
    agent_id TEXT REFERENCES agents(id),
    topic TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (agent_id, topic)
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_topic ON subscriptions(topic);

-- messages: persisted message log
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    from_agent TEXT,
    to_agent TEXT,
    group_id TEXT,
    topic TEXT,
    content_json TEXT,
    message_type TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_messages_from ON messages(from_agent);
CREATE INDEX IF NOT EXISTS idx_messages_to ON messages(to_agent);
CREATE INDEX IF NOT EXISTS idx_messages_group ON messages(group_id);
CREATE INDEX IF NOT EXISTS idx_messages_topic ON messages(topic);

-- timeline_events: per-agent feed
CREATE TABLE IF NOT EXISTS timeline_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT REFERENCES agents(id),
    event_type TEXT,
    source_agent TEXT,
    message_id TEXT REFERENCES messages(id),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_timeline_agent ON timeline_events(agent_id, created_at DESC);

-- agent_cards: cached remote agent cards
CREATE TABLE IF NOT EXISTS agent_cards (
    agent_id TEXT PRIMARY KEY,
    card_json TEXT NOT NULL,
    etag TEXT,
    fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME
);

-- pending_messages: offline message queue
CREATE TABLE IF NOT EXISTS pending_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_agent TEXT NOT NULL,
    message_json TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pending_target ON pending_messages(target_agent);

-- audit_log
CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    session_id TEXT,
    from_agent TEXT,
    to_agent TEXT,
    action TEXT,
    payload_hash TEXT,
    details_json TEXT
);

CREATE INDEX IF NOT EXISTS idx_audit_time ON audit_log(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log(action);

-- likes
CREATE TABLE IF NOT EXISTS likes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT REFERENCES agents(id),
    message_id TEXT REFERENCES messages(id),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(agent_id, message_id)
);

-- push_notification_configs
CREATE TABLE IF NOT EXISTS push_notification_configs (
    id TEXT PRIMARY KEY,
    task_id TEXT REFERENCES tasks(id),
    url TEXT NOT NULL,
    token TEXT,
    auth_scheme TEXT,
    auth_credentials TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_push_task ON push_notification_configs(task_id);
