CREATE TABLE IF NOT EXISTS events (
  cursor INTEGER PRIMARY KEY AUTOINCREMENT,
  id TEXT NOT NULL,
  type TEXT NOT NULL,
  agent TEXT,
  message_id TEXT,
  thread_id TEXT,
  from_agent TEXT,
  to_json TEXT,
  body TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS messages (
  message_id TEXT PRIMARY KEY,
  thread_id TEXT,
  from_agent TEXT,
  to_json TEXT,
  body TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS inbox_index (
  agent TEXT NOT NULL,
  cursor INTEGER NOT NULL,
  message_id TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_inbox_agent_cursor ON inbox_index(agent, cursor);

CREATE TABLE IF NOT EXISTS agents (
  id TEXT PRIMARY KEY,
  session_id TEXT,
  name TEXT NOT NULL,
  project TEXT,
  capabilities_json TEXT,
  metadata_json TEXT,
  status TEXT,
  created_at TEXT NOT NULL,
  last_seen TEXT NOT NULL
);
