CREATE TABLE IF NOT EXISTS events (
  cursor INTEGER PRIMARY KEY AUTOINCREMENT,
  id TEXT NOT NULL,
  type TEXT NOT NULL,
  agent TEXT,
  project TEXT NOT NULL DEFAULT '',
  message_id TEXT,
  thread_id TEXT,
  from_agent TEXT,
  to_json TEXT,
  body TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS messages (
  project TEXT NOT NULL DEFAULT '',
  message_id TEXT NOT NULL,
  thread_id TEXT,
  from_agent TEXT,
  to_json TEXT,
  body TEXT,
  created_at TEXT NOT NULL,
  PRIMARY KEY (project, message_id)
);

CREATE TABLE IF NOT EXISTS inbox_index (
  project TEXT NOT NULL DEFAULT '',
  agent TEXT NOT NULL,
  cursor INTEGER NOT NULL,
  message_id TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_inbox_agent_cursor ON inbox_index(project, agent, cursor);

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
